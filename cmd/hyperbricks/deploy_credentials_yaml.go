package main

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Developer-interface changes splice source text rather than encoding the document.
// This deliberately handles block mappings, the package's normal form, and
// refuses ambiguous structures instead of changing unrelated formatting.
func patchDeployCredentials(content []byte, update deployCredentialsUpdate) ([]byte, error) {
	result := bytes.Clone(content)
	for _, field := range []struct {
		name  string
		value *string
	}{{"user", update.User}, {"password", update.Password}} {
		if field.value == nil {
			continue
		}
		var err error
		result, err = patchDeployCredentialField(result, field.name, *field.value)
		if err != nil {
			return nil, err
		}
	}
	for _, field := range []struct {
		path  []string
		value *bool
	}{
		{[]string{"hyperbricks", "development", "dashboard", "enabled"}, update.DashboardEnabled},
		{[]string{"hyperbricks", "development", "frontend_editing", "spaces", "enabled"}, update.SpacesEnabled},
	} {
		if field.value == nil {
			continue
		}
		var err error
		result, err = patchDeployCredentialPath(result, field.path, strconv.FormatBool(*field.value))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func patchDeployCredentialField(content []byte, name, value string) ([]byte, error) {
	quoted, _ := json.Marshal(value)
	return patchDeployCredentialPath(content, []string{"hyperbricks", "development", "dashboard", "credentials", name}, string(quoted))
}

func patchDeployCredentialPath(content []byte, path []string, encodedValue string) ([]byte, error) {
	root, err := parseDeployCredentialDocument(content)
	if err != nil {
		return nil, err
	}
	lines := strings.SplitAfter(string(content), "\n")
	lineBreak := "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		lineBreak = "\r\n"
	}
	current := root
	for depth, part := range path {
		if current.Kind != yaml.MappingNode || current.Style&yaml.FlowStyle != 0 || current.Anchor != "" || current.Style&yaml.TaggedStyle != 0 {
			return nil, errCredentialSourceEdit
		}
		key, child := deployCredentialField(current, part)
		if child == nil {
			if len(current.Content) == 0 {
				return nil, errCredentialSourceEdit
			}
			first := current.Content[0]
			if first.Line < 1 || first.Line > len(lines) || first.Column < 1 {
				return nil, errCredentialSourceEdit
			}
			indent := first.Column - 1
			var insertion strings.Builder
			for index, missing := range path[depth:] {
				insertion.WriteString(strings.Repeat(" ", indent))
				insertion.WriteString(missing)
				insertion.WriteByte(':')
				if index == len(path)-depth-1 {
					insertion.WriteByte(' ')
					insertion.WriteString(encodedValue)
				}
				insertion.WriteString(lineBreak)
				indent += 2
			}
			lines[first.Line-1] = insertion.String() + lines[first.Line-1]
			return []byte(strings.Join(lines, "")), nil
		}
		if depth == len(path)-1 {
			return replaceDeployCredentialValue(lines, key, child, encodedValue)
		}
		if child.Kind == yaml.ScalarNode && child.Tag == "!!null" {
			return insertDeployCredentialHierarchy(lines, key, child, path[depth+1:], encodedValue, lineBreak)
		}
		current = child
	}
	return nil, errCredentialSourceEdit
}

func insertDeployCredentialHierarchy(lines []string, key, value *yaml.Node, path []string, quoted, lineBreak string) ([]byte, error) {
	if value.Line != key.Line || value.Anchor != "" || value.Style != 0 {
		return nil, errCredentialSourceEdit
	}
	colon, err := deployCredentialKeyColon(lines, key)
	if err != nil {
		return nil, err
	}
	lineIndex := key.Line - 1
	line := lines[lineIndex]
	start := colon + 1
	for start < len(line) && (line[start] == ' ' || line[start] == '\t') {
		start++
	}
	end, ok := deployCredentialInlineTokenEnd(line, start)
	if !ok {
		return nil, errCredentialSourceEdit
	}
	suffix := line[end:]
	if strings.HasPrefix(suffix, "#") {
		suffix = " " + suffix
	}
	header := line[:colon+1] + suffix
	hadLineBreak := strings.HasSuffix(header, "\n")
	if !hadLineBreak {
		header += lineBreak
	}
	var insertion strings.Builder
	indent := key.Column + 1
	for index, name := range path {
		insertion.WriteString(strings.Repeat(" ", indent))
		insertion.WriteString(name)
		insertion.WriteByte(':')
		if index == len(path)-1 {
			insertion.WriteByte(' ')
			insertion.WriteString(quoted)
		}
		if index != len(path)-1 || hadLineBreak {
			insertion.WriteString(lineBreak)
		}
		indent += 2
	}
	lines[lineIndex] = header + insertion.String()
	return []byte(strings.Join(lines, "")), nil
}

func deployCredentialKeyColon(lines []string, key *yaml.Node) (int, error) {
	if key.Line < 1 || key.Line > len(lines) || key.Column < 1 {
		return 0, errCredentialSourceEdit
	}
	line := lines[key.Line-1]
	keyStart, ok := deployCredentialColumnOffset(line, key.Column)
	if !ok || strings.TrimSpace(line[:keyStart]) != "" {
		return 0, errCredentialSourceEdit
	}
	keyEnd, ok := deployCredentialInlineTokenEnd(line, keyStart)
	if !ok {
		return 0, errCredentialSourceEdit
	}
	colon := keyEnd
	for colon < len(line) && line[colon] == ' ' {
		colon++
	}
	if colon >= len(line) || line[colon] != ':' {
		return 0, errCredentialSourceEdit
	}
	return colon, nil
}

func replaceDeployCredentialValue(lines []string, key, value *yaml.Node, quoted string) ([]byte, error) {
	if key.Line < 1 || key.Line > len(lines) || key.Column < 1 || value.Anchor != "" || value.Style&yaml.TaggedStyle != 0 {
		return nil, errCredentialSourceEdit
	}
	lineIndex := key.Line - 1
	line := lines[lineIndex]
	// Locate the delimiter only after the complete key token, including a
	// quoted key. Explicit '?' mapping keys are intentionally unsupported.
	colon, err := deployCredentialKeyColon(lines, key)
	if err != nil {
		return nil, err
	}
	start := colon + 1
	for start < len(line) && (line[start] == ' ' || line[start] == '\t') {
		start++
	}
	end := start
	blockValue := value.Line > key.Line || value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0
	if blockValue {
		// A block resolver or block scalar owns the deeper-indented content.
		// Keep comments/blank lines verbatim even when replacing its value.
		if value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			for end < len(line) && line[end] != ' ' && line[end] != '\t' && line[end] != '\r' && line[end] != '\n' {
				end++
			}
		} else if start < len(line) && line[start] != '#' && line[start] != '\r' && line[start] != '\n' {
			return nil, errCredentialSourceEdit
		}
		for next := lineIndex + 1; next < len(lines); next++ {
			trimmed := strings.TrimSpace(lines[next])
			if trimmed == "" {
				continue
			}
			indent := len(lines[next]) - len(strings.TrimLeft(lines[next], " "))
			if strings.HasPrefix(trimmed, "#") && (value.Style&(yaml.LiteralStyle|yaml.FoldedStyle) == 0 || indent <= key.Column-1) {
				continue
			}
			if indent <= key.Column-1 {
				break
			}
			lines[next] = ""
		}
	} else {
		if value.Kind != yaml.ScalarNode {
			return nil, errCredentialSourceEdit
		}
		var ok bool
		end, ok = deployCredentialInlineTokenEnd(line, start)
		if !ok {
			return nil, errCredentialSourceEdit
		}
		// A continuation would be silently orphaned by an inline replacement.
		for next := lineIndex + 1; next < len(lines); next++ {
			trimmed := strings.TrimSpace(lines[next])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			indent := len(lines[next]) - len(strings.TrimLeft(lines[next], " "))
			if indent > key.Column-1 {
				return nil, errCredentialSourceEdit
			}
			break
		}
	}
	suffix := line[end:]
	if strings.HasPrefix(suffix, "#") {
		suffix = " " + suffix
	}
	lines[lineIndex] = line[:colon+1] + " " + quoted + suffix
	return []byte(strings.Join(lines, "")), nil
}

func deployCredentialColumnOffset(line string, column int) (int, bool) {
	current := 1
	for offset := range line {
		if current == column {
			return offset, true
		}
		current++
	}
	return len(line), current == column
}

// Returns the end of a single-line scalar without consuming its comment. A
// colon followed by whitespace is the separator when scanning a mapping key.
func deployCredentialInlineTokenEnd(line string, start int) (int, bool) {
	if start >= len(line) {
		return start, true
	}
	quote := line[start]
	if quote == '\'' || quote == '"' {
		for i := start + 1; i < len(line); i++ {
			if line[i] == '\n' || line[i] == '\r' {
				return 0, false
			}
			if quote == '"' && line[i] == '\\' {
				i++
				continue
			}
			if line[i] == quote {
				if quote == '\'' && i+1 < len(line) && line[i+1] == '\'' {
					i++
					continue
				}
				return i + 1, true
			}
		}
		return 0, false
	}
	end := start
	for end < len(line) {
		if line[end] == '\r' || line[end] == '\n' || (line[end] == '#' && (end == start || line[end-1] == ' ' || line[end-1] == '\t')) {
			break
		}
		if line[end] == ':' && (end+1 == len(line) || strings.ContainsRune(" \t\r\n", rune(line[end+1]))) {
			break
		}
		end++
	}
	for end > start && (line[end-1] == ' ' || line[end-1] == '\t') {
		end--
	}
	return end, true
}
