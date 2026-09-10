package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func call(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestSaveValidatesAndRejectsStaleVersion(t *testing.T) {
	f := newFixture()
	h := f.handler()
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"name":"","version":"1"}`, http.StatusUnprocessableEntity},
		{`{"name":"Garden","version":"bad"}`, http.StatusUnprocessableEntity},
		{`{"name":"  Garden team  ","version":"1"}`, http.StatusOK},
		{`{"name":"Stale overwrite","version":"1"}`, http.StatusConflict},
	} {
		if w := call(h, "POST", "/api/project", tc.body, ""); w.Code != tc.want {
			t.Fatalf("save %s: status %d, want %d: %s", tc.body, w.Code, tc.want, w.Body.String())
		}
	}
	w := call(h, "GET", "/api/project", "", "")
	var got project
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "Garden team" || got.Version != 2 {
		t.Fatalf("invalid or stale save changed data: %+v", got)
	}
	if fresh := newFixture().project; fresh.Version != 1 || fresh.Name != "Community garden" {
		t.Fatalf("new API did not reset the sample: %+v", fresh)
	}
}

func TestConcurrentEditorsCannotOverwriteSameVersion(t *testing.T) {
	f := newFixture()
	h := f.handler()
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"name":"Editor %d","version":"1"}`, i)
			statuses <- call(h, "POST", "/api/project", body, "").Code
		}(i)
	}
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusConflict] != 1 {
		t.Fatalf("expected one save and one conflict, got %v", counts)
	}
}

func TestSettingsChecksAccessWithoutPageGuard(t *testing.T) {
	h := newFixture().handler()
	for _, path := range []string{"/auth/owner", "/api/settings"} {
		for _, tc := range []struct {
			token string
			want  int
		}{
			{"", http.StatusUnauthorized},
			{"unknown", http.StatusUnauthorized},
			{"demo-viewer", http.StatusForbidden},
			{"demo-owner", http.StatusOK},
		} {
			want := tc.want
			if path == "/auth/owner" && tc.token == "demo-owner" {
				want = http.StatusNoContent
			}
			w := call(h, "GET", path, "", tc.token)
			if w.Code != want || w.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("%s token=%q: status=%d cache=%q", path, tc.token, w.Code, w.Header().Get("Cache-Control"))
			}
		}
	}
}

func TestMethodsAndDemoRolesAreExplicit(t *testing.T) {
	h := newFixture().handler()
	if w := call(h, "DELETE", "/api/project", "", ""); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE status=%d", w.Code)
	}
	if w := call(h, "POST", "/auth/demo-login", `{"role":"admin"}`, ""); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invented role status=%d", w.Code)
	}
	w := call(h, "POST", "/auth/demo-login", `{"role":"viewer"}`, "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"token":"demo-viewer"`) {
		t.Fatalf("demo viewer login: %d %s", w.Code, w.Body.String())
	}
}
