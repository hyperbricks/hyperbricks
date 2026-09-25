package spaces

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/markdown"
)

type FileUsage struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Field   string `json:"field"`
	Label   string `json:"label"`
	Kind    string `json:"kind"`
	Trashed bool   `json:"trashed"`
	policy  Field
}
type Document struct {
	Revision     string      `json:"revision"`
	FileRevision string      `json:"file_revision"`
	Reference    string      `json:"reference"`
	Filename     string      `json:"filename"`
	Content      string      `json:"content"`
	MaxBytes     int64       `json:"max_bytes"`
	Usages       []FileUsage `json:"usages"`
	Shared       bool        `json:"shared"`
}
type DocumentMutation struct {
	Action       string `json:"action"`
	Name         string `json:"name"`
	Field        string `json:"field"`
	Revision     string `json:"revision"`
	FileRevision string `json:"file_revision"`
	Content      string `json:"content"`
	Shared       bool   `json:"shared"`
}

func fileRevision(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func (s *service) documentTarget(c *catalog, name, id string) (Space, Field, string, error) {
	e, err := c.findEntry(name)
	if err != nil {
		return Space{}, Field{}, "", err
	}
	if e.trashed {
		return Space{}, Field{}, "", fmt.Errorf("restore this Space before editing its document")
	}
	sp, err := s.space(c, e)
	if err != nil {
		return sp, Field{}, "", err
	}
	for _, f := range sp.Fields {
		if f.ID != id {
			continue
		}
		if f.Type != "asset" || f.Edit == nil || f.Edit.Type != "markdown" {
			return sp, f, "", fmt.Errorf("document editing is not permitted by the source")
		}
		p, err := s.assetPath(f, f.Value)
		if err != nil {
			return sp, f, "", err
		}
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".md" && ext != ".markdown" {
			return sp, f, "", fmt.Errorf("only Markdown documents are editable")
		}
		return sp, f, p, nil
	}
	return sp, Field{}, "", fmt.Errorf("unknown source-owned document field")
}

// Relations come from declared asset fields, not arbitrary strings or a registry.
func (s *service) fileUsages(c *catalog, target string) ([]FileUsage, error) {
	snap, err := s.snapshot(c)
	if err != nil {
		return nil, err
	}
	usages := []FileUsage{}
	instances := map[string]bool{}
	add := func(name, title, kind string, trashed bool, fields []Field) {
		for _, f := range fields {
			if f.Type != "asset" || f.Value == "" {
				continue
			}
			p, err := s.assetLocation(f, f.Value)
			if err == nil && p == target {
				usages = append(usages, FileUsage{Name: name, Title: title, Field: f.ID, Label: f.Label, Kind: kind, Trashed: trashed, policy: f})
			}
		}
	}
	for _, sp := range snap.Spaces {
		instances[sp.Name] = true
		add(sp.Name, sp.Title, "space", sp.Trashed, sp.Fields)
	}
	for _, src := range snap.Sources {
		if !instances[src.Name] {
			add(src.Name, src.Title, "source", false, src.Fields)
		}
	}
	sort.Slice(usages, func(i, j int) bool {
		if usages[i].Name != usages[j].Name {
			return usages[i].Name < usages[j].Name
		}
		return usages[i].Field < usages[j].Field
	})
	return usages, nil
}

func (s *service) document(c *catalog, name, id string) (Document, error) {
	_, f, p, err := s.documentTarget(c, name, id)
	if err != nil {
		return Document{}, err
	}
	b, err := s.readAssetFile(p, f.Edit.MaxBytes)
	if err != nil {
		return Document{}, err
	}
	if err := validateAsset(b, strings.ToLower(filepath.Ext(p))); err != nil {
		return Document{}, err
	}
	usages, err := s.fileUsages(c, p)
	if err != nil {
		return Document{}, err
	}
	doc := Document{Revision: c.revision, FileRevision: fileRevision(b), Reference: f.Value, Filename: filepath.Base(p), Content: string(b), MaxBytes: f.Edit.MaxBytes, Usages: usages}
	for _, use := range usages {
		if use.Name != name || use.Field != id {
			doc.Shared = true
		}
	}
	if err := s.verify(c); err != nil {
		return Document{}, err
	}
	return doc, nil
}

func (s *service) renderDocumentPreview(content []byte) string {
	body := markdown.Render(content)
	// Display inside a sandboxed iframe. The document cannot load remote assets,
	// execute scripts, submit forms, or navigate its parent editor.
	return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'self'; base-uri 'none'; form-action 'none'"><link rel="stylesheet" href="` + html.EscapeString(s.route+"/web/document.css") + `"></head><body>` + body + `</body></html>`
}

func (s *service) mutateDocument(c *catalog, m DocumentMutation) (Document, string, error) {
	if !s.write {
		return Document{}, "", &statusError{403, "Spaces is read-only"}
	}
	if m.Revision == "" || m.Revision != c.revision {
		return Document{}, "", conflict("Source files changed; review the current document and its references")
	}
	doc, err := s.document(c, m.Name, m.Field)
	if err != nil {
		return doc, "", err
	}
	if m.FileRevision == "" || m.FileRevision != doc.FileRevision {
		return doc, "", conflict("The Markdown file changed; compare your draft with the saved document")
	}
	sp, f, p, err := s.documentTarget(c, m.Name, m.Field)
	if err != nil {
		return doc, "", err
	}
	b := []byte(m.Content)
	if int64(len(b)) > doc.MaxBytes {
		return doc, "", &statusError{413, "document exceeds the source editing limit"}
	}
	if f.Upload != nil && int64(len(b)) > f.Upload.MaxBytes {
		return doc, "", &statusError{413, "document exceeds the field's asset limit"}
	}
	if err := validateAsset(b, strings.ToLower(filepath.Ext(p))); err != nil {
		return doc, "", err
	}
	if m.Action == "preview" {
		return doc, s.renderDocumentPreview(b), nil
	}
	if m.Action != "save" && m.Action != "copy" {
		return doc, "", fmt.Errorf("unknown document action")
	}
	if m.Action == "save" {
		if doc.Shared && !m.Shared {
			return doc, "", conflict("This document is shared; confirm editing all known references or make a copy")
		}
		for _, use := range doc.Usages {
			if use.policy.Upload != nil && int64(len(b)) > use.policy.Upload.MaxBytes {
				return doc, "", fmt.Errorf("document exceeds the asset limit of %s / %s", use.Name, use.Label)
			}
		}
	}
	if err := s.verify(c); err != nil {
		return doc, "", err
	}
	current, err := s.readAssetFile(p, f.Edit.MaxBytes)
	if err != nil {
		return doc, "", err
	}
	if fileRevision(current) != m.FileRevision {
		return doc, "", conflict("The Markdown file changed before saving")
	}
	if m.Action == "save" {
		if err := s.replaceFile(p, b, current); err != nil {
			return doc, "", err
		}
	} else {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return doc, "", err
		}
		d := fieldDirectory(f)
		rel := path.Join(d.Path, hex.EncodeToString(random)+strings.ToLower(filepath.Ext(p)))
		target, err := contained(s.dirs[d.Base], rel)
		if err != nil {
			return doc, "", err
		}
		ref := rel
		if d.Base == "static" {
			ref = "/static/" + ref
		}
		m := Mutation{Action: "save", Revision: c.revision, Name: sp.Name, Title: sp.Title, Route: sp.Route}
		// Copy permission comes from edit, independently of upload permission.
		if err := s.mutate(c, m, &pendingUpload{path: target, reference: ref, field: f.ID, data: b}); err != nil {
			return doc, "", err
		}
	}
	fresh, err := s.catalog()
	if err != nil {
		return doc, "", fmt.Errorf("document saved, but catalog refresh failed: %w", err)
	}
	result, err := s.document(fresh, m.Name, m.Field)
	if err != nil {
		return doc, "", fmt.Errorf("document saved, but reload failed: %w", err)
	}
	return result, "", nil
}
