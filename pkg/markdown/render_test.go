package markdown

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestRenderMarkdownAndSanitize(t *testing.T) {
	for _, input := range []string{
		"# Heading\n\n**Strong** and [link](https://example.com).\n\n```go\nfmt.Println(1)\n```",
		`[bad](javascript:alert%281%29) ![bad](javascript:alert%281%29)`,
		`![image](data:text/html;base64,PHNjcmlwdD4=)`,
		`<script>alert(1)</script><iframe src="https://example.com"></iframe>`,
		`<img src=x onerror=alert(1)> <svg onload=alert(1)>test</svg>`,
		`[entity](jav&#x61;script:alert%281%29) ![entity](jav&#x61;script:alert%281%29)`,
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			out := Render([]byte(input))
			doc, err := html.Parse(strings.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			var check func(*html.Node)
			check = func(n *html.Node) {
				if n.Type == html.ElementNode {
					if n.Data == "script" || n.Data == "iframe" || n.Data == "svg" {
						t.Fatalf("unsafe element in %s", out)
					}
					for _, a := range n.Attr {
						if strings.HasPrefix(a.Key, "on") {
							t.Fatalf("event attribute in %s", out)
						}
						if a.Key == "href" || a.Key == "src" {
							v := strings.ToLower(strings.TrimSpace(a.Val))
							if strings.HasPrefix(v, "javascript:") || strings.HasPrefix(v, "data:") {
								t.Fatalf("unsafe URL in %s", out)
							}
						}
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					check(c)
				}
			}
			check(doc)
		})
	}
	out := Render([]byte("# Heading\n\n**Strong**\n\n|A|B|\n|---|---|\n|1|2|\n"))
	for _, want := range []string{"<h1>Heading</h1>", "<strong>Strong</strong>", "<table>"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}
