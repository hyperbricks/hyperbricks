package main

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestDedicatedEchoFixtureUsesCurrentJWTSecret(t *testing.T) {
	configureDedicatedJWTEnvironment(t)
	first := startDedicatedEchoFixture(t)
	second := startDedicatedEchoFixture(t)
	if first == second || strings.HasSuffix(first, ":8090") || strings.HasSuffix(second, ":8090") {
		t.Fatalf("expected independently owned ephemeral fixtures: %s, %s", first, second)
	}
	for _, baseURL := range []string{first, second} {
		req, err := http.NewRequest(http.MethodPost, baseURL+"/echo/token/validate", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+os.Getenv("PGRST_TEST_USER_JWT"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var payload struct {
			Valid bool `json:"valid"`
		}
		err = json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK || !payload.Valid {
			t.Fatalf("fixture rejected current JWT: status=%d valid=%t error=%v", resp.StatusCode, payload.Valid, err)
		}
	}
}

func TestNormalizeDedicatedFixtureURLsUsesOwnedEchoServer(t *testing.T) {
	t.Setenv("POSTGREST_PORT", "3100")
	input := "http://localhost:8090/echo/query http://127.0.0.1:8090/echo/token/validate http://localhost:3000/tasks"
	got := normalizeDedicatedFixtureURLs(input, "http://127.0.0.1:43210")
	want := "http://127.0.0.1:43210/echo/query http://127.0.0.1:43210/echo/token/validate http://127.0.0.1:3100/tasks"
	if got != want {
		t.Fatalf("normalized URLs: got %q, want %q", got, want)
	}
}
