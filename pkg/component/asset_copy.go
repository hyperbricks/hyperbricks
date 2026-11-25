package component

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CopyStaticAssets takes a self-contained copy while cooperating asset writers
// cannot publish or prune a generation. It never changes source ownership.
// Symlinks are materialized here, not followed later during ZIP creation.
func CopyStaticAssets(ctx context.Context, source, destination string) error {
	return copyStaticAssets(ctx, source, destination, "")
}

// ResetAndCopyStaticAssets recreates an already reserved export destination
// under the same destination write lock used by its later asset publications.
// The CLI validates and reserves renderDir before calling this function.
func ResetAndCopyStaticAssets(ctx context.Context, source, renderDir string) error {
	// This child is removed by the reset. Following its previous target would
	// bind the operation to a different tree after the symlink is removed.
	child := filepath.Join(renderDir, "static")
	if info, err := os.Lstat(child); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("export static destination must not be a symlink: %s", child)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return copyStaticAssets(ctx, source, filepath.Join(renderDir, "static"), renderDir)
}

func copyStaticAssets(ctx context.Context, source, destination, reset string) error {
	src, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	src, err = esbuildExistingPath(src)
	if err != nil {
		return err
	}
	dst, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	dst, err = esbuildExistingPath(dst)
	if err != nil {
		return err
	}
	if src == dst || esbuildOutputPath(src, dst) == nil || esbuildOutputPath(dst, src) == nil {
		return fmt.Errorf("static copy source and destination must not overlap")
	}
	// A single order also prevents opposing copy operations from deadlocking.
	roots := []string{src, dst}
	sort.Strings(roots)
	for _, root := range roots {
		unlock, err := LockAssetTree(ctx, root, root == src)
		if err != nil {
			return err
		}
		defer unlock()
	}
	if err := validateStaticAssetHistory(src); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if reset != "" {
		base, err := filepath.Abs(reset)
		if err != nil {
			return err
		}
		base, err = esbuildExistingPath(base)
		if err != nil {
			return err
		}
		if base == src || base == dst || esbuildOutputPath(base, src) == nil || esbuildOutputPath(src, base) == nil {
			return fmt.Errorf("export reset would overlap the source static tree")
		}
		if err := os.RemoveAll(base); err != nil {
			return err
		}
		if err := os.MkdirAll(base, 0755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return os.MkdirAll(dst, 0755)
	} else if err != nil {
		return err
	}
	return copyStaticAsset(ctx, src, dst, map[string]bool{})
}

// Only the newest retained claim for a physical path matters. Older fixed-name
// generations legitimately contain different bytes for the same filename.
func validateStaticAssetHistory(root string) error {
	expected := map[string][32]byte{}
	add := func(path string, hash [32]byte) error {
		if old, exists := expected[path]; exists && old != hash {
			return fmt.Errorf("conflicting asset ownership for %s", path)
		}
		expected[path] = hash
		return nil
	}
	// These are private, unshared readers; no renderer/component mutex or
	// mutation callback is acquired while the tree read lock is held.
	renderer := NewEsbuildRenderer(root)
	history, err := renderer.store.loadHistory()
	if err != nil {
		return err
	}
	for _, owner := range history.Owners {
		seen := map[string]bool{}
		for _, generation := range owner.Generations {
			for path, stamp := range generation.Outputs {
				if seen[path] {
					continue
				}
				seen[path] = true
				if err := add(path, stamp.Hash); err != nil {
					return err
				}
			}
		}
	}
	images, err := NewImageProcessor().storeFor(filepath.Join(root, "images"))
	if err != nil {
		return err
	}
	if err := images.load(); err != nil {
		return err
	}
	for _, owner := range images.history.Owners {
		seen := map[string]bool{}
		for _, generation := range owner.Generations {
			if seen[generation.Name] {
				continue
			}
			seen[generation.Name] = true
			if err := add(filepath.Join(images.root, generation.Name), generation.Hash); err != nil {
				return err
			}
		}
	}
	for path, hash := range expected {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("copy known asset %s: %w", path, err)
		}
		if sha256.Sum256(data) != hash {
			return fmt.Errorf("known asset differs from its published generation: %s", path)
		}
	}
	return nil
}

func privateStaticAsset(name string) bool {
	for _, prefix := range []string{".hb-esbuild-", ".image-", ".spaces-", ".scaffold-"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return name == ".hb-esbuild-cache" // legacy private build manifests
}

func copyStaticAsset(ctx context.Context, source, destination string, parents map[string]bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return fmt.Errorf("resolve static asset %s: %w", source, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if parents[resolved] {
			return fmt.Errorf("static symlink cycle at %s", source)
		}
		parents[resolved] = true
		defer delete(parents, resolved)
		if err := os.MkdirAll(destination, info.Mode().Perm()|0700); err != nil {
			return err
		}
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if privateStaticAsset(entry.Name()) {
				continue
			}
			if err := copyStaticAsset(ctx, filepath.Join(resolved, entry.Name()), filepath.Join(destination, entry.Name()), parents); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("static asset is not a regular file: %s", source)
	}
	input, err := os.Open(resolved)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	buffer := make([]byte, 64*1024)
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		var n int
		n, err = input.Read(buffer)
		if n > 0 {
			if _, writeErr := output.Write(buffer[:n]); writeErr != nil {
				err = writeErr
				break
			}
		}
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	closeErr := output.Close()
	if err != nil {
		return err
	}
	return closeErr
}
