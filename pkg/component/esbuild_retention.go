package component

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// Generation records use stable private ownership storage. They are
// independent of build-manifest storage and cache reuse: even cache:false
// outputs have an owner/history.
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
	if s.ownershipDirErr != nil {
		return "", s.ownershipDirErr
	}
	return s.historyPathIn(s.ownershipDir)
}

func (s *esbuildStore) legacyHistoryPath() (string, error) {
	if s.cacheDirErr != nil {
		return "", s.cacheDirErr
	}
	return s.historyPathIn(s.cacheDir)
}

func (s *esbuildStore) historyPathIn(directory string) (string, error) {
	if directory == "" {
		return "", fmt.Errorf("esbuild asset ownership requires a private directory")
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
	path, err := filepath.Abs(filepath.Join(directory, hex.EncodeToString(hash[:])+".generations.json"))
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
	if os.IsNotExist(err) && !s.freshOutput {
		legacy, pathErr := s.legacyHistoryPath()
		if pathErr != nil {
			return h, pathErr
		}
		if legacy != path {
			data, err = os.ReadFile(legacy)
		}
	}
	if os.IsNotExist(err) {
		if s.freshOutput {
			return h, nil
		}
		return s.adoptLegacyHistory(h)
	}
	if err != nil {
		return h, err
	}
	if err = json.Unmarshal(data, &h); err != nil {
		return h, fmt.Errorf("esbuild generation history: %w", err)
	}
	if err := normalizeEsbuildHistory(&h, root); err != nil {
		return h, err
	}
	return h, nil
}

// Historic records may use a symlink spelling such as macOS /var. Canonicalize
// validated owned paths so copy validation and new publication agree on one
// physical output root without abandoning existing ownership evidence.
func normalizeEsbuildHistory(h *esbuildHistory, root string) error {
	if h.Version != 1 || h.Owners == nil || !filepath.IsAbs(h.Root) {
		return fmt.Errorf("esbuild generation history has invalid identity")
	}
	storedRoot, err := esbuildExistingPath(h.Root)
	if err != nil || storedRoot != root {
		return fmt.Errorf("esbuild generation history has invalid identity")
	}
	owners := make(map[string]*esbuildOwnerHistory, len(h.Owners))
	for owner, history := range h.Owners {
		canonicalOwner, err := canonicalEsbuildOwnedPath(h.Root, root, owner)
		if history == nil || history.Keep < 0 || err != nil {
			return fmt.Errorf("invalid esbuild owner %s", owner)
		}
		for i, generation := range history.Generations {
			if !generation.Outputs[generation.Entry].Exists {
				return fmt.Errorf("invalid esbuild generation for %s", owner)
			}
			entry, err := canonicalEsbuildOwnedPath(h.Root, root, generation.Entry)
			if err != nil {
				return err
			}
			outputs := make(map[string]esbuildFingerprint, len(generation.Outputs))
			for output, stamp := range generation.Outputs {
				canonical, err := canonicalEsbuildOwnedPath(h.Root, root, output)
				if !stamp.Exists || err != nil {
					return fmt.Errorf("invalid owned esbuild output %s", output)
				}
				if previous, exists := outputs[canonical]; exists && previous != stamp {
					return fmt.Errorf("conflicting esbuild output aliases: %s", output)
				}
				outputs[canonical] = stamp
			}
			history.Generations[i] = esbuildGeneration{Entry: entry, Outputs: outputs}
		}
		if previous, exists := owners[canonicalOwner]; exists && !reflect.DeepEqual(previous, history) {
			return fmt.Errorf("conflicting esbuild owner aliases: %s", owner)
		}
		owners[canonicalOwner] = history
	}
	h.Root, h.Owners = root, owners
	return nil
}

func canonicalEsbuildOwnedPath(storedRoot, root, path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", fmt.Errorf("invalid owned esbuild path %s", path)
	}
	// Normalize the root alias only. Preserve the owned leaf so cleanup's Lstat
	// still rejects a replacement symlink instead of claiming its target.
	canonical, ok := esbuildMapRoot(path, storedRoot, root)
	if !ok {
		canonical, ok = esbuildMapRoot(path, root, root)
	}
	if !ok {
		return "", fmt.Errorf("owned esbuild path is outside its root: %s", path)
	}
	if err := esbuildOutputPath(root, canonical); err != nil {
		return "", err
	}
	return canonical, nil
}

// migrateLegacyHistory is called only while holding the output-tree write gate.
// The first valid stable record is authoritative; never merge a second history.
func (s *esbuildStore) migrateLegacyHistory() error {
	path, err := s.historyPath()
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	legacy, err := s.legacyHistoryPath()
	if err != nil {
		return err
	}
	if legacy == path {
		return nil
	}
	if info, err := os.Lstat(legacy); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("esbuild ownership migration requires a regular history file")
	}
	h, err := s.loadHistory()
	if err != nil {
		return err
	}
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	// An explicit empty record is authoritative too: persist it before removal
	// so old build manifests cannot later re-adopt discarded ownership.
	if err := writeAssetHistory(path, data, false, ".hb-esbuild-history-*"); err != nil {
		return err
	}
	if err := os.Remove(legacy); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
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
	return writeAssetHistory(path, data, len(h.Owners) == 0, ".hb-esbuild-history-*")
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
	keep := s.configured[p.spec.Outfile].cacheKeep
	if active, ok := s.activeOwners[p.spec.Outfile]; ok {
		keep = active
	}
	if owner != nil && owner.Keep == keep && len(owner.Generations) > 0 &&
		reflect.DeepEqual(owner.Generations[0], esbuildGeneration{Entry: result.entry, Outputs: result.outputs}) {
		// A legacy build manifest can supply this generation before an ownership
		// record exists. Persist that adoption once, including its retention.
		path, pathErr := s.historyPath()
		if pathErr != nil {
			return pathErr
		}
		if _, statErr := os.Stat(path); statErr == nil {
			return nil
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
	}
	if owner == nil {
		owner = &esbuildOwnerHistory{}
		h.Owners[p.spec.Outfile] = owner
	}
	owner.Keep = keep
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

// ScheduleAssetReconciliation records complete owner discovery without deleting
// files. Active API renders may need nested requests before they can finish;
// cleanup retries under the exclusive runtime gate after those requests finish.
func (r *EsbuildRenderer) ScheduleAssetReconciliation(active map[string]int) {
	s := r.store
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setActiveOwners(active)
	s.pendingCleanup.Store(true)
}

// ReconcileAssets is called only after complete successful owner discovery.
// nil active means enforce retention without declaring any owners orphaned.
func (r *EsbuildRenderer) ReconcileAssets(active map[string]int) error {
	s := r.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if active != nil {
		s.setActiveOwners(active)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	unlock, err := s.lockAssetOperation(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.migrateLegacyHistory(); err != nil {
		return err
	}
	h, err := s.loadHistory()
	if err != nil {
		return err
	}
	if active == nil && s.activeOwners == nil {
		for output, configured := range s.configured {
			if owner := h.Owners[output]; owner != nil {
				owner.Keep = configured.cacheKeep
			}
		}
	}
	return s.pruneHistory(h, s.activeOwners)
}

func (s *esbuildStore) setActiveOwners(active map[string]int) {
	s.activeOwners = make(map[string]int, len(active))
	for owner, keep := range active {
		s.activeOwners[owner] = keep
	}
	for output, configured := range s.configured {
		if keep, exists := active[output]; !exists {
			delete(s.results, configured.key)
			delete(s.configured, output)
		} else if configured.cacheKeep != keep {
			policy := *configured
			policy.cacheKeep = keep
			s.configured[output] = &policy
		}
	}
}

// HasPendingAssetCleanup reports obsolete generations whose removal was
// deferred while their outputs were protected. Ordinary requests do no I/O.
func (r *EsbuildRenderer) HasPendingAssetCleanup() bool {
	return r.store.pendingCleanup.Load()
}

// SetAssetProtection lets the runtime protect outputs referenced by requests
// and response caches. The callback must not call back into this renderer.
func (r *EsbuildRenderer) SetAssetProtection(protect func(string) bool) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.protectOutput = protect
}

func (s *esbuildStore) pruneHistory(h esbuildHistory, active map[string]int) error {
	// Leave maintenance pending on failure so the runtime can retry safely.
	s.pendingCleanup.Store(true)
	pending := false
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
			if protect && i >= limit {
				pending = true
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
				s.invalidateChangedOutputs(map[string]esbuildFingerprint{path: {}})
				// Keep the known generated path for this session: the watcher may
				// receive its removal event after cleanup releases the store lock.
				// Durable ownership still comes exclusively from history records.
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
	s.pendingCleanup.Store(pending)
	return nil
}

func (s *esbuildStore) lockAssetOperation(ctx context.Context) (func(), error) {
	releaseSession := func() {}
	if s.sessionUnlock == nil {
		var err error
		releaseSession, err = LockAssetSession(s.staticDir, "esbuild")
		if err != nil {
			return nil, err
		}
	}
	releaseTree, err := LockAssetTree(ctx, s.staticDir, false)
	if err != nil {
		releaseSession()
		return nil, err
	}
	return func() { releaseTree(); releaseSession() }, nil
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
	var err error
	s.sessionUnlock, err = LockAssetSession(s.staticDir, "esbuild")
	return err
}

// ResetAssetsForFreshExport discards only this output root's old ownership.
// The caller holds its destination reservation and has not yet copied assets
// or prepared routes. Old private manifests must not adopt those copied files.
func (r *EsbuildRenderer) ResetAssetsForFreshExport(ctx context.Context) error {
	s := r.store
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.prepared || s.sessionUnlock == nil {
		return fmt.Errorf("esbuild fresh export requires an unprepared asset session")
	}
	unlock, err := s.lockAssetOperation(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	path, err := s.historyPath()
	if err != nil {
		return err
	}
	legacy, err := s.legacyHistoryPath()
	if err != nil {
		return err
	}
	for _, owned := range []string{path, legacy} {
		if info, err := os.Lstat(owned); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return err
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("esbuild ownership reset requires a regular history file: %s", owned)
		}
	}
	for _, owned := range []string{path, legacy} {
		if err := os.Remove(owned); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	s.freshOutput, s.reuseDisk = true, false
	s.results = make(map[string]*esbuildResult)
	s.owners = make(map[string]string)
	s.generated = make(map[string]bool)
	s.assetFailure = nil
	s.pendingCleanup.Store(false)
	return nil
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
		if json.Unmarshal(data, &m) != nil || m.Version != esbuildManifestVersion || !m.Outputs[m.Entry].Exists {
			continue
		}
		legacy := esbuildHistory{Version: 1, Root: m.Spec.StaticDir, Owners: map[string]*esbuildOwnerHistory{
			m.Spec.Outfile: {Generations: []esbuildGeneration{{Entry: m.Entry, Outputs: m.Outputs}}},
		}}
		if normalizeEsbuildHistory(&legacy, h.Root) != nil {
			continue
		}
		for output, owner := range legacy.Owners {
			m.Spec.StaticDir, m.Spec.Outfile = h.Root, output
			m.Entry, m.Outputs = owner.Generations[0].Entry, owner.Generations[0].Outputs
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
