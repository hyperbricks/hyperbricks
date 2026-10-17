package component

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

type esbuildFingerprint struct {
	Exists bool
	Hash   [32]byte
}

type esbuildResult struct {
	inputs  map[string]esbuildFingerprint
	outputs map[string]esbuildFingerprint
	entry   string
}

// OwnsOutput identifies paths generated in this session, including removed
// generations whose queued filesystem events must not trigger source reloads.
// It is not the durable ownership evidence used to delete files.
func (r *EsbuildRenderer) OwnsOutput(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	// Watch events may use a root alias such as macOS /var while publication
	// records the canonical /private/var path. Removed generations still resolve
	// through their nearest existing ancestor.
	abs, err = esbuildExistingPath(abs)
	if err != nil {
		return false
	}
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	return r.store.generated[abs]
}

func (s *esbuildStore) build(ctx context.Context, p *PreparedEsbuild) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// Serializing this module's asset builds also protects shared auxiliary files.
	// A waiting cached request checks the result again after the first build ends.
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	current := s.configured[p.spec.Outfile]
	if current == nil || current.key != p.key {
		return "", fmt.Errorf("esbuild configuration for %s was replaced or removed; render the current route configuration", p.spec.Outfile)
	}
	if p.cache {
		if result := s.results[p.key]; result != nil {
			if p.debug {
				logging.GetLogger().Infow("esbuild cache hit", "entry", p.spec.Entry)
			}
			return result.entry, nil
		}
	}
	unlock, err := s.lockAssetOperation(ctx)
	if err != nil {
		s.assetFailure = err
		return "", err
	}
	defer unlock()
	if err := s.migrateLegacyHistory(); err != nil {
		s.assetFailure = err
		return "", err
	}
	// A shared build may outlive its initiating request; its independent deadline
	// bounds compiler work without cancelling other waiting requests.
	buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	engine := esbuildEngineIdentity()
	if p.cache && s.reuseDisk {
		if result := s.restore(p, engine); result != nil {
			if err := s.claimOutputs(p, result.outputs); err != nil {
				return "", err
			}
			if p.debug {
				logging.GetLogger().Infow("esbuild persistent cache hit", "entry", p.spec.Entry)
			}
			if err := s.retain(p, result); err != nil {
				return "", err
			}
			s.results[p.key] = result
			return result.entry, nil
		}
	}
	if p.debug {
		logging.GetLogger().Infow("esbuild build", "options", p.spec, "cache", p.cache, "engine", engine)
	}
	s.buildCount++
	started := time.Now()
	outputs, metafile, err := p.spec.compile(buildCtx)
	if err != nil {
		return "", err
	}
	entry, err := p.spec.entryOutput(metafile, outputs)
	if err != nil {
		return "", err
	}
	outputs, entry, err = p.spec.lowercaseEntryHashes(outputs, metafile, entry)
	if err != nil {
		return "", err
	}
	paths, err := p.spec.dependencies(metafile)
	if err != nil {
		return "", err
	}
	for path := range outputs {
		if paths[path] {
			return "", fmt.Errorf("esbuild output would overwrite source %q", path)
		}
	}
	files, err := esbuildFingerprints(paths)
	if err != nil {
		return "", err
	}
	// Do not cache a potentially mixed build if a dependency changed during it.
	stable := true
	for path := range paths {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(started) {
			stable = false
		}
	}
	result := &esbuildResult{inputs: files, outputs: make(map[string]esbuildFingerprint, len(outputs)), entry: entry}
	for path, data := range outputs {
		result.outputs[path] = esbuildFingerprint{Exists: true, Hash: sha256.Sum256(data)}
	}
	// Publication can replace a file consumed by another cached build. Evict
	// before mutation, including partial publication failures, without polling
	// those dependencies on subsequent requests.
	s.invalidateChangedOutputs(result.outputs)
	if err := s.publish(p, outputs, entry); err != nil {
		return "", err
	}
	if err := s.retain(p, result); err != nil {
		return "", err
	}
	if p.cache && stable {
		s.results[p.key] = result
		s.persist(p, engine, result)
	} else {
		delete(s.results, p.key)
	}
	return entry, nil
}

// invalidateChangedOutputs runs under the store mutex as part of a known
// mutation, never on the warm request path.
func (s *esbuildStore) invalidateChangedOutputs(changed map[string]esbuildFingerprint) {
	for key, result := range s.results {
		for path, next := range changed {
			input, reads := result.inputs[path]
			output, writes := result.outputs[path]
			if (reads && input != next) || (writes && output != next) {
				delete(s.results, key)
				break
			}
		}
	}
}

func (r *esbuildResult) valid() bool {
	for _, files := range []map[string]esbuildFingerprint{r.inputs, r.outputs} {
		paths := make(map[string]bool, len(files))
		for path := range files {
			paths[path] = true
		}
		current, err := esbuildFingerprints(paths)
		if err != nil {
			return false
		}
		for path, previous := range files {
			if current[path] != previous {
				return false
			}
		}
	}
	return true
}

func esbuildFingerprints(paths map[string]bool) (map[string]esbuildFingerprint, error) {
	out := make(map[string]esbuildFingerprint, len(paths))
	for path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			out[path] = esbuildFingerprint{}
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("esbuild dependency %s: %w", path, err)
		}
		out[path] = esbuildFingerprint{Exists: true, Hash: sha256.Sum256(data)}
	}
	return out, nil
}

func (s esbuildSpec) compile(ctx context.Context) (map[string][]byte, string, error) {
	opts, err := s.options()
	if err != nil {
		return nil, "", err
	}
	build, contextErr := api.Context(opts)
	if contextErr != nil {
		return nil, "", esbuildMessages(contextErr.Errors)
	}
	defer build.Dispose()
	stop := context.AfterFunc(ctx, build.Cancel)
	defer stop()
	result := build.Rebuild()
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}
	if len(result.Errors) != 0 {
		return nil, "", esbuildMessages(result.Errors)
	}
	for _, message := range api.FormatMessages(result.Warnings, api.FormatMessagesOptions{Kind: api.WarningMessage, Color: false}) {
		logging.GetLogger().Warnw("esbuild warning", "entry", s.Entry, "message", message)
	}
	outputs := make(map[string][]byte, len(result.OutputFiles))
	for _, output := range result.OutputFiles {
		outputs[output.Path] = output.Contents
	}
	return outputs, result.Metafile, nil
}

func esbuildMessages(messages []api.Message) error {
	return fmt.Errorf("esbuild: %s", strings.Join(api.FormatMessages(messages, api.FormatMessagesOptions{Kind: api.ErrorMessage, Color: false}), "\n"))
}

func (s esbuildSpec) dependencies(metafile string) (map[string]bool, error) {
	var meta struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal([]byte(metafile), &meta); err != nil {
		return nil, fmt.Errorf("esbuild metadata: %w", err)
	}
	entry, err := s.entryInput()
	if err != nil {
		return nil, err
	}
	paths := map[string]bool{entry: true}
	for input := range meta.Inputs {
		if !filepath.IsAbs(input) {
			input = filepath.Join(s.WorkingDir, input)
		}
		paths[filepath.Clean(input)] = true
	}
	// esbuild's input graph excludes nearby resolution/configuration files.
	// Track absence as well, so adding a tsconfig or package.json invalidates reuse.
	inputs := make([]string, 0, len(paths))
	for path := range paths {
		inputs = append(inputs, path)
		if s.SourceStaticDir != "" {
			if authored, ok := esbuildMapRoot(path, s.StaticDir, s.SourceStaticDir); ok {
				inputs = append(inputs, authored)
			}
		}
	}
	for _, input := range inputs {
		for dir := filepath.Dir(input); ; dir = filepath.Dir(dir) {
			for _, name := range []string{"package.json", "tsconfig.json", "jsconfig.json"} {
				paths[filepath.Join(dir, name)] = true
			}
			if dir == filepath.Dir(dir) {
				break
			}
		}
	}
	return paths, nil
}

func (s *esbuildStore) publish(p *PreparedEsbuild, outputs map[string][]byte, entry string) error {
	if _, ok := outputs[entry]; !ok {
		return fmt.Errorf("esbuild produced no requested output %s", entry)
	}
	paths := make([]string, 0, len(outputs))
	for path := range outputs {
		if err := esbuildOutputPath(p.spec.StaticDir, path); err != nil {
			return err
		}
		if owner := s.owners[path]; owner != "" && owner != p.spec.Outfile {
			return fmt.Errorf("esbuild output %s is already owned by %s", path, owner)
		}
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool {
		if paths[i] == entry {
			return false
		}
		if paths[j] == entry {
			return true
		}
		return paths[i] < paths[j]
	})
	staged := make(map[string]string)
	defer func() {
		for _, temp := range staged {
			_ = os.Remove(temp)
		}
	}()
	// Stage every file before replacing any output. Auxiliary files are published
	// before the entry; each rename is atomic, not the whole multi-file generation.
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		file, err := os.CreateTemp(filepath.Dir(path), ".hb-esbuild-*")
		if err != nil {
			return err
		}
		staged[path] = file.Name()
		if err := file.Chmod(0644); err != nil {
			file.Close()
			return err
		}
		if _, err := file.Write(outputs[path]); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	for _, path := range paths {
		actual, displaced, err := esbuildDisplaceCaseAlias(path)
		if err != nil {
			return err
		}
		if err := os.Rename(staged[path], path); err != nil {
			if displaced != "" {
				_ = os.Rename(displaced, actual)
			}
			return err
		}
		delete(staged, path)
		if displaced != "" {
			if err := os.Remove(displaced); err != nil {
				// Publication must not leave a hidden copy behind. Restore the
				// previous spelling when cleanup fails.
				_ = os.Remove(path)
				_ = os.Rename(displaced, actual)
				return err
			}
		}
		s.generated[path] = true
		s.owners[path] = p.spec.Outfile
	}
	return nil
}

// esbuildDisplaceCaseAlias makes the requested filename spelling observable on
// case-insensitive filesystems. Replacing "App.HASH.js" through a lowercase path
// updates its contents on macOS but can retain the old directory-entry casing.
func esbuildDisplaceCaseAlias(path string) (actual, displaced string, err error) {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return "", "", nil
	} else if err != nil {
		return "", "", err
	}
	dir, base := filepath.Dir(path), filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", err
	}
	for _, entry := range entries {
		if entry.Name() == base {
			return "", "", nil
		}
	}
	for _, entry := range entries {
		if !strings.EqualFold(entry.Name(), base) {
			continue
		}
		if entry.IsDir() {
			return "", "", fmt.Errorf("esbuild output %s conflicts with directory %s", path, entry.Name())
		}
		file, err := os.CreateTemp(dir, ".hb-esbuild-case-*")
		if err != nil {
			return "", "", err
		}
		temp := file.Name()
		if err := file.Close(); err != nil {
			_ = os.Remove(temp)
			return "", "", err
		}
		if err := os.Remove(temp); err != nil {
			return "", "", err
		}
		actual = filepath.Join(dir, entry.Name())
		if err := os.Rename(actual, temp); err != nil {
			return "", "", err
		}
		return actual, temp, nil
	}
	return "", "", nil
}
