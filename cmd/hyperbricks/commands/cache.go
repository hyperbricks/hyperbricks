package commands

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// CachePurgeResult reports entries removed from one running process.
type CachePurgeResult struct {
	MemoryEntries int   `json:"memory_entries"`
	DiskEntries   int   `json:"disk_entries"`
	Bytes         int64 `json:"bytes"`
}

type cacheControlInstance struct {
	ModuleRoot string `json:"module_root"`
	Instance   string `json:"instance"`
	PID        int    `json:"pid"`
}

type cachePurgeRequest struct {
	All   bool   `json:"all"`
	Route string `json:"route,omitempty"`
}

func NewCacheCommand() *cobra.Command {
	var module, route, instance string
	var all bool
	cmd := &cobra.Command{Use: "cache", Short: "Manage a running module's rendered-response cache"}
	purge := &cobra.Command{
		Use: "purge", Short: "Purge all cached responses or every variant of one route",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			request := cachePurgeRequest{All: all, Route: route}
			if err := request.validate(); err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			selection, err := resolveModuleSelection(module, cwd)
			if err != nil {
				return err
			}
			result, selected, err := purgeRunningCache(cmd.Context(), selection.Root, instance, request)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Purged %d memory entries and %d disk entries (%d bytes) from instance %s.\n", result.MemoryEntries, result.DiskEntries, result.Bytes, selected)
			return nil
		},
	}
	purge.Flags().StringVarP(&module, "module", "m", "default", "Module name or directory path")
	_ = purge.RegisterFlagCompletionFunc("module", completeModuleSelection)
	purge.Flags().BoolVar(&all, "all", false, "Purge every cached route")
	purge.Flags().StringVar(&route, "route", "", "Route whose cached variants should be purged")
	purge.Flags().StringVar(&instance, "instance", "", "Running instance ID (required when several instances use this module)")
	purge.MarkFlagsMutuallyExclusive("all", "route")
	purge.MarkFlagsOneRequired("all", "route")
	cmd.AddCommand(purge)
	return cmd
}

func (request *cachePurgeRequest) validate() error {
	request.Route = strings.TrimSpace(request.Route)
	if request.All == (request.Route != "") {
		return errors.New("select exactly one of --all or --route")
	}
	if request.All {
		return nil
	}
	if strings.ContainsAny(request.Route, "?#\\\r\n\x00") || strings.Contains(request.Route, "://") {
		return errors.New("--route must be a route path without a query, fragment, or URL scheme")
	}
	for _, segment := range strings.Split(request.Route, "/") {
		if segment == "." || segment == ".." {
			return errors.New("--route cannot contain . or .. path segments")
		}
	}
	request.Route = strings.Trim(request.Route, "/")
	if request.Route == "" {
		request.Route = "/"
	}
	return nil
}

// Cache control uses only a private Unix socket. It is never registered on the
// application's HTTP listener, and needs no writable directory in the module.
func StartCacheControl(moduleRoot string, purge func(string) (CachePurgeResult, error)) (func(context.Context) error, error) {
	root, directory, err := cacheControlDirectory(moduleRoot, true)
	if err != nil {
		return nil, err
	}
	var nonce [6]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	instance := cacheControlInstance{ModuleRoot: root, Instance: fmt.Sprintf("%d-%x", os.Getpid(), nonce), PID: os.Getpid()}
	socket := filepath.Join(directory, instance.Instance+".sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("start private cache control: %w", err)
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(instance)
	})
	mux.HandleFunc("POST /v1/purge", func(w http.ResponseWriter, r *http.Request) {
		var request cachePurgeRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "invalid purge request", http.StatusBadRequest)
			return
		}
		var trailing interface{}
		if err := decoder.Decode(&trailing); err != io.EOF {
			http.Error(w, "invalid purge request", http.StatusBadRequest)
			return
		}
		if err := request.validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		result, err := purge(request.Route)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	return func(ctx context.Context) error {
		err := server.Shutdown(ctx)
		if err != nil {
			_ = server.Close()
		}
		return err
	}, nil
}

func cacheControlDirectory(moduleRoot string, create bool) (string, string, error) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return "", "", errors.New("local cache control requires macOS or Linux")
	}
	root, err := filepath.Abs(moduleRoot)
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		return "", "", fmt.Errorf("resolve module directory: %w", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", "", fmt.Errorf("module root must be a directory: %s", root)
	}
	cacheHome, err := os.UserCacheDir()
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(root))
	directory := filepath.Join(cacheHome, "hyperbricks", "control", hex.EncodeToString(sum[:8]))
	for _, current := range []string{filepath.Join(cacheHome, "hyperbricks"), filepath.Join(cacheHome, "hyperbricks", "control"), directory} {
		if create {
			if err := os.MkdirAll(current, 0o700); err != nil {
				return "", "", err
			}
		}
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && !create {
			return root, directory, nil
		}
		if err != nil {
			return "", "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", "", fmt.Errorf("cache control directory must be a real directory: %s", current)
		}
		if current != filepath.Join(cacheHome, "hyperbricks") && info.Mode().Perm()&0o077 != 0 {
			return "", "", fmt.Errorf("cache control directory must have private permissions: %s", current)
		}
	}
	return root, directory, nil
}

func cacheControlClient(socket string) *http.Client {
	return &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
		},
	}}
}

func purgeRunningCache(ctx context.Context, moduleRoot, instance string, request cachePurgeRequest) (CachePurgeResult, string, error) {
	var result CachePurgeResult
	if err := request.validate(); err != nil {
		return result, "", err
	}
	root, directory, err := cacheControlDirectory(moduleRoot, false)
	if err != nil {
		return result, "", err
	}
	paths, err := filepath.Glob(filepath.Join(directory, "*.sock"))
	if err != nil {
		return result, "", err
	}
	active := make(map[string]string)
	for _, socket := range paths {
		info, err := os.Lstat(socket)
		if err != nil || info.Mode()&os.ModeSocket == 0 || info.Mode().Perm()&0o077 != 0 {
			continue
		}
		probe, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://local/v1/status", nil)
		if err != nil {
			return result, "", err
		}
		response, err := cacheControlClient(socket).Do(probe)
		if err != nil {
			continue // A crashed process may have left a socket behind.
		}
		var status cacheControlInstance
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&status)
		_ = response.Body.Close()
		if response.StatusCode == http.StatusOK && decodeErr == nil && status.ModuleRoot == root && status.Instance+".sock" == filepath.Base(socket) {
			active[status.Instance] = socket
		}
	}
	if err := ctx.Err(); err != nil {
		return result, "", err
	}
	ids := make([]string, 0, len(active))
	for id := range active {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return result, "", fmt.Errorf("no running cache-control instance for module %s", root)
	}
	if instance == "" {
		if len(ids) > 1 {
			return result, "", fmt.Errorf("multiple running instances for module %s; select --instance %s", root, strings.Join(ids, " or "))
		}
		instance = ids[0]
	}
	socket, ok := active[instance]
	if !ok {
		return result, "", fmt.Errorf("instance %q is not running for module %s; available: %s", instance, root, strings.Join(ids, ", "))
	}
	body, err := json.Marshal(request)
	if err != nil {
		return result, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://local/v1/purge", strings.NewReader(string(body)))
	if err != nil {
		return result, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := cacheControlClient(socket).Do(req)
	if err != nil {
		return result, "", fmt.Errorf("purge cache: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return result, "", fmt.Errorf("purge cache: %s", strings.TrimSpace(string(message)))
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result); err != nil {
		return result, "", fmt.Errorf("read purge result: %w", err)
	}
	return result, instance, nil
}
