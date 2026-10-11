//go:build !unix

package staticserver

import "os"

func openFile(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}
