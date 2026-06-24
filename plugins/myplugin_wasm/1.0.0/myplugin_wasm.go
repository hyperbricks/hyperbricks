package main

import (
	"encoding/json"
	"html"
	"strings"
	"unsafe"
)

const arenaSize = 1 << 20

var (
	arena       [arenaSize]byte
	arenaOffset uint32 = 8
)

type pluginInput struct {
	Data pluginData `json:"data"`
}

type pluginData struct {
	Title    string `json:"title"`
	Message  string `json:"message"`
	Eyebrow  string `json:"eyebrow"`
	CTALabel string `json:"cta_label"`
	CTAHref  string `json:"cta_href"`
	Accent   string `json:"accent"`
	Class    string `json:"class"`
}

type renderResponse struct {
	Kind   string        `json:"kind"`
	HTML   string        `json:"html,omitempty"`
	Errors []renderError `json:"errors,omitempty"`
}

type renderError struct {
	Err      string `json:"err,omitempty"`
	Rejected bool   `json:"rejected,omitempty"`
	Type     string `json:"type,omitempty"`
}

//go:wasmexport alloc
func alloc(size uint32) uint32 {
	if size == 0 {
		size = 1
	}
	aligned := align8(arenaOffset)
	next := align8(aligned + size)
	if next > uint32(len(arena)) {
		return 0
	}
	arenaOffset = next
	return uint32(uintptr(unsafe.Pointer(&arena[aligned])))
}

//go:wasmexport render
func render(inputPtr uint32, inputLen uint32) uint64 {
	var input pluginInput
	if err := json.Unmarshal(memory(inputPtr, inputLen), &input); err != nil {
		return writeResponse(renderResponse{
			Kind: "html",
			HTML: "<!-- failed to decode myplugin_wasm input -->",
			Errors: []renderError{{
				Err:      "failed to decode plugin input: " + err.Error(),
				Rejected: true,
				Type:     "<PLUGIN>",
			}},
		})
	}

	return writeResponse(renderResponse{
		Kind: "html",
		HTML: renderCard(input.Data),
	})
}

func main() {}

func align8(value uint32) uint32 {
	return (value + 7) &^ 7
}

func memory(ptr uint32, length uint32) []byte {
	if ptr == 0 || length == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), int(length))
}

func writeResponse(response renderResponse) uint64 {
	payload, err := json.Marshal(response)
	if err != nil {
		payload = []byte(`{"kind":"html","html":"<!-- failed to encode myplugin_wasm output -->"}`)
	}
	ptr := alloc(uint32(len(payload)))
	if ptr == 0 {
		return 0
	}
	copy(memory(ptr, uint32(len(payload))), payload)
	return (uint64(ptr) << 32) | uint64(len(payload))
}

func renderCard(data pluginData) string {
	title := fallback(data.Title, "Hello from MyPlugin WASM")
	message := fallback(data.Message, "This HTML was rendered inside a Go/WASM plugin.")
	eyebrow := fallback(data.Eyebrow, "HyperBricks WASM")
	accent := sanitizeCSSColor(fallback(data.Accent, "#2563eb"))
	className := strings.TrimSpace(data.Class)
	if className == "" {
		className = "myplugin-wasm-card"
	}

	var out strings.Builder
	out.WriteString(`<section class="`)
	out.WriteString(html.EscapeString(className))
	out.WriteString(`" style="--myplugin-accent: `)
	out.WriteString(html.EscapeString(accent))
	out.WriteString(`; border: 1px solid color-mix(in srgb, var(--myplugin-accent), transparent 72%); border-left: 4px solid var(--myplugin-accent); border-radius: 8px; padding: 1rem; background: color-mix(in srgb, var(--myplugin-accent), white 94%);">`)
	out.WriteString("\n  <p style=\"margin: 0 0 .35rem; color: var(--myplugin-accent); font-size: .78rem; font-weight: 700; letter-spacing: .06em; text-transform: uppercase;\">")
	out.WriteString(html.EscapeString(eyebrow))
	out.WriteString("</p>\n  <h2 style=\"margin: 0 0 .5rem; font-size: 1.25rem; line-height: 1.2;\">")
	out.WriteString(html.EscapeString(title))
	out.WriteString("</h2>\n  <p style=\"margin: 0; line-height: 1.5;\">")
	out.WriteString(html.EscapeString(message))
	out.WriteString("</p>\n")

	if label := strings.TrimSpace(data.CTALabel); label != "" {
		if href := sanitizeHref(data.CTAHref); href != "" {
			out.WriteString("  <a href=\"")
			out.WriteString(html.EscapeString(href))
			out.WriteString("\" style=\"display: inline-flex; margin-top: .85rem; color: var(--myplugin-accent); font-weight: 700; text-decoration: none;\">")
			out.WriteString(html.EscapeString(label))
			out.WriteString("</a>\n")
		}
	}

	out.WriteString("</section>\n")
	return out.String()
}

func fallback(value string, fallbackValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallbackValue
	}
	return value
}

func sanitizeCSSColor(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "#2563eb"
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '#', '(', ')', ',', '.', '%', ' ', '-':
			continue
		default:
			return "#2563eb"
		}
	}
	return value
}

func sanitizeHref(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "#") {
		return value
	}
	return ""
}
