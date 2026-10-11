package catalog_test

import (
	"os"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"golang.org/x/net/html"
)

func TestCatalogInitialFilterValues(t *testing.T) {
	source, err := os.ReadFile("../templates/catalog.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		params   map[string]interface{}
		search   string
		category string
		unknown  bool
	}{
		{name: "no query"},
		{name: "filtered", params: map[string]interface{}{"search": "lamp", "category": "Office"}, search: "lamp", category: "Office"},
		{name: "repeated uses first value", params: map[string]interface{}{"search": []string{"lamp", "bowl"}, "category": []string{"Office", "Home"}}, search: "lamp", category: "Office"},
		{name: "repeated empty first value", params: map[string]interface{}{"search": []string{"", "lamp"}, "category": []string{"", "Office"}}},
		{name: "escaped query", params: map[string]interface{}{"search": `" autofocus onfocus="alert(1)<script>&`, "category": "Home"}, search: `" autofocus onfocus="alert(1)<script>&`, category: "Home"},
		{name: "unknown category", params: map[string]interface{}{"category": "Unknown"}, category: "Unknown", unknown: true},
		{name: "case mismatched category", params: map[string]interface{}{"category": "office"}, category: "office", unknown: true},
		{name: "escaped unknown category", params: map[string]interface{}{"category": `"><script>alert(1)</script>&`}, category: `"><script>alert(1)</script>&`, unknown: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, errs := shared.ApplyTemplate(string(source), map[string]interface{}{"Params": tc.params, "feed": ""})
			if len(errs) != 0 {
				t.Fatalf("render errors: %v", errs)
			}
			root, err := html.Parse(strings.NewReader(output))
			if err != nil {
				t.Fatal(err)
			}
			var search string
			var category string
			var categoryLabel string
			var foundSearch bool
			var visit func(*html.Node)
			visit = func(n *html.Node) {
				attrs := map[string]string{}
				for _, a := range n.Attr {
					attrs[a.Key] = a.Val
				}
				if n.Type == html.ElementNode && attrs["id"] == "search" {
					foundSearch = true
					search = attrs["value"]
					for _, forbidden := range []string{"autofocus", "onfocus"} {
						if _, exists := attrs[forbidden]; exists {
							t.Errorf("query introduced %s attribute", forbidden)
						}
					}
				}
				if n.Type == html.ElementNode && n.Data == "option" {
					if _, selected := attrs["selected"]; selected {
						category = attrs["value"]
						for child := n.FirstChild; child != nil; child = child.NextSibling {
							if child.Type != html.TextNode {
								t.Errorf("category label introduced unexpected %s element", child.Data)
							}
							categoryLabel += child.Data
						}
					}
				}
				if n.Type == html.ElementNode && n.Data == "script" {
					t.Error("query introduced a script element")
				}
				for child := n.FirstChild; child != nil; child = child.NextSibling {
					visit(child)
				}
			}
			visit(root)
			if !foundSearch || search != tc.search || category != tc.category {
				t.Errorf("filters = %q / %q, want %q / %q (search found: %v)", search, category, tc.search, tc.category, foundSearch)
			}
			if tc.unknown && categoryLabel != "Unknown category: "+tc.category {
				t.Errorf("unknown category label = %q, want %q", categoryLabel, "Unknown category: "+tc.category)
			}
		})
	}
}
