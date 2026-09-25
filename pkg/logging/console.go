package logging

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.uber.org/zap/zapcore"
)

// consoleCore renders sanitized structured events for people. JSON sinks keep
// the original fields, and each multiline event is written as one locked block.
type consoleCore struct {
	zapcore.LevelEnabler
	output zapcore.WriteSyncer
	color  bool
	fields []zapcore.Field
}

func (c *consoleCore) With(fields []zapcore.Field) zapcore.Core {
	clone := *c
	clone.fields = append(append([]zapcore.Field(nil), c.fields...), fields...)
	return &clone
}

func (c *consoleCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c *consoleCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	var out strings.Builder
	encoder := zapcore.NewMapObjectEncoder()
	keys := make([]string, 0, len(c.fields)+len(fields))
	seen := make(map[string]bool)
	for _, group := range [][]zapcore.Field{c.fields, fields} {
		for _, field := range group {
			field.AddTo(encoder)
			if !seen[field.Key] {
				keys = append(keys, field.Key)
				seen[field.Key] = true
			}
		}
	}
	prefix := entry.Time.Format("15:04:05.000") + "  " + consoleLevel(entry.Level, c.color) + "  "
	if entry.Level < zapcore.ErrorLevel {
		fmt.Fprintf(&out, "%s%s  %s", prefix, entry.LoggerName, entry.Message)
		if context := consoleInlineFields(encoder.Fields, keys); context != "" {
			fmt.Fprintf(&out, "  %s", context)
		}
		out.WriteByte('\n')
	} else {
		fmt.Fprintf(&out, "%s%s  %s\n", prefix, entry.LoggerName, entry.Message)
		var details strings.Builder
		writeConsoleFields(&details, encoder.Fields, keys, "")
		indent := strings.Repeat(" ", len(entry.Time.Format("15:04:05.000"))+2+5+2+len(entry.LoggerName)+2)
		for _, line := range strings.Split(strings.TrimSuffix(details.String(), "\n"), "\n") {
			fmt.Fprintf(&out, "%s%s\n", indent, line)
		}
	}
	n, err := c.output.Write([]byte(out.String()))
	if err == nil && n != out.Len() {
		err = io.ErrShortWrite
	}
	return err
}

func (c *consoleCore) Sync() error { return c.output.Sync() }

func consoleLevel(level zapcore.Level, color bool) string {
	label := fmt.Sprintf("%-5s", level.CapitalString())
	if !color {
		return label
	}
	code := "36"
	if level == zapcore.DebugLevel {
		code = "90"
	} else if level == zapcore.WarnLevel {
		code = "33"
	} else if level >= zapcore.ErrorLevel {
		code = "31"
	}
	return "\x1b[" + code + "m" + label + "\x1b[0m"
}

func consoleInlineFields(values map[string]interface{}, keys []string) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value, exists := values[key]
		if !exists {
			continue
		}
		parts = append(parts, SafeText(key)+"="+consoleInlineValue(value))
	}
	return strings.Join(parts, " ")
}

func consoleInlineValue(value interface{}) string {
	switch value := value.(type) {
	case map[string]interface{}:
		return fmt.Sprintf("{%d}", len(value))
	case []interface{}:
		return fmt.Sprintf("[%d]", len(value))
	default:
		text, _ := consoleScalar(value)
		if strings.ContainsAny(text, " \t=\"") {
			return strconv.Quote(text)
		}
		return text
	}
}

func consoleLabel(key string) string {
	key = SafeText(key)
	// Paths and other user-owned keys must retain their exact spelling.
	if strings.ContainsAny(key, "/. ") {
		return key
	}
	words := strings.Split(strings.ReplaceAll(key, "-", "_"), "_")
	for i, word := range words {
		switch strings.ToLower(word) {
		case "id", "url", "api", "http", "https", "pid", "gomaxprocs":
			words[i] = strings.ToUpper(word)
		default:
			runes := []rune(word)
			if len(runes) > 0 {
				runes[0] = unicode.ToUpper(runes[0])
			}
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

func consoleKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func consoleScalar(value interface{}) (string, bool) {
	switch value.(type) {
	case map[string]interface{}, []interface{}:
		return "", false
	case nil:
		return "-", true
	default:
		text := SafeText(fmt.Sprint(value))
		if text == "" {
			text = "-"
		}
		return text, true
	}
}

func writeConsoleFields(out *strings.Builder, values map[string]interface{}, keys []string, indent string) {
	width := 0
	for _, key := range keys {
		if _, ok := consoleScalar(values[key]); ok {
			width = max(width, utf8.RuneCountInString(consoleLabel(key))+1)
		}
	}
	for _, key := range keys {
		value, exists := values[key]
		if !exists {
			continue
		}
		label := consoleLabel(key) + ":"
		if scalar, ok := consoleScalar(value); ok {
			fmt.Fprintf(out, "%s%-*s  %s\n", indent, width, label, scalar)
		} else {
			fmt.Fprintf(out, "%s%s\n", indent, label)
			writeConsoleValue(out, value, indent+"  ")
		}
	}
}

func writeConsoleValue(out *strings.Builder, value interface{}, indent string) {
	switch value := value.(type) {
	case map[string]interface{}:
		if len(value) == 0 {
			fmt.Fprintf(out, "%s(none)\n", indent)
		}
		writeConsoleFields(out, value, consoleKeys(value), indent)
	case []interface{}:
		if len(value) == 0 {
			fmt.Fprintf(out, "%s(none)\n", indent)
			return
		}
		if writeConsoleTable(out, value, indent) {
			return
		}
		for i, item := range value {
			if scalar, ok := consoleScalar(item); ok {
				fmt.Fprintf(out, "%s- %s\n", indent, scalar)
			} else {
				fmt.Fprintf(out, "%s%d.\n", indent, i+1)
				writeConsoleValue(out, item, indent+"  ")
			}
		}
	}
}

// Flat record lists become tables; long or nested records use labeled blocks
// instead so paths and diagnostic values never have to be truncated.
func writeConsoleTable(out *strings.Builder, values []interface{}, indent string) bool {
	columns := make(map[string]interface{})
	rows := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		row, ok := value.(map[string]interface{})
		if !ok || len(row) == 0 {
			return false
		}
		for key, item := range row {
			if _, ok := consoleScalar(item); !ok {
				return false
			}
			columns[key] = nil
		}
		rows = append(rows, row)
	}
	keys := consoleKeys(columns)
	widths := make([]int, len(keys))
	width := len(indent)
	for i, key := range keys {
		widths[i] = utf8.RuneCountInString(consoleLabel(key))
		for _, row := range rows {
			value, _ := consoleScalar(row[key])
			widths[i] = max(widths[i], utf8.RuneCountInString(value))
		}
		width += widths[i]
		if i > 0 {
			width += 2
		}
	}
	if width+len("00:00:00.000  INFO       ") > 80 {
		return false
	}
	writeRow := func(row map[string]interface{}) {
		out.WriteString(indent)
		for i, key := range keys {
			value := consoleLabel(key)
			if row != nil {
				value, _ = consoleScalar(row[key])
			}
			if i == len(keys)-1 {
				out.WriteString(value)
			} else {
				fmt.Fprintf(out, "%-*s  ", widths[i], value)
			}
		}
		out.WriteByte('\n')
	}
	writeRow(nil)
	for _, row := range rows {
		writeRow(row)
	}
	return true
}
