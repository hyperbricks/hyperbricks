package main

import (
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type environment struct {
	OS                string            `json:"os"`
	Architecture      string            `json:"architecture"`
	OSVersion         string            `json:"os_version"`
	Kernel            string            `json:"kernel"`
	CPU               string            `json:"cpu"`
	LogicalCPUs       int               `json:"logical_cpus"`
	ClientGoVersion   string            `json:"client_go_version"`
	ClientGOMAXPROCS  int               `json:"client_gomaxprocs"`
	InheritedGoTuning map[string]string `json:"inherited_go_tuning"`
}

type provenance struct {
	GitRevision     string       `json:"git_revision"`
	GitBranch       string       `json:"git_branch"`
	GitDirty        bool         `json:"git_dirty"`
	GitStatus       []string     `json:"git_status"`
	GitDiffSHA256   string       `json:"git_tracked_diff_sha256"`
	ServerBinary    binaryRecord `json:"server_binary"`
	RunnerBinary    binaryRecord `json:"runner_binary"`
	Fixture         manifest     `json:"fixture_source"`
	CopiedFixture   manifest     `json:"isolated_fixture"`
	RunnerSource    manifest     `json:"runner_source"`
	UntrackedSource manifest     `json:"untracked_source_files"`
}

type binaryRecord struct {
	SHA256    string            `json:"sha256"`
	Bytes     int64             `json:"bytes"`
	GoVersion string            `json:"go_version"`
	Settings  map[string]string `json:"build_settings"`
}

type manifest struct {
	SHA256 string       `json:"sha256"`
	Files  []sourceFile `json:"files"`
}

type sourceFile struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

func collectEnvironment() environment {
	value := environment{OS: runtime.GOOS, Architecture: runtime.GOARCH, LogicalCPUs: runtime.NumCPU(), ClientGoVersion: runtime.Version(), ClientGOMAXPROCS: runtime.GOMAXPROCS(0), InheritedGoTuning: make(map[string]string)}
	value.Kernel = optionalCommand("uname", "-sr")
	if runtime.GOOS == "darwin" {
		value.OSVersion = optionalCommand("sw_vers", "-productVersion")
		value.CPU = optionalCommand("sysctl", "-n", "machdep.cpu.brand_string")
	} else {
		value.OSVersion = value.Kernel
		if raw, err := os.ReadFile("/proc/cpuinfo"); err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				key, val, ok := strings.Cut(line, ":")
				if ok && (strings.TrimSpace(key) == "model name" || strings.TrimSpace(key) == "Hardware") {
					value.CPU = strings.TrimSpace(val)
					break
				}
			}
		}
	}
	if value.CPU == "" {
		value.CPU = "unavailable"
	}
	// Record only explicitly relevant tuning; never dump arbitrary environment.
	for _, key := range []string{"GOGC", "GOMEMLIMIT", "GODEBUG"} {
		if val, ok := os.LookupEnv(key); ok {
			value.InheritedGoTuning[key] = val
		}
	}
	return value
}

func optionalCommand(name string, args ...string) string {
	raw, err := exec.Command(name, args...).Output()
	if err != nil {
		return "unavailable"
	}
	return strings.TrimSpace(string(raw))
}

func collectProvenance(cfg options) (provenance, error) {
	var value provenance
	git := func(args ...string) ([]byte, error) {
		command := exec.Command("git", args...)
		command.Dir = cfg.Repo
		return command.Output()
	}
	revision, err := git("rev-parse", "HEAD")
	if err != nil {
		return value, fmt.Errorf("record git revision: %w", err)
	}
	value.GitRevision = strings.TrimSpace(string(revision))
	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return value, err
	}
	value.GitBranch = strings.TrimSpace(string(branch))
	status, err := git("status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return value, err
	}
	value.GitDirty = len(status) != 0
	if value.GitDirty {
		value.GitStatus = strings.Split(strings.TrimSuffix(string(status), "\n"), "\n")
	}
	diff, err := git("diff", "--binary", "HEAD", "--", ".", ":(exclude)benchmarks/response-cache/results", ":(exclude)benchmarks/response-cache/baselines")
	if err != nil {
		return value, err
	}
	digest := sha256.Sum256(diff)
	value.GitDiffSHA256 = hex.EncodeToString(digest[:])
	// The tracked diff cannot identify newly authored source files. Include a
	// content manifest of untracked build/fixture/runner inputs as well.
	untracked, err := git("ls-files", "--others", "--exclude-standard", "-z", "--", "cmd", "pkg", "assets", "go.mod", "go.sum", "modules/response-cache-test", "benchmarks/response-cache")
	if err != nil {
		return value, err
	}
	value.UntrackedSource.Files = []sourceFile{}
	for _, path := range strings.Split(string(untracked), "\x00") {
		if path == "" || strings.HasPrefix(path, "benchmarks/response-cache/results/") || strings.HasPrefix(path, "benchmarks/response-cache/baselines/") {
			continue
		}
		info, err := os.Lstat(filepath.Join(cfg.Repo, path))
		if err != nil {
			return value, err
		}
		if !info.Mode().IsRegular() {
			return value, fmt.Errorf("untracked input is not regular: %s", path)
		}
		hash, count, err := hashFile(filepath.Join(cfg.Repo, path))
		if err != nil {
			return value, err
		}
		value.UntrackedSource.Files = append(value.UntrackedSource.Files, sourceFile{Path: path, Bytes: count, SHA256: hash, Mode: uint32(info.Mode().Perm())})
	}
	if err := finishManifest(&value.UntrackedSource); err != nil {
		return value, err
	}
	if value.ServerBinary, err = inspectBinary(cfg.Binary); err != nil {
		return value, fmt.Errorf("server binary provenance: %w", err)
	}
	runner, err := os.Executable()
	if err != nil {
		return value, err
	}
	if value.RunnerBinary, err = inspectBinary(runner); err != nil {
		return value, fmt.Errorf("runner binary provenance: %w", err)
	}
	if value.Fixture, err = sourceManifest(cfg.Module, false); err != nil {
		return value, fmt.Errorf("fixture provenance: %w", err)
	}
	if value.RunnerSource, err = sourceManifest(filepath.Join(cfg.Repo, "benchmarks", "response-cache"), true); err != nil {
		return value, fmt.Errorf("runner source provenance: %w", err)
	}
	return value, nil
}

func inspectBinary(path string) (binaryRecord, error) {
	digest, count, err := hashFile(path)
	if err != nil {
		return binaryRecord{}, err
	}
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return binaryRecord{}, err
	}
	value := binaryRecord{SHA256: digest, Bytes: count, GoVersion: info.GoVersion, Settings: make(map[string]string)}
	for _, setting := range info.Settings {
		value.Settings[setting.Key] = setting.Value
	}
	return value, nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	count, err := io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), count, err
}

func sourceManifest(root string, runner bool) (manifest, error) {
	value := manifest{Files: []sourceFile{}}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.Name() == ".cache" || entry.Name() == ".git" || !runner && relative == "generated-cache" || runner && (relative == "results" || relative == "baselines") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == ".DS_Store" || entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("fixture/runner source must be self-contained; symlink %s", relative)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported non-regular source file %s", relative)
		}
		hash, count, err := hashFile(path)
		if err != nil {
			return err
		}
		value.Files = append(value.Files, sourceFile{Path: filepath.ToSlash(relative), Bytes: count, SHA256: hash, Mode: uint32(info.Mode().Perm())})
		return nil
	})
	if err != nil {
		return value, err
	}
	err = finishManifest(&value)
	return value, err
}

func finishManifest(value *manifest) error {
	sort.Slice(value.Files, func(i, j int) bool { return value.Files[i].Path < value.Files[j].Path })
	encoded, err := json.Marshal(value.Files)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	value.SHA256 = hex.EncodeToString(digest[:])
	return nil
}

func copyFixture(source, target string, recorded manifest) error {
	if err := os.Mkdir(target, 0755); err != nil {
		return err
	}
	for _, file := range recorded.Files {
		destination := filepath.Join(target, filepath.FromSlash(file.Path))
		if !pathWithin(target, destination) {
			return fmt.Errorf("source path escaped fixture: %s", file.Path)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		input, err := os.Open(filepath.Join(source, filepath.FromSlash(file.Path)))
		if err != nil {
			return err
		}
		output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, os.FileMode(file.Mode))
		if err != nil {
			_ = input.Close()
			return err
		}
		hash := sha256.New()
		count, copyErr := io.Copy(io.MultiWriter(output, hash), input)
		_ = input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if count != file.Bytes || hex.EncodeToString(hash.Sum(nil)) != file.SHA256 {
			return fmt.Errorf("source file changed while copying: %s", file.Path)
		}
	}
	return nil
}

func pathWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func serverEnvironment(procs int) []string {
	var values []string
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if strings.HasPrefix(key, "HB_DEPLOY_") || key == "HB_PRODUCTION" || key == "GOMAXPROCS" || key == "HB_NO_KEYBOARD" || key == "NO_COLOR" || key == "TERM" {
			continue
		}
		values = append(values, value)
	}
	return append(values, "GOMAXPROCS="+strconv.Itoa(procs), "HB_NO_KEYBOARD=1", "NO_COLOR=1", "TERM=dumb")
}
