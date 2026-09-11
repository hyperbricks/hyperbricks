package shared

import (
	"fmt"
	"sync"
	"testing"
)

func TestAPIResponseCookieCaptureStoresGroupsConcurrentlyAndSeals(t *testing.T) {
	capture := &APIResponseCookieCapture{}
	const groupCount = 24

	var wait sync.WaitGroup
	wait.Add(groupCount)
	for index := 0; index < groupCount; index++ {
		index := index
		go func() {
			defer wait.Done()
			group := []string{
				fmt.Sprintf("first_%d=value; Path=/", index),
				fmt.Sprintf("second_%d=value; Path=/", index),
			}
			if err := capture.Store(group); err != nil {
				t.Errorf("Store(%d) error = %v", index, err)
			}
			group[0] = "mutated"
		}()
	}
	wait.Wait()

	got := capture.Result()
	if len(got) != groupCount*2 {
		t.Fatalf("Result() returned %d cookies, want %d", len(got), groupCount*2)
	}
	for offset := 0; offset < len(got); offset += 2 {
		var index int
		if _, err := fmt.Sscanf(got[offset], "first_%d=value; Path=/", &index); err != nil {
			t.Fatalf("cookie group starts with %q: %v", got[offset], err)
		}
		wantSecond := fmt.Sprintf("second_%d=value; Path=/", index)
		if got[offset+1] != wantSecond {
			t.Fatalf("cookie group was interleaved: %q followed by %q, want %q", got[offset], got[offset+1], wantSecond)
		}
	}

	got[0] = "mutated result"
	if repeated := capture.Result(); repeated[0] == "mutated result" {
		t.Fatal("Result exposed the capture's internal slice")
	}
	if err := capture.Store([]string{"late=value"}); err == nil {
		t.Fatal("Store after Result succeeded")
	}
}
