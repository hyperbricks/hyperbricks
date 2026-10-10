package commands

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/assets"
)

func skillTestFiles(version, text string) map[string][]byte {
	document := []byte("# Quickstart\n" + text)
	manifest, _ := json.Marshal(map[string]any{
		"schema_version": 2, "hyperbricks_version": version,
		"documents": []map[string]string{{"bundled_path": "docs/QUICKSTART.md", "bundled_sha256": skillHash(document)}},
	})
	return map[string][]byte{
		"SKILL.md":                               []byte("---\nname: hyperbricks\ndescription: Build HyperBricks projects\n---\n" + text),
		"references/DOCUMENTATION_INDEX.md":      []byte("[Quickstart](docs/QUICKSTART.md)"),
		"references/documentation-manifest.json": manifest,
		"references/docs/QUICKSTART.md":          document,
		"agents/openai.yaml":                     []byte("interface:\n  display_name: HyperBricks\n"),
	}
}

func skillTestZIP(t *testing.T, files map[string][]byte, extra func(*zip.Writer)) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create("hyperbricks/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if extra != nil {
		extra(writer)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func skillTestArchive(t *testing.T, version, text string) string {
	t.Helper()
	data := skillTestZIP(t, skillTestFiles(version, text), nil)
	name := filepath.Join(t.TempDir(), "hyperbricks-skill-"+version+".zip")
	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name+".sha256", []byte(skillHash(data)+"  "+filepath.Base(name)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

func runSkillTestInstall(t *testing.T, options skillInstallOptions, version string) error {
	t.Helper()
	_, err := installHyperbricksSkill(context.Background(), options, version, "http://unused.invalid", nil)
	return err
}

func TestSkillsOfflineInstallPreservesProjectAndIsIdempotent(t *testing.T) {
	const version = "v1.3.0-beta"
	project := t.TempDir()
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md", ".agents/skills/other/SKILL.md"} {
		filename := filepath.Join(project, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte("Keep this"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	options := skillInstallOptions{project: project, agent: "all", archive: skillTestArchive(t, version, "original")}
	if err := runSkillTestInstall(t, options, version); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(project, filepath.FromSlash(skillRelativePath))
	before, err := installedSkillHashes(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := runSkillTestInstall(t, options, version); err != nil {
		t.Fatal(err)
	}
	after, err := installedSkillHashes(target)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("idempotent install changed files: %v", err)
	}
	link, err := os.Readlink(filepath.Join(project, filepath.FromSlash(skillClaudePath)))
	if err != nil || link != skillClaudeTarget {
		t.Fatalf("Claude link = %q, %v", link, err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(project, filepath.FromSlash(skillClaudePath)))
	canonical, _ := filepath.EvalSymlinks(target)
	if err != nil || resolved != canonical {
		t.Fatalf("Claude link did not resolve to shared skill: %q, %v", resolved, err)
	}
	for _, name := range []string{"AGENTS.md", "CLAUDE.md", "GEMINI.md", ".agents/skills/other/SKILL.md"} {
		data, err := os.ReadFile(filepath.Join(project, filepath.FromSlash(name)))
		if err != nil || string(data) != "Keep this" {
			t.Fatalf("changed unrelated file %s: %v", name, err)
		}
	}
}

func TestSkillsManagedUpdateAndLocalChangeProtection(t *testing.T) {
	const version = "v1.3.0-beta"
	for _, localChange := range []string{"none", "edit", "add", "delete", "symlink", "receipt"} {
		t.Run(localChange, func(t *testing.T) {
			project := t.TempDir()
			options := skillInstallOptions{project: project, agent: "codex", archive: skillTestArchive(t, version, "first")}
			if err := runSkillTestInstall(t, options, version); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(project, filepath.FromSlash(skillRelativePath))
			switch localChange {
			case "edit":
				if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("local edit"), 0644); err != nil {
					t.Fatal(err)
				}
			case "add":
				if err := os.WriteFile(filepath.Join(target, "custom.md"), []byte("local addition"), 0644); err != nil {
					t.Fatal(err)
				}
			case "delete":
				if err := os.Remove(filepath.Join(target, "SKILL.md")); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("SKILL.md", filepath.Join(target, "custom.md")); err != nil {
					t.Fatal(err)
				}
			case "receipt":
				if err := os.WriteFile(filepath.Join(target, skillReceiptName), []byte("invalid"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			options.archive = skillTestArchive(t, version, "second")
			if err := runSkillTestInstall(t, options, version); err == nil {
				t.Fatal("replacement accepted without --update")
			}
			options.update = true
			err := runSkillTestInstall(t, options, version)
			if localChange == "none" {
				if err != nil {
					t.Fatal(err)
				}
				data, _ := os.ReadFile(filepath.Join(target, "SKILL.md"))
				if !strings.Contains(string(data), "second") {
					t.Fatal("update did not replace skill")
				}
			} else if err == nil {
				t.Fatal("local change was overwritten")
			}
			if _, err := os.Lstat(filepath.Join(project, filepath.FromSlash(skillClaudePath))); !os.IsNotExist(err) {
				t.Fatalf("Codex install created Claude link: %v", err)
			}
		})
	}
}

func TestSkillsRejectsUnmanagedCopyAndConflictingDiscovery(t *testing.T) {
	const version = "v1.3.0-beta"
	for _, conflict := range []string{"unmanaged", "claude", "gemini", "parent-symlink", "target-symlink"} {
		t.Run(conflict, func(t *testing.T) {
			project := t.TempDir()
			target := filepath.Join(project, filepath.FromSlash(skillRelativePath))
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				t.Fatal(err)
			}
			switch conflict {
			case "unmanaged":
				if err := os.MkdirAll(target, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("custom skill"), 0644); err != nil {
					t.Fatal(err)
				}
			case "claude", "gemini":
				if err := os.MkdirAll(filepath.Join(project, "."+conflict, "skills", "hyperbricks"), 0755); err != nil {
					t.Fatal(err)
				}
			case "parent-symlink":
				if err := os.RemoveAll(filepath.Join(project, ".agents")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), filepath.Join(project, ".agents")); err != nil {
					t.Fatal(err)
				}
			case "target-symlink":
				if err := os.Symlink(t.TempDir(), target); err != nil {
					t.Fatal(err)
				}
			}
			options := skillInstallOptions{project: project, agent: "all", archive: skillTestArchive(t, version, "new"), update: true}
			if err := runSkillTestInstall(t, options, version); err == nil {
				t.Fatal("conflicting skill was accepted")
			}
		})
	}
}

func TestSkillsArchiveValidation(t *testing.T) {
	const version = "v1.3.0-beta"
	for _, malformed := range []string{"version", "hash", "missing", "extra-doc", "manifest-path", "traversal", "absolute", "backslash", "duplicate", "case-collision", "symlink", "receipt", "receipt-case", "receipt-directory", "large"} {
		t.Run(malformed, func(t *testing.T) {
			files := skillTestFiles(version, "test")
			var extra func(*zip.Writer)
			switch malformed {
			case "version":
				files = skillTestFiles("v1.3.0", "test")
			case "hash":
				files["references/docs/QUICKSTART.md"] = []byte("corrupt")
			case "missing":
				delete(files, "references/docs/QUICKSTART.md")
			case "extra-doc":
				files["references/docs/EXTRA.md"] = []byte("unlisted")
			case "manifest-path":
				files["references/documentation-manifest.json"] = []byte(`{"schema_version":2,"hyperbricks_version":"v1.3.0-beta","documents":[{"bundled_path":"docs/../SKILL.md","bundled_sha256":"x"}]}`)
			case "receipt":
				files[skillReceiptName] = []byte("installer state")
			case "receipt-case":
				files[strings.ToUpper(skillReceiptName)] = []byte("installer state")
			case "receipt-directory":
				files[strings.ToUpper(skillReceiptName)+"/child"] = []byte("installer state")
			case "large":
				files["large.bin"] = make([]byte, skillFileLimit+1)
			default:
				names := map[string]string{"traversal": "hyperbricks/../../escape", "absolute": "/escape", "backslash": `hyperbricks\escape`, "duplicate": "hyperbricks/SKILL.md", "case-collision": "hyperbricks/skill.md", "symlink": "hyperbricks/link"}
				extra = func(writer *zip.Writer) {
					header := &zip.FileHeader{Name: names[malformed], Method: zip.Deflate}
					if malformed == "symlink" {
						header.SetMode(os.ModeSymlink | 0777)
					}
					entry, err := writer.CreateHeader(header)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := entry.Write([]byte("payload")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := decodeSkillArchive(skillTestZIP(t, files, extra), version); err == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}

func TestSkillsDownloadExactVersionAndChecksum(t *testing.T) {
	const version = "v1.3.0-beta"
	archive := skillTestZIP(t, skillTestFiles(version, "download"), nil)
	for _, failure := range []string{"", "checksum", "missing", "oversized-checksum"} {
		t.Run(failure, func(t *testing.T) {
			var requested []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requested = append(requested, r.URL.Path)
				if failure == "missing" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				switch r.URL.Path {
				case "/hyperbricks-skill-" + version + ".zip.sha256":
					checksum := skillHash(archive)
					if failure == "checksum" {
						checksum = strings.Repeat("0", 64)
					}
					if failure == "oversized-checksum" {
						checksum = strings.Repeat("0", 4097)
					}
					fmt.Fprintln(w, checksum)
				case "/hyperbricks-skill-" + version + ".zip":
					_, _ = w.Write(archive)
				default:
					t.Errorf("unexpected version route: %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			project := t.TempDir()
			_, err := installHyperbricksSkill(context.Background(), skillInstallOptions{project: project, agent: "codex"}, version, server.URL, server.Client())
			if failure == "" && err != nil {
				t.Fatal(err)
			}
			if failure != "" {
				if err == nil {
					t.Fatal("failed download accepted")
				}
				if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
					t.Fatalf("download failure changed project: %v", err)
				}
			}
			if len(requested) == 0 {
				t.Fatal("no exact-version download attempted")
			}
		})
	}
}

func TestSkillsValidatesOptionsBeforeDownload(t *testing.T) {
	for _, options := range []skillInstallOptions{{project: t.TempDir(), agent: "unknown"}, {project: filepath.Join(t.TempDir(), "missing"), agent: "codex"}} {
		if _, err := installHyperbricksSkill(context.Background(), options, "v1.3.0", "http://must-not-fetch", nil); err == nil {
			t.Fatal("invalid install options accepted")
		}
	}
}

func TestSkillsCLIUsesRuntimeVersion(t *testing.T) {
	previousExit, previousExitCode := Exit, ExitCode
	t.Cleanup(func() { Exit, ExitCode = previousExit, previousExitCode })
	version := strings.TrimSpace(assets.VersionMD)
	cmd := NewSkillsCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"install", "--project", t.TempDir(), "--archive", skillTestArchive(t, version, "cli"), "--agent", "gemini"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), version) || !strings.Contains(output.String(), "Project instructions were preserved") {
		t.Fatalf("missing install result: %s", output.String())
	}
}

func TestSkillsDownloadRejectsHTTPSDowngrade(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("download followed an insecure redirect")
	}))
	defer plain.Close()
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL, http.StatusFound)
	}))
	defer secure.Close()
	if _, err := downloadSkillFile(context.Background(), secure.Client(), secure.URL, 4096); err == nil || !strings.Contains(err.Error(), "insecure") {
		t.Fatalf("HTTPS downgrade was not rejected: %v", err)
	}
}

func TestSkillsOfflineChecksumRequired(t *testing.T) {
	const version = "v1.3.0-beta"
	archive := skillTestArchive(t, version, "offline")
	if err := os.Remove(archive + ".sha256"); err != nil {
		t.Fatal(err)
	}
	project := t.TempDir()
	err := runSkillTestInstall(t, skillInstallOptions{project: project, agent: "codex", archive: archive}, version)
	if err == nil || !strings.Contains(err.Error(), ".sha256") {
		t.Fatalf("missing checksum not diagnosed: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".agents")); !os.IsNotExist(err) {
		t.Fatalf("failed validation modified project: %v", err)
	}
}

func TestSkillsFailedClaudeLinkPreservesPreviousSkill(t *testing.T) {
	const version = "v1.3.0-beta"
	project := t.TempDir()
	options := skillInstallOptions{project: project, agent: "codex", archive: skillTestArchive(t, version, "original")}
	if err := runSkillTestInstall(t, options, version); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(project, filepath.FromSlash(skillRelativePath))
	before, err := installedSkillHashes(target)
	if err != nil {
		t.Fatal(err)
	}
	claudeParent := filepath.Join(project, ".claude", "skills")
	if err := os.MkdirAll(claudeParent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(claudeParent, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(claudeParent, 0755) })
	if err := os.WriteFile(filepath.Join(claudeParent, "probe"), nil, 0600); err == nil {
		t.Skip("filesystem permissions cannot force a link-creation failure for this user")
	}
	options.agent, options.update, options.archive = "all", true, skillTestArchive(t, version, "replacement")
	if err := runSkillTestInstall(t, options, version); err == nil || !strings.Contains(err.Error(), "Claude skill link") {
		t.Fatalf("expected staged install to fail creating Claude link: %v", err)
	}
	after, err := installedSkillHashes(target)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("failed install changed previous skill: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(target))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".hyperbricks-install") {
			t.Fatalf("failed install left staging or lock: %s", entry.Name())
		}
	}
}
