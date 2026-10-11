package commands

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/hyperbricks/hyperbricks/pkg/staticserver"
	"github.com/spf13/cobra"
)

type serveOptions struct {
	directory string
	host      string
	port      int
}

func NewServeCommand() *cobra.Command {
	options := serveOptions{host: "127.0.0.1", port: 8080}
	cmd := &cobra.Command{
		Use:     "serve [directory]",
		Short:   "Preview an existing static site without rendering",
		Long:    "Serve files from an existing directory (default: the current directory). No module or package configuration is needed. Clean URLs resolve .html and .htm files; directories use index.html or index.htm. Missing files return 404, and directory listings are disabled.",
		Example: "  hyperbricks serve ./site\n  hyperbricks serve . --host 127.0.0.1 --port 8081",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			options.directory = "."
			if len(args) == 1 {
				options.directory = args[0]
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return serveDirectory(ctx, options, func(directory, address string) {
				fmt.Fprintf(cmd.OutOrStdout(), "Serving %s\nURL: %s\nPress Ctrl+C to stop.\n", directory, address)
			})
		},
	}
	cmd.Flags().StringVar(&options.host, "host", options.host, "Address to listen on (use 0.0.0.0 to allow network access)")
	cmd.Flags().IntVarP(&options.port, "port", "p", options.port, "Port to listen on (0 selects an available port)")
	cmd.ValidArgsFunction = cobra.FixedCompletions(nil, cobra.ShellCompDirectiveFilterDirs)
	return cmd
}

func serveDirectory(ctx context.Context, options serveOptions, announce func(string, string)) error {
	if options.port < 0 || options.port > 65535 {
		return fmt.Errorf("--port must be between 0 and 65535")
	}
	if strings.TrimSpace(options.host) == "" {
		return fmt.Errorf("--host cannot be empty; use 127.0.0.1 for local access")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := filepath.Abs(options.directory)
	if err != nil {
		return fmt.Errorf("resolve site directory: %w", err)
	}
	handler, err := staticserver.New(directory, shared.RoutingConfig{CleanURLs: true})
	if err != nil {
		return err
	}
	defer handler.Close()

	address := net.JoinHostPort(options.host, strconv.Itoa(options.port))
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}
	defer listener.Close()
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	defer server.Close()

	bound := listener.Addr().(*net.TCPAddr)
	host := bound.IP.String()
	if bound.IP.IsUnspecified() {
		host = "127.0.0.1"
		if bound.IP.To4() == nil {
			host = "::1"
		}
	}
	if bound.Zone != "" {
		host += "%" + bound.Zone
	}
	link := url.URL{Scheme: "http", Host: net.JoinHostPort(host, strconv.Itoa(bound.Port)), Path: "/"}
	announce(directory, link.String())
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			return fmt.Errorf("stop static preview: %w", err)
		}
		return nil
	}
}
