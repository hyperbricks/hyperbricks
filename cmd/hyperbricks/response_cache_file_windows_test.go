package main

import (
	"os"
	"sync"
	"testing"
)

func blockResponseCacheRemoval(t *testing.T, path string) func() {
	t.Helper()
	// An external reader without FILE_SHARE_DELETE prevents removal on Windows.
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	unblock := func() {
		once.Do(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
	}
	t.Cleanup(unblock)
	return unblock
}
