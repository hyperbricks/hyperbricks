package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

const (
	maxPackageConfigBytes        = 1 << 20
	maxPackageConfigRequestBytes = maxPackageConfigBytes + 64*1024
)

type packageConfigUpdateRequest struct {
	Content        string `json:"content"`
	ExpectedSHA256 string `json:"expected_sha256"`
}

type deployPackageConfigLocation struct {
	moduleRoot  string
	path        string
	scope       string
	running     bool
	runtimeMode string
}

var packageConfigWriteMu sync.Mutex

func packageConfigSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func writePackageConfigResponse(w http.ResponseWriter, module string, buildID string, location deployPackageConfigLocation, content []byte) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module":           module,
		"build_id":         buildID,
		"content":          string(content),
		"scope":            location.scope,
		"restart_required": location.running,
		"runtime_mode":     location.runtimeMode,
		"sha256":           packageConfigSHA256(content),
	})
}

func decodePackageConfigUpdate(w http.ResponseWriter, r *http.Request) (packageConfigUpdateRequest, error) {
	var request packageConfigUpdateRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxPackageConfigRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		var limitError *http.MaxBytesError
		if errors.As(err, &limitError) {
			return request, fmt.Errorf("package configuration request exceeds %d bytes", maxPackageConfigRequestBytes)
		}
		return request, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return request, errors.New("package configuration request must contain one JSON object")
		}
		var limitError *http.MaxBytesError
		if errors.As(err, &limitError) {
			return request, fmt.Errorf("package configuration request exceeds %d bytes", maxPackageConfigRequestBytes)
		}
		return request, errors.New("package configuration request must contain one JSON object")
	}
	if len(request.Content) > maxPackageConfigBytes {
		return request, fmt.Errorf("package configuration exceeds %d bytes", maxPackageConfigBytes)
	}
	if !utf8.ValidString(request.Content) {
		return request, errors.New("package configuration must be valid UTF-8")
	}
	request.ExpectedSHA256 = strings.ToLower(strings.TrimSpace(request.ExpectedSHA256))
	if request.ExpectedSHA256 != "" && (len(request.ExpectedSHA256) != sha256.Size*2 || !isHex(request.ExpectedSHA256)) {
		return request, errors.New("expected_sha256 must be a SHA-256 hex digest")
	}
	return request, nil
}

func isHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil
}

func readRegularConfinedFile(root string, path string) ([]byte, os.FileMode, error) {
	resolved, err := confinedRegularFile(root, path)
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, 0, err
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		return nil, 0, err
	}
	return content, info.Mode().Perm(), nil
}

func savePackageConfig(location deployPackageConfigLocation, request packageConfigUpdateRequest) ([]byte, int, error) {
	packageConfigWriteMu.Lock()
	defer packageConfigWriteMu.Unlock()

	current, mode, err := readRegularConfinedFile(location.moduleRoot, location.path)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	currentHash := packageConfigSHA256(current)
	if request.ExpectedSHA256 != "" && request.ExpectedSHA256 != currentHash {
		return nil, http.StatusConflict, errors.New("package configuration changed since it was opened; reload before saving")
	}
	content := []byte(request.Content)
	if _, err := shared.ValidatePackageConfigBytes(content, location.moduleRoot); err != nil {
		return nil, http.StatusUnprocessableEntity, err
	}
	if err := atomicWriteDeployFile(location.path, content, mode); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	return content, http.StatusOK, nil
}

func atomicWriteDeployFile(path string, content []byte, mode os.FileMode) (err error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".hyperbricks-deploy-edit-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if mode == 0 {
		mode = 0o644
	}
	if err = temporary.Chmod(mode.Perm()); err != nil {
		return err
	}
	if _, err = temporary.Write(content); err != nil {
		return err
	}
	if err = temporary.Sync(); err != nil {
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

func confinedRegularFile(root string, candidate string) (string, error) {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if err := pathWithinRoot(rootAbs, candidateAbs); err != nil {
		return "", err
	}
	info, err := os.Lstat(candidateAbs)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("selected file is not a regular file")
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", err
	}
	candidateReal, err := filepath.EvalSymlinks(candidateAbs)
	if err != nil {
		return "", err
	}
	if err := pathWithinRoot(rootReal, candidateReal); err != nil {
		return "", err
	}
	return candidateAbs, nil
}

func confinedDirectory(root string, candidate string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", err
	}
	if err := pathWithinRoot(rootAbs, candidateAbs); err != nil {
		return "", err
	}
	info, err := os.Stat(candidateAbs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("selected path is not a directory")
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", err
	}
	candidateReal, err := filepath.EvalSymlinks(candidateAbs)
	if err != nil {
		return "", err
	}
	if err := pathWithinRoot(rootReal, candidateReal); err != nil {
		return "", err
	}
	return candidateAbs, nil
}

func validateExistingDirectory(root string, candidate string) error {
	if _, err := os.Lstat(candidate); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_, err := confinedDirectory(root, candidate)
	return err
}

func pathWithinRoot(root string, candidate string) error {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return errors.New("selected file escapes the deployment root")
	}
	return nil
}

func resolveIndexedArtifact(root string, indexedPath string, extension string) (string, error) {
	indexedPath = filepath.FromSlash(strings.TrimSpace(indexedPath))
	if indexedPath == "" || indexedPath == "." {
		return "", errors.New("build archive is unavailable")
	}
	candidates := []string{indexedPath}
	if !filepath.IsAbs(indexedPath) {
		candidates = append(candidates, filepath.Join(root, indexedPath))
	}
	var lastErr error
	for _, candidate := range candidates {
		if !strings.EqualFold(filepath.Ext(candidate), extension) {
			lastErr = fmt.Errorf("build archive does not use the %s extension", extension)
			continue
		}
		resolved, err := confinedRegularFile(root, candidate)
		if err == nil {
			return resolved, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("build archive is unavailable")
	}
	return "", lastErr
}

func resolveIndexedArchive(root string, indexedPath string, format string) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(format), "hra") {
		return "", errors.New("build is not an HRA archive")
	}
	return resolveIndexedArtifact(root, indexedPath, ".hra")
}

func resolveIndexedRuntimeArchive(root string, indexedPath string, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "hra":
		return resolveIndexedArtifact(root, indexedPath, ".hra")
	case "zip":
		return resolveIndexedArtifact(root, indexedPath, ".zip")
	default:
		return "", errors.New("build is not a supported runtime archive")
	}
}

func resolveRemoteIndexedArchive(moduleRoot string, indexedPath string, format string) (string, error) {
	archivePath, err := resolveIndexedArchive(moduleRoot, indexedPath, format)
	if err != nil {
		return "", err
	}
	moduleReal, err := filepath.EvalSymlinks(moduleRoot)
	if err != nil {
		return "", err
	}
	archiveReal, err := filepath.EvalSymlinks(archivePath)
	if err != nil {
		return "", err
	}
	relativeToModule, err := filepath.Rel(moduleReal, archiveReal)
	if err != nil {
		return "", err
	}
	if filepath.Dir(relativeToModule) == "." {
		return archivePath, nil
	}

	archivesPath := filepath.Join(moduleRoot, "archives")
	archivesInfo, err := os.Lstat(archivesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("build archive is outside the supported module locations")
		}
		return "", err
	}
	if archivesInfo.Mode()&os.ModeSymlink != 0 || !archivesInfo.IsDir() {
		return "", errors.New("build archive is outside the supported module locations")
	}
	archivesRoot, err := confinedDirectory(moduleRoot, archivesPath)
	if err != nil {
		return "", err
	}
	archivesReal, err := filepath.EvalSymlinks(archivesRoot)
	if err != nil {
		return "", err
	}
	if err := pathWithinRoot(archivesReal, archiveReal); err != nil {
		return "", errors.New("build archive is outside the supported module locations")
	}
	return archivePath, nil
}

func serveBuildArchive(w http.ResponseWriter, r *http.Request, archivePath string) {
	file, err := os.Open(archivePath)
	if err != nil {
		writeError(w, http.StatusNotFound, errors.New("build archive is unavailable"))
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, errors.New("build archive is unavailable"))
		return
	}
	w.Header().Set("Content-Type", "application/vnd.hyperbricks.hra")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(archivePath)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, filepath.Base(archivePath), info.ModTime(), file)
}

func (api *deployLocalServer) localBuildArchive(module string, buildID string) (string, error) {
	if !validDeployPathPart(module) || !validDeployPathPart(buildID) || api.isDevBuildID(buildID) {
		return "", errors.New("invalid module or build_id")
	}
	index, err := loadLocalBuildIndex(api.indexPath(module))
	if err != nil {
		return "", err
	}
	row, ok := findLocalRow(index, buildID)
	if !ok {
		return "", errors.New("build_id not found")
	}
	moduleRoot, err := confinedDirectory(api.buildRoot, filepath.Join(api.buildRoot, module))
	if err != nil {
		return "", err
	}
	return resolveIndexedArchive(moduleRoot, row.File, row.Format)
}

func (api *deployLocalServer) localBuildRuntimeArchive(module string, buildID string) (string, error) {
	if !validDeployPathPart(module) || !validDeployPathPart(buildID) || api.isDevBuildID(buildID) {
		return "", errors.New("invalid module or build_id")
	}
	index, err := loadLocalBuildIndex(api.indexPath(module))
	if err != nil {
		return "", err
	}
	row, ok := findLocalRow(index, buildID)
	if !ok {
		return "", errors.New("build_id not found")
	}
	moduleRoot, err := confinedDirectory(api.buildRoot, filepath.Join(api.buildRoot, module))
	if err != nil {
		return "", err
	}
	return resolveIndexedRuntimeArchive(moduleRoot, row.File, row.Format)
}

func (api *deployAPI) remoteBuildArchive(module string, buildID string) (string, error) {
	if !validDeployPathPart(module) || !validDeployPathPart(buildID) {
		return "", errors.New("invalid module or build_id")
	}
	index, err := loadDeployIndex(api.indexPath(module))
	if err != nil {
		return "", err
	}
	row, ok := findDeployRow(index, buildID)
	if !ok {
		return "", errors.New("build_id not found")
	}
	moduleRoot, err := confinedDirectory(api.root, filepath.Join(api.root, module))
	if err != nil {
		return "", err
	}
	return resolveRemoteIndexedArchive(moduleRoot, row.File, row.Format)
}

func (api *deployLocalServer) localPackageConfig(module string, buildID string) (deployPackageConfigLocation, error) {
	if !validDeployPathPart(module) || !validDeployPathPart(buildID) {
		return deployPackageConfigLocation{}, errors.New("invalid module or build_id")
	}
	if api.isDevBuildID(buildID) {
		moduleRoot, err := confinedDirectory(api.modulesDir, filepath.Join(api.modulesDir, module))
		if err != nil {
			return deployPackageConfigLocation{}, err
		}
		location := deployPackageConfigLocation{
			moduleRoot:  moduleRoot,
			path:        filepath.Join(moduleRoot, shared.PackageConfigFileName),
			scope:       "source",
			runtimeMode: shared.DEVELOPMENT_MODE,
		}
		_, location.running = api.readBuildProcess(module, localDevBuildID)
		return location, nil
	}
	archivePath, err := api.localBuildRuntimeArchive(module, buildID)
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	moduleRoot, err := confinedDirectory(api.buildRoot, filepath.Join(api.buildRoot, module))
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	runtimeParent := filepath.Join(moduleRoot, "runtime")
	runtimeCandidate := filepath.Join(runtimeParent, buildID)
	if err := validateExistingDirectory(moduleRoot, runtimeParent); err != nil {
		return deployPackageConfigLocation{}, err
	}
	if err := validateExistingDirectory(moduleRoot, runtimeCandidate); err != nil {
		return deployPackageConfigLocation{}, err
	}
	runtimeRoot, err := commands.EnsureRuntimeExtracted(archivePath, api.buildRoot, module, buildID)
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	runtimeRoot, err = confinedDirectory(moduleRoot, runtimeRoot)
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	location := deployPackageConfigLocation{
		moduleRoot: runtimeRoot,
		path:       filepath.Join(runtimeRoot, shared.PackageConfigFileName),
		scope:      "runtime",
	}
	_, location.running = api.readBuildProcess(module, buildID)
	index, err := loadLocalBuildIndex(api.indexPath(module))
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	row, ok := findLocalRow(index, buildID)
	if !ok {
		return deployPackageConfigLocation{}, errors.New("build_id not found")
	}
	location.runtimeMode = normalizedDeployRuntimeMode(row.RuntimeMode, row.Production)
	return location, nil
}

func (api *deployAPI) remotePackageConfig(module string, buildID string) (deployPackageConfigLocation, error) {
	archivePath, err := api.remoteBuildArchive(module, buildID)
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	moduleRoot, err := confinedDirectory(api.root, filepath.Join(api.root, module))
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	runtimeParent := filepath.Join(moduleRoot, "runtime")
	runtimeCandidate := filepath.Join(runtimeParent, buildID)
	if err := validateExistingDirectory(moduleRoot, runtimeParent); err != nil {
		return deployPackageConfigLocation{}, err
	}
	if err := validateExistingDirectory(moduleRoot, runtimeCandidate); err != nil {
		return deployPackageConfigLocation{}, err
	}
	runtimeRoot, err := commands.EnsureRuntimeExtracted(archivePath, api.root, module, buildID)
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	runtimeRoot, err = confinedDirectory(moduleRoot, runtimeRoot)
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	location := deployPackageConfigLocation{
		moduleRoot: runtimeRoot,
		path:       filepath.Join(runtimeRoot, shared.PackageConfigFileName),
		scope:      "runtime",
	}
	_, location.running = api.readBuildProcess(module, buildID)
	index, err := loadDeployIndex(api.indexPath(module))
	if err != nil {
		return deployPackageConfigLocation{}, err
	}
	row, ok := findDeployRow(index, buildID)
	if !ok {
		return deployPackageConfigLocation{}, errors.New("build_id not found")
	}
	location.runtimeMode = normalizedDeployRuntimeMode(row.RuntimeMode, row.Production)
	return location, nil
}

func handlePackageConfigRequest(w http.ResponseWriter, r *http.Request, module string, buildID string, location deployPackageConfigLocation) {
	content, _, err := readRegularConfinedFile(location.moduleRoot, location.path)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("package configuration is unavailable"))
		return
	}
	if r.Method == http.MethodGet {
		writePackageConfigResponse(w, module, buildID, location, content)
		return
	}
	request, err := decodePackageConfigUpdate(w, r)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "exceeds") {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, err)
		return
	}
	content, status, err := savePackageConfig(location, request)
	if err != nil {
		writeError(w, status, err)
		return
	}
	writePackageConfigResponse(w, module, buildID, location, content)
}

func (api *deployLocalServer) handleBuildPackageConfig(w http.ResponseWriter, r *http.Request, module string, buildID string) {
	location, err := api.localPackageConfig(module, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	handlePackageConfigRequest(w, r, module, buildID, location)
}

func (api *deployAPI) handleBuildPackageConfig(w http.ResponseWriter, r *http.Request, module string, buildID string) {
	location, err := api.remotePackageConfig(module, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	handlePackageConfigRequest(w, r, module, buildID, location)
}

func (api *deployLocalServer) handleBuildArchive(w http.ResponseWriter, r *http.Request, module string, buildID string) {
	archivePath, err := api.localBuildArchive(module, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	serveBuildArchive(w, r, archivePath)
}

func (api *deployAPI) handleBuildArchive(w http.ResponseWriter, r *http.Request, module string, buildID string) {
	archivePath, err := api.remoteBuildArchive(module, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	serveBuildArchive(w, r, archivePath)
}
