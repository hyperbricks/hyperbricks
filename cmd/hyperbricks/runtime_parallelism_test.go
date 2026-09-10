package main

import (
	"runtime"
	"testing"
)

func TestResolveGoMaxProcs(t *testing.T) {
	for _, tc := range []struct {
		name       string
		value      any
		cpus, want int
		invalid    bool
	}{
		{"omitted", nil, 8, 0, false}, {"auto", "auto", 8, 0, false},
		{"one", 1, 8, 1, false}, {"hardware ceiling", 8, 8, 8, false},
		{"resolver string", "4", 8, 4, false}, {"one CPU", 1, 1, 1, false},
		{"zero", 0, 8, 0, true}, {"negative", -1, 8, 0, true},
		{"oversubscription", 9, 8, 0, true}, {"fraction", 1.5, 8, 0, true},
		{"boolean", true, 8, 0, true}, {"empty", "", 8, 0, true},
		{"invalid mode", "unlimited", 8, 0, true}, {"overflow", "999999999999999999999999", 8, 0, true},
		{"boolean text", "true", 8, 0, true},
		{"fraction text", "1.5", 8, 0, true},
		{"zero text", "0", 8, 0, true},
		{"oversubscription text", "9", 8, 0, true},
		{"object", map[string]any{"value": 4}, 8, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveGoMaxProcs(tc.value, tc.cpus)
			if (err != nil) != tc.invalid || got != tc.want {
				t.Fatalf("got (%d, %v), want (%d, invalid=%v)", got, err, tc.want, tc.invalid)
			}
		})
	}
}

func TestConfigureGoMaxProcs(t *testing.T) {
	previous := runtime.GOMAXPROCS(0)
	t.Cleanup(func() { runtime.GOMAXPROCS(previous) })
	if err := configureGoMaxProcs(1); err != nil {
		t.Fatal(err)
	}
	if got := runtime.GOMAXPROCS(0); got != 1 {
		t.Fatalf("fixed setting: got %d", got)
	}
	if err := configureGoMaxProcs(-1); err == nil {
		t.Fatal("expected invalid setting to fail")
	}
	if got := runtime.GOMAXPROCS(0); got != 1 {
		t.Fatalf("invalid setting mutated runtime to %d", got)
	}
	t.Setenv("GOMAXPROCS", "999")
	if err := configureGoMaxProcs("auto"); err != nil {
		t.Fatal(err)
	}
	if got := runtime.GOMAXPROCS(0); got < 1 || got > runtime.NumCPU() {
		t.Fatalf("auto produced unsafe CPU count %d", got)
	}
}
