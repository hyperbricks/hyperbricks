package component

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Use real files, injecting only the selected failure. A failed write writes a
// prefix first; a failed Close closes the descriptor before reporting failure.
type failingHistoryFile struct {
	*os.File
	stage   string
	failure error
}

func (f *failingHistoryFile) Write(data []byte) (int, error) {
	if f.stage == "write" {
		n, err := f.File.Write(data[:len(data)/2])
		if err != nil {
			return n, err
		}
		return n, f.failure
	}
	return f.File.Write(data)
}
func (f *failingHistoryFile) Sync() error {
	if f.stage == "sync" {
		return f.failure
	}
	return f.File.Sync()
}
func (f *failingHistoryFile) Close() error {
	err := f.File.Close()
	if err != nil {
		return err
	}
	if f.stage == "close" {
		return f.failure
	}
	return nil
}

func TestAssetHistoryFailurePreservesPreviousAndRetries(t *testing.T) {
	for _, stage := range []string{"read", "mkdir", "create", "write", "sync", "close", "rename"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := filepath.Join(dir, "history.json")
			previous := []byte(`{"version":1,"generation":"previous"}`)
			next := []byte(`{"version":1,"generation":"replacement"}`)
			if err := writeAssetHistory(path, previous, false, ".history-*"); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("injected " + stage + " failure")
			fs := systemAssetHistoryIO()
			calls := 0
			switch stage {
			case "read":
				fs.readFile = func(string) ([]byte, error) { calls++; return nil, failure }
			case "mkdir":
				fs.mkdirAll = func(string, os.FileMode) error { calls++; return failure }
			case "create":
				fs.createTemp = func(string, string) (assetHistoryFile, error) { calls++; return nil, failure }
			case "rename":
				fs.rename = func(string, string) error { calls++; return failure }
			default:
				fs.createTemp = func(dir, pattern string) (assetHistoryFile, error) {
					f, err := os.CreateTemp(dir, pattern)
					if err != nil {
						return nil, err
					}
					calls++
					return &failingHistoryFile{File: f, stage: stage, failure: failure}, nil
				}
			}
			if err := writeAssetHistoryWithIO(path, next, false, ".history-*", fs); !errors.Is(err, failure) {
				t.Fatalf("lost failure: %v", err)
			}
			if calls != 1 {
				t.Fatalf("failure injection calls = %d", calls)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != string(previous) {
				t.Fatalf("previous history changed: %s %v", got, err)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
				t.Fatal("failed attempt replaced previous history")
			}
			assertHistoryDirectory(t, dir, "history.json")
			if err := writeAssetHistory(path, next, false, ".history-*"); err != nil {
				t.Fatalf("retry: %v", err)
			}
			got, err = os.ReadFile(path)
			if err != nil || string(got) != string(next) {
				t.Fatalf("retry did not persist complete replacement: %s %v", got, err)
			}
			assertHistoryDirectory(t, dir, "history.json")
		})
	}
}
func assertHistoryDirectory(t *testing.T, dir, name string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		t.Fatalf("temporary file leaked: %v", entries)
	}
}

func TestAssetHistoryNoWriteForUnchangedOrAbsentEmpty(t *testing.T) {
	for _, existing := range []bool{false, true} {
		name := "absent-empty"
		if existing {
			name = "unchanged"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "private", "history.json")
			data := []byte(`{"version":1,"owners":{}}`)
			if existing {
				if err := writeAssetHistory(path, data, false, ".history-*"); err != nil {
					t.Fatal(err)
				}
			}
			fs := systemAssetHistoryIO()
			fs.mkdirAll = func(string, os.FileMode) error { t.Fatal("no-op attempted a directory write"); return nil }
			fs.createTemp = func(string, string) (assetHistoryFile, error) {
				t.Fatal("no-op attempted a file write")
				return nil, nil
			}
			if err := writeAssetHistoryWithIO(path, data, true, ".history-*", fs); err != nil {
				t.Fatal(err)
			}
			if !existing {
				if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
					t.Fatal("empty init created directory", err)
				}
			}
		})
	}
}
