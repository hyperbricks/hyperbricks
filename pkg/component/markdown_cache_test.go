package component

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperbricks/hyperbricks/pkg/markdown"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

func TestMarkdownCacheFreshContentAndWrappers(t *testing.T) {
	for _, cached := range []bool{false, true} {
		t.Run(fmt.Sprint(cached), func(t *testing.T) {
			r := NewMarkdownRenderer(t.TempDir())
			path := filepath.Join(r.ResourcesRoot, "page.md")
			config := MarkdownConfig{File: "page.md", Cache: cached}
			for _, text := range []string{"# First", "# Other", "# First"} {
				if err := os.WriteFile(path, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
				// Same size and timestamp must not hide changed bytes.
				stamp := time.Unix(100, 0)
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				for range 2 {
					got, errs := r.Render(config, nil)
					if len(errs) != 0 || got != markdown.Render([]byte(text)) {
						t.Fatalf("%q %v", got, errs)
					}
				}
			}
			if (len(r.cache.entries) > 0) != cached {
				t.Fatal("cache option ignored")
			}
			config.Class = `x" onclick="bad`
			config.Component = shared.Component{Enclose: "<article>|</article>"}
			out, errs := r.Render(config, nil)
			if len(errs) != 0 || !strings.Contains(out, `class="x&#34; onclick=&#34;bad"`) || !strings.HasPrefix(out, "<article>") {
				t.Fatalf("stale wrappers: %s %v", out, errs)
			}
		})
	}
}

func TestMarkdownCacheValidationBeforeReuse(t *testing.T) {
	r := NewMarkdownRenderer(t.TempDir())
	path := filepath.Join(r.ResourcesRoot, "page.md")
	config := MarkdownConfig{File: "page.md", Cache: true}
	write := func(data []byte) {
		t.Helper()
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write([]byte("# Good"))
	if _, errs := r.Render(config, nil); len(errs) != 0 {
		t.Fatal(errs)
	}
	config.MaxBytes = 2
	if out, errs := r.Render(config, nil); out != "" || len(errs) != 1 {
		t.Fatal("cached result bypassed new limit")
	}
	config.MaxBytes = 0
	for _, data := range [][]byte{{0xff}, {'x', 0}, []byte(strings.Repeat("x", int(DefaultMarkdownMaxBytes)+1))} {
		write(data)
		if out, errs := r.Render(config, nil); out != "" || len(errs) != 1 {
			t.Fatal("invalid file reused old output")
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if out, errs := r.Render(config, nil); out != "" || len(errs) != 1 {
		t.Fatal("missing file reused old output")
	}
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("# Good"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Fatal(err)
	}
	if out, errs := r.Render(config, nil); out != "" || len(errs) != 1 {
		t.Fatal("escaping symlink reused cached output")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	write([]byte("# Recovered"))
	if out, errs := r.Render(config, nil); len(errs) != 0 || !strings.Contains(out, "Recovered") {
		t.Fatalf("%s %v", out, errs)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, errs := r.Render(config, ctx); out != "" || len(errs) != 1 {
		t.Fatal("cancelled cache hit succeeded")
	}
}

func TestMarkdownCacheSanitizationAndConcurrentIsolation(t *testing.T) {
	r := &MarkdownRenderer{}
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			input := fmt.Sprintf("# Document %d\n<script>alert(1)</script>\n[bad](javascript:alert%%281%%29)", i)
			expected := markdown.Render([]byte(input))
			for range 10 {
				out, errs := r.Render(MarkdownConfig{Content: &input, Cache: true}, nil)
				if len(errs) != 0 || out != expected || strings.Contains(out, "<script>") || strings.Contains(out, "javascript:") {
					t.Errorf("unsafe or mixed output: %s %v", out, errs)
				}
			}
		}()
	}
	wg.Wait()
	empty := ""
	if out, errs := r.Render(MarkdownConfig{Content: &empty, Cache: true}, nil); out != "" || len(errs) != 0 {
		t.Fatal(out, errs)
	}
}

func TestMarkdownCacheBoundsAndLRU(t *testing.T) {
	var c markdownCache
	for i := range markdownCacheEntries {
		c.render([]byte(fmt.Sprint(i)), nil)
	}
	c.render([]byte("0"), nil)
	c.render([]byte("extra"), nil)
	if len(c.entries) != markdownCacheEntries || c.entries[sha256.Sum256([]byte("0"))] == nil || c.entries[sha256.Sum256([]byte("1"))] != nil {
		t.Fatal("LRU entry bound failed")
	}
	for i := range 12 {
		c.render([]byte(fmt.Sprint(i)+strings.Repeat("x", 1<<20)), nil)
		if c.bytes > markdownCacheBytes || len(c.entries) > markdownCacheEntries {
			t.Fatal("cache exceeds bounds")
		}
	}
	before := c.bytes
	big := []byte(strings.Repeat("y", markdownCacheBytes+1))
	if out := c.render(big, nil); len(out) <= markdownCacheBytes {
		t.Fatal("large output missing")
	}
	if c.bytes != before || c.entries[sha256.Sum256(big)] != nil {
		t.Fatal("oversized output retained")
	}
	total := 0
	for _, e := range c.entries {
		total += len(e.Value.(markdownCacheEntry).html)
	}
	if total != c.bytes || c.lru.Len() != len(c.entries) {
		t.Fatal("incorrect retained byte accounting")
	}
}
