package component

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// Generation records share the native esbuild cache directory. They are
// independent of cache reuse: even cache:false outputs have an owner/history.
type esbuildGeneration struct {
	Entry   string                        `json:"entry"`
	Outputs map[string]esbuildFingerprint `json:"outputs"`
}
type esbuildOwnerHistory struct {
	Keep        int                 `json:"keep"`
	Generations []esbuildGeneration `json:"generations"`
}
type esbuildHistory struct {
	Version int                             `json:"version"`
	Root    string                          `json:"root"`
	Owners  map[string]*esbuildOwnerHistory `json:"owners"`
}

func (s *esbuildStore) historyPath() (string, error) {
	if s.cacheDirErr != nil {
		return "", s.cacheDirErr
	}
	if s.cacheDir == "" {
		return "", fmt.Errorf("esbuild asset ownership requires a private cache directory")
	}
	root, err := filepath.Abs(s.staticDir)
	if err != nil {
		return "", err
	}
	root, err = esbuildExistingPath(root)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(root))
	path, err := filepath.Abs(filepath.Join(s.cacheDir, hex.EncodeToString(hash[:])+".generations.json"))
	if err != nil {
		return "", err
	}
	resolved, err := esbuildExistingPath(path)
	if err != nil {
		return "", err
	}
	if esbuildOutputPath(root, resolved) == nil {
		return "", fmt.Errorf("esbuild ownership storage must be outside static")
	}
	return path, nil
}

func (s *esbuildStore) loadHistory() (esbuildHistory, error) {
	path, err := s.historyPath()
	if err != nil {
		return esbuildHistory{}, err
	}
	root, err := filepath.Abs(s.staticDir)
	if err != nil {
		return esbuildHistory{}, err
	}
	h := esbuildHistory{Version: 1, Root: root, Owners: map[string]*esbuildOwnerHistory{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s.adoptLegacyHistory(h)
	}
	if err != nil {
		return h, err
	}
	if err = json.Unmarshal(data, &h); err != nil {
		return h, fmt.Errorf("esbuild generation history: %w", err)
	}
	if h.Version != 1 || h.Root != root || h.Owners == nil {
		return h, fmt.Errorf("esbuild generation history has invalid identity")
	}
	for owner, history := range h.Owners {
		if history == nil || history.Keep < 0 || esbuildOutputPath(root, owner) != nil {
			return h, fmt.Errorf("invalid esbuild owner %s", owner)
		}
		for _, generation := range history.Generations {
			if !generation.Outputs[generation.Entry].Exists {
				return h, fmt.Errorf("invalid esbuild generation for %s", owner)
			}
			for output, stamp := range generation.Outputs {
				if !stamp.Exists || !filepath.IsAbs(output) || esbuildOutputPath(root, output) != nil {
					return h, fmt.Errorf("invalid owned esbuild output %s", output)
				}
			}
		}
	}
	return h, nil
}

func (s *esbuildStore) saveHistory(h esbuildHistory) error {
	path, err := s.historyPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".hb-esbuild-history-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func (s *esbuildStore) retain(p *PreparedEsbuild, result *esbuildResult) (failure error) {
	defer func() {
		if failure != nil {
			s.assetFailure = failure
		}
	}()
	h, err := s.loadHistory()
	if err != nil {
		return fmt.Errorf("asset_cleanup: %w", err)
	}
	owner := h.Owners[p.spec.Outfile]
	if owner == nil {
		owner = &esbuildOwnerHistory{}
		h.Owners[p.spec.Outfile] = owner
	}
	owner.Keep = p.cacheKeep
	current := esbuildGeneration{Entry: result.entry, Outputs: result.outputs}
	// Reusing a generation makes it current without manufacturing a duplicate.
	var previous []esbuildGeneration
	for _, generation := range owner.Generations {
		if !reflect.DeepEqual(generation, current) {
			previous = append(previous, generation)
		}
	}
	owner.Generations = append([]esbuildGeneration{current}, previous...)
	if err = s.saveHistory(h); err != nil {
		return fmt.Errorf("asset_cleanup: record successful generation: %w", err)
	}
	return s.pruneHistory(h, s.activeOwners)
}

// ReconcileAssets is called only after complete successful owner discovery.
// nil active means enforce retention without declaring any owners orphaned.
func (r *EsbuildRenderer) ReconcileAssets(active map[string]int) error {
	s := r.store
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.lockAssetOperation()
	if err != nil {
		return err
	}
	defer unlock()
	h, err := s.loadHistory()
	if err != nil {
		return err
	}
	if active != nil {
		s.activeOwners = make(map[string]int, len(active))
		for owner, keep := range active {
			s.activeOwners[owner] = keep
		}
	}
	return s.pruneHistory(h, s.activeOwners)
}

// SetAssetProtection lets the runtime protect outputs referenced by requests
// and response caches. The callback must not call back into this renderer.
func (r *EsbuildRenderer) SetAssetProtection(protect func(string) bool) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.protectOutput = protect
}

func (s *esbuildStore) pruneHistory(h esbuildHistory, active map[string]int) error {
	retained := map[string]bool{}
	kept := map[string][]esbuildGeneration{}
	for name, owner := range h.Owners {
		limit := owner.Keep + 1
		if active != nil {
			keep, exists := active[name]
			if !exists {
				limit = 0
			} else {
				owner.Keep = keep
				limit = keep + 1
			}
		}
		for i, generation := range owner.Generations {
			protect := i < limit
			for path := range generation.Outputs {
				if s.protectOutput != nil && s.protectOutput(path) {
					protect = true
				}
			}
			if protect {
				kept[name] = append(kept[name], generation)
				for path := range generation.Outputs {
					retained[path] = true
				}
			}
		}
	}
	names := make([]string, 0, len(h.Owners))
	for name := range h.Owners {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, generation := range h.Owners[name].Generations {
			for path, stamp := range generation.Outputs {
				if retained[path] {
					continue
				}
				if err := esbuildOutputPath(h.Root, path); err != nil {
					return fmt.Errorf("asset_cleanup: %w", err)
				}
				info, err := os.Lstat(path)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return fmt.Errorf("asset_cleanup: %w", err)
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("asset_cleanup: owned output is no longer a regular file: %s", path)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("asset_cleanup: %w", err)
				}
				if sha256.Sum256(data) != stamp.Hash {
					logging.GetLogger().Warnw("Preserving externally modified esbuild output; releasing ownership", "path", path)
					continue
				}
				if err = os.Remove(path); err != nil {
					return fmt.Errorf("asset_cleanup: remove %s: %w", path, err)
				}
				delete(s.generated, path)
			}
		}
		if len(kept[name]) == 0 {
			delete(h.Owners, name)
		} else {
			h.Owners[name].Generations = kept[name]
		}
	}
	if err := s.saveHistory(h); err != nil {
		return fmt.Errorf("asset_cleanup: %w", err)
	}
	return nil
}

func (s *esbuildStore) lockAssetOperation() (func(), error) {
	if s.sessionUnlock != nil {
		return func() {}, nil
	}
	path, err := s.historyPath()
	if err != nil {
		return nil, err
	}
	return lockEsbuildFile(path + ".lock")
}

// BeginAssetSession excludes another runtime/export using the same output root.
// File locks are released by the OS on process exit as well as by CloseAssets.
func (r *EsbuildRenderer) BeginAssetSession() error {
	s := r.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionUnlock != nil {
		return nil
	}
	path, err := s.historyPath()
	if err != nil {
		return err
	}
	s.sessionUnlock, err = lockEsbuildFile(path + ".lock")
	return err
}
func (r *EsbuildRenderer) CloseAssets() {
	s := r.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessionUnlock != nil {
		s.sessionUnlock()
		s.sessionUnlock = nil
	}
}

// Existing native manifests provide evidence for adoption. No filename pattern
// is used to infer that a public asset belongs to esbuild.
func (s *esbuildStore) adoptLegacyHistory(h esbuildHistory) (esbuildHistory, error) {
	entries, err := os.ReadDir(s.cacheDir)
	if os.IsNotExist(err) {
		return h, nil
	}
	if err != nil {
		return h, err
	}
	type previous struct {
		manifest esbuildManifest
		modified int64
	}
	var candidates []previous
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.cacheDir, entry.Name()))
		if err != nil {
			return h, err
		}
		var m esbuildManifest
		if json.Unmarshal(data, &m) != nil || m.Version != esbuildManifestVersion || m.Spec.StaticDir != h.Root || !m.Outputs[m.Entry].Exists {
			continue
		}
		valid := esbuildOutputPath(h.Root, m.Spec.Outfile) == nil
		for path, stamp := range m.Outputs {
			if !stamp.Exists || esbuildOutputPath(h.Root, path) != nil {
				valid = false
			}
		}
		if !valid {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return h, err
		}
		candidates = append(candidates, previous{m, info.ModTime().UnixNano()})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].modified > candidates[j].modified })
	for _, candidate := range candidates {
		m := candidate.manifest
		owner := h.Owners[m.Spec.Outfile]
		if owner == nil {
			owner = &esbuildOwnerHistory{}
			h.Owners[m.Spec.Outfile] = owner
		}
		owner.Generations = append(owner.Generations, esbuildGeneration{Entry: m.Entry, Outputs: m.Outputs})
	}
	return h, nil
}

// AssetFailure reports an ownership/retention failure from this loaded
// configuration, allowing the CLI to classify a failed snapshot accurately.
func (r *EsbuildRenderer) AssetFailure() error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return r.store.assetFailure
}
