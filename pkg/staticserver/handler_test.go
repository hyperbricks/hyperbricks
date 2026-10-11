package staticserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func writeFixture(t *testing.T, root, name, contents string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T, routing shared.RoutingConfig) (*Handler, string) {
	t.Helper()
	root := t.TempDir()
	for name, contents := range map[string]string{
		"index.html": "<main>Home</main>", "nested/index.htm": "<main>Nested</main>",
		"about.html": "<main>About</main>", "fragment.htm": "<p>Fragment</p>",
		"assets/app.css": "body { color: red; }", "assets/image.v1.svg": "<svg></svg>",
		"dotted.dir/page.html": "<main>Dotted directory</main>", "plain": "exact extensionless file",
		"priority": "exact candidate", "priority.html": "HTML candidate", "empty/unlisted.txt": "No listing",
		"welcome.txt": "Custom root index", "custom/home.txt": "Custom directory index",
	} {
		writeFixture(t, root, name, contents)
	}
	handler, err := New(root, routing)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := handler.Close(); err != nil {
			t.Error(err)
		}
	})
	return handler, root
}

func request(handler *Handler, method, target string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, target, nil))
	return response
}

func TestFilesIndexesAndCleanURLs(t *testing.T) {
	handler, _ := newFixture(t, shared.RoutingConfig{CleanURLs: true})
	for _, tc := range []struct{ target, content, mime string }{
		{"/", "<main>Home</main>", "text/html"},
		{"/index.html", "<main>Home</main>", "text/html"},
		{"/nested/", "<main>Nested</main>", "text/html"},
		{"/nested/index.htm", "<main>Nested</main>", "text/html"},
		{"/about?source=preview", "<main>About</main>", "text/html"},
		{"/fragment", "<p>Fragment</p>", "text/html"},
		{"/dotted.dir/page", "<main>Dotted directory</main>", "text/html"},
		{"/assets/app.css", "body { color: red; }", "text/css"},
		{"/assets/image.v1.svg", "<svg></svg>", "image/svg+xml"},
		{"/plain", "exact extensionless file", "text/plain"},
		{"/priority", "HTML candidate", "text/html"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			response := request(handler, http.MethodGet, tc.target)
			if response.Code != http.StatusOK || response.Body.String() != tc.content || !strings.HasPrefix(response.Header().Get("Content-Type"), tc.mime) {
				t.Fatalf("response = %d %q (%s)", response.Code, response.Body.String(), response.Header().Get("Content-Type"))
			}
		})
	}
}

func TestDirectoryRedirectPreservesQueryAndRequiresIndex(t *testing.T) {
	handler, root := newFixture(t, shared.RoutingConfig{CleanURLs: true})
	writeFixture(t, root, "space dir/index.html", "Space index")
	for _, tc := range []struct{ target, location string }{
		{"/nested?tag=one&tag=two", "/nested/?tag=one&tag=two"},
		{"/space%20dir?return=%2Fpage", "/space%20dir/?return=%2Fpage"},
	} {
		response := request(handler, http.MethodGet, tc.target)
		if response.Code != http.StatusMovedPermanently || response.Header().Get("Location") != tc.location {
			t.Fatalf("%s redirect = %d %q", tc.target, response.Code, response.Header().Get("Location"))
		}
	}
	for _, target := range []string{"/empty", "/empty/", "/missing", "/nested/missing", "/about.css", "/assets/"} {
		response := request(handler, http.MethodGet, target)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "unlisted") || strings.Contains(response.Body.String(), "<main>Home") {
			t.Fatalf("%s must return a real 404 without listing or SPA fallback: %d %q", target, response.Code, response.Body.String())
		}
	}
}

func TestCustomRoutingAndDisabledCleanURLs(t *testing.T) {
	routing := shared.RoutingConfig{IndexFiles: []string{" welcome.txt ", "/home.txt"}, Extensions: []string{" .txt "}}
	handler, _ := newFixture(t, routing)
	for _, tc := range []struct {
		target, body string
		status       int
	}{
		{"/", "Custom root index", 200}, {"/custom/", "Custom directory index", 200},
		{"/index.html", "<main>Home</main>", 200}, {"/priority", "exact candidate", 200},
		{"/about", "", 404}, {"/nested/", "", 404},
	} {
		response := request(handler, http.MethodGet, tc.target)
		if response.Code != tc.status || tc.body != "" && response.Body.String() != tc.body {
			t.Fatalf("%s = %d %q", tc.target, response.Code, response.Body.String())
		}
	}
	routing.CleanURLs = true
	clean, _ := newFixture(t, routing)
	if got := request(clean, http.MethodGet, "/welcome"); got.Code != 200 || got.Body.String() != "Custom root index" {
		t.Fatalf("custom extension = %d %q", got.Code, got.Body.String())
	}
	if got := request(clean, http.MethodGet, "/about"); got.Code != 404 {
		t.Fatalf("unexpected default extension with configured extensions: %d", got.Code)
	}
}

func TestHEADRangeAndConditionalRequests(t *testing.T) {
	handler, _ := newFixture(t, shared.RoutingConfig{CleanURLs: true})
	get := request(handler, http.MethodGet, "/about")
	head := request(handler, http.MethodHead, "/about")
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Length") != get.Header().Get("Content-Length") || head.Header().Get("Content-Type") != get.Header().Get("Content-Type") {
		t.Fatalf("HEAD differs from GET: %d %v %q", head.Code, head.Header(), head.Body.String())
	}
	for _, tc := range []struct {
		header, value string
		status        int
		body          string
	}{
		{"Range", "bytes=6-10", http.StatusPartialContent, "About"},
		{"Range", "bytes=1000-2000", http.StatusRequestedRangeNotSatisfiable, ""},
		{"If-Modified-Since", get.Header().Get("Last-Modified"), http.StatusNotModified, ""},
	} {
		req := httptest.NewRequest(http.MethodGet, "/about", nil)
		req.Header.Set(tc.header, tc.value)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != tc.status || tc.body != "" && response.Body.String() != tc.body {
			t.Fatalf("%s: %d %q", tc.header, response.Code, response.Body.String())
		}
		if tc.status == http.StatusNotModified && response.Body.Len() != 0 {
			t.Fatal("304 included a body")
		}
		if tc.status == http.StatusPartialContent && response.Header().Get("Content-Range") != "bytes 6-10/18" {
			t.Fatalf("range header = %q", response.Header().Get("Content-Range"))
		}
	}
}

func TestMethodsAndTraversalAreRejected(t *testing.T) {
	handler, root := newFixture(t, shared.RoutingConfig{CleanURLs: true})
	writeFixture(t, root, "private.html", "This must not be reached by traversal")
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions} {
		response := request(handler, method, "/")
		if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != "GET, HEAD" {
			t.Fatalf("%s = %d %v", method, response.Code, response.Header())
		}
	}
	for _, target := range []string{"/../private.html", "/%2e%2e/private.html", "/nested/../../private.html", "/nested/%2e%2e/private.html", "/nested/./index.htm", "//private.html", "/%2fprivate.html", "/nested%5c..%5cprivate.html", "/%00"} {
		response := request(handler, http.MethodGet, target)
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "This must not") {
			t.Fatalf("unsafe path %q = %d %q", target, response.Code, response.Body.String())
		}
	}
}

func TestSymlinkContainment(t *testing.T) {
	handler, root := newFixture(t, shared.RoutingConfig{CleanURLs: true})
	outside := t.TempDir()
	writeFixture(t, outside, "secret.html", "outside secret")
	writeFixture(t, outside, "index.html", "outside index")
	links := map[string]string{"inside.html": "about.html", "inside-dir": "nested", "outside.html": filepath.Join(outside, "secret.html"), "outside-dir": outside}
	outsideRelative, err := filepath.Rel(root, outside)
	if err != nil {
		t.Fatal(err)
	}
	links["outside-relative"] = outsideRelative
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("cannot create symlinks on this platform: %v", err)
		}
	}
	for _, tc := range []struct {
		target, body string
		status       int
	}{
		{"/inside", "<main>About</main>", 200}, {"/inside-dir/", "<main>Nested</main>", 200},
		{"/outside.html", "", 404}, {"/outside", "", 404}, {"/outside-dir/", "", 404},
		{"/outside-dir/secret", "", 404}, {"/outside-relative/secret.html", "", 404},
	} {
		response := request(handler, http.MethodGet, tc.target)
		if response.Code != tc.status || tc.body != "" && response.Body.String() != tc.body || strings.Contains(response.Body.String(), "outside") {
			t.Fatalf("symlink %s = %d %q", tc.target, response.Code, response.Body.String())
		}
	}
}

func TestConcurrentSymlinkReplacementCannotEscape(t *testing.T) {
	handler, root := newFixture(t, shared.RoutingConfig{CleanURLs: true})
	outside := t.TempDir()
	writeFixture(t, outside, "secret.html", "outside secret")
	link, replacement := filepath.Join(root, "changing.html"), filepath.Join(root, "replacement-link")
	if err := os.Symlink("about.html", link); err != nil {
		t.Skipf("cannot create symlinks: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		for i := 0; i < 100; i++ {
			target := "about.html"
			if i%2 == 0 {
				target = filepath.Join(outside, "secret.html")
			}
			if err := os.Symlink(target, replacement); err != nil {
				done <- err
				return
			}
			if err := os.Rename(replacement, link); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	for i := 0; i < 100; i++ {
		response := request(handler, http.MethodGet, "/changing.html")
		if response.Code != 404 && (response.Code != 200 || response.Body.String() != "<main>About</main>") {
			t.Errorf("symlink swap escaped confinement: %d %q", response.Code, response.Body.String())
			break
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestNewRejectsMissingOrNonDirectoryRoot(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "file", "contents")
	for _, directory := range []string{filepath.Join(root, "missing"), filepath.Join(root, "file"), ""} {
		if handler, err := New(directory, shared.RoutingConfig{}); err == nil {
			handler.Close()
			t.Fatalf("invalid root accepted: %q", directory)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "missing")); !os.IsNotExist(err) {
		t.Fatalf("missing directory was created: %v", err)
	}
}

func TestNormalizeRoutingPreservesDefaultsAndCopiesLists(t *testing.T) {
	for _, tc := range []struct{ input, want shared.RoutingConfig }{
		{shared.RoutingConfig{}, shared.RoutingConfig{IndexFiles: []string{"index.html", "index.htm"}, Extensions: []string{"html", "htm"}}},
		{shared.RoutingConfig{CleanURLs: true, IndexFiles: []string{" ", "/"}, Extensions: []string{" ", "."}}, shared.RoutingConfig{CleanURLs: true, IndexFiles: []string{"index.html"}, Extensions: []string{"html"}}},
		{shared.RoutingConfig{IndexFiles: []string{" /home.html ", " alternate.htm "}, Extensions: []string{" .txt ", "md"}}, shared.RoutingConfig{IndexFiles: []string{"home.html", "alternate.htm"}, Extensions: []string{"txt", "md"}}},
	} {
		if got := NormalizeRouting(tc.input); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("normalized routing = %#v, want %#v", got, tc.want)
		}
	}
	routing := shared.RoutingConfig{IndexFiles: []string{"index.html"}, Extensions: []string{"html"}}
	normalized := NormalizeRouting(routing)
	normalized.IndexFiles[0], normalized.Extensions[0] = "other", "txt"
	if routing.IndexFiles[0] != "index.html" || routing.Extensions[0] != "html" {
		t.Fatal("normalization changed caller-owned slices")
	}
}

func TestOpenedRootDoesNotFollowDirectoryReplacement(t *testing.T) {
	parent := t.TempDir()
	directory := filepath.Join(parent, "site")
	writeFixture(t, directory, "index.html", "original")
	handler, err := New(directory, shared.RoutingConfig{})
	if err != nil {
		t.Fatal(err)
	}
	defer handler.Close()
	if err := os.Rename(directory, filepath.Join(parent, "old-site")); err != nil {
		t.Skipf("platform cannot rename opened directory: %v", err)
	}
	writeFixture(t, directory, "index.html", "replacement")
	response := request(handler, http.MethodGet, "/")
	if response.Code != 200 || response.Body.String() != "original" {
		t.Fatalf("root changed after directory replacement: %d %q", response.Code, response.Body.String())
	}
}
