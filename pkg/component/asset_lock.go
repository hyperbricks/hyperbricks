package component

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"time"
)

// Asset locks are keyed by the canonical output path, independently of a
// process's cache and temporary-directory settings. Lock files are never
// unlinked: removing one would let another process lock a different inode.
func assetLockPath(root, kind string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	abs, err = esbuildExistingPath(abs)
	if err != nil {
		return "", err
	}
	base, err := assetLockDirectory()
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(kind + "\x00" + abs))
	return filepath.Join(base, fmt.Sprintf("hyperbricks-asset-%x.lock", key)), nil
}

// LockAssetSession reserves an output destination for one writer lifecycle.
// A conflicting session fails immediately, before hooks or destructive work.
func LockAssetSession(root, kind string) (func(), error) {
	path, err := assetLockPath(root, "session:"+kind)
	if err != nil {
		return nil, err
	}
	return lockAssetFile(path, kind)
}

// LockAssetTree coordinates a static copy (shared) with complete publication
// and cleanup operations (exclusive). It does not replace the writer session.
func LockAssetTree(ctx context.Context, root string, shared bool) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path, err := assetLockPath(root, "tree")
	if err != nil {
		return nil, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		unlock, busy, err := tryLockAssetTree(path, shared)
		if err != nil {
			return nil, err
		}
		if !busy {
			return unlock, nil
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
