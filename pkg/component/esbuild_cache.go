package component

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

const esbuildManifestVersion = 2

// Test executables may omit dependency build info. Keep this fallback in sync
// with go.mod; TestEsbuildEmbeddedVersion checks the pinned dependency.
const esbuildEmbeddedVersion = "v0.25.7"

type esbuildManifest struct {
	Version int                           `json:"version"`
	Key     string                        `json:"key"`
	Engine  string                        `json:"engine"`
	Spec    esbuildSpec                   `json:"spec"`
	Entry   string                        `json:"entry_output"`
	Inputs  map[string]esbuildFingerprint `json:"inputs"`
	Outputs map[string]esbuildFingerprint `json:"outputs"`
}

func (s esbuildSpec) engineIdentity(ctx context.Context) (string, error) {
	if s.Binary != "" {
		version, err := exec.CommandContext(ctx, s.Binary, "--version").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("esbuild binary version: %w: %s", err, version)
		}
		return "external:" + strings.TrimSpace(string(version)), nil
	}
	version := esbuildEmbeddedVersion
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/evanw/esbuild" {
				if dep.Replace != nil {
					dep = dep.Replace
				}
				version = dep.Path + "@" + dep.Version + ":" + dep.Sum
				break
			}
		}
	}
	return "embedded:" + version, nil
}

func (s esbuildSpec) outputStem() string {
	return strings.TrimSuffix(filepath.Base(s.Outfile), filepath.Ext(s.Outfile))
}

func (s esbuildSpec) entryOutput(metafile string, outputs map[string][]byte) (string, error) {
	if !s.Fingerprint {
		if _, ok := outputs[s.Outfile]; ok {
			return s.Outfile, nil
		}
		return "", fmt.Errorf("esbuild produced no requested output %s", s.Outfile)
	}
	var meta struct {
		Outputs map[string]struct {
			EntryPoint string `json:"entryPoint"`
		} `json:"outputs"`
	}
	if err := json.Unmarshal([]byte(metafile), &meta); err != nil {
		return "", fmt.Errorf("esbuild output metadata: %w", err)
	}
	entry := ""
	for path, output := range meta.Outputs {
		if output.EntryPoint == "" || filepath.Ext(path) != filepath.Ext(s.Outfile) {
			continue
		}
		candidate := filepath.Join(filepath.Dir(s.Outfile), filepath.Base(path))
		if _, ok := outputs[candidate]; !ok || !s.validEntryOutput(candidate) || entry != "" {
			return "", fmt.Errorf("esbuild produced an unexpected or ambiguous entry output: %s", path)
		}
		entry = candidate
	}
	if entry == "" {
		return "", fmt.Errorf("esbuild produced no fingerprinted entry for %s", s.Outfile)
	}
	return entry, nil
}

func (s esbuildSpec) validEntryOutput(path string) bool {
	if !s.Fingerprint {
		return path == s.Outfile
	}
	if filepath.Dir(path) != filepath.Dir(s.Outfile) || filepath.Ext(path) != filepath.Ext(s.Outfile) {
		return false
	}
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	prefix := s.outputStem() + "."
	return strings.HasPrefix(stem, prefix) && len(stem) > len(prefix)
}

func (s *esbuildStore) manifestPath(p *PreparedEsbuild) (string, error) {
	if s.cacheDirErr != nil {
		return "", s.cacheDirErr
	}
	if s.cacheDir == "" {
		return "", fmt.Errorf("no user cache directory is available")
	}
	path, err := filepath.Abs(filepath.Join(s.cacheDir, p.key+".json"))
	if err != nil {
		return "", err
	}
	root, err := esbuildExistingPath(p.spec.StaticDir)
	if err != nil {
		return "", err
	}
	resolved, err := esbuildExistingPath(path)
	if err != nil {
		return "", err
	}
	if err := esbuildOutputPath(root, resolved); err == nil {
		return "", fmt.Errorf("private esbuild cache must not be inside static")
	}
	return path, nil
}

func (s *esbuildStore) restore(p *PreparedEsbuild, engine string) *esbuildResult {
	path, err := s.manifestPath(p)
	if err != nil {
		return nil // persist reports unavailable storage after a successful build.
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.GetLogger().Warnw("esbuild persistent cache unavailable; rebuilding", "error", err)
		}
		return nil
	}
	var manifest esbuildManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		logging.GetLogger().Warnw("esbuild invalid cache manifest; rebuilding", "error", err)
		return nil
	}
	if manifest.Version != esbuildManifestVersion || manifest.Key != p.key || manifest.Engine != engine || !reflect.DeepEqual(manifest.Spec, p.spec) || !p.spec.validEntryOutput(manifest.Entry) {
		return nil
	}
	if !manifest.Inputs[p.spec.Entry].Exists || !manifest.Outputs[manifest.Entry].Exists {
		return nil
	}
	if p.spec.Binary != "" && !manifest.Inputs[p.spec.Binary].Exists {
		return nil
	}
	for path := range manifest.Inputs {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return nil
		}
	}
	for path, fingerprint := range manifest.Outputs {
		if !fingerprint.Exists || esbuildOutputPath(p.spec.StaticDir, path) != nil || !esbuildHasExactFilename(path) {
			return nil
		}
		if _, input := manifest.Inputs[path]; input {
			return nil
		}
	}
	result := &esbuildResult{inputs: manifest.Inputs, outputs: manifest.Outputs, entry: manifest.Entry}
	if !result.valid() {
		return nil
	}
	return result
}

func (s *esbuildStore) persist(p *PreparedEsbuild, engine string, result *esbuildResult) {
	err := func() error {
		path, err := s.manifestPath(p)
		if err != nil {
			return err
		}
		manifest := esbuildManifest{Version: esbuildManifestVersion, Key: p.key, Engine: engine, Spec: p.spec, Entry: result.entry, Inputs: result.inputs, Outputs: result.outputs}
		data, err := json.Marshal(manifest)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		file, err := os.CreateTemp(filepath.Dir(path), ".hb-esbuild-*")
		if err != nil {
			return err
		}
		defer os.Remove(file.Name())
		if _, err := file.Write(data); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
		return os.Rename(file.Name(), path)
	}()
	if err != nil {
		logging.GetLogger().Warnw("esbuild built successfully without persistent cache", "entry", p.spec.Entry, "error", err)
	}
}

func (s *esbuildStore) claimOutputs(p *PreparedEsbuild, outputs map[string]esbuildFingerprint) error {
	for path := range outputs {
		if err := esbuildOutputPath(p.spec.StaticDir, path); err != nil {
			return err
		}
		if owner := s.owners[path]; owner != "" && owner != p.spec.Outfile {
			return fmt.Errorf("esbuild output %s is already owned by %s", path, owner)
		}
	}
	for path := range outputs {
		s.generated[path] = true
		s.owners[path] = p.spec.Outfile
	}
	return nil
}
