package commands

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/spf13/cobra"
)

var (
	buildModule        string
	buildOutDir        string
	buildHRA           bool
	buildZip           bool
	buildForce         bool
	buildReplaceTarget string
)

const versionIndexFile = "hyperbricks.versions.json"

type buildFile struct {
	abs   string
	rel   string
	info  fs.FileInfo
	isDir bool
}

type buildIndex struct {
	Current  string          `json:"current"`
	Versions []buildIndexRow `json:"versions"`
}

type buildIndexRow struct {
	BuildID       string `json:"build_id"`
	ModuleVersion string `json:"moduleversion"`
	Format        string `json:"format"`
	File          string `json:"file"`
	BuiltAt       string `json:"built_at"`
	Commit        string `json:"commit"`
	SourceHash    string `json:"source_hash"`
}

// NewBuildCommand creates the "build" subcommand.
func NewBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a Hypermedia Runtime Archive",
		Run: func(cmd *cobra.Command, args []string) {
			if err := runBuild(); err != nil {
				fmt.Printf("Error building archive: %v\n", err)
				Exit = true
				return
			}
		},
	}

	cmd.Flags().BoolVar(&buildHRA, "hra", false, "Build .hra archive (default)")
	cmd.Flags().BoolVar(&buildZip, "zip", false, "Build .zip archive")
	cmd.Flags().BoolVar(&buildForce, "force", false, "Build even if no source changes are detected")
	cmd.Flags().StringVar(&buildReplaceTarget, "replace", "", "Replace the current build or a specific build ID")
	cmd.Flags().Lookup("replace").NoOptDefVal = "current"
	cmd.Flags().StringVar(&buildOutDir, "out", "deploy", "output directory for build archives")
	cmd.Flags().StringVarP(&buildModule, "module", "m", "default", "module in the ./modules directory")

	return cmd
}

func runBuild() error {
	format, ext, err := resolveBuildFormat()
	if err != nil {
		return err
	}

	if strings.TrimSpace(buildModule) == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	moduleDir := filepath.Join("modules", buildModule)
	if _, err := os.Stat(moduleDir); err != nil {
		return fmt.Errorf("module directory not found: %s", moduleDir)
	}

	configPath := filepath.Join(moduleDir, "package.hyperbricks")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	files, err := collectModuleFiles(moduleDir)
	if err != nil {
		return err
	}

	sourceHash, err := computeSourceHash(files)
	if err != nil {
		return err
	}

	outDir := filepath.Join(buildOutDir, buildModule)
	indexPath := filepath.Join(outDir, versionIndexFile)
	index, err := loadBuildIndex(indexPath)
	if err != nil {
		return err
	}
	replaceTarget := strings.TrimSpace(buildReplaceTarget)
	if !(buildForce || replaceTarget != "") {
		if current, ok := findBuildIndex(index, index.Current); ok {
			if current.SourceHash == sourceHash && current.SourceHash != "" {
				fmt.Printf("No changes detected. Current build %s matches source hash. Use --force or --replace to rebuild.\n", index.Current)
				return nil
			}
		}
	}

	commit := gitShortCommit()
	builtAt := time.Now().UTC().Format(time.RFC3339)
	hbVersion := strings.TrimSpace(assets.VersionMD)

	updates := map[string]string{
		"module":         buildModule,
		"commit":         commit,
		"built_at":       builtAt,
		"hyperbricks":    hbVersion,
		"format":         format,
		"format_version": "1",
	}

	updatedConfig, moduleVersion, err := updatePackageMetadata(configPath, string(configContent), updates)
	if err != nil {
		return err
	}

	buildID, err := computeBuildID(files, []byte(updatedConfig))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", outDir, err)
	}

	filename := fmt.Sprintf("%s-%s-%s.%s", buildModule, moduleVersion, buildID, ext)
	outPath := filepath.Join(outDir, filename)

	if err := writeArchive(outPath, files, []byte(updatedConfig)); err != nil {
		return err
	}

	oldFile, err := updateBuildIndex(indexPath, buildID, moduleVersion, format, outPath, builtAt, commit, sourceHash, replaceTarget)
	if err != nil {
		return err
	}
	if oldFile != "" {
		oldPath := filepath.Clean(filepath.FromSlash(oldFile))
		newPath := filepath.Clean(outPath)
		if oldPath != newPath {
			if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove previous archive %s: %w", oldPath, err)
			}
		}
	}

	fmt.Printf("Built archive: %s\n", outPath)
	return nil
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

func gitShortCommit() string {
	cmd := exec.Command("git", "rev-parse", "--short=7", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func collectModuleFiles(root string) ([]buildFile, error) {
	var files []buildFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if isExcludedDir(name) {
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
		if isExcludedFile(name) {
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
	case ".git", "node_modules":
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
		if rel == "package.hyperbricks" {
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
	archiveFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create archive %s: %w", outPath, err)
	}
	zipWriter := zip.NewWriter(archiveFile)

	for _, fileEntry := range files {
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

		if header.Name == "package.hyperbricks" {
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

		if _, err := io.Copy(writer, source); err != nil {
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

func updateBuildIndex(indexPath string, buildID string, moduleVersion string, format string, outPath string, builtAt string, commit string, sourceHash string, replaceTarget string) (string, error) {
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
	}

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

func updatePackageMetadata(configPath string, content string, updates map[string]string) (string, string, error) {
	lines := strings.Split(content, "\n")

	hyperStart := -1
	metaStart := -1
	metaEnd := -1
	depth := 0
	hyperDepth := -1
	metaDepth := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(stripLineComment(line))
		if trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, "{") {
			keyPart := strings.TrimSpace(strings.TrimSuffix(trimmed, "{"))
			if eq := strings.Index(keyPart, "="); eq != -1 {
				keyPart = strings.TrimSpace(keyPart[:eq])
			}
			if keyPart == "hyperbricks" && hyperStart == -1 {
				hyperStart = i
				hyperDepth = depth + 1
			} else if keyPart == "metadata" && hyperStart != -1 && metaStart == -1 && depth >= hyperDepth {
				metaStart = i
				metaDepth = depth + 1
			}
			depth++
			continue
		}
		if trimmed == "}" {
			depth--
			if metaStart != -1 && metaEnd == -1 && depth < metaDepth {
				metaEnd = i
			}
		}
	}

	var missing []string
	if hyperStart == -1 {
		missing = append(missing, "hyperbricks")
	}
	if hyperStart != -1 && metaStart == -1 {
		missing = append(missing, "hyperbricks.metadata")
	}
	if len(missing) > 0 {
		return "", "", fmt.Errorf("missing required objects in %s: %s", configPath, strings.Join(missing, ", "))
	}
	if metaEnd == -1 {
		return "", "", fmt.Errorf("missing closing brace for hyperbricks.metadata in %s", configPath)
	}

	keyLines := make(map[string][]int)
	fieldIndent := ""
	moduleVersion := ""

	for i := metaStart + 1; i < metaEnd; i++ {
		lineWithoutComment := strings.TrimSpace(stripLineComment(lines[i]))
		if lineWithoutComment == "" {
			continue
		}
		if !strings.Contains(lineWithoutComment, "=") {
			continue
		}
		if fieldIndent == "" {
			fieldIndent = leadingWhitespace(lines[i])
		}

		key, value := splitAssignment(lineWithoutComment)
		keyLines[key] = append(keyLines[key], i)
		if key == "moduleversion" && moduleVersion == "" {
			moduleVersion = cleanValue(value)
		}
	}

	if moduleVersion == "" {
		return "", "", fmt.Errorf("missing required field in %s: hyperbricks.metadata.moduleversion", configPath)
	}

	if fieldIndent == "" {
		fieldIndent = leadingWhitespace(lines[metaStart]) + "    "
	}

	for key, value := range updates {
		if indexes, ok := keyLines[key]; ok {
			for _, idx := range indexes {
				lines[idx] = replaceAssignmentLine(lines[idx], key, value)
			}
		}
	}

	insertOrder := []string{"module", "commit", "built_at", "hyperbricks", "format", "format_version"}
	var newLines []string
	for _, key := range insertOrder {
		if _, ok := keyLines[key]; ok {
			continue
		}
		newLines = append(newLines, fmt.Sprintf("%s%s = %s", fieldIndent, key, updates[key]))
	}
	if len(newLines) > 0 {
		before := append([]string{}, lines[:metaEnd]...)
		after := append([]string{}, lines[metaEnd:]...)
		lines = append(before, newLines...)
		lines = append(lines, after...)
	}

	return strings.Join(lines, "\n"), moduleVersion, nil
}

func stripLineComment(line string) string {
	if idx := strings.Index(line, "#"); idx != -1 {
		return line[:idx]
	}
	return line
}

func splitAssignment(line string) (string, string) {
	parts := strings.SplitN(line, "=", 2)
	key := strings.TrimSpace(parts[0])
	value := ""
	if len(parts) > 1 {
		value = strings.TrimSpace(parts[1])
	}
	return key, value
}

func cleanValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'")
	return value
}

func leadingWhitespace(line string) string {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return line[:i]
		}
	}
	return line
}

func replaceAssignmentLine(line string, key string, value string) string {
	indent := leadingWhitespace(line)
	comment := ""
	if idx := strings.Index(line, "#"); idx != -1 {
		comment = strings.TrimSpace(line[idx:])
	}
	if comment != "" {
		return fmt.Sprintf("%s%s = %s %s", indent, key, value, comment)
	}
	return fmt.Sprintf("%s%s = %s", indent, key, value)
}
