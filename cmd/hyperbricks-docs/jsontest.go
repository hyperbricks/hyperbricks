package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// Assuming these types are defined elsewhere:
// type Composite struct { ... }
// type HxResponse struct { ... }

type FragmentConfig struct {
	// Embedded and regular fields
	shared.Composite     `mapstructure:",squash"`
	composite.HxResponse `mapstructure:"response" description:"HTMX response header configuration."`
	MetaDocDescription   string                 `mapstructure:"@doc" description:"..."`
	HxResponseWriter     http.ResponseWriter    `mapstructure:"hx_response"`
	Title                string                 `mapstructure:"title" description:"The title of the fragment"`
	Route                string                 `mapstructure:"route" description:"The route for the fragment"`
	Section              string                 `mapstructure:"section" description:"The section the fragment belongs to"`
	Items                map[string]interface{} `mapstructure:",remain"`
	BodyTag              string                 `mapstructure:"bodytag" description:"Special body wrap..."`
	Enclose              string                 `mapstructure:"enclose" description:"Wrapping property"`
	Favicon              string                 `mapstructure:"favicon" description:"Path to the favicon"`
	Template             map[string]interface{} `mapstructure:"template" description:"Template configurations"`
	File                 string                 `mapstructure:"@file"`
	IsStatic             bool                   `mapstructure:"isstatic"`
	Static               string                 `mapstructure:"static" description:"Static file path"`
	Index                int                    `mapstructure:"index" description:"Index number..."`
	Meta                 map[string]string      `mapstructure:"meta" description:"Metadata for the fragment"`
	Doctype              string                 `mapstructure:"doctype" description:"Doctype for the HTML document"`
	HtmlTag              string                 `mapstructure:"htmltag" description:"The opening HTML tag with attributes"`
	Head                 map[string]interface{} `mapstructure:"head" description:"Configurations for the head section"`
	Css                  []string               `mapstructure:"css" description:"CSS files associated with the fragment"`
	Js                   []string               `mapstructure:"js" description:"JavaScript files associated with the fragment"`
}

func main() {
	// Example instance of FragmentConfig
	cfg := FragmentConfig{
		Title:   "Example Title",
		Route:   "/example",
		Enclose: "<div>|</div>",
		// Populate other fields as needed...
	}

	// Convert the struct to JSON
	jsonBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling struct to JSON:", err)
		return
	}

	// Print the resulting JSON
	fmt.Println(string(jsonBytes))
}
