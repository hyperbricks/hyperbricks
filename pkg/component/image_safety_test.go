package component

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"golang.org/x/net/html"
)

func imageTestDirectories(t *testing.T) string {
	t.Helper()
	shared.Init_configuration()
	config := shared.GetHyperBricksConfiguration()
	previous := config.Directories
	root := t.TempDir()
	config.Directories = map[string]string{"static": filepath.Join(root, "static"), "render": filepath.Join(root, "rendered")}
	t.Cleanup(func() { config.Directories = previous })
	return root
}

func writeTestImage(t *testing.T, path string, shade color.RGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 12, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 12; x++ {
			img.SetRGBA(x, y, shade)
		}
	}
	if strings.EqualFold(filepath.Ext(path), ".jpg") {
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
	} else {
		err = png.Encode(f, img)
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("write image: %v / %v", err, closeErr)
	}
}

func parsedImageAttributes(t *testing.T, markup string) map[string]string {
	t.Helper()
	tokenizer := html.NewTokenizer(strings.NewReader(markup))
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			t.Fatalf("missing img in %s", markup)
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data != "img" {
				continue
			}
			attrs := map[string]string{}
			for _, attr := range token.Attr {
				if _, exists := attrs[attr.Key]; exists {
					t.Fatalf("duplicate %s attribute in %s", attr.Key, markup)
				}
				attrs[attr.Key] = attr.Val
			}
			return attrs
		}
	}
}

func TestImageEscapesAttributesAndServesFromNestedRoute(t *testing.T) {
	root := imageTestDirectories(t)
	src := filepath.Join(root, `cat " & #?.PNG`)
	writeTestImage(t, src, color.RGBA{R: 255, A: 255})
	payload := `Cat " onerror="alert(1)" & <tag> '`
	processor := &ImageProcessor{}
	markup, err := processor.ProcessSingleImage(SingleImageConfig{
		Src: src, Width: 4, Alt: payload, Title: payload, Class: payload, Id: payload, Loading: "eager",
		Component: shared.Component{ExtraAttributes: map[string]interface{}{
			"srcset": payload, "sizes": payload, "onerror": "alert(2)", "class": "ignored", "loading": "lazy",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	attrs := parsedImageAttributes(t, markup)
	for _, key := range []string{"alt", "title", "class", "id", "srcset", "sizes"} {
		if attrs[key] != payload {
			t.Fatalf("%s did not round-trip as one attribute: %q", key, attrs[key])
		}
	}
	if _, exists := attrs["onerror"]; exists || len(attrs) != 10 {
		t.Fatalf("unexpected injected attributes: %#v", attrs)
	}
	if attrs["width"] != "4" || attrs["height"] != "2" || attrs["loading"] != "eager" {
		t.Fatalf("incorrect dimensions/loading: %#v", attrs)
	}
	base, _ := url.Parse("http://example.test/guides/nested/page")
	asset, err := url.Parse(attrs["src"])
	if err != nil || !strings.HasPrefix(attrs["src"], "/static/images/") || asset.RawQuery != "" || asset.Fragment != "" {
		t.Fatalf("invalid public URL: %q, %v", attrs["src"], err)
	}
	request := httptest.NewRequest(http.MethodGet, base.ResolveReference(asset).String(), nil)
	response := httptest.NewRecorder()
	http.FileServer(http.Dir(root)).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("nested route image returned %d: %s", response.Code, response.Body.String())
	}
	decoded, _, err := image.Decode(response.Body)
	if err != nil || decoded.Bounds().Dx() != 4 || decoded.Bounds().Dy() != 2 {
		t.Fatalf("incorrect served image: %v", err)
	}
}

func TestImageGeneratedNamesTrackContentAndEncoding(t *testing.T) {
	root := imageTestDirectories(t)
	a, b := filepath.Join(root, "a", "photo.jpg"), filepath.Join(root, "b", "photo.jpg")
	writeTestImage(t, a, color.RGBA{R: 255, A: 255})
	writeTestImage(t, b, color.RGBA{B: 255, A: 255})
	processor := &ImageProcessor{}
	dest := filepath.Join(root, "output")
	generate := func(path string, width, quality int) string {
		t.Helper()
		name, err := processor.processImage(path, dest, SingleImageConfig{Width: width, Quality: quality})
		if err != nil {
			t.Fatal(err)
		}
		return name
	}
	first := generate(a, 4, 0)
	original, err := os.ReadFile(filepath.Join(dest, first))
	if err != nil {
		t.Fatal(err)
	}
	if again := generate(a, 4, 90); again != first {
		t.Fatal("equivalent settings changed the generated URL")
	}
	names := []string{first, generate(b, 4, 0), generate(a, 4, 60), generate(a, 8, 0)}
	writeTestImage(t, a, color.RGBA{G: 255, A: 255})
	names = append(names, generate(a, 4, 0))
	seen := map[string]bool{}
	for _, name := range names {
		if seen[name] {
			t.Fatalf("different sources or settings collided at %s", name)
		}
		seen[name] = true
		f, err := os.Open(filepath.Join(dest, name))
		if err != nil {
			t.Fatal(err)
		}
		_, _, decodeErr := image.Decode(f)
		f.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
	}
	retained, _ := os.ReadFile(filepath.Join(dest, first))
	if string(retained) != string(original) {
		t.Fatal("a new generation overwrote an earlier page's image")
	}
}

func TestImageConcurrentGenerationPublishesCompleteFiles(t *testing.T) {
	root := imageTestDirectories(t)
	src := filepath.Join(root, "photo.png")
	writeTestImage(t, src, color.RGBA{R: 180, A: 255})
	dest := filepath.Join(root, "output")
	processor := &ImageProcessor{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			name, err := processor.processImage(src, dest, SingleImageConfig{Width: 4})
			if err != nil {
				t.Error(err)
				return
			}
			f, err := os.Open(filepath.Join(dest, name))
			if err != nil {
				t.Error(err)
				return
			}
			defer f.Close()
			if _, _, err := image.Decode(f); err != nil {
				t.Errorf("incomplete published image: %v", err)
			}
		})
	}
	wg.Wait()
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one output and no temporary files: %v, %v", entries, err)
	}
}

func TestImageLongSourceNamesRemainPublishable(t *testing.T) {
	root := imageTestDirectories(t)
	processor := &ImageProcessor{}
	for _, basename := range []string{strings.Repeat("a", 230), strings.Repeat("猫", 72)} {
		src := filepath.Join(root, basename+".png")
		writeTestImage(t, src, color.RGBA{A: 255})
		markup, err := processor.ProcessSingleImage(SingleImageConfig{Src: src, Width: 4})
		if err != nil {
			t.Fatal(err)
		}
		attrs := parsedImageAttributes(t, markup)
		asset, err := url.Parse(attrs["src"])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(root, asset.Path)); err != nil {
			t.Fatalf("truncated source name is not reachable: %v", err)
		}
	}
}

func TestImageGalleryReportsInvalidFiles(t *testing.T) {
	root := imageTestDirectories(t)
	dir := filepath.Join(root, "gallery")
	writeTestImage(t, filepath.Join(dir, "a.png"), color.RGBA{A: 255})
	if err := os.Mkdir(filepath.Join(dir, "nested.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}
	processor := &ImageProcessor{}
	markup, err := processor.ProcessMultipleImages(MultipleImagesConfig{Directory: dir, Width: 4})
	if err != nil || strings.Count(markup, "<img ") != 1 {
		t.Fatalf("gallery should skip directories and unsupported files: %s, %v", markup, err)
	}
	attrs := parsedImageAttributes(t, markup)
	if _, exists := attrs["id"]; exists {
		t.Fatal("gallery must not invent ids without a configured base")
	}
	if value, exists := attrs["alt"]; !exists || value != "" {
		t.Fatal("decorative image needs an empty alt attribute")
	}
	for _, name := range []string{"b.png", "c.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("invalid image"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	renderer := &MultipleImagesRenderer{ImageProcessorInstance: processor}
	markup, errs := renderer.Render(MultipleImagesConfig{Directory: dir, Width: 4}, context.Background())
	if markup != "" || len(errs) != 1 {
		t.Fatalf("expected gallery failure, got %q, %v", markup, errs)
	}
	componentErr, ok := errs[0].(shared.ComponentError)
	if !ok || !componentErr.Rejected || !strings.Contains(componentErr.Err, "b.png") || !strings.Contains(componentErr.Err, "c.jpg") {
		t.Fatalf("gallery diagnostics must identify both invalid files: %v", errs)
	}
}

func TestImageRejectsInvalidDimensionsAndQuality(t *testing.T) {
	root := imageTestDirectories(t)
	src := filepath.Join(root, "photo.png")
	writeTestImage(t, src, color.RGBA{A: 255})
	renderer := &SingleImageRenderer{ImageProcessorInstance: &ImageProcessor{}}
	for _, config := range []SingleImageConfig{
		{Src: src, Width: -1}, {Src: src, Height: -1}, {Src: src, Quality: -1}, {Src: src, Quality: 101},
	} {
		markup, errs := renderer.Render(config, context.Background())
		if markup != "" || len(errs) == 0 {
			t.Fatalf("invalid config rendered: %#v => %q, %v", config, markup, errs)
		}
	}
	markup, errs := renderer.Render(SingleImageConfig{Src: src}, context.Background())
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	attrs := parsedImageAttributes(t, markup)
	if attrs["width"] != "12" || attrs["height"] != "6" {
		t.Fatalf("omitted dimensions did not preserve source size: %#v", attrs)
	}
}
