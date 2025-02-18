package component

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

type MultipleImagesConfig struct {
	shared.Component `mapstructure:",squash"`
	Directory        string `mapstructure:"directory" validate:"required" description:"The directory path containing the images" example:"{!{images-directory.hyperbricks}}"`
	Width            int    `mapstructure:"width" validate:"min=1" description:"The width of the images (can be a number or percentage)" example:"{!{images-width.hyperbricks}}"`
	Height           int    `mapstructure:"height" validate:"min=1" description:"The height of the images (can be a number or percentage)" example:"{!{images-height.hyperbricks}}"`
	Id               string `mapstructure:"id" description:"Id of images with a index added to it" example:"{!{images-id.hyperbricks}}"`
	Class            string `mapstructure:"class" description:"CSS class for styling the image" example:"{!{images-class.hyperbricks}}"`
	IsStatic         bool   `mapstructure:"is_static" description:"Flag indicating if the images are static" example:"{!{images-is_static.hyperbricks}}"`
	Alt              string `mapstructure:"alt" description:"Alternative text for the image" example:"{!{images-alt.hyperbricks}}"`
	Title            string `mapstructure:"title" description:"The title attribute of the image" example:"{!{images-title.hyperbricks}}"`
	Quality          int    `mapstructure:"quality" description:"Image quality for optimization" example:"{!{images-quality.hyperbricks}}"`
	Loading          string `mapstructure:"loading" description:"Lazy loading strategy (e.g., 'lazy', 'eager')" example:"{!{images-loading.hyperbricks}}"`
}

func MultipleImagesConfigGetName() string {
	return "<IMAGES>"
}

type MultipleImagesRenderer struct {
	ImageProcessorInstance *ImageProcessor
}

var _ shared.ComponentRenderer = (*MultipleImagesRenderer)(nil)

func (r *MultipleImagesRenderer) Types() []string {
	return []string{
		MultipleImagesConfigGetName(),
	}
}

func (config *MultipleImagesConfig) Validate() []error {
	errors := shared.Validate(config)

	if config.Directory == "" {
		errors = append(errors, fmt.Errorf("missing 'directory' attribute for multiple images"))
	} else if _, err := os.Stat(config.Directory); os.IsNotExist(err) {
		errors = append(errors, fmt.Errorf("directory does not exist: %s", config.Directory))
	}

	return errors
}

func (mir *MultipleImagesRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(MultipleImagesConfig)
	if !ok {
		errors = append(errors, shared.ComponentError{
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: MultipleImagesConfigGetName(),
			Err:  fmt.Errorf("invalid configuration type for MultipleImagesRenderer").Error(),
		})
		return "", errors
	}

	errors = append(errors, config.Validate()...)

	processor := ImageProcessor{}
	result, err := processor.ProcessMultipleImages(config)
	if err != nil {
		errors = append(errors, shared.ComponentError{
			Key:  config.Component.Meta.HyperBricksKey,
			Path: config.Component.Meta.HyperBricksPath,
			File: config.Component.Meta.HyperBricksFile,
			Type: MultipleImagesConfigGetName(),
			Err:  fmt.Errorf("failed to process multiple images: %w", err).Error(),
		})
		return builder.String(), errors
	}

	if config.Enclose != "" {
		result = shared.EncloseContent(config.Enclose, result)
	}

	builder.WriteString(result)

	return builder.String(), errors
}
