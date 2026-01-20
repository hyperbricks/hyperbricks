package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hyperbricks/hyperbricks/cmd/hyperbricks/commands"
	"github.com/hyperbricks/hyperbricks/pkg/parser"
)

type pluginListEntry struct {
	Name       string `json:"name,omitempty"`
	Version    string `json:"version,omitempty"`
	BinaryName string `json:"binary_name"`
	ConfigName string `json:"config_name"`
	Kind       string `json:"kind"`
	Module     string `json:"module,omitempty"`
	BuildID    string `json:"build_id,omitempty"`
	SourcePath string `json:"source_path,omitempty"`
	BinaryPath string `json:"binary_path,omitempty"`
	Status     string `json:"status,omitempty"`
}

type pluginSourceEntry struct {
	Plugin     string
	Version    string
	Meta       commands.PluginMeta
	SourceDir  string
	ConfigName string
	OutputName string
}

func splitConfigName(configName string) (string, string) {
	idx := strings.LastIndex(configName, "@")
	if idx == -1 {
		return configName, ""
	}
	return configName[:idx], configName[idx+1:]
}

func listPluginBinaries(pluginDir string, includeCustom bool) ([]pluginListEntry, error) {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []pluginListEntry{}, nil
		}
		return nil, err
	}

	list := make([]pluginListEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".so") {
			continue
		}
		configName := strings.TrimSuffix(name, ".so")
		if !includeCustom && strings.Contains(configName, "__") {
			continue
		}
		base, version := splitConfigName(configName)
		list = append(list, pluginListEntry{
			Name:       base,
			Version:    version,
			BinaryName: name,
			ConfigName: configName,
			Kind:       "global",
			BinaryPath: filepath.Join("bin", "plugins", name),
			Status:     "installed",
		})
	}
	return list, nil
}

func pluginBinaryExists(pluginDir string, outputName string) bool {
	_, err := os.Stat(filepath.Join(pluginDir, outputName))
	return err == nil
}

func removePluginBinary(workingDir string, configName string) (bool, error) {
	name := strings.TrimSpace(configName)
	if name == "" {
		return false, fmt.Errorf("plugin config name is required")
	}
	if filepath.Base(name) != name {
		return false, fmt.Errorf("invalid plugin name: %s", name)
	}
	pluginDir := filepath.Join(workingDir, "bin", "plugins")
	target := filepath.Join(pluginDir, name+".so")
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := os.Remove(target); err != nil {
		return false, err
	}
	return true, nil
}

func readPluginConfigNames(configPath string) ([]string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	parsed := parser.ParseHyperScript(string(data))
	root := parsed
	if hyper, ok := parsed["hyperbricks"].(map[string]interface{}); ok {
		root = hyper
	}
	plugins, ok := root["plugins"].(map[string]interface{})
	if !ok {
		return []string{}, nil
	}
	raw := plugins["enabled"]
	switch v := raw.(type) {
	case []interface{}:
		names := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				names = append(names, s)
			}
		}
		return names, nil
	case []string:
		return v, nil
	default:
		return []string{}, nil
	}
}

func scanPluginSourceEntries(root string, module string) (map[string]pluginSourceEntry, error) {
	entries := make(map[string]pluginSourceEntry)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return entries, nil
	}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != "manifest.json" {
			return nil
		}
		manifestDir := filepath.Dir(path)
		version := filepath.Base(manifestDir)
		pluginName := filepath.Base(filepath.Dir(manifestDir))

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var meta commands.PluginMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil
		}
		if strings.TrimSpace(meta.Source) == "" {
			return nil
		}
		if strings.TrimSpace(meta.Version) == "" {
			meta.Version = version
		}
		configName, outputName := commands.PluginOutputNames(meta, meta.Source, module, meta.Version)
		entries[configName] = pluginSourceEntry{
			Plugin:     pluginName,
			Version:    meta.Version,
			Meta:       meta,
			SourceDir:  manifestDir,
			ConfigName: configName,
			OutputName: outputName,
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("failed to scan plugins: %w", walkErr)
	}
	return entries, nil
}
