package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/packagemetadata"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// ErrRuntimeChanged means the extracted runtime changed while it was being
// copied. The caller can ask the user to retry rather than publish a mixed tree.
var ErrRuntimeChanged = errors.New("runtime files changed during snapshot; retry duplication")

// Duplicate snapshots are initiated by an authenticated HTTP request. Keep
// their private staging disk use bounded without changing normal CLI builds.
const (
	MaxRuntimeSnapshotFiles = 50_000
	MaxRuntimeSnapshotBytes = int64(2 << 30)
)

var ErrRuntimeSnapshotTooLarge = fmt.Errorf("runtime snapshot exceeds duplicate limits (%d entries, %d bytes)", MaxRuntimeSnapshotFiles, MaxRuntimeSnapshotBytes)

type RuntimeSnapshotOptions struct {
	Context              context.Context
	Module               string
	RuntimeDir           string
	ArchiveDir           string
	OriginBuildID        string
	OriginCommit         string
	HyperBricks          string
	ExpectedConfigSHA256 string
}

type RuntimeSnapshotResult struct {
	BuildID       string
	ArchivePath   string
	ModuleVersion string
	BuiltAt       string
	Commit        string
}

// PackageRuntimeSnapshot makes a new immutable HRA from an extracted runtime.
// Unlike BuildModuleWithOptions, this does not change any build index or active
// process. The caller owns registration of the returned archive.
func PackageRuntimeSnapshot(opts RuntimeSnapshotOptions) (RuntimeSnapshotResult, error) {
	var result RuntimeSnapshotResult
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if strings.TrimSpace(opts.Module) == "" || strings.TrimSpace(opts.OriginBuildID) == "" ||
		strings.TrimSpace(opts.HyperBricks) == "" {
		return result, errors.New("module, origin build ID, and HyperBricks version are required")
	}
	if opts.Module == "." || filepath.Base(opts.Module) != opts.Module || strings.ContainsAny(opts.Module, `/\`) {
		return result, errors.New("module must be a single path component")
	}
	archiveInfo, err := os.Stat(opts.ArchiveDir)
	if err != nil {
		return result, fmt.Errorf("archive directory is unavailable: %w", err)
	}
	if !archiveInfo.IsDir() {
		return result, errors.New("archive destination is not a directory")
	}
	archiveLink, err := os.Lstat(opts.ArchiveDir)
	if err != nil || archiveLink.Mode()&os.ModeSymlink != 0 {
		return result, errors.New("archive destination must not be a symbolic link")
	}
	runtimeLink, err := os.Lstat(opts.RuntimeDir)
	if err != nil || runtimeLink.Mode()&os.ModeSymlink != 0 || !runtimeLink.IsDir() {
		return result, errors.New("runtime source must be a real directory")
	}
	working, err := os.MkdirTemp(opts.ArchiveDir, ".hyperbricks-duplicate-*")
	if err != nil {
		return result, fmt.Errorf("create private snapshot: %w", err)
	}
	defer os.RemoveAll(working)
	snapshotRoot := filepath.Join(working, "runtime")
	if err := os.Mkdir(snapshotRoot, 0o700); err != nil {
		return result, err
	}
	if err := copyRuntimeSnapshot(ctx, opts.RuntimeDir, snapshotRoot); err != nil {
		return result, err
	}
	if err := verifyRuntimeSnapshot(ctx, opts.RuntimeDir, snapshotRoot); err != nil {
		return result, err
	}

	configPath := filepath.Join(snapshotRoot, PackageConfigFileName)
	config, err := os.ReadFile(configPath)
	if err != nil {
		return result, fmt.Errorf("read snapshot package configuration: %w", err)
	}
	if opts.ExpectedConfigSHA256 != "" {
		sum := sha256.Sum256(config)
		if hex.EncodeToString(sum[:]) != opts.ExpectedConfigSHA256 {
			return result, ErrRuntimeChanged
		}
	}
	if _, err := shared.ValidatePackageConfigBytes(config, snapshotRoot); err != nil {
		return result, fmt.Errorf("invalid current package configuration: %w", err)
	}
	commit := strings.TrimSpace(opts.OriginCommit)
	if commit == "" {
		commit = packagemetadata.UnknownCommit
	}
	files, err := collectRuntimeSnapshotFiles(snapshotRoot)
	if err != nil {
		return result, err
	}
	stagingPath := filepath.Join(working, "snapshot.hra")
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		builtAt := time.Now().UTC().Add(time.Duration(attempt) * time.Nanosecond).Format(time.RFC3339Nano)
		artifact, err := renderPackageArtifact(config, configPath, snapshotRoot, packagemetadata.ArtifactOptions{
			Module:        opts.Module,
			Format:        "hra",
			FormatVersion: "1",
			Commit:        commit,
			OriginBuildID: opts.OriginBuildID,
			BuiltAt:       builtAt,
			HyperBricks:   opts.HyperBricks,
		})
		if err != nil {
			return result, fmt.Errorf("render duplicate archive metadata: %w", err)
		}
		buildID, err := hashRuntimeFiles(ctx, snapshotRoot, files, artifact.Content)
		if err != nil {
			return result, err
		}
		archivePath := filepath.Join(opts.ArchiveDir,
			fmt.Sprintf("%s-%s-%s.hra", opts.Module, artifact.Metadata.ModuleVersion, buildID))
		if err := writeArchiveWithContext(ctx, stagingPath, files, artifact.Content); err != nil {
			return result, err
		}
		if err := verifyRuntimeSnapshot(ctx, opts.RuntimeDir, snapshotRoot); err != nil {
			return result, err
		}
		// A hard link publishes the complete archive without replacing an
		// existing build, even if two requests receive the same content ID.
		if err := os.Link(stagingPath, archivePath); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return result, fmt.Errorf("publish duplicate archive: %w", err)
		}
		return RuntimeSnapshotResult{
			BuildID:       buildID,
			ArchivePath:   archivePath,
			ModuleVersion: artifact.Metadata.ModuleVersion,
			BuiltAt:       builtAt,
			Commit:        commit,
		}, nil
	}
	return result, errors.New("duplicate archive ID already exists; retry")
}

// collectRuntimeSnapshotFiles applies the ordinary HRA source filters plus
// known in-progress editor/Spaces/esbuild staging names. New Spaces documents
// and assets are intentionally included.
func collectRuntimeSnapshotFiles(root string) ([]buildFile, error) {
	excludeCache, err := moduleCacheExclusion(root)
	if err != nil {
		return nil, err
	}
	var files []buildFile
	var totalBytes int64
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		name := entry.Name()
		if entry.IsDir() && (isExcludedDir(name) || excludeCache(path)) {
			return fs.SkipDir
		}
		if !entry.IsDir() && (isExcludedFile(name) || isRuntimeStagingFile(name) || excludeCache(path)) {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("runtime contains symbolic link: %s", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf("runtime contains non-regular file: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, buildFile{abs: path, rel: rel, info: info, isDir: info.IsDir()})
		if len(files) > MaxRuntimeSnapshotFiles {
			return ErrRuntimeSnapshotTooLarge
		}
		if !info.IsDir() {
			if info.Size() < 0 || info.Size() > MaxRuntimeSnapshotBytes-totalBytes {
				return ErrRuntimeSnapshotTooLarge
			}
			totalBytes += info.Size()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := verifyPackageArchiveInputs(root, files); err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	return files, nil
}

func isRuntimeStagingFile(name string) bool {
	return strings.HasPrefix(name, ".spaces-") ||
		strings.HasPrefix(name, ".hb-esbuild-") ||
		strings.HasPrefix(name, ".hyperbricks-deploy-edit-")
}

func copyRuntimeSnapshot(ctx context.Context, sourceRoot, snapshotRoot string) error {
	files, err := collectRuntimeSnapshotFiles(sourceRoot)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(sourceRoot)
	if err != nil {
		return err
	}
	defer root.Close()
	var copiedBytes int64
	for _, entry := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		dest := filepath.Join(snapshotRoot, entry.rel)
		if entry.isDir {
			if err := os.Mkdir(dest, 0o700); err != nil {
				return err
			}
			continue
		}
		input, err := root.Open(entry.rel)
		if err != nil {
			return err
		}
		opened, err := input.Stat()
		if err != nil || !opened.Mode().IsRegular() {
			input.Close()
			return fmt.Errorf("runtime file changed before snapshot: %s", entry.rel)
		}
		output, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			input.Close()
			return err
		}
		written, copyErr := io.Copy(output, runtimeContextReader{ctx: ctx, reader: io.LimitReader(input, MaxRuntimeSnapshotBytes-copiedBytes+1)})
		closeErr := output.Close()
		input.Close()
		copiedBytes += written
		if copiedBytes > MaxRuntimeSnapshotBytes {
			return ErrRuntimeSnapshotTooLarge
		}
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if err := os.Chmod(dest, entry.info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chtimes(dest, entry.info.ModTime(), entry.info.ModTime()); err != nil {
			return err
		}
	}
	// Directory permissions are restored after writing all descendants.
	for i := len(files) - 1; i >= 0; i-- {
		entry := files[i]
		if entry.isDir {
			if err := os.Chmod(filepath.Join(snapshotRoot, entry.rel), entry.info.Mode().Perm()); err != nil {
				return err
			}
		}
	}
	return nil
}

func verifyRuntimeSnapshot(ctx context.Context, sourceRoot, snapshotRoot string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sourceFiles, err := collectRuntimeSnapshotFiles(sourceRoot)
	if err != nil {
		return err
	}
	snapshotFiles, err := collectRuntimeSnapshotFiles(snapshotRoot)
	if err != nil {
		return err
	}
	if len(sourceFiles) != len(snapshotFiles) {
		return ErrRuntimeChanged
	}
	for i, source := range sourceFiles {
		copy := snapshotFiles[i]
		if source.rel != copy.rel || source.isDir != copy.isDir || source.info.Mode().Perm() != copy.info.Mode().Perm() {
			return ErrRuntimeChanged
		}
	}
	sourceHash, err := hashRuntimeFiles(ctx, sourceRoot, sourceFiles, nil)
	if err != nil {
		return err
	}
	snapshotHash, err := hashRuntimeFiles(ctx, snapshotRoot, snapshotFiles, nil)
	if err != nil {
		return err
	}
	if sourceHash != snapshotHash {
		return ErrRuntimeChanged
	}
	return nil
}

func hashRuntimeFiles(ctx context.Context, rootPath string, files []buildFile, packageOverride []byte) (string, error) {
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", err
	}
	defer root.Close()
	hasher := sha256.New()
	var hashedBytes int64
	for _, entry := range files {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		rel := filepath.ToSlash(entry.rel)
		if entry.isDir {
			rel += "/"
		}
		if _, err := io.WriteString(hasher, rel+"\n"); err != nil {
			return "", err
		}
		if entry.isDir {
			if _, err := io.WriteString(hasher, "dir\n"); err != nil {
				return "", err
			}
			continue
		}
		if packageOverride != nil && rel == PackageConfigFileName {
			if int64(len(packageOverride)) > MaxRuntimeSnapshotBytes-hashedBytes {
				return "", ErrRuntimeSnapshotTooLarge
			}
			if _, err := hasher.Write(packageOverride); err != nil {
				return "", err
			}
			hashedBytes += int64(len(packageOverride))
			continue
		}
		info, err := root.Lstat(entry.rel)
		if err != nil || !info.Mode().IsRegular() {
			return "", ErrRuntimeChanged
		}
		file, err := root.Open(entry.rel)
		if err != nil {
			return "", err
		}
		opened, err := file.Stat()
		if err != nil || !opened.Mode().IsRegular() {
			file.Close()
			return "", ErrRuntimeChanged
		}
		readBytes, copyErr := io.Copy(hasher, runtimeContextReader{ctx: ctx, reader: io.LimitReader(file, MaxRuntimeSnapshotBytes-hashedBytes+1)})
		closeErr := file.Close()
		hashedBytes += readBytes
		if hashedBytes > MaxRuntimeSnapshotBytes {
			return "", ErrRuntimeSnapshotTooLarge
		}
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type runtimeContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r runtimeContextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(buffer)
}
