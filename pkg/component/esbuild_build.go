package component

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
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

func (r *EsbuildRenderer) OwnsOutput(path string) bool {
	abs, err := filepath.Abs(path)
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
	if p.cache {
		if result := s.results[p.key]; result != nil && result.valid() {
			if p.debug {
				logging.GetLogger().Infow("esbuild cache hit", "entry", p.spec.Entry)
			}
			return result.entry, nil
		}
	}
	// A shared build may outlive its initiating request; its independent deadline
	// bounds compiler work without cancelling other waiting requests.
	buildCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	engine, err := p.spec.engineIdentity(buildCtx)
	if err != nil {
		return "", err
	}
	if p.cache && s.reuseDisk {
		if result := s.restore(p, engine); result != nil {
			if err := s.claimOutputs(p, result.outputs); err != nil {
				return "", err
			}
			s.results[p.key] = result
			if p.debug {
				logging.GetLogger().Infow("esbuild persistent cache hit", "entry", p.spec.Entry)
			}
			return result.entry, nil
		}
	}
	if p.debug {
		logging.GetLogger().Infow("esbuild build", "options", p.spec, "cache", p.cache, "engine", engine)
	}
	s.buildCount++
	started := time.Now()
	outputs, metafile, err := p.spec.compile(buildCtx, p.debug)
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
	if err := s.publish(p, outputs, entry); err != nil {
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

func (s esbuildSpec) compile(ctx context.Context, debug bool) (map[string][]byte, string, error) {
	if s.Binary != "" {
		return s.compileExternal(ctx, debug)
	}
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

func (s esbuildSpec) compileExternal(ctx context.Context, debug bool) (map[string][]byte, string, error) {
	stage, err := os.MkdirTemp("", "hb-esbuild-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(stage)
	outputRoot := filepath.Join(stage, "output")
	stagedOutput := filepath.Join(outputRoot, filepath.Base(s.Outfile))
	metadata := filepath.Join(stage, "metadata.json")
	args := s.externalArgs(stagedOutput, metadata)
	if debug {
		logging.GetLogger().Infow("external esbuild", "binary", s.Binary, "args", args)
	}
	cmd := exec.CommandContext(ctx, s.Binary, args...)
	cmd.Dir = s.WorkingDir
	diagnostics, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", fmt.Errorf("external esbuild: %w: %s", err, diagnostics)
	}
	if len(diagnostics) != 0 && debug {
		logging.GetLogger().Infow("external esbuild output", "diagnostics", string(diagnostics))
	}
	meta, err := os.ReadFile(metadata)
	if err != nil {
		return nil, "", fmt.Errorf("read esbuild metadata: %w", err)
	}
	outputs := make(map[string][]byte)
	err = filepath.WalkDir(outputRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("external esbuild produced a non-regular output: %s", path)
		}
		rel, err := filepath.Rel(outputRoot, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		destination := filepath.Join(filepath.Dir(s.Outfile), rel)
		// esbuild source paths are relative to the staged map, not its published location.
		if strings.HasSuffix(path, ".js.map") || strings.HasSuffix(path, ".css.map") {
			data, err = esbuildRebaseSourceMap(data, filepath.Dir(path), filepath.Dir(destination))
			if err != nil {
				return err
			}
		}
		outputs[destination] = data
		return nil
	})
	return outputs, string(meta), err
}

func esbuildRebaseSourceMap(data []byte, from, to string) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("esbuild source map: %w", err)
	}
	var sources []string
	if err := json.Unmarshal(fields["sources"], &sources); err != nil {
		return nil, fmt.Errorf("esbuild source map sources: %w", err)
	}
	for i, source := range sources {
		u, err := url.Parse(source)
		if filepath.IsAbs(source) || (err == nil && u.IsAbs()) {
			continue
		}
		rel, err := filepath.Rel(to, filepath.Join(from, filepath.FromSlash(source)))
		if err != nil {
			return nil, err
		}
		sources[i] = filepath.ToSlash(rel)
	}
	encoded, err := json.Marshal(sources)
	if err != nil {
		return nil, err
	}
	fields["sources"] = encoded
	return json.Marshal(fields)
}

func (s esbuildSpec) externalArgs(outfile, metafile string) []string {
	args := []string{s.Entry, "--bundle", "--outfile=" + outfile, "--metafile=" + metafile, "--color=false"}
	if s.Fingerprint {
		args = []string{s.outputStem() + "=" + s.Entry, "--bundle", "--outdir=" + filepath.Dir(outfile), "--entry-names=[name].[hash]", "--metafile=" + metafile, "--color=false"}
	}
	if s.Minify {
		args = append(args, "--minify-whitespace", "--minify-syntax")
	}
	if s.MinifyIdentifiers {
		args = append(args, "--minify-identifiers")
	}
	if s.Mangle {
		args = append(args, "--mangle-props=.*")
	}
	if s.Sourcemap {
		args = append(args, "--sourcemap=linked")
	}
	if len(s.Target) != 0 {
		args = append(args, "--target="+strings.Join(s.Target, ","))
	}
	keys := make([]string, 0, len(s.Loader))
	for key := range s.Loader {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--loader:"+key+"="+s.Loader[key])
	}
	for _, value := range s.External {
		args = append(args, "--external:"+value)
	}
	return args
}

func (s esbuildSpec) dependencies(metafile string) (map[string]bool, error) {
	var meta struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal([]byte(metafile), &meta); err != nil {
		return nil, fmt.Errorf("esbuild metadata: %w", err)
	}
	paths := map[string]bool{s.Entry: true}
	if s.Binary != "" {
		paths[s.Binary] = true
	}
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
	}
	for _, input := range inputs {
		if input == s.Binary {
			continue
		}
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
