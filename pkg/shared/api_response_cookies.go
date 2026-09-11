package shared

import (
	"fmt"
	"sync"
)

// APIResponseCookieCaptureKey identifies the request-local cookie capture used
// by API_FRAGMENT_RENDER. The HTTP server owns the eventual header commit.
const APIResponseCookieCaptureKey contextKey = "apiResponseCookieCapture"

// APIResponseCookieCapture collects complete, already validated cookie groups
// during a render. Store keeps each component's group contiguous and Result
// returns an immutable snapshot after all render work has finished.
type APIResponseCookieCapture struct {
	mu      sync.Mutex
	cookies []string
	sealed  bool
}

// Store stages one component's complete cookie group. Callers must validate the
// entire group before Store so a failed component never contributes partially.
func (c *APIResponseCookieCapture) Store(cookies []string) error {
	if c == nil {
		return fmt.Errorf("API response cookie capture is unavailable")
	}
	if len(cookies) == 0 {
		return nil
	}

	staged := append([]string(nil), cookies...)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sealed {
		return fmt.Errorf("API response cookie capture is already finalized")
	}
	c.cookies = append(c.cookies, staged...)
	return nil
}

// Result seals the capture and returns a copy safe for the response commit
// phase. Repeated calls return the same snapshot.
func (c *APIResponseCookieCapture) Result() []string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sealed = true
	return append([]string(nil), c.cookies...)
}
