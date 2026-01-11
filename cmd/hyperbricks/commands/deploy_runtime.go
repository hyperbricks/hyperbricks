package commands

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const runtimeDirName = "runtime"

func ResolveDeployArchive(module string, deployDir string, buildID string) (string, string, error) {
	indexPath := filepath.Join(deployDir, module, versionIndexFile)
	index, err := loadBuildIndex(indexPath)
	if err != nil {
		return "", "", err
	}

	resolvedID := strings.TrimSpace(buildID)
	if resolvedID == "" {
		resolvedID = index.Current
	}
	if resolvedID == "" {
		return "", "", fmt.Errorf("no current build set in %s", indexPath)
	}

	row, ok := findBuildIndex(index, resolvedID)
	if !ok {
		return "", "", fmt.Errorf("build id not found: %s", resolvedID)
	}

	archivePath := filepath.Clean(filepath.FromSlash(row.File))
	return archivePath, resolvedID, nil
}

func EnsureRuntimeExtracted(archivePath string, deployDir string, module string, buildID string) (string, error) {
	runtimeDir := filepath.Join(deployDir, module, runtimeDirName, buildID)
	configPath := filepath.Join(runtimeDir, "package.hyperbricks")
	if _, err := os.Stat(configPath); err == nil {
		return runtimeDir, nil
	}

	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create runtime directory %s: %w", runtimeDir, err)
	}
	if err := extractZipArchive(archivePath, runtimeDir); err != nil {
		return "", err
	}
	return runtimeDir, nil
}

func extractZipArchive(archivePath string, dest string) error {
	if _, err := os.Stat(archivePath); err != nil {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive %s: %w", archivePath, err)
	}
	defer reader.Close()

	destClean := filepath.Clean(dest)
	for _, file := range reader.File {
		targetPath, err := safeArchivePath(destClean, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(targetPath), err)
		}

		source, err := file.Open()
		if err != nil {
			return fmt.Errorf("failed to open archive entry %s: %w", file.Name, err)
		}

		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			source.Close()
			return fmt.Errorf("failed to create file %s: %w", targetPath, err)
		}

		if _, err := io.Copy(out, source); err != nil {
			out.Close()
			source.Close()
			return fmt.Errorf("failed to write file %s: %w", targetPath, err)
		}
		if err := out.Close(); err != nil {
			source.Close()
			return fmt.Errorf("failed to close file %s: %w", targetPath, err)
		}
		if err := source.Close(); err != nil {
			return fmt.Errorf("failed to close archive entry %s: %w", file.Name, err)
		}
	}

	return nil
}

func safeArchivePath(dest string, name string) (string, error) {
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
		return "", fmt.Errorf("invalid archive path: %s", name)
	}
	targetPath := filepath.Join(dest, cleanName)
	destPrefix := filepath.Clean(dest) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(targetPath)+string(os.PathSeparator), destPrefix) {
		return "", fmt.Errorf("invalid archive path: %s", name)
	}
	return targetPath, nil
}
