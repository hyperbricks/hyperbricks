package component

import (
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

// MultipleImagesConfig represents configuration for multiple images.
type MultipleImagesConfig struct {
	shared.Component `mapstructure:",squash"`
	Directory        string `mapstructure:"directory" validate:"required" description:"The directory path containing the images" example:"{!{images-directory.hyperbricks}}"`
	Width            int    `mapstructure:"width" validate:"min=1" description:"The width of the images (can be a number or percentage)" example:"{!{images-width.hyperbricks}}"`
	Height           int    `mapstructure:"height" validate:"min=1" description:"The height of the images (can be a number or percentage)" example:"{!{images-height.hyperbricks}}"`
	Id               string `mapstructure:"id" description:"Id of images with a index added to it" example:"{!{images-id.hyperbricks}}"`
	IsStatic         bool   `mapstructure:"is_static" description:"Flag indicating if the images are static" example:"{!{images-is_static.hyperbricks}}"`
	Alt              string `mapstructure:"alt" description:"Alternative text for the image" example:"{!{images-alt.hyperbricks}}"`
	Title            string `mapstructure:"title" description:"The title attribute of the image" example:"{!{images-title.hyperbricks}}"`
	Quality          int    `mapstructure:"quality" description:"Image quality for optimization" example:"{!{images-quality.hyperbricks}}"`
	Loading          string `mapstructure:"loading" description:"Lazy loading strategy (e.g., 'lazy', 'eager')" example:"{!{images-loading.hyperbricks}}"`
}

// MultipleImagesConfigGetName returns the HyperBricks type associated with the MultipleImagesConfig.
func MultipleImagesConfigGetName() string {
	return "<IMAGES>"
}

// MultipleImagesRenderer handles rendering for multiple images
type MultipleImagesRenderer struct {
	ImageProcessorInstance *ImageProcessor
}

// Ensure MultipleImagesRenderer implements shared.ComponentRenderer
var _ shared.ComponentRenderer = (*MultipleImagesRenderer)(nil)

func (r *MultipleImagesRenderer) Types() []string {
	return []string{
		MultipleImagesConfigGetName(),
	}
}

// Validate ensures that the configuration is valid and complete
func (config *MultipleImagesConfig) Validate() []error {
	errors := shared.Validate(config)

	if config.Directory == "" {
		errors = append(errors, fmt.Errorf("missing 'directory' attribute for multiple images"))
	} else if _, err := os.Stat(config.Directory); os.IsNotExist(err) {
		errors = append(errors, fmt.Errorf("directory does not exist: %s", config.Directory))
	}

	return errors
}

// Render processes the multiple images configuration and generates the output
func (mir *MultipleImagesRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(MultipleImagesConfig)
	if !ok {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Errorf("invalid configuration type for MultipleImagesRenderer").Error(),
		})
		return "", errors
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	// Process the images using ImageProcessor
	processor := ImageProcessor{} // Assuming ImageProcessor is defined elsewhere
	result, err := processor.ProcessMultipleImages(config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Err: fmt.Errorf("failed to process multiple images: %w", err).Error(),
		})
		return builder.String(), errors
	}

	// Wrap the result if specified
	if config.Enclose != "" {
		result = shared.EncloseContent(config.Enclose, result)
	}

	builder.WriteString(result)

	return builder.String(), errors
}
