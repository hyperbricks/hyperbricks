package component

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func esbuildHasExactFilename(path string) bool {
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return false
	}
	base := filepath.Base(path)
	for _, entry := range entries {
		if entry.Name() == base {
			return true
		}
	}
	return false
}

func (s esbuildSpec) lowercaseEntryHashes(outputs map[string][]byte, metafile, entry string) (map[string][]byte, string, error) {
	if !s.Fingerprint {
		return outputs, entry, nil
	}
	var meta struct {
		Outputs map[string]struct {
			CSSBundle string `json:"cssBundle"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(metafile), &meta); err != nil {
		return nil, "", fmt.Errorf("esbuild output metadata: %w", err)
	}
	entries := []string{entry}
	for path, output := range meta.Outputs {
		if filepath.Join(filepath.Dir(entry), filepath.Base(path)) == entry && output.CSSBundle != "" {
			entries = append(entries, filepath.Join(filepath.Dir(entry), filepath.Base(output.CSSBundle)))
		}
	}
	normalized := make(map[string][]byte, len(outputs))
	for path, data := range outputs {
		normalized[path] = data
	}
	for _, old := range entries {
		data, ok := outputs[old]
		prefix, ext := s.outputStem()+".", filepath.Ext(old)
		base := filepath.Base(old)
		if !ok || !strings.HasPrefix(base, prefix) || (ext != ".js" && ext != ".css") {
			return nil, "", fmt.Errorf("unexpected fingerprinted esbuild entry: %s", old)
		}
		hash := strings.TrimSuffix(strings.TrimPrefix(base, prefix), ext)
		if hash == "" {
			return nil, "", fmt.Errorf("missing esbuild entry hash: %s", old)
		}
		lower := filepath.Join(filepath.Dir(old), prefix+strings.ToLower(hash)+ext)
		if lower == old {
			continue
		}
		if _, exists := normalized[lower]; exists {
			return nil, "", fmt.Errorf("lowercase esbuild output conflicts with %s", lower)
		}
		if sourceMap, ok := outputs[old+".map"]; ok {
			if _, exists := normalized[lower+".map"]; exists {
				return nil, "", fmt.Errorf("lowercase esbuild source map conflicts with %s.map", lower)
			}
			var err error
			data, err = esbuildRenameMapLink(data, old, lower)
			if err != nil {
				return nil, "", err
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(sourceMap, &fields); err != nil {
				return nil, "", fmt.Errorf("esbuild source map: %w", err)
			}
			if _, hasFile := fields["file"]; hasFile {
				fields["file"], _ = json.Marshal(filepath.Base(lower))
				sourceMap, err = json.Marshal(fields)
				if err != nil {
					return nil, "", err
				}
			}
			delete(normalized, old+".map")
			normalized[lower+".map"] = sourceMap
		}
		delete(normalized, old)
		normalized[lower] = data
		if old == entry {
			entry = lower
		}
	}
	return normalized, entry, nil
}

func esbuildRenameMapLink(data []byte, old, lower string) ([]byte, error) {
	footer := func(path string) []byte {
		name := (&url.URL{Path: filepath.Base(path) + ".map"}).EscapedPath()
		if filepath.Ext(path) == ".css" {
			return []byte("/*# sourceMappingURL=" + name + " */")
		}
		return []byte("//# sourceMappingURL=" + name)
	}
	// Rewrite only esbuild's trailing directive. Application strings and embedded
	// source content can contain similar text and must remain byte-for-byte intact.
	trimmed, previous := bytes.TrimRight(data, "\r\n"), footer(old)
	if !bytes.HasSuffix(trimmed, previous) {
		return nil, fmt.Errorf("missing linked source-map footer for %s", old)
	}
	result := make([]byte, 0, len(data))
	result = append(result, trimmed[:len(trimmed)-len(previous)]...)
	result = append(result, footer(lower)...)
	result = append(result, data[len(trimmed):]...)
	return result, nil
}
