package component

import (
	"fmt"
	"html/template"
	"log"
	"sort"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// HyperMediaConfig represents configuration for a single page in the menu.
type HyperMediaConfig struct {
	Title string `json:"title"`
	Route string `json:"route"`
	Index int    `json:"index"`
}

// MenuConfig represents configuration for the MENU component.
type MenuConfig struct {
	shared.Component     `mapstructure:",squash"`
	Section              string `mapstructure:"section" validate:"required" description:"The section of the menu to display." example:"{!{menu-section.hyperbricks}}"`
	Order                string `mapstructure:"order" validate:"oneof=asc desc" description:"The order of items in the menu ('asc' or 'desc')." example:"{!{menu-order.hyperbricks}}"`
	Sort                 string `mapstructure:"sort" validate:"oneof=title route index" description:"The field to sort menu items by ('title', 'route', or 'index')." example:"{!{menu-sort.hyperbricks}}"`
	Active               string `mapstructure:"active" validate:"required" description:"Template for the active menu item." example:"{!{menu-active.hyperbricks}}"`
	Item                 string `mapstructure:"item" validate:"required" description:"Template for regular menu items." example:"{!{menu-item.hyperbricks}}"`
	Enclose              string `mapstructure:"enclose" description:"Template to wrap the menu items." example:"{!{menu-enclose.hyperbricks}}"`
	HyperMediasBySection map[string][]composite.HyperMediaConfig
}

// MenuConfigGetName returns the HyperBricks type associated with the MenuConfig.
func MenuConfigGetName() string {
	return "<MENU>"
}

// MenuRenderer handles rendering of MENU components.
type MenuRenderer struct {
	TemplateProvider     func(templateName string) (string, bool) // Function to retrieve templates
	HyperMediasBySection map[string][]composite.HyperMediaConfig
	CurrentRoute         string // Added to track the current page's route
}

// Ensure MenuRenderer implements shared.ComponentRenderer
var _ shared.ComponentRenderer = (*MenuRenderer)(nil)

func (r *MenuRenderer) Types() []string {
	return []string{
		MenuConfigGetName(),
	}
}

// SortOptions defines the options for sorting pages.
type SortOptions struct {
	SortBy    string // "title", "route", or "index"
	Ascending bool   // true for ascending order, false for descending
}

// Validate ensures the configuration is valid for rendering the MENU.
func (mc *MenuConfig) Validate() []error {
	errors := shared.Validate(mc)

	if mc.Sort != "" && !(mc.Sort == "route" || mc.Sort == "index" || mc.Sort == "title") {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Errorf("unknown 'sort' value '%s', defaulting to 'title'", mc.Sort).Error(),
		})
		mc.Sort = "title"
	}

	if mc.Order != "" && !(mc.Order == "asc" || mc.Order == "desc") {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Errorf("unknown 'order' value '%s', defaulting to 'asc'", mc.Order).Error(),
		})
		mc.Order = "asc"
	}

	return errors
}

// Render generates the MENU output based on the configuration and context.
func (mr *MenuRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(MenuConfig)
	if !ok {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Errorf("invalid type for MenuRenderer").Error(),
		})
		return "", errors
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	config.HyperMediasBySection = mr.HyperMediasBySection

	// Determine sorting and order options
	sortValue := "title"
	if config.Sort != "" {
		sortValue = config.Sort
	}

	orderValue := true // ascending by default
	if strings.ToLower(config.Order) == "desc" {
		orderValue = false
	}

	options := SortOptions{
		SortBy:    sortValue,
		Ascending: orderValue,
	}

	// Sort pages by section
	sortedHyperMedias, err := SortHyperMediasBySection(config.HyperMediasBySection, options)
	if err != nil {
		log.Printf("Error sorting pages: %v", err)
		builder.WriteString(fmt.Sprintf("<!-- Error sorting pages: %v -->\n", err))
		errors = append(errors, shared.ComponentError{
			Err: err.Error(),
		})
		return builder.String(), errors
	}

	pages, ok := sortedHyperMedias[config.Section]
	if !ok || len(pages) == 0 {
		builder.WriteString(fmt.Sprintf("<!-- No pages found for section '%s' -->\n", config.Section))
		errors = append(errors, fmt.Errorf("no pages found for section '%s'", config.Section))
		return builder.String(), errors
	}

	// Parse the templates
	tmplActive, err := template.New("active").Parse(config.Active)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to parse 'active' template: %w", err))
		return "", errors
	}

	tmplItem, err := template.New("item").Parse(config.Item)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to parse 'item' template: %w", err))
		return "", errors
	}

	// Build the menu items
	var menuItems []string
	currentRoute := mr.CurrentRoute // Use the current page's route

	for _, page := range pages {
		var buf strings.Builder
		var tmpl *template.Template
		if currentRoute == "" {
			currentRoute = "index"
		}
		if page.Route == currentRoute {
			tmpl = tmplActive
		} else {
			tmpl = tmplItem
		}
		err := tmpl.Execute(&buf, page)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to execute template: %w", err))
			continue
		}
		menuItems = append(menuItems, buf.String())
	}

	// Combine the menu items
	menuContent := strings.Join(menuItems, "\n")

	// Apply enclosing with extra attributes if specified
	if config.Enclose != "" {
		menuContent = shared.EncloseContent(config.Enclose, menuContent)
	}

	builder.WriteString(menuContent)

	return builder.String(), errors
}

// SortHyperMediasBySection creates a sorted copy of pagesBySection based on the provided options.
// It returns the sorted copy and an error if the SortBy option is invalid.
func SortHyperMediasBySection(pagesBySection map[string][]composite.HyperMediaConfig, options SortOptions) (map[string][]composite.HyperMediaConfig, error) {
	// Define valid SortBy options
	validSortBy := map[string]bool{"title": true, "route": true, "index": true}
	if !validSortBy[options.SortBy] {
		return nil, fmt.Errorf("invalid SortBy option: %s", options.SortBy)
	}

	// Initialize a new map to hold the sorted copy
	sortedCopy := make(map[string][]composite.HyperMediaConfig, len(pagesBySection))

	// Iterate over each section in the original map
	for section, pages := range pagesBySection {
		// Create a copy of the slice to avoid modifying the original
		pagesCopy := make([]composite.HyperMediaConfig, len(pages))
		copy(pagesCopy, pages)

		// Sort the copied slice based on SortOptions
		sort.Slice(pagesCopy, func(i, j int) bool {
			var less bool
			switch options.SortBy {
			case "title":
				less = pagesCopy[i].Title < pagesCopy[j].Title
			case "route":
				less = pagesCopy[i].Route < pagesCopy[j].Route
			case "index":
				less = pagesCopy[i].Index < pagesCopy[j].Index
			}
			if !options.Ascending {
				return !less
			}
			return less
		})

		// Assign the sorted copy to the new map
		sortedCopy[section] = pagesCopy
	}

	return sortedCopy, nil
}
