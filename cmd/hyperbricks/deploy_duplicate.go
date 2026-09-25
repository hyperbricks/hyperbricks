package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

// Duplicate records a new immutable build but deliberately does not select or
// start it. The selected build's extracted runtime is the snapshot source.
func (api *deployLocalServer) handleBuildDuplicate(w http.ResponseWriter, r *http.Request, module, buildID string) {
	if !validDeployPathPart(module) || !validDeployPathPart(buildID) || api.isDevBuildID(buildID) {
		writeError(w, http.StatusBadRequest, errors.New("select an archived HRA build to duplicate"))
		return
	}
	indexPath := api.indexPath(module)
	index, err := loadLocalBuildIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	origin, ok := findLocalRow(index, buildID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("build_id not found"))
		return
	}
	if _, err := api.localBuildArchive(module, buildID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	location, err := api.localPackageConfig(module, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	moduleRoot, err := confinedDirectory(api.buildRoot, filepath.Join(api.buildRoot, module))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	snapshot, status, err := packageDuplicateRuntime(r.Context(), module, buildID, location, moduleRoot, origin.Commit, origin.HyperBricks)
	if err != nil {
		writeError(w, status, err)
		return
	}
	registered := false
	defer func() {
		if !registered {
			_ = os.Remove(snapshot.ArchivePath)
		}
	}()
	current, runtimeMode, commitStatus, err := api.registerDuplicateBuild(module, buildID, origin, snapshot)
	if err != nil {
		writeError(w, commitStatus, err)
		return
	}
	registered = true
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"module": module, "build_id": snapshot.BuildID, "origin_build_id": buildID,
		"runtime_mode": runtimeMode, "current": current, "new_build": true,
	})
}

func (api *deployAPI) handleBuildDuplicate(w http.ResponseWriter, r *http.Request, module, buildID string) {
	if !validDeployPathPart(module) || !validDeployPathPart(buildID) {
		writeError(w, http.StatusBadRequest, errors.New("invalid module or build_id"))
		return
	}
	indexPath := api.indexPath(module)
	index, err := loadDeployIndex(indexPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	origin, ok := findDeployRow(index, buildID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("build_id not found"))
		return
	}
	if _, err := api.remoteBuildArchive(module, buildID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	location, err := api.remotePackageConfig(module, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	moduleRoot, err := confinedDirectory(api.root, filepath.Join(api.root, module))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	archiveDir, err := duplicateRemoteArchiveDir(moduleRoot)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	snapshot, status, err := packageDuplicateRuntime(r.Context(), module, buildID, location, archiveDir, origin.Commit, origin.HyperBricks)
	if err != nil {
		writeError(w, status, err)
		return
	}
	registered := false
	defer func() {
		if !registered {
			_ = os.Remove(snapshot.ArchivePath)
		}
	}()
	current, runtimeMode, commitStatus, err := api.registerDuplicateBuild(module, buildID, origin, snapshot)
	if err != nil {
		writeError(w, commitStatus, err)
		return
	}
	registered = true
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"module": module, "build_id": snapshot.BuildID, "origin_build_id": buildID,
		"runtime_mode": runtimeMode, "current": current, "new_build": true,
	})
}

func (api *deployLocalServer) registerDuplicateBuild(module, buildID string, origin localBuildRow, snapshot commands.RuntimeSnapshotResult) (current, mode string, status int, err error) {
	status = http.StatusConflict
	err = commands.WithBuildIndexLock(func() error {
		api.runtimeMu.Lock()
		defer api.runtimeMu.Unlock()
		indexPath := api.indexPath(module)
		latest, loadErr := loadLocalBuildIndex(indexPath)
		if loadErr != nil {
			status = http.StatusInternalServerError
			return loadErr
		}
		latestOrigin, ok := findLocalRow(latest, buildID)
		if !ok || !sameLocalDuplicateOrigin(origin, latestOrigin) {
			return errors.New("original build changed during duplication; retry")
		}
		if _, exists := findLocalRow(latest, snapshot.BuildID); exists {
			return errors.New("duplicate build ID already exists; retry")
		}
		mode = normalizedDeployRuntimeMode(latestOrigin.RuntimeMode, latestOrigin.Production)
		latest.Versions = append(latest.Versions, localBuildRow{
			BuildID: snapshot.BuildID, ModuleVersion: snapshot.ModuleVersion,
			Format: "hra", File: filepath.ToSlash(snapshot.ArchivePath),
			BuiltAt: snapshot.BuiltAt, Commit: snapshot.Commit, OriginBuildID: buildID,
			SourceHash:  "", // Derived content is not the original source tree.
			HyperBricks: strings.TrimSpace(assets.VersionMD), RuntimeMode: mode,
			Production: mode == shared.LIVE_MODE,
		})
		current = latest.Current
		if saveErr := saveLocalBuildIndex(indexPath, latest); saveErr != nil {
			status = http.StatusInternalServerError
			return saveErr
		}
		return nil
	})
	if err == nil {
		status = http.StatusCreated
	}
	return
}

func (api *deployAPI) registerDuplicateBuild(module, buildID string, origin deployIndexRow, snapshot commands.RuntimeSnapshotResult) (current, mode string, status int, err error) {
	status = http.StatusConflict
	api.runtimeMu.Lock()
	defer api.runtimeMu.Unlock()
	indexPath := api.indexPath(module)
	latest, loadErr := loadDeployIndex(indexPath)
	if loadErr != nil {
		return "", "", http.StatusInternalServerError, loadErr
	}
	latestOrigin, ok := findDeployRow(latest, buildID)
	if !ok || !sameRemoteDuplicateOrigin(origin, latestOrigin) {
		return "", "", status, errors.New("original build changed during duplication; retry")
	}
	if _, exists := findDeployRow(latest, snapshot.BuildID); exists {
		return "", "", status, errors.New("duplicate build ID already exists; retry")
	}
	mode = normalizedDeployRuntimeMode(latestOrigin.RuntimeMode, latestOrigin.Production)
	latest = upsertDeployRow(latest, deployIndexRow{
		BuildID: snapshot.BuildID, ModuleVersion: snapshot.ModuleVersion,
		Format: "hra", File: api.relativePath(snapshot.ArchivePath),
		BuiltAt: snapshot.BuiltAt, Commit: snapshot.Commit, OriginBuildID: buildID,
		SourceHash: "", HyperBricks: strings.TrimSpace(assets.VersionMD),
		RuntimeMode: mode, Production: mode == shared.LIVE_MODE,
	})
	current = latest.Current
	if err = saveDeployIndex(indexPath, latest); err != nil {
		return "", "", http.StatusInternalServerError, err
	}
	return current, mode, http.StatusCreated, nil
}

func sameLocalDuplicateOrigin(before, after localBuildRow) bool {
	return strings.EqualFold(after.Format, "hra") && before.File == after.File &&
		before.Commit == after.Commit && before.ModuleVersion == after.ModuleVersion &&
		before.HyperBricks == after.HyperBricks
}

func sameRemoteDuplicateOrigin(before, after deployIndexRow) bool {
	return strings.EqualFold(after.Format, "hra") && before.File == after.File &&
		before.Commit == after.Commit && before.ModuleVersion == after.ModuleVersion &&
		before.HyperBricks == after.HyperBricks
}

func duplicateRemoteArchiveDir(moduleRoot string) (string, error) {
	path := filepath.Join(moduleRoot, "archives")
	if err := os.Mkdir(path, 0o755); err != nil && !os.IsExist(err) {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("archives destination must be a real directory")
	}
	return confinedDirectory(moduleRoot, path)
}

func packageDuplicateRuntime(ctx context.Context, module, buildID string, location deployPackageConfigLocation, archiveDir, originCommit, originVersion string) (commands.RuntimeSnapshotResult, int, error) {
	var empty commands.RuntimeSnapshotResult
	packageConfigWriteMu.Lock()
	config, _, err := readRegularConfinedFile(location.moduleRoot, location.path)
	if err != nil {
		packageConfigWriteMu.Unlock()
		return empty, http.StatusUnprocessableEntity, fmt.Errorf("read current package configuration: %w", err)
	}
	if len(config) > maxPackageConfigBytes {
		packageConfigWriteMu.Unlock()
		return empty, http.StatusRequestEntityTooLarge, fmt.Errorf("package configuration exceeds %d bytes", maxPackageConfigBytes)
	}
	metadata, _, err := readMetadataAndPort(location.path)
	if err != nil {
		packageConfigWriteMu.Unlock()
		return empty, http.StatusUnprocessableEntity, fmt.Errorf("read current package metadata: %w", err)
	}
	latestConfig, _, err := readRegularConfinedFile(location.moduleRoot, location.path)
	packageConfigWriteMu.Unlock()
	if err != nil || packageConfigSHA256(latestConfig) != packageConfigSHA256(config) {
		return empty, http.StatusConflict, commands.ErrRuntimeChanged
	}
	if status, err := validateDeployArchiveMetadata(module, metadata); err != nil {
		return empty, status, err
	}
	if originVersion != "" && originVersion != metadata["hyperbricks"] {
		return empty, http.StatusConflict, fmt.Errorf("original build HyperBricks version %q differs from current package %q; duplication requires the same deploy-host version", originVersion, metadata["hyperbricks"])
	}
	snapshot, err := commands.PackageRuntimeSnapshot(commands.RuntimeSnapshotOptions{
		Context: ctx,
		Module:  module, RuntimeDir: location.moduleRoot, ArchiveDir: archiveDir,
		OriginBuildID: buildID, OriginCommit: strings.TrimSpace(originCommit), HyperBricks: strings.TrimSpace(assets.VersionMD),
		ExpectedConfigSHA256: packageConfigSHA256(config),
	})
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return empty, http.StatusRequestTimeout, err
		}
		if errors.Is(err, commands.ErrRuntimeChanged) {
			return empty, http.StatusConflict, err
		}
		if errors.Is(err, commands.ErrRuntimeSnapshotTooLarge) {
			return empty, http.StatusRequestEntityTooLarge, err
		}
		return empty, http.StatusUnprocessableEntity, err
	}
	return snapshot, http.StatusCreated, nil
}
