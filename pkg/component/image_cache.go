package component

import (
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
	cache, err = filepath.Abs(cache)
	if err != nil {
		return nil, err
	}
	resolved, err := esbuildExistingPath(cache)
	if err != nil {
		return nil, err
	}
	if esbuildOutputPath(root, resolved) == nil {
		return nil, fmt.Errorf("image ownership storage must be outside generated images")
	}
	hash := sha256.Sum256([]byte(root))
	s := &imageStore{root: root, historyPath: filepath.Join(cache, hex.EncodeToString(hash[:])+".json"), generated: map[string]bool{}, protect: p.protectOutput, now: time.Now}
	p.stores[root] = s
	return s, nil
}
func (s *imageStore) lock() (func(), error) {
	if s.unlock != nil {
		return func() {}, nil
	}
	return lockAssetFile(s.historyPath+".lock", "image")
}
func (s *imageStore) load() error {
	if s.loaded {
		return nil
	}
	h := imageHistory{Version: 1, Root: s.root, Owners: map[string]*imageOwnerRecord{}}
	data, err := os.ReadFile(s.historyPath)
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
	s.loaded = true
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
	return writeAssetHistory(s.historyPath, data, len(s.history.Owners) == 0, ".hb-image-history-*")
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
func (s *imageStore) updateGroup(group string, members map[string]imagePolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	if err := s.load(); err != nil {
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
		files, err := os.ReadDir(multiple.Directory)
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
	if err := s.load(); err != nil {
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
