package shared

import (
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestSortedUniqueKeysIgnoresTypeAndSortsNumbersBeforeStrings(t *testing.T) {
	input := map[string]interface{}{
		"beta":  true,
		"10":    "ten",
		"2":     "two",
		"alpha": true,
		"@type": "<HTML>",
	}

	got := SortedUniqueKeys(input)
	want := []string{"2", "10", "alpha", "beta"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected sorted keys %#v, got %#v", want, got)
	}
}

func TestEncloseContentSupportsNoWrapPairAndTagForms(t *testing.T) {
	tests := []struct {
		name    string
		enclose string
		content string
		want    string
	}{
		{name: "empty", enclose: "", content: "body", want: "body"},
		{name: "paired", enclose: "<main>|</main>", content: "body", want: "<main>body</main>"},
		{name: "tag name", enclose: "section", content: "body", want: "<section>body</section>"},
		{name: "tag brackets", enclose: "<article>", content: "body", want: "<article>body</article>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EncloseContent(tt.enclose, tt.content); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestCloneMapDeepCopiesNestedMapsAndSlices(t *testing.T) {
	source := map[string]interface{}{
		"title": "demo",
		"nested": map[string]interface{}{
			"count": 1,
		},
		"items": []interface{}{
			map[string]interface{}{"label": "first"},
		},
	}

	clone := CloneMapDeep(source)
	clone["title"] = "changed"
	clone["nested"].(map[string]interface{})["count"] = 2
	clone["items"].([]interface{})[0].(map[string]interface{})["label"] = "changed"

	if source["title"] != "demo" {
		t.Fatalf("source title changed: %#v", source["title"])
	}
	if source["nested"].(map[string]interface{})["count"] != 1 {
		t.Fatalf("source nested map changed: %#v", source["nested"])
	}
	if source["items"].([]interface{})[0].(map[string]interface{})["label"] != "first" {
		t.Fatalf("source nested slice map changed: %#v", source["items"])
	}
}

func TestExtractHxDataReadsHeadersFormAndQuery(t *testing.T) {
	form := url.Values{
		"title": {"Hello"},
		"tags":  {"go", "html"},
	}
	req := httptest.NewRequest("POST", "/submit?page=1&page=2", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("HX-Trigger", "button")

	got, err := ExtractHxData(req)
	if err != nil {
		t.Fatalf("unexpected extraction error: %v", err)
	}

	headers := got["headers"].(map[string]interface{})
	if headers["Hx-Trigger"] != "button" {
		t.Fatalf("expected header extraction, got %#v", headers)
	}

	formData := got["form"].(map[string]interface{})
	if formData["title"] != "Hello" {
		t.Fatalf("expected title form field, got %#v", formData["title"])
	}
	if !reflect.DeepEqual(formData["tags"], []string{"go", "html"}) {
		t.Fatalf("expected multi-value form field, got %#v", formData["tags"])
	}

	query := got["query"].(map[string]interface{})
	if !reflect.DeepEqual(query["page"], []string{"1", "2"}) {
		t.Fatalf("expected multi-value query field, got %#v", query["page"])
	}
}

func TestApplyTemplateUsesGenericFunctions(t *testing.T) {
	got, errs := ApplyTemplate(`{{ valueOrEmpty .Title }} {{ safe .HTML }}`, map[string]interface{}{
		"Title": "Hello",
		"HTML":  "<strong>world</strong>",
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected template errors: %#v", errs)
	}
	if got != "Hello <strong>world</strong>" {
		t.Fatalf("unexpected template output: %q", got)
	}
}
