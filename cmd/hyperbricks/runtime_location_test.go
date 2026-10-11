package main

import "testing"

func TestRuntimeLocationWithoutExternalIPv4(t *testing.T) {
	for _, tc := range []struct {
		name string
		ips  []string
		want string
	}{
		{"loopback-only host", nil, "127.0.0.1:8080"},
		{"empty interface result", []string{}, "127.0.0.1:8080"},
		{"existing preferred interface", []string{"192.0.2.10", "192.0.2.11"}, "192.0.2.10:8080"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runtimeLocationForIPs(tc.ips, 8080); got != tc.want {
				t.Fatalf("runtime URL location = %q, want %q", got, tc.want)
			}
		})
	}
}
