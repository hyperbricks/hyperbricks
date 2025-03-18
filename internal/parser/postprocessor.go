package parser

import (
	"regexp"
	"strings"
	"sync"

	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

var (
	templateStore                      = make(map[string]string)
	PostProcessedHyperScriptStoreMutex sync.RWMutex
)

// StripCDATAAndStore extracts custom CDATA sections with metadata and content,
// replaces them with the metadata name, and stores them in the global templateStore via AddTemplate.
func StripCDATAAndStore(input string) string {
	// Updated regex with (?s) to allow '.' to match newline characters
	//re := regexp.MustCompile(`(?s)<\!\[(\w+)\[(.*?)\]\]>`)
	re := regexp.MustCompile(`(?s)<\!\[(.*?)\[(.*?)\]\]>`)

	// Find all matches for CDATA sections
	matches := re.FindAllStringSubmatch(input, -1)

	// Debug: Print matches found
	logging.GetLogger().Debug("INPUT: ", input, "\n")
	logging.GetLogger().Debug("Found CDATA sections: ", matches, "\n")

	// Loop through matches to extract metadata and content
	for _, match := range matches {
		if len(match) == 3 {
			metadata := match[1] // The metadata inside the first set of brackets
			content := match[2]  // The content inside the second set of brackets

			// Debug: Print extracted metadata and content
			//fmt.Printf("Extracted metadata: %s, content: %s\n", metadata, content)

			// Store the content in the global templateStore with metadata as the key
			AddTemplate(metadata, content)

			// Replace the CDATA section in the input with just the metadata key
			input = strings.ReplaceAll(input, match[0], metadata)

			// Debug: Print the updated input after replacement
			//fmt.Printf("Replaced CDATA: %s with metadata key: %s\n", match[0], metadata)
		}
	}

	LogTemplates()

	// Return the modified input (with CDATA sections replaced by metadata names)
	return input
}

// AddTemplate stores a template in the global map using a unique key (metadata).
func AddTemplate(metadata, content string) {
	PostProcessedHyperScriptStoreMutex.Lock()
	defer PostProcessedHyperScriptStoreMutex.Unlock()
	templateStore[metadata] = content
}

func LogTemplates() {
	logging.GetLogger().Debug("Debugging templateStore:")
	PostProcessedHyperScriptStoreMutex.RLock()         // Acquire a read lock
	defer PostProcessedHyperScriptStoreMutex.RUnlock() // Release the lock at the end

	for key, value := range templateStore {
		logging.GetLogger().Debug("Key: ", key, " Value: ", value, "\n")
	}
}

// GetTemplate retrieves a template by its metadata key.
func GetTemplate(metadata string) (string, bool) {
	PostProcessedHyperScriptStoreMutex.RLock()
	defer PostProcessedHyperScriptStoreMutex.RUnlock()
	content, found := templateStore[metadata]
	return content, found
}

// ClearTemplateStore clears all templates in the global template store.
func ClearTemplateStore() {
	PostProcessedHyperScriptStoreMutex.Lock()
	defer PostProcessedHyperScriptStoreMutex.Unlock()
	templateStore = make(map[string]string)
}

// GetTemplateStore retrieves a copy of the current template store.
func GetTemplateStore() map[string]string {
	PostProcessedHyperScriptStoreMutex.RLock()
	defer PostProcessedHyperScriptStoreMutex.RUnlock()
	storeCopy := make(map[string]string)
	for k, v := range templateStore {
		storeCopy[k] = v
	}
	return storeCopy
}

func StripComments(input string) string {
	var output []rune
	inString := false
	inComment := false
	inBlock := false // To track if we're within a custom block
	stringChar := rune(0)
	inputRunes := []rune(input)
	i := 0

	for i < len(inputRunes) {
		// Handle multi-character start delimiter for custom blocks: <<[
		if !inString && !inComment && !inBlock && i+2 < len(inputRunes) && string(inputRunes[i:i+3]) == "<<[" {
			inBlock = true
			output = append(output, inputRunes[i:i+3]...)
			i += 3
			continue
		}

		// Handle multi-character end delimiter for custom blocks: ]>>
		if inBlock && i+2 < len(inputRunes) && string(inputRunes[i:i+3]) == "]>>" {
			inBlock = false
			output = append(output, inputRunes[i:i+3]...)
			i += 3
			continue
		}

		// If inside a custom block, do not process comments or strings.
		if inBlock {
			output = append(output, inputRunes[i])
			i++
			continue
		}

		c := inputRunes[i]

		// If inside a multi-line comment, look for its end (*/)
		if inComment {
			if c == '*' && i+1 < len(inputRunes) && inputRunes[i+1] == '/' {
				inComment = false
				i += 2
				// Optionally, append a newline if the comment ended immediately before one.
				if i < len(inputRunes) && inputRunes[i] == '\n' {
					output = append(output, '\n')
					i++
				}
				continue
			}
			i++
			continue
		}

		// Process string literals – simply copy until the closing quote.
		if inString {
			output = append(output, c)
			if c == '\\' && i+1 < len(inputRunes) {
				i++
				output = append(output, inputRunes[i])
			} else if c == stringChar {
				inString = false
			}
			i++
			continue
		}

		// Detect start of single-line comments (//) if not in a string or block.
		if c == '/' && i+1 < len(inputRunes) && inputRunes[i+1] == '/' {
			// Special exception: if the slash is part of a URL (after ':') then keep it.
			if i > 0 && inputRunes[i-1] == ':' {
				output = append(output, c)
				i++
				continue
			}
			// Skip until the end of the line.
			for i < len(inputRunes) && inputRunes[i] != '\n' {
				i++
			}
			if i < len(inputRunes) {
				output = append(output, '\n')
				i++
			}
			continue
		}
		// 4.	Hash (#) Handling:
		// •	When a # is encountered, the function looks back (ignoring spaces and tabs) to see what character appears before it.
		// •	If the last non-whitespace character is an equal sign (=), then the # is considered part of the value and is output. This handles the test case hx_reselect = #response correctly.
		// •	Otherwise, if the # appears at the beginning of the line or is preceded by whitespace (and not following an =), the rest of the line is skipped as a comment.
		// Detect start of '#' comments (outside strings/blocks).
		if c == '#' && !inString {
			// Look backwards from the current index, ignoring whitespace.
			j := i - 1
			for j >= 0 && (inputRunes[j] == ' ' || inputRunes[j] == '\t') {
				j--
			}
			// If the last non-whitespace character is '=', then treat '#' as literal.
			if j >= 0 && inputRunes[j] == '=' {
				output = append(output, c)
				i++
				continue
			}
			// Otherwise, if '#' is at the beginning or preceded by whitespace, treat it as a comment.
			if i == 0 || inputRunes[i-1] == ' ' || inputRunes[i-1] == '\t' || inputRunes[i-1] == '\n' {
				// Skip all characters until the end of the line.
				for i < len(inputRunes) && inputRunes[i] != '\n' {
					i++
				}
				if i < len(inputRunes) {
					output = append(output, '\n')
					i++
				}
				continue
			}
			// In other cases, output '#' as literal.
			output = append(output, c)
			i++
			continue
		}

		// Detect start of multi-line comments: /* ... */
		if c == '/' && i+1 < len(inputRunes) && inputRunes[i+1] == '*' {
			inComment = true
			i += 2
			if len(output) > 0 && output[len(output)-1] == '\n' {
				output = output[:len(output)-1]
			}
			continue
		}

		// Detect start of a new string literal.
		if (c == '"' || c == '\'') && !inString {
			inString = true
			stringChar = c
			output = append(output, c)
			i++
			continue
		}

		// Normal character, simply copy it.
		output = append(output, c)
		i++
	}
	return string(output)
}

func StripCommentsV4(input string) string {
	var output []rune
	inString := false
	inComment := false
	inBlock := false // To track if we're within a custom block
	stringChar := rune(0)
	inputRunes := []rune(input)
	i := 0

	for i < len(inputRunes) {
		// Handle multi-character start delimiter
		if !inString && !inComment && !inBlock && i+2 < len(inputRunes) && string(inputRunes[i:i+3]) == "<<[" {
			inBlock = true
			output = append(output, inputRunes[i:i+3]...) // Append the start delimiter
			i += 3
			continue
		}

		// Handle multi-character end delimiter
		if inBlock && i+2 < len(inputRunes) && string(inputRunes[i:i+3]) == "]>>" {
			inBlock = false
			output = append(output, inputRunes[i:i+3]...) // Append the end delimiter
			i += 3
			continue
		}

		// Ignore comments if inside the block
		if inBlock {
			output = append(output, inputRunes[i])
			i++
			continue
		}

		c := inputRunes[i]

		// Handle multi-line comments
		if inComment {
			if c == '*' && i+1 < len(inputRunes) && inputRunes[i+1] == '/' {
				inComment = false
				i += 2
				if i < len(inputRunes) && inputRunes[i] == '\n' {
					output = append(output, '\n')
					i++
				}
				continue
			}
			i++
			continue
		}

		// Handle string literals to skip over them
		if inString {
			output = append(output, c)
			if c == '\\' && i+1 < len(inputRunes) {
				i++
				output = append(output, inputRunes[i])
			} else if c == stringChar {
				inString = false
			}
			i++
			continue
		}

		// Detect start of single-line comments, not within strings or block
		if c == '/' && i+1 < len(inputRunes) && inputRunes[i+1] == '/' && !inString && !inBlock {
			if i > 0 && inputRunes[i-1] == ':' {
				output = append(output, c)
				i++
				continue
			}
			for i < len(inputRunes) && inputRunes[i] != '\n' {
				i++
			}
			if i < len(inputRunes) {
				output = append(output, '\n')
				i++
			}
			continue
		}

		// Detect start of # comments, not within strings or block
		if c == '#' && !inString && !inBlock {
			// Check if it is immediately after an '=' or '= '
			if i > 0 && inputRunes[i-1] == '=' {
				output = append(output, c)
				i++
				continue
			}
			if i > 1 && inputRunes[i-2] == '=' && inputRunes[i-1] == ' ' {
				output = append(output, c)
				i++
				continue
			}
			for i < len(inputRunes) && inputRunes[i] != '\n' {
				i++
			}
			if i < len(inputRunes) {
				output = append(output, '\n')
				i++
			}
			continue
		}

		// Detect start of multi-line comments
		if c == '/' && i+1 < len(inputRunes) && inputRunes[i+1] == '*' && !inString && !inBlock {
			inComment = true
			i += 2
			if len(output) > 0 && output[len(output)-1] == '\n' {
				output = output[:len(output)-1]
			}
			continue
		}

		// Start of a new string literal
		if (c == '"' || c == '\'') && !inString && !inBlock {
			inString = true
			stringChar = c
			output = append(output, c)
			i++
			continue
		}

		// Normal character, append to output
		output = append(output, c)
		i++
	}
	return string(output)
}

func StripCommentsV3(input string) string {
	var output []rune
	inString := false
	inComment := false
	inBlock := false // To track if we're within a custom block
	stringChar := rune(0)
	inputRunes := []rune(input)
	i := 0

	for i < len(inputRunes) {
		// Handle multi-character start delimiter
		if !inString && !inComment && !inBlock && i+2 < len(inputRunes) && string(inputRunes[i:i+3]) == "<<[" {
			inBlock = true
			output = append(output, inputRunes[i:i+3]...) // Append the start delimiter
			i += 3
			continue
		}

		// Handle multi-character end delimiter
		if inBlock && i+2 < len(inputRunes) && string(inputRunes[i:i+3]) == "]>>" {
			inBlock = false
			output = append(output, inputRunes[i:i+3]...) // Append the end delimiter
			i += 3
			continue
		}

		// Ignore comments if inside the block
		if inBlock {
			output = append(output, inputRunes[i])
			i++
			continue
		}

		c := inputRunes[i]

		// Handle multi-line comments
		if inComment {
			if c == '*' && i+1 < len(inputRunes) && inputRunes[i+1] == '/' {
				inComment = false
				i += 2
				if i < len(inputRunes) && inputRunes[i] == '\n' {
					output = append(output, '\n')
					i++
				}
				continue
			}
			i++
			continue
		}

		// Handle string literals to skip over them
		if inString {
			output = append(output, c)
			if c == '\\' && i+1 < len(inputRunes) {
				i++
				output = append(output, inputRunes[i])
			} else if c == stringChar {
				inString = false
			}
			i++
			continue
		}

		// Detect start of single-line comments, not within strings or block
		if c == '/' && i+1 < len(inputRunes) && inputRunes[i+1] == '/' && !inString && !inBlock {
			if i > 0 && inputRunes[i-1] == ':' {
				output = append(output, c)
				i++
				continue
			}
			for i < len(inputRunes) && inputRunes[i] != '\n' {
				i++
			}
			if i < len(inputRunes) {
				output = append(output, '\n')
				i++
			}
			continue
		}

		// Detect start of # comments, not within strings or block
		if c == '#' && !inString && !inBlock {
			for i < len(inputRunes) && inputRunes[i] != '\n' {
				i++
			}
			if i < len(inputRunes) {
				output = append(output, '\n')
				i++
			}
			continue
		}

		// Detect start of multi-line comments
		if c == '/' && i+1 < len(inputRunes) && inputRunes[i+1] == '*' && !inString && !inBlock {
			inComment = true
			i += 2
			if len(output) > 0 && output[len(output)-1] == '\n' {
				output = output[:len(output)-1]
			}
			continue
		}

		// Start of a new string literal
		if (c == '"' || c == '\'') && !inString && !inBlock {
			inString = true
			stringChar = c
			output = append(output, c)
			i++
			continue
		}

		// Normal character, append to output
		output = append(output, c)
		i++
	}
	return string(output)
}

func StripCommentsV2(input string) string {
	var output []rune
	inString := false
	inComment := false
	inBlock := false // To track if we're within a custom block
	stringChar := rune(0)
	inputRunes := []rune(input)
	i := 0

	// Tracking variables for handling '=' per line
	afterEqual := false

	for i < len(inputRunes) {
		c := inputRunes[i]

		// Handle newline to reset afterEqual flag
		if c == '\n' {
			afterEqual = false
			output = append(output, c)
			i++
			continue
		}

		// Handle multi-character start delimiter
		if !inString && !inComment && !inBlock && i+2 < len(inputRunes) && string(inputRunes[i:i+3]) == "<<[" {
			inBlock = true
			output = append(output, inputRunes[i:i+3]...) // Append the start delimiter
			i += 3
			continue
		}

		// Handle multi-character end delimiter
		if inBlock && i+2 < len(inputRunes) && string(inputRunes[i:i+3]) == "]>>" {
			inBlock = false
			output = append(output, inputRunes[i:i+3]...) // Append the end delimiter
			i += 3
			continue
		}

		// If inside a custom block, preserve everything
		if inBlock {
			output = append(output, inputRunes[i])
			i++
			continue
		}

		// Check for '=' to set afterEqual flag
		if !inString && !inComment && c == '=' {
			afterEqual = true
			output = append(output, c)
			i++
			continue
		}

		// Handle multi-line comments
		if inComment {
			if c == '*' && i+1 < len(inputRunes) && inputRunes[i+1] == '/' {
				inComment = false
				i += 2
				if i < len(inputRunes) && inputRunes[i] == '\n' {
					output = append(output, '\n')
					i++
				}
				continue
			}
			i++
			continue
		}

		// Handle string literals to skip over them
		if inString {
			output = append(output, c)
			if c == '\\' && i+1 < len(inputRunes) {
				i++
				output = append(output, inputRunes[i])
			} else if c == stringChar {
				inString = false
			}
			i++
			continue
		}

		// Detect start of single-line comments, not within strings or block
		if c == '/' && i+1 < len(inputRunes) && inputRunes[i+1] == '/' && !inString && !inBlock {
			if afterEqual {
				// Preserve comment after '='
				for i < len(inputRunes) && inputRunes[i] != '\n' {
					output = append(output, inputRunes[i])
					i++
				}
				continue
			} else {
				// Strip the comment
				for i < len(inputRunes) && inputRunes[i] != '\n' {
					i++
				}
				if i < len(inputRunes) {
					output = append(output, '\n')
					i++
				}
				continue
			}
		}

		// Detect start of # comments, not within strings or block
		if c == '#' && !inString && !inBlock {
			if afterEqual {
				// Preserve comment after '='
				for i < len(inputRunes) && inputRunes[i] != '\n' {
					output = append(output, inputRunes[i])
					i++
				}
				continue
			} else {
				// Strip the comment
				for i < len(inputRunes) && inputRunes[i] != '\n' {
					i++
				}
				if i < len(inputRunes) {
					output = append(output, '\n')
					i++
				}
				continue
			}
		}

		// Detect start of multi-line comments
		if c == '/' && i+1 < len(inputRunes) && inputRunes[i+1] == '*' && !inString && !inBlock {
			if afterEqual {
				// Preserve comment after '='
				output = append(output, c)
				i++
				continue
			} else {
				inComment = true
				i += 2
				if len(output) > 0 && output[len(output)-1] == '\n' {
					output = output[:len(output)-1]
				}
				continue
			}
		}

		// Start of a new string literal
		if (c == '"' || c == '\'') && !inString && !inBlock {
			inString = true
			stringChar = c
			output = append(output, c)
			i++
			continue
		}

		// Normal character, append to output
		output = append(output, c)
		i++
	}

	return string(output)
}
