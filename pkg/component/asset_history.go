package component

import (
	"bytes"
	"os"
	"path/filepath"
)

// writeAssetHistory persists already encoded ownership history. Callers retain
// responsibility for schema, path validation, locking, and retention policy.
// Unchanged records and absent empty histories never cause a write.
func writeAssetHistory(path string, data []byte, empty bool, tempPattern string) error {
	return writeAssetHistoryWithIO(path, data, empty, tempPattern, systemAssetHistoryIO())
}

// Dependencies are passed per call so failure tests never replace global state.
// This seam covers persistence only; ownership and retention stay with callers.
type assetHistoryFile interface {
	Name() string
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type assetHistoryIO struct {
	readFile   func(string) ([]byte, error)
	mkdirAll   func(string, os.FileMode) error
	createTemp func(string, string) (assetHistoryFile, error)
	remove     func(string) error
	rename     func(string, string) error
}

func systemAssetHistoryIO() assetHistoryIO {
	return assetHistoryIO{
		readFile: os.ReadFile, mkdirAll: os.MkdirAll, remove: os.Remove, rename: os.Rename,
		createTemp: func(dir, pattern string) (assetHistoryFile, error) { return os.CreateTemp(dir, pattern) },
	}
}

func writeAssetHistoryWithIO(path string, data []byte, empty bool, tempPattern string, fs assetHistoryIO) error {
	existing, err := fs.readFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if os.IsNotExist(err) && empty {
		return nil
	}
	if err = fs.mkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := fs.createTemp(filepath.Dir(path), tempPattern)
	if err != nil {
		return err
	}
	defer fs.remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return fs.rename(file.Name(), path)
}
