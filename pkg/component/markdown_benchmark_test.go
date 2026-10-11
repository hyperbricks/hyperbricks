package component

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hyperbricks/hyperbricks/pkg/markdown"
)

// Validate full output on every iteration, including the bounded file read.
func BenchmarkMarkdown(b *testing.B) {
	for _, size := range []int{2 << 10, 32 << 10} {
		unit := "# Documentation\n\nA **formatted** paragraph with [a link](https://example.com).\n\n|A|B|\n|---|---|\n|one|two|\n\n```go\nfmt.Println(42)\n```\n\n"
		text := strings.Repeat(unit, size/len(unit)+1)
		expected := markdown.Render([]byte(text))
		for _, file := range []bool{false, true} {
			for _, cached := range []bool{false, true} {
				b.Run(fmt.Sprintf("bytes=%d/file=%t/cache=%t", len(text), file, cached), func(b *testing.B) {
					r := NewMarkdownRenderer(b.TempDir())
					config := MarkdownConfig{Content: &text, Cache: cached}
					if file {
						config.Content = nil
						config.File = "document.md"
						if err := os.WriteFile(filepath.Join(r.ResourcesRoot, config.File), []byte(text), 0600); err != nil {
							b.Fatal(err)
						}
					}
					if out, errs := r.Render(config, nil); len(errs) != 0 || out != expected {
						b.Fatal("warm-up output mismatch", errs)
					}
					b.ReportAllocs()
					b.SetBytes(int64(len(text)))
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if out, errs := r.Render(config, nil); len(errs) != 0 || out != expected {
							b.Fatal("output mismatch", errs)
						}
					}
				})
			}
		}
	}
}
