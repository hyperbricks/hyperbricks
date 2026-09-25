package spaces

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	_ "golang.org/x/image/webp"
)

type Asset struct {
	Reference string `json:"reference"`
	Preview   string `json:"preview,omitempty"`
	Name      string `json:"name"`
	Image     bool   `json:"image"`
	Size      int64  `json:"size"`
}
type pendingUpload struct {
	path, reference, field string
	data                   []byte
}

func validateAsset(data []byte, ext string) error {
	if !supportedExtension(ext) {
		return fmt.Errorf("unsupported file extension")
	}
	if ext == ".md" || ext == ".markdown" {
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return fmt.Errorf("Markdown must be a UTF-8 text document")
		}
		for _, b := range data {
			if b < 32 && b != '\n' && b != '\r' && b != '\t' {
				return fmt.Errorf("Markdown contains binary control characters")
			}
		}
		return nil
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("file content is not a supported image")
	}
	expected := strings.TrimPrefix(ext, ".")
	if expected == "jpg" {
		expected = "jpeg"
	}
	if format != expected {
		return fmt.Errorf("file content does not match %s", ext)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 40000000 {
		return fmt.Errorf("image exceeds 40 million pixels")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		return fmt.Errorf("invalid or incomplete image: %w", err)
	}
	return nil
}
func (s *service) assetPath(f Field, ref string) (string, error) {
	p, err := s.assetLocation(f, ref)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(p))
	limit := int64(20 << 20)
	if f.markdownFile {
		limit = f.markdownMaxBytes
	}
	if f.Upload != nil {
		limit = f.Upload.MaxBytes
		allowed := false
		for _, e := range f.Upload.Accept {
			if e == ext {
				allowed = true
			}
		}
		if !allowed {
			return "", fmt.Errorf("extension is not permitted by this field")
		}
	}
	data, err := s.readAssetFile(p, limit)
	if err != nil {
		return "", err
	}
	if err := validateAsset(data, ext); err != nil {
		return "", err
	}
	return p, nil
}

func (s *service) assetLocation(f Field, ref string) (string, error) {
	d := fieldDirectory(f)
	if err := validateDirectory(d); err != nil {
		return "", err
	}
	rel := ref
	if d.Base == "static" {
		if !strings.HasPrefix(ref, "/static/") {
			return "", fmt.Errorf("select an asset from static")
		}
		rel = strings.TrimPrefix(ref, "/static/")
	}
	if d.Path != "" && !strings.HasPrefix(rel, d.Path+"/") {
		return "", fmt.Errorf("asset is outside the field's directory")
	}
	p, err := contained(s.dirs[d.Base], rel)
	if err != nil {
		return "", err
	}
	ext := strings.ToLower(filepath.Ext(p))
	if f.markdownFile && (filepath.Ext(p) != ext || (ext != ".md" && ext != ".markdown")) {
		return "", fmt.Errorf("select a .md or .markdown document")
	}
	if !supportedExtension(ext) {
		return "", fmt.Errorf("unsupported asset extension")
	}
	return p, nil
}

func (s *service) readAssetFile(p string, limit int64) ([]byte, error) {
	if _, err := contained(s.module, relative(s.module, p)); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.module)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(relative(s.module, p))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, fmt.Errorf("asset exceeds field size limit or is not a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("asset exceeds size limit")
	}
	return data, nil
}

func (s *service) assetField(c *catalog, name, id string) (Field, error) {
	if id == "@meta.og:image" {
		if s.sharingImage == nil {
			return Field{}, fmt.Errorf("sharing image asset policy is not configured")
		}
		if s.publicOrigin == "" {
			return Field{}, fmt.Errorf("set public_origin or enter an explicit sharing image URL")
		}
		if s.sharingImage.Directory.Base != "static" {
			return Field{}, fmt.Errorf("sharing images must use static storage")
		}
		return Field{ID: id, Key: "og:image", Type: "asset", Label: "Sharing image", Upload: s.sharingImage}, nil
	}
	e, err := c.findEntry(name)
	if err != nil {
		return Field{}, err
	}
	space, err := s.space(c, e)
	if err != nil {
		return Field{}, err
	}
	for _, f := range space.Fields {
		if f.ID == id && f.Type == "asset" {
			return f, nil
		}
	}
	return Field{}, fmt.Errorf("unknown source-owned asset field")
}

func (s *service) assetList(c *catalog, name, id string) ([]Asset, error) {
	f, err := s.assetField(c, name, id)
	if err != nil {
		return nil, err
	}
	d := fieldDirectory(f)
	root, err := contained(s.dirs[d.Base], emptyDot(d.Path))
	if err != nil {
		return nil, err
	}
	assets := []Asset{}
	count := 0
	err = filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if os.IsNotExist(err) && p == root {
			return fs.SkipDir
		}
		if err != nil {
			return err
		}
		count++
		if count > 10000 {
			return fmt.Errorf("asset directory exceeds 10000 entries; choose a narrower directory")
		}
		if e.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if e.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		if !supportedExtension(ext) {
			return nil
		}
		ref := relative(s.dirs[d.Base], p)
		if d.Base == "static" {
			ref = "/static/" + ref
		}
		if _, err := s.assetPath(f, ref); err != nil {
			return nil
		}
		st, err := e.Info()
		if err != nil {
			return err
		}
		preview := ""
		if d.Base == "static" {
			preview = ref
		}
		if id == "@meta.og:image" {
			ref = s.publicOrigin + ref
		}
		assets = append(assets, Asset{Reference: ref, Preview: preview, Name: e.Name(), Image: ext != ".md" && ext != ".markdown", Size: st.Size()})
		return nil
	})
	sort.Slice(assets, func(i, j int) bool { return assets[i].Reference < assets[j].Reference })
	return assets, err
}
func emptyDot(p string) string {
	if p == "" {
		return "."
	}
	return p
}

func (s *service) prepareUpload(c *catalog, name, id, filename string, data []byte) (*pendingUpload, error) {
	f, err := s.assetField(c, name, id)
	if err != nil {
		return nil, err
	}
	if f.Upload == nil {
		return nil, fmt.Errorf("uploads are not enabled for this field")
	}
	p := f.Upload
	if int64(len(data)) > p.MaxBytes {
		return nil, &statusError{413, "upload exceeds the source field's size limit"}
	}
	ext := strings.ToLower(filepath.Ext(filename))
	accepted := false
	for _, e := range p.Accept {
		if ext == e {
			accepted = true
		}
	}
	if !accepted {
		return nil, fmt.Errorf("file extension is not allowed for this field")
	}
	if err := validateAsset(data, ext); err != nil {
		return nil, err
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	namePart := hex.EncodeToString(random) + ext
	rel := path.Join(p.Directory.Path, namePart)
	target, err := contained(s.dirs[p.Directory.Base], rel)
	if err != nil {
		return nil, err
	}
	ref := rel
	if p.Directory.Base == "static" {
		ref = "/static/" + ref
	}
	if id == "@meta.og:image" {
		ref = s.publicOrigin + ref
	}
	return &pendingUpload{path: target, reference: ref, field: id, data: data}, nil
}

func publicOrigin(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("public_origin must be an explicit HTTP(S) origin without a path")
	}
	return u.Scheme + "://" + u.Host, nil
}
