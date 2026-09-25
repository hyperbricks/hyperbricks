package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/term"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func IsTerminal(writer io.Writer) bool {
	file, ok := writer.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(file.Fd())
}

func ColorEnabled(writer io.Writer) bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	return !noColor && os.Getenv("TERM") != "dumb" && IsTerminal(writer)
}

func WriteError(writer io.Writer, err error) {
	label := "error:"
	if ColorEnabled(writer) {
		label = "\x1b[31merror:\x1b[0m"
	}
	fmt.Fprintf(writer, "%s %s\n", label, SafeText(err.Error()))
}

func WriteHeading(writer io.Writer, version string) {
	if !IsTerminal(writer) || os.Getenv("TERM") == "dumb" {
		return
	}
	name := "HyperBricks"
	if ColorEnabled(writer) {
		name = "\x1b[1mHyperBricks\x1b[0m"
	}
	fmt.Fprintf(writer, "\n%s  %s\n\n", name, strings.TrimSpace(version))
}

var (
	terminalEscape      = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\))`)
	logURL              = regexp.MustCompile(`https?://[^\s<>"']+`)
	diagnosticRequestID = regexp.MustCompile(`^hb-[0-9]+$`)
)

// SafeText prevents terminal control injection and removes URL credentials/query
// values. Call sites must still avoid logging arbitrary bodies or credentials.
func SafeText(value string) string {
	value = terminalEscape.ReplaceAllString(value, "")
	value = logURL.ReplaceAllStringFunc(value, func(raw string) string {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "[invalid URL]"
		}
		if parsed.User != nil {
			parsed.User = url.User("REDACTED")
		}
		query := parsed.Query()
		publicDiagnosticLink := parsed.Path == "/__hyperbricks/render-diagnostics" && len(query) == 1 && len(query["request_id"]) == 1 && diagnosticRequestID.MatchString(query.Get("request_id"))
		if (parsed.RawQuery != "" || parsed.ForceQuery) && !publicDiagnosticLink {
			parsed.RawQuery, parsed.ForceQuery = "REDACTED", false
		}
		parsed.Fragment, parsed.RawFragment = "", ""
		return parsed.String()
	})
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return ' '
		}
		return r
	}, value)
}

func sensitiveKey(key string) bool {
	key = strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(key))
	for _, part := range []string{"authorization", "cookie", "secret", "password", "token", "apikey", "requestbody", "responsebody"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return key == "body" || key == "headers" || key == "query" || key == "queryparams"
}

func safeValue(value interface{}) interface{} {
	switch value := value.(type) {
	case string:
		return SafeText(value)
	case map[string]interface{}:
		result := make(map[string]interface{}, len(value))
		for key, item := range value {
			if sensitiveKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = safeValue(item)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(value))
		for index, item := range value {
			result[index] = safeValue(item)
		}
		return result
	case nil, bool, int, int32, int64, uint, uint32, uint64, float32, float64, json.Number:
		return value
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return "[unavailable]"
		}
		var decoded interface{}
		decoder := json.NewDecoder(strings.NewReader(string(encoded)))
		decoder.UseNumber()
		if decoder.Decode(&decoded) != nil {
			return "[unavailable]"
		}
		return safeValue(decoded)
	}
}

func cloneFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}
	return safeValue(fields).(map[string]interface{})
}

func safeFields(fields []zapcore.Field) []zapcore.Field {
	if len(fields) == 0 {
		return nil
	}
	encoder := zapcore.NewMapObjectEncoder()
	for _, field := range fields {
		field.AddTo(encoder)
	}
	clean := cloneFields(encoder.Fields)
	result := make([]zapcore.Field, 0, len(clean))
	// Preserve input field order; namespaces and reflected objects remain nested.
	for _, field := range fields {
		if value, ok := clean[field.Key]; ok {
			result = append(result, zap.Any(field.Key, value))
			delete(clean, field.Key)
		}
	}
	for key, value := range clean {
		result = append(result, zap.Any(key, value))
	}
	return result
}

type safeCore struct{ zapcore.Core }

func (core *safeCore) With(fields []zapcore.Field) zapcore.Core {
	return &safeCore{Core: core.Core.With(safeFields(fields))}
}

func (core *safeCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if core.Enabled(entry.Level) {
		return checked.AddCore(entry, core)
	}
	return checked
}

func (core *safeCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Message = SafeText(entry.Message)
	entry.LoggerName = SafeText(entry.LoggerName)
	if entry.LoggerName == "" {
		entry.LoggerName = "runtime"
	}
	return core.Core.Write(entry, safeFields(fields))
}
