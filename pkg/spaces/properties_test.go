package spaces

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/component"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

const nativeImageSource = `portfolio_home_image:
  - type: image
  - src: {path: {base: resources, path: images/original.png}}
  - width: 6
  - quality: 90
  - alt: Original painting
  - class: home-artwork-image
  - editable:
      src:
        type: asset
        label: Home image
        upload:
          directory: {base: resources, path: images}
          accept: [.png, .jpg, .jpeg, .gif]
          max_bytes: 10000
      alt: text
portfolio_page:
  - type: hypermedia
  - title: Home
  - template:
      - type: template
      - inline: '{{.content}}'
      - values:
          content:
            - type: template
            - inline: '{{.image}}{{.caption}}{{.details.subtitle}}'
            - values:
                image:
                  - inherit: portfolio_home_image
                caption: Original caption
                details:
                  subtitle: Selected work
                  'a.b/~': Punctuation
                  '0': Numeric key
            - editable:
                caption: text
                subtitle: {path: [details, subtitle], type: text}
                punctuation: {path: [details, 'a.b/~'], type: text}
                numeric_key: {path: [details, '0'], type: text}
`

var mountedImage = []string{"template", "values", "content", "values", "image"}

func originalPNG(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, color.RGBA{R: shade, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func imageService(t *testing.T) *service {
	t.Helper()
	s := testService(t)
	writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), nativeImageSource)
	for i, name := range []string{"original.png", "selected.png"} {
		file := filepath.Join(s.dirs["resources"], "images", name)
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, originalPNG(t, uint8(i*100)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestNativeImagePropertyWorkflow(t *testing.T) {
	s := imageService(t)
	createTest(t, s, "first")
	createTest(t, s, "second")
	c, sp := currentSpace(t, s, "first")
	id := fieldID(t, sp, "src")
	if id != "/template/values/content/values/image/src" {
		t.Fatal(id)
	}
	if len(sp.Fields) != 6 {
		t.Fatalf("unexpected fields: %+v", sp.Fields)
	}
	if fieldID(t, sp, "punctuation") != "/template/values/content/values/details/a.b~1~0" {
		t.Fatal("pointer escaping")
	}
	file := filepath.Join(s.dirs["hyperbricks"], sp.File)
	sourceFile := filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml")
	sourceBefore := fileText(t, sourceFile)
	originalBefore := fileText(t, filepath.Join(s.dirs["resources"], "images/original.png"))
	sibling := filepath.Join(s.dirs["hyperbricks"], "spaces/portfolio_page/second.hyperbricks.yaml")
	siblingBefore := fileText(t, sibling)
	assets, err := s.assetList(c, sp.Name, id)
	if err != nil || len(assets) != 2 {
		t.Fatalf("assets %+v: %v", assets, err)
	}
	mutation := saveMutation(c, sp)
	mutation.Values[id] = "images/selected.png"
	mutation.Values[fieldID(t, sp, "subtitle")] = "Updated nested data"
	mutation.Values[fieldID(t, sp, "alt")] = "Selected painting"
	if err := s.mutate(c, mutation, nil); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "first")
	verify := func(reference string) {
		t.Helper()
		for _, f := range sp.Fields {
			if f.ID == id && (f.Value != reference || f.Default != "images/original.png") {
				t.Fatalf("field %+v", f)
			}
		}
		imageNode := getMap(c.defs[sp.Name].effective, mountedImage).(map[string]interface{})
		if str(imageNode["width"]) != "6" || str(imageNode["quality"]) != "90" || imageNode["class"] != "home-artwork-image" {
			t.Fatalf("inherited settings lost: %+v", imageNode)
		}
		if imageNode["src"] != filepath.Join(s.dirs["resources"], filepath.FromSlash(reference)) {
			t.Fatalf("unresolved source: %v", imageNode["src"])
		}
		if fileText(t, sourceFile) != sourceBefore || fileText(t, sibling) != siblingBefore || fileText(t, filepath.Join(s.dirs["resources"], "images/original.png")) != originalBefore {
			t.Fatal("source, sibling, or original changed")
		}
		raw := fileText(t, file)
		if strings.Contains(raw, s.module) || !strings.Contains(raw, "base: resources") {
			t.Fatalf("nonportable override: %s", raw)
		}
		doc, err := parseYAML([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if at(child(doc, sp.Name), append(append([]string{}, mountedImage...), "width")) != nil {
			t.Fatal("copied inherited processing settings")
		}
	}
	verify("images/selected.png")
	if getMap(c.defs[sp.Name].effective, []string{"template", "values", "content", "values", "caption"}) != "Original caption" {
		t.Fatal("caption changed")
	}
	renderImageField(t, s, c.defs[sp.Name].effective)
	upload, err := s.prepareUpload(c, sp.Name, id, "new.png", originalPNG(t, 220))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.mutate(c, saveMutation(c, sp), upload); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "first")
	verify(upload.reference)
	if got := fileText(t, upload.path); got != string(upload.data) {
		t.Fatal("uploaded original changed")
	}
	renderImageField(t, s, c.defs[sp.Name].effective)
	// Removing just src restores the inherited source; other overrides survive.
	doc, err := parseYAML([]byte(fileText(t, file)))
	if err != nil {
		t.Fatal(err)
	}
	remove(at(child(doc, sp.Name), mountedImage), "src")
	raw, err := encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, raw, 0644); err != nil {
		t.Fatal(err)
	}
	c, sp = currentSpace(t, s, "first")
	for _, f := range sp.Fields {
		if f.ID == id && (f.Value != "images/original.png" || f.Value != f.Default) {
			t.Fatalf("reset: %+v", f)
		}
	}
	if getMap(c.defs[sp.Name].effective, append(append([]string{}, mountedImage...), "alt")) != "Selected painting" {
		t.Fatal("reset lost other override")
	}
	renderImageField(t, s, c.defs[sp.Name].effective)
}

func renderImageField(t *testing.T, s *service, root map[string]interface{}) {
	t.Helper()
	shared.Init_configuration()
	cfg := shared.GetHyperBricksConfiguration()
	previous := cfg.Directories
	cfg.Directories = s.dirs
	defer func() { cfg.Directories = previous }()
	var config component.SingleImageConfig
	if err := mapstructure.WeakDecode(getMap(root, mountedImage), &config); err != nil {
		t.Fatal(err)
	}
	renderer := &component.SingleImageRenderer{ImageProcessorInstance: &component.ImageProcessor{}}
	markup, errs := renderer.Render(config, nil)
	if len(errs) > 0 || !strings.Contains(markup, `class="home-artwork-image"`) {
		t.Fatalf("render %q: %v", markup, errs)
	}
	start := strings.Index(markup, `src="`) + len(`src="`)
	if start < len(`src="`) {
		t.Fatal(markup)
	}
	url := strings.Split(markup[start:], `"`)[0]
	if !strings.HasPrefix(url, "/static/images/") {
		t.Fatal(url)
	}
	output, err := os.Open(filepath.Join(s.dirs["static"], strings.TrimPrefix(url, "/static/")))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	image, _, err := image.DecodeConfig(output)
	if err != nil || image.Width != 6 || image.Height != 4 {
		t.Fatalf("processed dimensions %+v: %v", image, err)
	}
}

func TestNativeImagePropertyRejectedMutations(t *testing.T) {
	s := imageService(t)
	createTest(t, s, "first")
	c, sp := currentSpace(t, s, "first")
	id := fieldID(t, sp, "src")
	file := filepath.Join(s.dirs["hyperbricks"], sp.File)
	before := fileText(t, file)
	for _, ref := range []string{"", "../outside.png", "/static/image.png", "images/../original.png", "images/missing.png", "images/file.webp", "images/file.md", s.dirs["resources"] + "/images/selected.png"} {
		m := saveMutation(c, sp)
		m.Values[id] = ref
		if err := s.mutate(c, m, nil); err == nil {
			t.Errorf("accepted %q", ref)
		}
		if fileText(t, file) != before {
			t.Fatal("rejected save modified YAML")
		}
	}
	for _, name := range []string{"bad.webp", "bad.md", "bad.png"} {
		if _, err := s.prepareUpload(c, sp.Name, id, name, []byte("invalid image")); err == nil {
			t.Errorf("accepted %s", name)
		}
	}
	m := saveMutation(c, sp)
	m.Values["/template/values/content/values/image/width"] = "100"
	if err := s.mutate(c, m, nil); err == nil {
		t.Fatal("undeclared property editable")
	}
	if fileText(t, file) != before {
		t.Fatal("invalid mutation wrote YAML")
	}
}

func TestExplicitPropertyPathValidation(t *testing.T) {
	good := func() map[string]interface{} {
		return map[string]interface{}{"@type": "<TEMPLATE>", "values": map[string]interface{}{"data": map[string]interface{}{"name": "ok", "count": 2, "list": []interface{}{"a"}}}, "editable": map[string]interface{}{"label": map[string]interface{}{"path": []interface{}{"data", "name"}, "type": "text"}}}
	}
	root := good()
	fields, err := SourceFields(root)
	if err != nil || len(fields) != 1 || fields[0].ID != "/values/data/name" || fields[0].Value != "ok" {
		t.Fatalf("%+v: %v", fields, err)
	}
	for _, tc := range []struct {
		name string
		path interface{}
	}{
		{"empty", []interface{}{}}, {"string", "data.name"}, {"number", []interface{}{"data", 0}}, {"missing", []interface{}{"data", "missing"}},
		{"scalar-parent", []interface{}{"data", "name", "x"}}, {"map", []interface{}{"data"}}, {"array", []interface{}{"data", "list"}}, {"array-index", []interface{}{"data", "list", "0"}},
		{"numeric-target", []interface{}{"data", "count"}}, {"structural", []interface{}{"data", "@type"}}, {"editable", []interface{}{"data", "editable"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := good()
			root["editable"].(map[string]interface{})["label"].(map[string]interface{})["path"] = tc.path
			if _, err := SourceFields(root); err == nil {
				t.Fatal("accepted invalid path")
			}
		})
	}
	root = good()
	definitions := root["editable"].(map[string]interface{})
	definitions["alias"] = definitions["label"]
	if _, err := SourceFields(root); err == nil || !strings.Contains(err.Error(), "duplicate editable target") {
		t.Fatalf("duplicate: %v", err)
	}
	// A path into a native component must obey its property/control contract.
	for _, typ := range []string{"<IMAGE>", "<MARKDOWN>", "<MENU>"} {
		root = good()
		root["values"].(map[string]interface{})["data"] = map[string]interface{}{"@type": typ, "name": "value"}
		if _, err := SourceFields(root); err == nil {
			t.Fatalf("bypassed %s policy", typ)
		}
	}
}

func TestNativeImagePropertyContractRestrictions(t *testing.T) {
	for _, tc := range []struct{ name, old, new string }{
		{"webp", "accept: [.png, .jpg, .jpeg, .gif]", "accept: [.webp]"},
		{"static", "directory: {base: resources, path: images}", "directory: {base: static, path: images}"},
		{"width-negative", "width: 6", "width: -1"}, {"width-fraction", "width: 6", "width: 1.5"}, {"quality", "quality: 90", "quality: 101"},
		{"nonasset", "type: asset", "type: text"}, {"alt-asset", "alt: text", "alt: asset"},
		{"unknown", "alt: text", "unknown: text"}, {"numeric-control", "alt: text", "width: text"},
		{"relative-source", "src: {path: {base: resources, path: images/original.png}}", "src: images/original.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := imageService(t)
			writeTestFile(t, filepath.Join(s.dirs["hyperbricks"], "portfolio.hyperbricks.yaml"), strings.Replace(nativeImageSource, tc.old, tc.new, 1))
			c := catalogTest(t, s)
			if _, err := s.snapshot(c); err == nil {
				t.Fatal("accepted invalid source")
			}
		})
	}
}

func TestNativeImageInstanceCannotBroadenContract(t *testing.T) {
	s := imageService(t)
	createTest(t, s, "first")
	c, sp := currentSpace(t, s, "first")
	file := filepath.Join(s.dirs["hyperbricks"], sp.File)
	doc, err := parseYAML([]byte(fileText(t, file)))
	if err != nil {
		t.Fatal(err)
	}
	image := at(child(doc, sp.Name), mountedImage)
	permissions := mapping()
	put(permissions, "width", scalar("text"))
	put(image, "editable", permissions)
	raw, err := encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, file, string(raw))
	c, sp = currentSpace(t, s, "first")
	for _, f := range sp.Fields {
		if f.Key == "width" {
			t.Fatal("instance granted rights")
		}
	}
	m := saveMutation(c, sp)
	m.Values["/template/values/content/values/image/width"] = "5"
	if err := s.mutate(c, m, nil); err == nil {
		t.Fatal("instance broadened source rights")
	}
	// An instance cannot change the native target into a different component.
	put(image, "type", scalar("template"))
	raw, _ = encode(doc)
	writeTestFile(t, file, string(raw))
	c = catalogTest(t, s)
	entry, _ := c.findEntry(sp.Name)
	if _, err := s.space(c, entry); err == nil {
		t.Fatal("accepted changed target semantics")
	}
}

func TestNativeImageHTTPSelectionReload(t *testing.T) {
	s := imageService(t)
	createTest(t, s, "first")
	c, sp := currentSpace(t, s, "first")
	m := saveMutation(c, sp)
	m.Values[fieldID(t, sp, "src")] = "images/selected.png"
	raw, _ := json.Marshal(m)
	request := httptest.NewRequest(http.MethodPost, "http://localhost"+s.route+"/api", bytes.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Spaces-Request", "1")
	response := httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("save %d: %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	s.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost"+s.route+"/api", nil))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var snapshot Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Spaces) != 1 {
		t.Fatal(snapshot)
	}
	for _, field := range snapshot.Spaces[0].Fields {
		if field.Key == "src" && field.Value != "images/selected.png" {
			t.Fatal(field)
		}
	}
}

func BenchmarkNestedPropertyFields(b *testing.B) {
	for _, count := range []int{10, 100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			values, definitions := map[string]interface{}{}, map[string]interface{}{}
			for i := 0; i < count; i++ {
				key := fmt.Sprintf("item%d", i)
				values[key] = map[string]interface{}{"label": "value"}
				definitions[key] = map[string]interface{}{"path": []interface{}{key, "label"}, "type": "text"}
			}
			root := map[string]interface{}{"@type": "<TEMPLATE>", "values": values, "editable": definitions}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				fields, err := SourceFields(root)
				if err != nil || len(fields) != count {
					b.Fatal(fields, err)
				}
			}
		})
	}
}

func TestLegacyTemplateEditableWithoutDefaults(t *testing.T) {
	root := map[string]interface{}{"@type": "<TEMPLATE>", "editable": []interface{}{"heading"}}
	fields, err := SourceFields(root)
	if err != nil || len(fields) != 1 || fields[0].Value != "" || fields[0].ID != "/values/heading" {
		t.Fatalf("legacy contract changed: %+v %v", fields, err)
	}
}

func TestExplicitPathUsesNativeImageAdapter(t *testing.T) {
	s := imageService(t)
	root := catalogTest(t, s).defs["portfolio_page"].effective
	mounted := getMap(root, mountedImage).(map[string]interface{})
	delete(mounted, "editable")
	container := getMap(root, []string{"template", "values", "content"}).(map[string]interface{})
	container["editable"].(map[string]interface{})["hero"] = map[string]interface{}{
		"path": []interface{}{"image", "src"}, "type": "asset", "directory": map[string]interface{}{"base": "resources", "path": "images"},
	}
	fields, err := SourceFields(root, s.dirs)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, field := range fields {
		if field.Key == "hero" {
			found = true
			if !field.imageSource || field.ID != "/template/values/content/values/image/src" || field.Value != "images/original.png" {
				t.Fatal(field)
			}
		}
	}
	if !found {
		t.Fatal("missing image alias")
	}
}

func TestNativeImageResourcePreviewAuthorization(t *testing.T) {
	s := imageService(t)
	createTest(t, s, "first")
	c, sp := currentSpace(t, s, "first")
	id := fieldID(t, sp, "src")
	assets, err := s.assetList(c, sp.Name, id)
	if err != nil || len(assets) != 2 {
		t.Fatalf("%+v %v", assets, err)
	}
	for _, asset := range assets {
		if !strings.HasPrefix(asset.Preview, s.route+"/api/asset?") {
			t.Fatal(asset)
		}
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			response := httptest.NewRecorder()
			s.ServeHTTP(response, httptest.NewRequest(method, "http://localhost"+asset.Preview, nil))
			if response.Code != 200 || response.Header().Get("Content-Type") != "image/png" || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("preview: %d %v", response.Code, response.Header())
			}
			if method == http.MethodGet && response.Body.String() != fileText(t, filepath.Join(s.dirs["resources"], asset.Reference)) {
				t.Fatal("preview altered original")
			}
			if method == http.MethodHead && response.Body.Len() != 0 {
				t.Fatal("HEAD has body")
			}
		}
	}
	for _, query := range []string{
		"name=first&field=" + id + "&reference=../outside.png",
		"name=first&field=" + id + "&reference=other/image.png",
		"name=first&field=" + id + "&reference=images/file.md",
		"name=first&field=/undeclared&reference=images/original.png",
		"name=missing&field=" + id + "&reference=images/original.png",
	} {
		response := httptest.NewRecorder()
		s.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://localhost"+s.route+"/api/asset?"+query, nil))
		if response.Code == 200 {
			t.Fatalf("unauthorized preview: %s", query)
		}
	}
}
