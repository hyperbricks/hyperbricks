//go:build unix

package staticserver

import (
	"net/http"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestSpecialFileDoesNotBlockRequest(t *testing.T) {
	handler, root := newFixture(t, shared.RoutingConfig{CleanURLs: true})
	if err := syscall.Mkfifo(filepath.Join(root, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "blocked.html"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/pipe", "/blocked"} {
		done := make(chan int, 1)
		go func() { done <- request(handler, http.MethodGet, target).Code }()
		select {
		case status := <-done:
			if status != http.StatusNotFound {
				t.Fatalf("FIFO status=%d", status)
			}
		case <-time.After(time.Second):
			t.Fatalf("FIFO request %s blocked", target)
		}
	}
}
