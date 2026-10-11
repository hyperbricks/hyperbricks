package commands

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type serveOutput chan string

func (output serveOutput) Write(data []byte) (int, error) {
	output <- string(data)
	return len(data), nil
}

func TestServeCommandWithoutModuleDoesNotWrite(t *testing.T) {
	directory := t.TempDir()
	t.Chdir(directory)
	for name, content := range map[string]string{
		"index.html": "<h1>Exported page</h1>", "hello-fragment.html": "<p>Exported fragment</p>",
		"package.hyperbricks.yaml": "invalid: [package is not loaded",
	} {
		if err := os.WriteFile(name, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotDoctorTree(t, directory)
	previousStart, previousRender := StartMode, RenderStatic
	command := NewServeCommand()
	command.SetArgs([]string{"--port", "0"})
	output := make(serveOutput, 1)
	command.SetOut(output)
	command.SetErr(io.Discard)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- command.ExecuteContext(ctx) }()
	t.Cleanup(cancel)
	var announcement string
	select {
	case announcement = <-output:
	case err := <-done:
		t.Fatalf("command stopped before announcing its URL: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not start")
	}
	_, link, ok := strings.Cut(announcement, "URL: ")
	if !ok {
		t.Fatalf("missing URL: %s", announcement)
	}
	link = strings.SplitN(link, "\n", 2)[0]
	parsed, err := url.Parse(link)
	if err != nil || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "0" || parsed.Port() == "" {
		t.Fatalf("default bind and actual port: %q, %v", link, err)
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{}}
	defer client.CloseIdleConnections()
	for route, content := range map[string]string{"": "Exported page", "hello-fragment": "Exported fragment"} {
		response, err := client.Get(link + route)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(string(body), content) {
			t.Fatalf("route %q: status=%d body=%q error=%v", route, response.StatusCode, body, err)
		}
	}
	client.CloseIdleConnections()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("server did not stop")
	}
	if _, err := client.Get(link); err == nil {
		t.Fatal("listener still accepts requests after shutdown")
	}
	if StartMode != previousStart || RenderStatic != previousRender {
		t.Fatal("serve enabled module startup or rendering")
	}
	if after := snapshotDoctorTree(t, directory); !reflect.DeepEqual(before, after) {
		t.Fatal("serve changed the source directory")
	}
}

func TestServeCommandRejectsInvalidInputs(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "index.html")
	if err := os.WriteFile(file, []byte("page"), 0644); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"missing directory": {filepath.Join(directory, "missing")},
		"file":              {file},
		"negative port":     {directory, "--port", "-1"},
		"large port":        {directory, "--port", "65536"},
		"empty host":        {directory, "--host", ""},
		"extra argument":    {directory, directory},
	} {
		t.Run(name, func(t *testing.T) {
			command := NewServeCommand()
			command.SilenceUsage, command.SilenceErrors = true, true
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetErr(&output)
			command.SetArgs(args)
			if err := command.Execute(); err == nil {
				t.Fatal("expected invalid input error")
			}
			if strings.Contains(output.String(), "URL:") {
				t.Fatal("advertised a failed server")
			}
		})
	}
}

func TestServeOccupiedPortDoesNotAnnounce(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	options := serveOptions{directory: t.TempDir(), host: "127.0.0.1", port: listener.Addr().(*net.TCPAddr).Port}
	announced := false
	err = serveDirectory(context.Background(), options, func(string, string) { announced = true })
	if err == nil || announced {
		t.Fatalf("failed listener error=%v announced=%t", err, announced)
	}
}

func TestServeCommandHelpDescribesStandalonePreview(t *testing.T) {
	command := NewServeCommand()
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"--help"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"serve [directory]", "No module or package configuration", "127.0.0.1", "8080", "--host", "--port"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("help missing %q", want)
		}
	}
}
