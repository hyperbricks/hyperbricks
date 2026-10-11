package commands

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/assets"
	"github.com/spf13/cobra"
)

const (
	skillRelativePath  = ".agents/skills/hyperbricks"
	skillClaudePath    = ".claude/skills/hyperbricks"
	skillClaudeTarget  = "../../.agents/skills/hyperbricks"
	skillReceiptName   = ".hyperbricks-install.json"
	skillArchiveLimit  = 32 << 20
	skillExpandedLimit = 64 << 20
	skillFileLimit     = 8 << 20
	skillEntryLimit    = 2000
)

var skillVersionPattern = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

type skillInstallOptions struct {
	project, agent, archive string
	update                  bool
}

type skillDocumentationManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Version       string `json:"hyperbricks_version"`
	Documents     []struct {
		Path   string `json:"bundled_path"`
		SHA256 string `json:"bundled_sha256"`
	} `json:"documents"`
}

type skillInstallReceipt struct {
	SchemaVersion int               `json:"schema_version"`
	Version       string            `json:"hyperbricks_version"`
	Files         map[string]string `json:"files"`
}

// NewSkillsCommand installs the complete, runtime-matched Agent Skills package.
func NewSkillsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "skills", Short: "Install HyperBricks skills for coding agents", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error { Exit = true; return cmd.Help() }
	var options skillInstallOptions
	install := &cobra.Command{
		Use: "install", Short: "Install the skill matching this HyperBricks executable", Args: cobra.NoArgs,
		Long: "Download the skill ZIP and SHA256 checksum matching this executable. Install one shared skill in .agents/skills/hyperbricks; Claude Code uses a symlink to that folder. Existing project instructions and unrelated skills are preserved. Offline installation requires the ZIP and its adjacent .sha256 file.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			Exit, ExitCode = true, 0
			version := strings.TrimSpace(assets.VersionMD)
			project, err := installHyperbricksSkill(cmd.Context(), options, version, "https://hyperbricks.eu/static/downloads", &http.Client{Timeout: 90 * time.Second})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "HyperBricks skill (%s): %s\n", version, filepath.Join(project, filepath.FromSlash(skillRelativePath)))
			if options.agent == "all" || options.agent == "claude" {
				fmt.Fprintf(cmd.OutOrStdout(), "Claude Code: %s -> %s\n", skillClaudePath, skillClaudeTarget)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Restart your agent or reload its skills. Project instructions were preserved.")
			return err
		},
	}
	install.Flags().StringVar(&options.project, "project", ".", "Existing project root (defaults to the current directory)")
	install.Flags().StringVar(&options.agent, "agent", "all", "Agent discovery: all, codex, gemini, or claude")
	install.Flags().StringVar(&options.archive, "archive", "", "Local skill ZIP, with an adjacent .sha256 checksum, for offline installation")
	install.Flags().BoolVar(&options.update, "update", false, "Replace an unchanged copy managed by this installer")
	cmd.AddCommand(install)
	return cmd
}

func installHyperbricksSkill(ctx context.Context, options skillInstallOptions, version, downloadBase string, client *http.Client) (string, error) {
	if !skillVersionPattern.MatchString(version) {
		return "", fmt.Errorf("runtime version %q cannot select a release skill archive", version)
	}
	claude := options.agent == "all" || options.agent == "claude"
	if !claude && options.agent != "codex" && options.agent != "gemini" {
		return "", fmt.Errorf("unknown agent %q; choose all, codex, gemini, or claude", options.agent)
	}
	project, err := filepath.Abs(options.project)
	if err != nil {
		return "", err
	}
	project, err = filepath.EvalSymlinks(project)
	if err != nil {
		return "", fmt.Errorf("project must be an existing directory: %w", err)
	}
	info, err := os.Stat(project)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("project must be an existing directory: %s", project)
	}
	if err := checkSkillDestination(project, claude, options.agent); err != nil {
		return "", err
	}
	var archive, checksum []byte
	if options.archive != "" {
		archive, err = readSkillFile(options.archive, skillArchiveLimit)
		if err != nil {
			return "", err
		}
		checksum, err = readSkillFile(options.archive+".sha256", 4096)
	} else {
		archiveURL := strings.TrimRight(downloadBase, "/") + "/hyperbricks-skill-" + version + ".zip"
		checksum, err = downloadSkillFile(ctx, client, archiveURL+".sha256", 4096)
		if err == nil {
			archive, err = downloadSkillFile(ctx, client, archiveURL, skillArchiveLimit)
		}
	}
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(checksum))
	if len(fields) < 1 || len(fields) > 2 || !validSkillHash(fields[0]) || !strings.EqualFold(fields[0], skillHash(archive)) {
		return "", fmt.Errorf("skill archive SHA256 checksum is invalid or does not match; download the ZIP and its .sha256 file again")
	}
	files, err := decodeSkillArchive(archive, version)
	if err != nil {
		return "", err
	}
	if err := commitSkillInstall(project, files, version, options); err != nil {
		return "", err
	}
	return project, nil
}

func downloadSkillFile(ctx context.Context, client *http.Client, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	secureClient := *client
	secureClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if req.URL.Scheme == "https" && next.URL.Scheme != "https" {
			return fmt.Errorf("skill download cannot redirect from HTTPS to an insecure URL")
		}
		if client.CheckRedirect != nil {
			return client.CheckRedirect(next, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("skill download exceeded the redirect limit")
		}
		return nil
	}
	response, err := secureClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download skill: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("skill download returned HTTP %d for %s; check that the skill package for this runtime is published, or use --archive with its matching ZIP and checksum", response.StatusCode, url)
	}
	return readSkillBytes(response.Body, limit)
}

func readSkillFile(name string, limit int64) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read skill package %s: %w", name, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("skill package must be a regular file: %s", name)
	}
	return readSkillBytes(file, limit)
}

func readSkillBytes(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("skill package exceeds the %d-byte size limit", limit)
	}
	return data, nil
}

func skillHash(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
func validSkillHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func safeSkillPath(name string) bool {
	if name == "" || strings.ContainsAny(name, "\\:\x00") || path.Clean(name) != name || !filepath.IsLocal(filepath.FromSlash(name)) {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "." || part == ".." || strings.TrimRight(part, ". ") != part {
			return false
		}
	}
	return true
}

func decodeSkillArchive(data []byte, version string) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid skill ZIP: %w", err)
	}
	if len(reader.File) > skillEntryLimit {
		return nil, fmt.Errorf("skill ZIP contains too many entries")
	}
	files := map[string][]byte{}
	seen := map[string]bool{}
	var expanded uint64
	for _, entry := range reader.File {
		name := strings.TrimSuffix(entry.Name, "/")
		if !safeSkillPath(name) || name != "hyperbricks" && !strings.HasPrefix(name, "hyperbricks/") {
			return nil, fmt.Errorf("unsafe skill ZIP path: %q", entry.Name)
		}
		key := strings.ToLower(name)
		if seen[key] {
			return nil, fmt.Errorf("duplicate skill ZIP path: %q", entry.Name)
		}
		seen[key] = true
		if entry.Mode()&os.ModeType != 0 && entry.Mode()&os.ModeType != os.ModeDir {
			return nil, fmt.Errorf("skill ZIP must contain only regular files and directories: %q", entry.Name)
		}
		if entry.Mode().IsDir() {
			continue
		}
		if !entry.Mode().IsRegular() || name == "hyperbricks" {
			return nil, fmt.Errorf("skill ZIP must contain only regular files and directories: %q", entry.Name)
		}
		relative := strings.TrimPrefix(name, "hyperbricks/")
		if strings.EqualFold(strings.SplitN(relative, "/", 2)[0], skillReceiptName) {
			return nil, fmt.Errorf("skill ZIP contains installer state")
		}
		expanded += entry.UncompressedSize64
		if entry.UncompressedSize64 > skillFileLimit || expanded > skillExpandedLimit {
			return nil, fmt.Errorf("expanded skill ZIP exceeds size limits")
		}
		stream, err := entry.Open()
		if err != nil {
			return nil, err
		}
		contents, readErr := readSkillBytes(stream, skillFileLimit)
		closeErr := stream.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		files[relative] = contents
	}
	for name := range files {
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			if _, exists := files[parent]; exists {
				return nil, fmt.Errorf("skill ZIP file is also a parent directory: %s", parent)
			}
		}
	}
	for _, required := range []string{"SKILL.md", "references/DOCUMENTATION_INDEX.md", "references/documentation-manifest.json"} {
		if len(bytes.TrimSpace(files[required])) == 0 {
			return nil, fmt.Errorf("skill ZIP is missing %s", required)
		}
	}
	var manifest skillDocumentationManifest
	if err := json.Unmarshal(files["references/documentation-manifest.json"], &manifest); err != nil {
		return nil, fmt.Errorf("invalid skill documentation manifest: %w", err)
	}
	if manifest.SchemaVersion != 2 || len(manifest.Documents) == 0 {
		return nil, fmt.Errorf("skill ZIP requires a supported, nonempty documentation manifest")
	}
	if strings.TrimPrefix(manifest.Version, "v") != strings.TrimPrefix(version, "v") {
		return nil, fmt.Errorf("skill documentation is for %s, not runtime %s", manifest.Version, version)
	}
	documents := map[string]bool{}
	for _, document := range manifest.Documents {
		name := "references/" + document.Path
		content, exists := files[name]
		if !safeSkillPath(document.Path) || !strings.HasPrefix(document.Path, "docs/") || documents[name] || !exists || !validSkillHash(document.SHA256) || !strings.EqualFold(skillHash(content), document.SHA256) {
			return nil, fmt.Errorf("skill documentation is missing, modified, or has an invalid manifest entry: %q", document.Path)
		}
		documents[name] = true
	}
	for name := range files {
		if strings.HasPrefix(name, "references/docs/") && !documents[name] {
			return nil, fmt.Errorf("skill documentation is absent from the manifest: %s", name)
		}
	}
	return files, nil
}

func checkSkillParents(project, relative string) error {
	current := project
	parts := strings.Split(relative, "/")
	for _, part := range parts[:len(parts)-1] {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("installation parent must be a real directory: %s", current)
		}
	}
	return nil
}

func checkSkillDestination(project string, claude bool, agent string) error {
	if err := checkSkillParents(project, skillRelativePath); err != nil {
		return err
	}
	target := filepath.Join(project, filepath.FromSlash(skillRelativePath))
	if info, err := os.Lstat(target); err == nil && !info.IsDir() {
		return fmt.Errorf("existing skill must be a real directory; preserve or move it first: %s", target)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if claude {
		if err := checkSkillParents(project, skillClaudePath); err != nil {
			return err
		}
		link := filepath.Join(project, filepath.FromSlash(skillClaudePath))
		if _, err := os.Lstat(link); err == nil {
			target, linkErr := os.Readlink(link)
			if linkErr != nil || target != skillClaudeTarget {
				return fmt.Errorf("existing Claude skill is not the shared link; preserve or move it first: %s", link)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if agent == "all" || agent == "gemini" {
		legacy := filepath.Join(project, ".gemini", "skills", "hyperbricks")
		if _, err := os.Lstat(legacy); err == nil {
			return fmt.Errorf("existing Gemini skill would duplicate the shared skill; preserve or move it first: %s", legacy)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func installedSkillHashes(directory string) (map[string]string, error) {
	files := map[string]string{}
	err := filepath.WalkDir(directory, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("installed skill contains a symlink or special file: %s", name)
		}
		relative, err := filepath.Rel(directory, name)
		if err != nil {
			return err
		}
		if relative == skillReceiptName {
			return nil
		}
		data, err := readSkillFile(name, skillFileLimit)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = skillHash(data)
		return nil
	})
	return files, err
}

func commitSkillInstall(project string, files map[string][]byte, version string, options skillInstallOptions) error {
	claude := options.agent == "all" || options.agent == "claude"
	if err := checkSkillDestination(project, claude, options.agent); err != nil {
		return err
	}
	target := filepath.Join(project, filepath.FromSlash(skillRelativePath))
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	lock := filepath.Join(parent, ".hyperbricks-install.lock")
	if err := os.Mkdir(lock, 0700); err != nil {
		return fmt.Errorf("cannot lock skill installation; another install may be active (%s): %w", lock, err)
	}
	defer os.Remove(lock)
	hashes := map[string]string{}
	for name, data := range files {
		hashes[name] = skillHash(data)
	}
	replace, exists := true, false
	if _, err := os.Lstat(target); err == nil {
		exists = true
		installed, err := installedSkillHashes(target)
		if err != nil {
			return err
		}
		if reflect.DeepEqual(installed, hashes) {
			replace = false
		} else {
			if !options.update {
				return fmt.Errorf("a different skill is installed; --update only replaces an unchanged installer-managed copy")
			}
			data, err := readSkillFile(filepath.Join(target, skillReceiptName), skillFileLimit)
			if err != nil {
				return fmt.Errorf("existing skill is not installer-managed; preserve or move it before installing: %w", err)
			}
			var receipt skillInstallReceipt
			if json.Unmarshal(data, &receipt) != nil || receipt.SchemaVersion != 1 || !reflect.DeepEqual(receipt.Files, installed) {
				return fmt.Errorf("installed skill has local changes or invalid installer state; preserve or move it before updating")
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	temporary, err := os.MkdirTemp(parent, ".hyperbricks-install-")
	if err != nil {
		return err
	}
	keepTemporary := false
	defer func() {
		if !keepTemporary {
			_ = os.RemoveAll(temporary)
		}
	}()
	staging, backup := filepath.Join(temporary, "skill"), filepath.Join(temporary, "previous")
	if replace {
		if err := os.Mkdir(staging, 0755); err != nil {
			return err
		}
		for name, contents := range files {
			destination := filepath.Join(staging, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(destination, contents, 0644); err != nil {
				return err
			}
		}
		receipt, err := json.MarshalIndent(skillInstallReceipt{1, version, hashes}, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(staging, skillReceiptName), append(receipt, '\n'), 0644); err != nil {
			return err
		}
	}
	// Complete staging before changing the old skill. A failed link or rename
	// restores the previous directory; unrelated skills and instructions are untouched.
	link := filepath.Join(project, filepath.FromSlash(skillClaudePath))
	madeLink := false
	committed := false
	defer func() {
		if madeLink && !committed {
			_ = os.Remove(link)
		}
	}()
	if claude {
		if _, err := os.Lstat(link); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
				return err
			}
			if err := os.Symlink(skillClaudeTarget, link); err != nil {
				return fmt.Errorf("create Claude skill link: %w; use --agent codex or --agent gemini to install without a symlink", err)
			}
			madeLink = true
		} else if err != nil {
			return err
		}
	}
	if replace {
		if exists {
			if err := os.Rename(target, backup); err != nil {
				return err
			}
		}
		if err := os.Rename(staging, target); err != nil {
			if exists {
				if restoreErr := os.Rename(backup, target); restoreErr != nil {
					// Retain the previous skill if an unexpected filesystem failure prevents rollback.
					keepTemporary = true
					return fmt.Errorf("install failed: %v; restore failed: %v; previous skill remains at %s", err, restoreErr, backup)
				}
			}
			return err
		}
	}
	committed = true
	return nil
}
