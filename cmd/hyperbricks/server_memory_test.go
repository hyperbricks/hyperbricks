package main

import (
	"fmt"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/composite"
)

func TestAbsentRouteGuardDoesNotAllocate(t *testing.T) {
	for index, config := range []map[string]interface{}{
		nil,
		{"@type": composite.HyperMediaConfigGetName()},
		{"@type": composite.FragmentConfigGetName(), "guard": nil},
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			allocs := testing.AllocsPerRun(100, func() {
				if _, enabled, err := resolveRouteGuard(config); enabled || err != nil {
					t.Fatal("absent guard changed behavior")
				}
			})
			if allocs != 0 {
				t.Fatalf("absent guard allocated %.0f objects", allocs)
			}
		})
	}
}

func BenchmarkAbsentRouteGuard(b *testing.B) {
	config := map[string]interface{}{"@type": composite.HyperMediaConfigGetName()}
	b.ReportAllocs()
	for b.Loop() {
		if _, enabled, err := resolveRouteGuard(config); enabled || err != nil {
			b.Fatal("unguarded route changed behavior")
		}
	}
}
