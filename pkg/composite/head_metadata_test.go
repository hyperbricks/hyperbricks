package composite_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
	yamlparser "github.com/hyperbricks/hyperbricks/pkg/yaml-parser"
)

func TestHeadMetadataEscapingOpenGraphAndInheritedRemoval(t *testing.T) {
	result, err := yamlparser.ProcessBytes([]byte(`source:
  - type: head
  - title: '<script>alert(1)</script>'
  - meta:
      author: Source
      description: 'A "quote" & <tag>'
      'custom"key': 'safe'
      og:title: 'Sharing "title"'
      empty: ''
instance:
  - inherit: source
  - meta:
      author: null
`), yamlparser.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rm := newHeadTestRenderManager()
	output, errs := rm.Render(composite.HeadConfigGetName(), result.Materialized["instance"].(map[string]interface{}), context.Background())
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, want := range []string{`<meta property="og:title" content="Sharing &#34;title&#34;">`, `<meta name="description" content="A &#34;quote&#34; &amp; &lt;tag&gt;">`, `<meta name="custom&#34;key" content="safe">`, `<meta name="empty" content="">`, `<title>&lt;script&gt;alert(1)&lt;/script&gt;</title>`} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %s in %s", want, output)
		}
	}
	if strings.Contains(output, `name="author"`) || strings.Contains(output, "<script>") {
		t.Fatal("suppression/escaping failed:", output)
	}
}
