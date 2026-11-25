package component

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

type imagePolicy struct {
	Keep  int           `json:"keep"`
	Grace time.Duration `json:"grace"`
}

// ImageAssetPolicy is the retention policy discovered from effective configs.
type ImageAssetPolicy = imagePolicy
type imageGeneration struct {
	Name      string   `json:"name"`
	Hash      [32]byte `json:"hash"`
	RetiredAt int64    `json:"retired_at,omitempty"`
}
type imageOwnerRecord struct {
	Group       string            `json:"group"`
	Policy      imagePolicy       `json:"policy"`
	Generations []imageGeneration `json:"generations"`
}
type imageHistory struct {
	Version int                          `json:"version"`
	Root    string                       `json:"root"`
	Owners  map[string]*imageOwnerRecord `json:"owners"`
}
type imageStore struct {
	mu                sync.Mutex
	root, historyPath string
	legacyHistoryPath string
	historySource     string
	treeRoot          string
	history           imageHistory
	loaded            bool
	active            map[string]imagePolicy
	generated         map[string]bool
	protect           func(string) bool
	pending           atomic.Bool
	unlock            func()
	now               func() time.Time
}

func imageRetentionPolicy(keep int, grace string) (imagePolicy, error) {
	if keep < 0 {
		return imagePolicy{}, fmt.Errorf("image cache_keep must be nonnegative")
	}
	d := time.Hour
	var err error
	if grace != "" {
		d, err = time.ParseDuration(grace)
	}
	if err != nil || d < 0 {
		return imagePolicy{}, fmt.Errorf("image cache_grace must be a nonnegative duration")
	}
	return imagePolicy{keep, d}, nil
}
func imageOwner(src string, width, height, quality int) (string, error) {
	path, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}
	if quality == 0 {
		quality = 90
	}
	data, _ := json.Marshal([]interface{}{filepath.Clean(path), width, height, quality})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
func imageGroup(src string, width, height, quality int, multiple bool) (string, error) {
	id, err := imageOwner(src, width, height, quality)
	return fmt.Sprintf("%t:%s", multiple, id), err
}

// NewImageProcessor shares generation ownership across IMAGE and IMAGES.
// Zero-value processors remain usable for component consumers.
func NewImageProcessor() *ImageProcessor { return &ImageProcessor{} }

// SetOutputRoot binds generated image destinations to an operation's output
// tree. Source paths remain authored paths. Configure it before acquiring any
// image store or session so publication, ownership and cleanup share one root.
func (p *ImageProcessor) SetOutputRoot(sourceStatic, outputStatic string) error {
	if strings.TrimSpace(sourceStatic) == "" || strings.TrimSpace(outputStatic) == "" {
		return fmt.Errorf("image source and output static roots are required")
	}
	roots := []string{sourceStatic, outputStatic}
	authoredStatic := ""
	for i, root := range roots {
		absolute, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		if i == 0 {
			authoredStatic = absolute
		}
		roots[i], err = esbuildExistingPath(absolute)
		if err != nil {
			return err
		}
	}
	sourceStatic, outputStatic = roots[0], roots[1]
	if sourceStatic != outputStatic && (esbuildOutputPath(sourceStatic, outputStatic) == nil || esbuildOutputPath(outputStatic, sourceStatic) == nil) {
		return fmt.Errorf("image source and output static roots must not overlap")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sourceStatic == sourceStatic && p.outputStatic == outputStatic {
		p.authoredStatic = authoredStatic
		return nil
	}
	if len(p.stores) != 0 {
		return fmt.Errorf("image output root must be configured before asset preparation")
	}
	p.sourceStatic, p.authoredStatic, p.outputStatic = sourceStatic, authoredStatic, outputStatic
	return nil
}

// Redirect reads of exported static inputs without changing authored identities
// used by image groups, owners, filenames, or public URLs.
func (p *ImageProcessor) sourcePath(path string) (string, error) {
	p.mu.Lock()
	source, authored, output := p.sourceStatic, p.authoredStatic, p.outputStatic
	p.mu.Unlock()
	if output == "" || output == source {
		return path, nil
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for _, root := range []string{authored, source} {
		if mapped, ok := esbuildMapRoot(absolute, root, output); ok {
			return mapped, nil
		}
	}
	resolved, err := esbuildExistingPath(absolute)
	if err != nil {
		return "", err
	}
	if mapped, ok := esbuildMapRoot(resolved, source, output); ok {
		return mapped, nil
	}
	return path, nil
}

func (p *ImageProcessor) storeFor(dest string) (*imageStore, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	root, err := filepath.Abs(dest)
	if err != nil {
		return nil, err
	}
	root, err = esbuildExistingPath(root)
	if err != nil {
		return nil, err
	}
	if p.outputStatic != "" {
		if esbuildOutputPath(p.sourceStatic, root) == nil {
			relative, err := filepath.Rel(p.sourceStatic, root)
			if err != nil {
				return nil, err
			}
			root = filepath.Join(p.outputStatic, relative)
		}
		if err := esbuildOutputPath(p.outputStatic, root); err != nil {
			return nil, fmt.Errorf("image destination outside configured output: %w", err)
		}
		root, err = esbuildExistingPath(root)
		if err != nil {
			return nil, err
		}
	}
	if p.stores == nil {
		p.stores = map[string]*imageStore{}
	}
	if s := p.stores[root]; s != nil {
		return s, nil
	}
	cache := p.cacheDir
	if cache == "" {
		cache, err = os.UserCacheDir()
		if err != nil {
			return nil, err
		}
		cache = filepath.Join(cache, "hyperbricks", "images")
	}
	ownership := p.ownershipDir
	if ownership == "" {
		ownership, err = assetOwnershipDirectory("images")
		if err != nil {
			return nil, err
		}
	}
	paths := []*string{&cache, &ownership}
	for _, path := range paths {
		*path, err = filepath.Abs(*path)
		if err != nil {
			return nil, err
		}
		resolved, err := esbuildExistingPath(*path)
		if err != nil {
			return nil, err
		}
		if root == resolved || esbuildOutputPath(root, resolved) == nil {
			return nil, fmt.Errorf("image ownership storage must be outside generated images")
		}
	}
	hash := sha256.Sum256([]byte(root))
	filename := hex.EncodeToString(hash[:]) + ".json"
	treeRoot := p.outputStatic
	if treeRoot == "" {
		treeRoot = filepath.Dir(root)
	}
	s := &imageStore{root: root, treeRoot: treeRoot, historyPath: filepath.Join(ownership, filename), legacyHistoryPath: filepath.Join(cache, filename), generated: map[string]bool{}, protect: p.protectOutput, now: time.Now}
	p.stores[root] = s
	return s, nil
}
func (s *imageStore) lock() (func(), error) {
	if s.unlock != nil {
		return func() {}, nil
	}
	return LockAssetSession(s.root, "image")
}
func (s *imageStore) load() error {
	if s.loaded {
		return nil
	}
	h := imageHistory{Version: 1, Root: s.root, Owners: map[string]*imageOwnerRecord{}}
	source := s.historyPath
	data, err := os.ReadFile(source)
	if os.IsNotExist(err) && s.legacyHistoryPath != s.historyPath {
		source = s.legacyHistoryPath
		data, err = os.ReadFile(source)
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err == nil {
		if err = json.Unmarshal(data, &h); err != nil {
			return fmt.Errorf("image ownership: %w", err)
		}
		if h.Version != 1 || h.Root != s.root || h.Owners == nil {
			return fmt.Errorf("invalid image ownership identity")
		}
		for owner, r := range h.Owners {
			if r == nil || !strings.HasPrefix(owner, r.Group+":") || r.Group == "" || r.Policy.Keep < 0 || r.Policy.Grace < 0 || len(r.Generations) == 0 {
				return fmt.Errorf("invalid image owner")
			}
			for _, g := range r.Generations {
				if !validImageName(g.Name) || g.RetiredAt < 0 {
					return fmt.Errorf("invalid owned image path")
				}
				s.generated[filepath.Join(s.root, g.Name)] = true
			}
		}
	}
	s.history = h
	s.historySource = source
	s.loaded = true
	return nil
}

// Only callers already holding this root's write gate may migrate ownership.
// Copy readers use load, which never creates or changes a history file.
func (s *imageStore) loadForMutation() error {
	if err := s.load(); err != nil {
		return err
	}
	if s.historySource == s.historyPath {
		return nil
	}
	if _, err := os.Lstat(s.historyPath); err == nil {
		// Another operation published canonical ownership after a read-only load.
		s.loaded = false
		return s.load()
	} else if !os.IsNotExist(err) {
		return err
	}
	if info, err := os.Lstat(s.legacyHistoryPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("image ownership migration requires a regular history file")
	}
	data, err := json.Marshal(s.history)
	if err != nil {
		return err
	}
	if err := writeAssetHistory(s.historyPath, data, false, ".hb-image-history-*"); err != nil {
		return err
	}
	if err := os.Remove(s.legacyHistoryPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	s.historySource = s.historyPath
	return nil
}
func validImageName(name string) bool {
	return name != "" && filepath.Base(name) == name && !strings.ContainsAny(name, "/\\\x00") && name != "." && name != ".." && SupportedExtensions[strings.ToLower(filepath.Ext(name))]
}
func (s *imageStore) safeOutput(name string) error {
	if !validImageName(name) {
		return fmt.Errorf("invalid image output")
	}
	path := filepath.Join(s.root, name)
	if err := esbuildOutputPath(s.root, path); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("image output is not a regular file")
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *imageStore) openRoot() (*os.Root, error) {
	resolved, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return nil, err
	}
	if resolved != s.root {
		return nil, fmt.Errorf("image output root changed through a symlink")
	}
	return os.OpenRoot(s.root)
}
func (s *imageStore) valid(owner, name string) bool {
	r := s.history.Owners[owner]
	if r == nil {
		return false
	}
	if s.safeOutput(name) != nil {
		return false
	}
	for _, g := range r.Generations {
		if g.Name == name {
			root, err := s.openRoot()
			if err != nil {
				return false
			}
			defer root.Close()
			data, err := root.ReadFile(name)
			return err == nil && sha256.Sum256(data) == g.Hash
		}
	}
	return false
}
func (s *imageStore) save() error {
	data, err := json.Marshal(s.history)
	if err != nil {
		return err
	}
	if err := writeAssetHistory(s.historyPath, data, len(s.history.Owners) == 0, ".hb-image-history-*"); err != nil {
		return err
	}
	s.historySource = s.historyPath
	return nil
}
func (s *imageStore) retain(owner, group, name string, policy imagePolicy) error {
	if active, ok := s.active[owner]; ok {
		policy.Keep = max(policy.Keep, active.Keep)
		policy.Grace = max(policy.Grace, active.Grace)
	}
	if s.valid(owner, name) {
		r := s.history.Owners[owner]
		if r.Policy == policy && r.Generations[0].Name == name && r.Generations[0].RetiredAt == 0 {
			return nil
		}
	}
	root, err := s.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()
	data, err := root.ReadFile(name)
	if err != nil {
		return err
	}
	current := imageGeneration{Name: name, Hash: sha256.Sum256(data)}
	previous := []imageGeneration{}
	if r := s.history.Owners[owner]; r != nil {
		for _, g := range r.Generations {
			if g.Name != name {
				if g.RetiredAt == 0 {
					g.RetiredAt = s.now().UnixNano()
				}
				previous = append(previous, g)
			}
		}
	}
	s.history.Owners[owner] = &imageOwnerRecord{Group: group, Policy: policy, Generations: append([]imageGeneration{current}, previous...)}
	if s.active != nil {
		s.active[owner] = policy
	}
	s.generated[filepath.Join(s.root, name)] = true
	if err := s.save(); err != nil {
		s.loaded = false
		return fmt.Errorf("image ownership publication: %w", err)
	}
	return s.prune()
}
func (s *imageStore) prune() (failure error) {
	defer func() {
		if failure != nil {
			s.loaded = false
		}
	}()
	s.pending.Store(true)
	// Compute references across all owners before unlinking a shared output.
	retained := map[string]bool{}
	keep := map[string][]imageGeneration{}
	pending := false
	now := s.now()
	for owner, r := range s.history.Owners {
		retainCurrent := true
		keepCount := r.Policy.Keep
		if s.active != nil {
			if policy, ok := s.active[owner]; ok {
				r.Policy = policy
				keepCount = policy.Keep
			} else {
				retainCurrent = false
			}
		}
		for i, g := range r.Generations {
			protected := retainCurrent && i <= keepCount
			if !protected && g.RetiredAt == 0 {
				g.RetiredAt = now.UnixNano()
			}
			if !protected && (now.Sub(time.Unix(0, g.RetiredAt)) < r.Policy.Grace || s.protect != nil && s.protect(filepath.Join(s.root, g.Name))) {
				protected = true
				pending = true
			}
			if protected {
				retained[g.Name] = true
				keep[owner] = append(keep[owner], g)
			}
		}
	}
	for owner, r := range s.history.Owners {
		for _, g := range r.Generations {
			if retained[g.Name] {
				continue
			}
			if err := s.safeOutput(g.Name); err != nil {
				return err
			}
			path := filepath.Join(s.root, g.Name)
			root, err := s.openRoot()
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			data, err := root.ReadFile(g.Name)
			if os.IsNotExist(err) {
				root.Close()
				continue
			}
			if err != nil {
				root.Close()
				return err
			}
			if sha256.Sum256(data) != g.Hash {
				logging.GetLogger().Warnw("Preserving externally modified image; releasing ownership", "path", path)
				root.Close()
				continue
			}
			err = root.Remove(g.Name)
			root.Close()
			if err != nil {
				return err
			}
		}
		if len(keep[owner]) == 0 {
			delete(s.history.Owners, owner)
		} else {
			r.Generations = keep[owner]
		}
	}
	if err := s.save(); err != nil {
		s.loaded = false
		return err
	}
	s.pending.Store(pending)
	return nil
}
func (s *imageStore) updateGroup(ctx context.Context, group string, members map[string]imagePolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	unlockTree, err := LockAssetTree(ctx, s.treeRoot, false)
	if err != nil {
		return err
	}
	defer unlockTree()
	if err := s.loadForMutation(); err != nil {
		return err
	}
	if s.active == nil {
		s.active = map[string]imagePolicy{}
		for id, r := range s.history.Owners {
			s.active[id] = r.Policy
		}
	}
	unchanged := true
	for id, r := range s.history.Owners {
		if r.Group == group {
			previous, exists := s.active[id]
			next, remains := members[id]
			if exists != remains || previous != next {
				unchanged = false
			}
		}
	}
	for id, policy := range members {
		if previous, ok := s.active[id]; !ok || previous != policy {
			unchanged = false
		}
	}
	if unchanged {
		return nil
	}
	for id, r := range s.history.Owners {
		if r.Group == group {
			delete(s.active, id)
		}
	}
	for id, policy := range members {
		s.active[id] = policy
	}
	return s.prune()
}

// DiscoverImages returns effective owners without decoding or generating images.
// A complete successful discovery is required before declaring owners orphaned.
func (p *ImageProcessor) DiscoverImages(single *SingleImageConfig, multiple *MultipleImagesConfig) (map[string]imagePolicy, error) {
	active := map[string]imagePolicy{}
	add := func(src, group string, width, height, quality, keep int, grace string) error {
		policy, err := imageRetentionPolicy(keep, grace)
		if err != nil {
			return err
		}
		if width < 0 || height < 0 || quality < 0 || quality > 100 {
			return fmt.Errorf("invalid image dimensions or quality")
		}
		id, err := imageOwner(src, width, height, quality)
		if err == nil {
			active[group+":"+id] = policy
		}
		return err
	}
	if single != nil {
		if single.Src == "" {
			return nil, fmt.Errorf("image src is required")
		}
		g, err := imageGroup(single.Src, single.Width, single.Height, single.Quality, false)
		if err != nil {
			return nil, err
		}
		if err := add(single.Src, g, single.Width, single.Height, single.Quality, single.CacheKeep, single.CacheGrace); err != nil {
			return nil, err
		}
	}
	if multiple != nil {
		if multiple.Width < 0 || multiple.Height < 0 || multiple.Quality < 0 || multiple.Quality > 100 {
			return nil, fmt.Errorf("invalid image dimensions or quality")
		}
		directory, err := p.sourcePath(multiple.Directory)
		if err != nil {
			return nil, err
		}
		files, err := os.ReadDir(directory)
		if err != nil {
			return nil, err
		}
		g, err := imageGroup(multiple.Directory, multiple.Width, multiple.Height, multiple.Quality, true)
		if err != nil {
			return nil, err
		}
		if _, err := imageRetentionPolicy(multiple.CacheKeep, multiple.CacheGrace); err != nil {
			return nil, err
		}
		for _, f := range files {
			if !f.IsDir() && SupportedExtensions[strings.ToLower(filepath.Ext(f.Name()))] {
				if err := add(filepath.Join(multiple.Directory, f.Name()), g, multiple.Width, multiple.Height, multiple.Quality, multiple.CacheKeep, multiple.CacheGrace); err != nil {
					return nil, err
				}
			}
		}
	}
	return active, nil
}

// ScheduleAssetReconciliation records effective owners without deleting output
// while active API renders may still require nested local asset requests.
func (p *ImageProcessor) ScheduleAssetReconciliation(dest string, active map[string]ImageAssetPolicy) error {
	s, err := p.storeFor(dest)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = make(map[string]imagePolicy, len(active))
	for owner, policy := range active {
		s.active[owner] = policy
	}
	s.pending.Store(true)
	return nil
}

func (p *ImageProcessor) ReconcileAssets(dest string, active map[string]imagePolicy) error {
	s, err := p.storeFor(dest)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	unlockTree, err := LockAssetTree(context.Background(), s.treeRoot, false)
	if err != nil {
		return err
	}
	defer unlockTree()
	if err := s.loadForMutation(); err != nil {
		return err
	}
	if active != nil {
		s.active = make(map[string]imagePolicy, len(active))
		for owner, policy := range active {
			s.active[owner] = policy
		}
	}
	return s.prune()
}

// ResetAssetsForFreshExport discards this destination's previous ownership
// before a new export adopts copied input files. It never removes image files
// or releases the export's already acquired session.
func (p *ImageProcessor) ResetAssetsForFreshExport(ctx context.Context, dest string) error {
	p.mu.Lock()
	export := p.sourceStatic != "" && p.outputStatic != p.sourceStatic
	p.mu.Unlock()
	if !export {
		return fmt.Errorf("image ownership reset requires a separate export output root")
	}
	s, err := p.storeFor(dest)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	unlockTree, err := LockAssetTree(ctx, s.treeRoot, false)
	if err != nil {
		return err
	}
	defer unlockTree()
	for _, path := range []string{s.historyPath, s.legacyHistoryPath} {
		if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
			return fmt.Errorf("image ownership reset requires a regular history file")
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	for _, path := range []string{s.historyPath, s.legacyHistoryPath} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	s.history = imageHistory{Version: 1, Root: s.root, Owners: map[string]*imageOwnerRecord{}}
	s.historySource = s.historyPath
	s.loaded = true
	s.active = nil
	s.generated = map[string]bool{}
	s.pending.Store(false)
	return nil
}
func (p *ImageProcessor) SetAssetProtection(protect func(string) bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.protectOutput = protect
	for _, s := range p.stores {
		s.mu.Lock()
		s.protect = protect
		s.mu.Unlock()
	}
}
func (p *ImageProcessor) HasPendingAssetCleanup() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.stores {
		if s.pending.Load() {
			return true
		}
	}
	return false
}
func (p *ImageProcessor) OwnsOutput(path string) bool {
	path, _ = filepath.Abs(path)
	if resolved, err := esbuildExistingPath(path); err == nil {
		path = resolved
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.stores {
		s.mu.Lock()
		owned := s.generated[path]
		s.mu.Unlock()
		if owned {
			return true
		}
	}
	return false
}
func (p *ImageProcessor) BeginAssetSession(dest string) error {
	s, err := p.storeFor(dest)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unlock != nil {
		return nil
	}
	s.unlock, err = s.lock()
	return err
}
func (p *ImageProcessor) CloseAssets() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.stores {
		s.mu.Lock()
		if s.unlock != nil {
			s.unlock()
			s.unlock = nil
		}
		s.mu.Unlock()
	}
}
