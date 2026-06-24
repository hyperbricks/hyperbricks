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
	Content string `json:"content"`
	Class   string `json:"class"`
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
			HTML: "<!-- failed to decode markdown wasm plugin input -->",
			Errors: []renderError{{
				Err:      "failed to decode plugin input: " + err.Error(),
				Rejected: true,
				Type:     "<PLUGIN>",
			}},
		})
	}

	return writeResponse(renderResponse{
		Kind: "html",
		HTML: renderMarkdown(input.Data.Content, input.Data.Class),
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
		payload = []byte(`{"kind":"html","html":"<!-- failed to encode markdown wasm plugin output -->"}`)
	}
	ptr := alloc(uint32(len(payload)))
	if ptr == 0 {
		return 0
	}
	copy(memory(ptr, uint32(len(payload))), payload)
	return (uint64(ptr) << 32) | uint64(len(payload))
}

func renderMarkdown(content string, className string) string {
	if className == "" {
		className = "markdown_plugin-content"
	}

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	trimmed := strings.TrimSpace(normalized)

	var out strings.Builder
	out.WriteString(`<div class="`)
	out.WriteString(html.EscapeString(className))
	out.WriteString("\">\n")

	if strings.HasPrefix(trimmed, "# ") {
		heading, rest, _ := strings.Cut(trimmed[2:], "\n")
		out.WriteString("<h1>")
		out.WriteString(renderInline(strings.TrimSpace(heading)))
		out.WriteString("</h1>\n")
		trimmed = strings.TrimSpace(rest)
	}

	for _, paragraph := range splitParagraphs(trimmed) {
		out.WriteString("<p>")
		for i, line := range strings.Split(paragraph, "\n") {
			if i > 0 {
				out.WriteString("<br>\n")
			}
			out.WriteString(renderInline(line))
		}
		out.WriteString("</p>\n")
	}

	out.WriteString("</div>\n")
	return out.String()
}

func splitParagraphs(input string) []string {
	var paragraphs []string
	for _, paragraph := range strings.Split(input, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	return paragraphs
}

func renderInline(input string) string {
	var out strings.Builder
	for {
		start := strings.Index(input, "**")
		if start == -1 {
			out.WriteString(html.EscapeString(input))
			return out.String()
		}
		end := strings.Index(input[start+2:], "**")
		if end == -1 {
			out.WriteString(html.EscapeString(input))
			return out.String()
		}
		out.WriteString(html.EscapeString(input[:start]))
		out.WriteString("<strong>")
		out.WriteString(html.EscapeString(input[start+2 : start+2+end]))
		out.WriteString("</strong>")
		input = input[start+2+end+2:]
	}
}
