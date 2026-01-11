package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otiai10/copy"
)

func exportStaticZip(renderDir string, module string, outDir string, excludeCSV string) (string, error) {
	renderDir = strings.TrimSpace(renderDir)
	if renderDir == "" {
		return "", fmt.Errorf("render directory is empty")
	}
	info, err := os.Stat(renderDir)
	if err != nil {
		return "", fmt.Errorf("render directory not found: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("render path is not a directory: %s", renderDir)
	}

	module = strings.TrimSpace(module)
	if module == "" {
		module = "default"
	}

	excludes, err := parseExcludeList(excludeCSV)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(outDir) == "" {
		outDir = filepath.Join("exports", module)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create export directory %s: %w", outDir, err)
	}

	timestamp := time.Now().Format("20060102-150405")
	outPath := filepath.Join(outDir, fmt.Sprintf("export-%s-%s.zip", module, timestamp))

	stageDir, err := os.MkdirTemp("", "hbstatic.")
	if err != nil {
		return "", fmt.Errorf("failed to create staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)

	if err := stageRenderedOutput(renderDir, stageDir, excludes); err != nil {
		return "", err
	}
	if err := renameExtensionlessHTML(stageDir); err != nil {
		return "", err
	}
	if err := normalizePermissions(stageDir); err != nil {
		return "", err
	}
	if err := zipDirectory(stageDir, outPath); err != nil {
		return "", err
	}

	return outPath, nil
}

func parseExcludeList(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	excludes := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}

		if filepath.IsAbs(p) {
			return nil, fmt.Errorf("exclude path must be relative to render root: %s", p)
		}

		p = filepath.Clean(p)
		p = filepath.ToSlash(p)
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimSuffix(p, "/")

		if p == "" || p == "." {
			continue
		}
		if p == ".." || strings.HasPrefix(p, "../") {
			return nil, fmt.Errorf("exclude path must not traverse up: %s", part)
		}

		excludes = append(excludes, p)
	}

	return excludes, nil
}

func stageRenderedOutput(srcDir string, destDir string, excludes []string) error {
	err := copy.Copy(srcDir, destDir, copy.Options{
		OnSymlink: func(string) copy.SymlinkAction {
			return copy.Deep
		},
		Skip: func(info os.FileInfo, src, dest string) (bool, error) {
			rel, err := filepath.Rel(srcDir, src)
			if err != nil {
				return false, err
			}
			if rel == "." {
				return false, nil
			}
			if info.Name() == ".DS_Store" {
				return true, nil
			}
			rel = filepath.ToSlash(rel)
			if shouldExclude(rel, excludes) {
				return true, nil
			}
			return false, nil
		},
	})
	if err != nil {
		return fmt.Errorf("failed to stage render output: %w", err)
	}
	return nil
}

func shouldExclude(rel string, excludes []string) bool {
	for _, ex := range excludes {
		if rel == ex || strings.HasPrefix(rel, ex+"/") {
			return true
		}
	}
	return false
}

func renameExtensionlessHTML(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if strings.Contains(d.Name(), ".") {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		buffer := make([]byte, 512)
		n, _ := file.Read(buffer)
		contentType := http.DetectContentType(buffer[:n])
		if strings.Contains(contentType, "text/html") {
			if err := os.Rename(path, path+".html"); err != nil {
				return err
			}
		}
		return nil
	})
}

func normalizePermissions(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			return os.Chmod(path, 0755)
		}
		return os.Chmod(path, 0644)
	})
}

func zipDirectory(srcDir string, outPath string) error {
	if info, err := os.Stat(outPath); err == nil && info.IsDir() {
		return fmt.Errorf("output path is a directory: %s", outPath)
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file %s: %w", outPath, err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel

		if d.IsDir() {
			header.Name += "/"
			header.Method = zip.Store
			_, err = zipWriter.CreateHeader(header)
			return err
		}

		header.Method = zip.Deflate
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}
