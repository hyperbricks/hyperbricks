//go:build unix

package staticserver

import (
	"os"
	"syscall"
)

// A FIFO can replace a regular file between requests. Open nonblocking before
// checking the opened descriptor's type, so it cannot stall an HTTP request.
func openFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK, 0)
}
