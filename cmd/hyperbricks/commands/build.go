package commands

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/hyperbricks/hyperbricks/pkg/packagemetadata"
	"github.com/spf13/cobra"
)

var (
	buildModule        string
	buildOutDir        string
	buildHRA           bool
	buildZip           bool
	buildForce         bool
	buildReplaceTarget string
	buildPush          bool
	buildPushTarget    string
)

var buildMu sync.Mutex

// WithBuildIndexLock serializes deployment-index registration with a source
// build in this process. It does not lock independent CLI processes.
func WithBuildIndexLock(update func() error) error {
	buildMu.Lock()
	defer buildMu.Unlock()
	return update()
}

const versionIndexFile = "hyperbricks.versions.json"

type buildFile struct {
	abs   string
	rel   string
	info  fs.FileInfo
	isDir bool
}

type buildIndex struct {
	Current  string          `json:"current"`
	Port     int             `json:"port,omitempty"`
	Versions []buildIndexRow `json:"versions"`
}

type buildIndexRow struct {
	BuildID       string `json:"build_id"`
	ModuleVersion string `json:"moduleversion"`
	Format        string `json:"format"`
	File          string `json:"file"`
	BuiltAt       string `json:"built_at"`
	Commit        string `json:"commit"`
	OriginBuildID string `json:"origin_build_id,omitempty"`
	SourceHash    string `json:"source_hash"`
	HyperBricks   string `json:"hyperbricks,omitempty"`
	RuntimeMode   string `json:"runtime_mode,omitempty"`
	Production    bool   `json:"production,omitempty"`
}

type buildResult struct {
	Module      string
	BuildID     string
	ArchivePath string
	Built       bool
}

type BuildOptions struct {
	Module        string
	OutDir        string
	Force         bool
	ReplaceTarget string
	Format        string
}

// NewBuildCommand creates the "build" subcommand.
func NewBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a Hypermedia Runtime Archive",
		Run: func(cmd *cobra.Command, args []string) {
			if cmd.Flags().NFlag() == 0 {
				RunBuildWizard()
				return
			}
			result, err := runBuild()
			if err != nil {
				failf("Error building archive: %v\n", err)
				Exit = true
				return
			}
			if buildPush {
				if err := runBuildPush(result); err != nil {
					failf("Error pushing build: %v\n", err)
					Exit = true
					return
				}
			}
		},
	}

	cmd.Flags().BoolVar(&buildHRA, "hra", false, "Build .hra archive (default)")
	cmd.Flags().BoolVar(&buildZip, "zip", false, "Build .zip archive")
	cmd.Flags().BoolVar(&buildForce, "force", false, "Build even if no source changes are detected")
	cmd.Flags().StringVar(&buildReplaceTarget, "replace", "", "Replace the current build or a specific build ID")
	cmd.Flags().Lookup("replace").NoOptDefVal = "current"
	cmd.Flags().StringVar(&buildOutDir, "out", "deploy", "output directory for build archives")
	cmd.Flags().StringVarP(&buildModule, "module", "m", "default", "module name or directory path")
	_ = cmd.RegisterFlagCompletionFunc("module", completeModuleSelection)
	cmd.Flags().BoolVar(&buildPush, "push", false, "Push build archive to a deploy target")
	cmd.Flags().StringVar(&buildPushTarget, "target", "", "Deploy target name for --push")

	return cmd
}

func BuildModuleWithOptions(opts BuildOptions) (buildResult, error) {
	if strings.TrimSpace(opts.Module) == "" {
		return buildResult{}, fmt.Errorf("module name cannot be empty")
	}
	buildMu.Lock()
	defer buildMu.Unlock()

	prevModule := buildModule
	prevOutDir := buildOutDir
	prevForce := buildForce
	prevReplace := buildReplaceTarget
	prevHRA := buildHRA
	prevZip := buildZip

	buildModule = opts.Module
	if strings.TrimSpace(opts.OutDir) != "" {
		buildOutDir = opts.OutDir
	}
	buildForce = opts.Force
	buildReplaceTarget = opts.ReplaceTarget
	buildHRA = false
	buildZip = false
	switch strings.ToLower(strings.TrimSpace(opts.Format)) {
	case "zip":
		buildZip = true
	case "hra", "":
	default:
		buildModule = prevModule
		buildOutDir = prevOutDir
		buildForce = prevForce
		buildReplaceTarget = prevReplace
		buildHRA = prevHRA
		buildZip = prevZip
		return buildResult{}, fmt.Errorf("unsupported build format: %s", opts.Format)
	}

	result, err := runBuild()

	buildModule = prevModule
	buildOutDir = prevOutDir
	buildForce = prevForce
	buildReplaceTarget = prevReplace
	buildHRA = prevHRA
	buildZip = prevZip

	return result, err
}

func runBuild() (buildResult, error) {
	result := buildResult{}
	format, ext, err := resolveBuildFormat()
	if err != nil {
		return result, err
	}

	if strings.TrimSpace(buildModule) == "" {
		return result, fmt.Errorf("module name cannot be empty")
	}

	workingDirectory, err := os.Getwd()
	if err != nil {
		return result, fmt.Errorf("resolve current working directory: %w", err)
	}
	selection, err := resolveModuleSelection(buildModule, workingDirectory)
	if err != nil {
		return result, fmt.Errorf("resolve module %q: %w", buildModule, err)
	}
	moduleDir := selection.Root
	moduleName := selection.Name
	result.Module = moduleName
	if _, err := os.Stat(moduleDir); err != nil {
		return result, fmt.Errorf("module directory not found: %s", moduleDir)
	}

	configPath := filepath.Join(moduleDir, PackageConfigFileName)
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return result, fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	files, err := collectModuleFiles(moduleDir)
	if err != nil {
		return result, err
	}

	sourceHash, err := computeSourceHash(files)
	if err != nil {
		return result, err
	}

	outDir := filepath.Join(buildOutDir, moduleName)
	indexPath := filepath.Join(outDir, versionIndexFile)
	index, err := loadBuildIndex(indexPath)
	if err != nil {
		return result, err
	}
	hbVersion := strings.TrimSpace(assets.VersionMD)
	replaceTarget := strings.TrimSpace(buildReplaceTarget)
	if !(buildForce || replaceTarget != "") {
		if current, ok := findBuildIndex(index, index.Current); ok {
			if current.SourceHash == sourceHash && current.SourceHash != "" && current.Format == format && current.HyperBricks == hbVersion {
				fmt.Printf("No changes detected. Current build %s matches the source, format, and HyperBricks version. Use --force or --replace to rebuild.\n", index.Current)
				result.BuildID = index.Current
				return result, nil
			}
		}
	}

	commit := packagemetadata.GitShortCommit(moduleDir)
	builtAt := time.Now().UTC().Format(time.RFC3339)
	artifact, err := packagemetadata.RenderArtifact(configContent, packagemetadata.ArtifactOptions{
		Module:        moduleName,
		Format:        format,
		FormatVersion: "1",
		Commit:        commit,
		BuiltAt:       builtAt,
		HyperBricks:   hbVersion,
	})
	if err != nil {
		return result, fmt.Errorf("prepare artifact metadata for %s: %w", configPath, err)
	}
	moduleVersion := artifact.Metadata.ModuleVersion

	buildID, err := computeBuildID(files, artifact.Content)
	if err != nil {
		return result, err
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return result, fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	filename := fmt.Sprintf("%s-%s-%s.%s", moduleName, moduleVersion, buildID, ext)
	outPath := filepath.Join(outDir, filename)

	if err := writeArchive(outPath, files, artifact.Content); err != nil {
		return result, err
	}

	oldFile, err := updateBuildIndex(indexPath, buildID, moduleVersion, format, outPath, builtAt, commit, sourceHash, hbVersion, replaceTarget)
	if err != nil {
		return result, err
	}
	if oldFile != "" {
		oldPath := filepath.Clean(filepath.FromSlash(oldFile))
		newPath := filepath.Clean(outPath)
		if oldPath != newPath {
			if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
				return result, fmt.Errorf("failed to remove previous archive %s: %w", oldPath, err)
			}
		}
	}

	fmt.Printf("Built archive: %s\n", outPath)
	result.BuildID = buildID
	result.ArchivePath = outPath
	result.Built = true
	return result, nil
}

func resolveBuildFormat() (string, string, error) {
	if buildHRA && buildZip {
		return "", "", fmt.Errorf("only one of --hra or --zip may be set")
	}
	if buildZip {
		return "zip", "zip", nil
	}
	return "hra", "hra", nil
}

func collectModuleFiles(root string) ([]buildFile, error) {
	excludeCache, err := moduleCacheExclusion(root)
	if err != nil {
		return nil, err
	}
	var files []buildFile
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if isExcludedDir(name) || excludeCache(path) {
				return fs.SkipDir
			}
			if path == root {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, buildFile{
				abs:   path,
				rel:   rel,
				info:  info,
				isDir: true,
			})
			return nil
		}
		if isExcludedFile(name) || excludeCache(path) {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, buildFile{
			abs:   path,
			rel:   rel,
			info:  info,
			isDir: false,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].rel < files[j].rel
	})
	return files, nil
}

func isExcludedDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".cache":
		return true
	default:
		return false
	}
}

func isExcludedFile(name string) bool {
	switch name {
	case ".DS_Store", ".gitignore", versionIndexFile:
		return true
	default:
		return false
	}
}

func computeSourceHash(files []buildFile) (string, error) {
	hasher := sha256.New()
	for _, file := range files {
		rel := filepath.ToSlash(file.rel)
		if file.isDir {
			rel += "/"
		}
		if _, err := io.WriteString(hasher, rel); err != nil {
			return "", err
		}
		if _, err := io.WriteString(hasher, "\n"); err != nil {
			return "", err
		}

		if file.isDir {
			if _, err := io.WriteString(hasher, "dir\n"); err != nil {
				return "", err
			}
			continue
		}

		data, err := os.ReadFile(file.abs)
		if err != nil {
			return "", err
		}

		if _, err := hasher.Write(data); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func computeBuildID(files []buildFile, updatedConfig []byte) (string, error) {
	hasher := sha256.New()
	for _, file := range files {
		rel := filepath.ToSlash(file.rel)
		if file.isDir {
			rel += "/"
		}
		if _, err := io.WriteString(hasher, rel); err != nil {
			return "", err
		}
		if _, err := io.WriteString(hasher, "\n"); err != nil {
			return "", err
		}

		if file.isDir {
			if _, err := io.WriteString(hasher, "dir\n"); err != nil {
				return "", err
			}
			continue
		}

		var content []byte
		if rel == PackageConfigFileName {
			content = updatedConfig
		} else {
			data, err := os.ReadFile(file.abs)
			if err != nil {
				return "", err
			}
			content = data
		}

		if _, err := hasher.Write(content); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeArchive(outPath string, files []buildFile, updatedConfig []byte) error {
	return writeArchiveWithContext(context.Background(), outPath, files, updatedConfig)
}

func writeArchiveWithContext(ctx context.Context, outPath string, files []buildFile, updatedConfig []byte) error {
	archiveFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create archive %s: %w", outPath, err)
	}
	defer archiveFile.Close()
	zipWriter := zip.NewWriter(archiveFile)

	for _, fileEntry := range files {
		if err := ctx.Err(); err != nil {
			archiveFile.Close()
			return err
		}
		header, err := zip.FileInfoHeader(fileEntry.info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(fileEntry.rel)
		if fileEntry.isDir {
			header.Name += "/"
			header.Method = zip.Store
		} else {
			header.Method = zip.Deflate
		}

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			archiveFile.Close()
			return err
		}

		if fileEntry.isDir {
			continue
		}

		if header.Name == PackageConfigFileName {
			if _, err := writer.Write(updatedConfig); err != nil {
				archiveFile.Close()
				return err
			}
			continue
		}

		source, err := os.Open(fileEntry.abs)
		if err != nil {
			archiveFile.Close()
			return err
		}

		if _, err := io.Copy(writer, runtimeContextReader{ctx: ctx, reader: source}); err != nil {
			source.Close()
			archiveFile.Close()
			return err
		}
		if err := source.Close(); err != nil {
			archiveFile.Close()
			return err
		}
	}

	if err := zipWriter.Close(); err != nil {
		archiveFile.Close()
		return err
	}
	return archiveFile.Close()
}

func updateBuildIndex(indexPath string, buildID string, moduleVersion string, format string, outPath string, builtAt string, commit string, sourceHash string, hyperBricks string, replaceTarget string) (string, error) {
	index, err := loadBuildIndex(indexPath)
	if err != nil {
		return "", err
	}

	oldFile := ""
	if replaceTarget != "" {
		targetID := replaceTarget
		if replaceTarget == "current" {
			if index.Current == "" {
				return "", fmt.Errorf("no current build to replace")
			}
			targetID = index.Current
		}
		targetRow, ok := findBuildIndex(index, targetID)
		if !ok {
			return "", fmt.Errorf("build id not found for replace: %s", targetID)
		}
		oldFile = targetRow.File
		filtered := index.Versions[:0]
		for _, row := range index.Versions {
			if row.BuildID != targetID {
				filtered = append(filtered, row)
			}
		}
		index.Versions = filtered
		if index.Current == targetID {
			index.Current = ""
		}
	}

	entry := buildIndexRow{
		BuildID:       buildID,
		ModuleVersion: moduleVersion,
		Format:        format,
		File:          filepath.ToSlash(outPath),
		BuiltAt:       builtAt,
		Commit:        commit,
		SourceHash:    sourceHash,
		HyperBricks:   hyperBricks,
		RuntimeMode:   "development",
	}
	if existing, ok := findBuildIndex(index, buildID); ok {
		entry.RuntimeMode = strings.ToLower(strings.TrimSpace(existing.RuntimeMode))
		if entry.RuntimeMode != "development" && entry.RuntimeMode != "live" {
			if existing.Production {
				entry.RuntimeMode = "live"
			} else {
				entry.RuntimeMode = "development"
			}
		}
		entry.Production = existing.Production
	}
	entry.Production = entry.RuntimeMode == "live"

	updated := false
	for i, row := range index.Versions {
		if row.BuildID == buildID {
			index.Versions[i] = entry
			updated = true
			break
		}
	}
	if !updated {
		index.Versions = append(index.Versions, entry)
	}
	index.Current = buildID

	payload, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize build index: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(indexPath, payload, 0644); err != nil {
		return "", fmt.Errorf("failed to write build index %s: %w", indexPath, err)
	}

	return oldFile, nil
}

func loadBuildIndex(path string) (buildIndex, error) {
	var index buildIndex
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return index, nil
		}
		return index, fmt.Errorf("failed to read build index %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return index, nil
	}
	if err := json.Unmarshal(data, &index); err != nil {
		return index, fmt.Errorf("invalid build index %s: %w", path, err)
	}
	return index, nil
}

func findBuildIndex(index buildIndex, buildID string) (buildIndexRow, bool) {
	for _, row := range index.Versions {
		if row.BuildID == buildID {
			return row, true
		}
	}
	return buildIndexRow{}, false
}
