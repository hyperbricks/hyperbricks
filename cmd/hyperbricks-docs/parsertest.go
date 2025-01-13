// package main

// import (
// 	"bytes"
// 	"encoding/json"
// 	"fmt"
// 	"regexp"
// 	"strings"
// )

// // ParsedContent holds the separated sections and optional scope after parsing.
// type ParsedContent struct {
// 	HyperbricksConfig      string
// 	HyperbricksConfigScope string
// 	Explainer              string
// 	ExpectedJSON           map[string]interface{}
// 	ExpectedOutput         string
// }

// // ParseContent parses the provided content string into its respective parts.
// func ParseContent(content string) (*ParsedContent, error) {
// 	headerRegex := regexp.MustCompile(`^====\s*(.*?)\s*====$`)
// 	sections := make(map[string]string)
// 	var currentSection string
// 	var sb strings.Builder

// 	lines := strings.Split(content, "\n")
// 	for _, line := range lines {
// 		line = strings.TrimSpace(line)
// 		if matches := headerRegex.FindStringSubmatch(line); matches != nil {
// 			if currentSection != "" {
// 				sections[strings.ToLower(currentSection)] = strings.TrimSpace(sb.String())
// 				sb.Reset()
// 			}
// 			currentSection = matches[1]
// 		} else {
// 			if currentSection != "" {
// 				sb.WriteString(line)
// 				sb.WriteString("\n")
// 			}
// 		}
// 	}
// 	if currentSection != "" {
// 		sections[strings.ToLower(currentSection)] = strings.TrimSpace(sb.String())
// 	}

// 	hyperbricksConfig := sections["hyperbricks config"]
// 	explainer := sections["explainer"]
// 	expectedJSONStr := sections["expected json"]
// 	expectedOutput := sections["expected output"]

// 	var expectedJSON map[string]interface{}
// 	if err := json.Unmarshal([]byte(expectedJSONStr), &expectedJSON); err != nil {
// 		return nil, fmt.Errorf("error parsing expected JSON: %v", err)
// 	}

// 	return &ParsedContent{
// 		HyperbricksConfig: hyperbricksConfig,
// 		Explainer:         explainer,
// 		ExpectedJSON:      expectedJSON,
// 		ExpectedOutput:    expectedOutput,
// 	}, nil
// }

// func main() {
// 	fileContent := `
// ==== hyperbricks config ====
// fragment = <FRAGMENT>
// fragment {
// 	enclose = <div>|</div>
// }
// ==== explainer ====
// This code does blah blah blah....
// And is hey hey hey
// ==== expected json ====
// {
// 	"enclose":"<div>|</div>"
// }
// ==== expected output ====
// <div></div>
// `

// 	parsed, err := ParseContent(fileContent)
// 	if err != nil {
// 		fmt.Println("Error:", err)
// 		return
// 	}

// 	fmt.Println("Hyperbricks Config:")
// 	fmt.Println(parsed.HyperbricksConfig)

// 	fmt.Println("\nExplainer:")
// 	fmt.Println(parsed.Explainer)

// 	fmt.Println("\nExpected JSON (Non-Escaped):")
// 	var buf bytes.Buffer
// 	encoder := json.NewEncoder(&buf)
// 	encoder.SetEscapeHTML(false) // Disable HTML escaping
// 	encoder.SetIndent("", "  ")  // Enable pretty printing with indentation
// 	if err := encoder.Encode(parsed.ExpectedJSON); err != nil {
// 		fmt.Println("Error encoding JSON:", err)
// 		return
// 	}
// 	fmt.Print(buf.String())

// 	fmt.Println("\nExpected Output:")
// 	fmt.Println(parsed.ExpectedOutput)
// }
