package component

import (
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/shared"
)

type SingleImageConfig struct {
	shared.Component `mapstructure:",squash"`
	Src              string `mapstructure:"src" validate:"required" description:"The source URL of the image" example:"{!{image-src.hyperbricks}}"`
	Width            int    `mapstructure:"width" validate:"min=1" description:"The width of the image (can be a number or percentage)" example:"{!{image-width.hyperbricks}}"`
	Height           int    `mapstructure:"height" validate:"min=1" description:"The height of the image (can be a number or percentage)" example:"{!{image-height.hyperbricks}}"`
	Alt              string `mapstructure:"alt" description:"Alternative text for the image" example:"{!{image-alt.hyperbricks}}"`
	Title            string `mapstructure:"title" description:"The title attribute of the image" example:"{!{image-title.hyperbricks}}"`
	Id               string `mapstructure:"id" description:"Id of image" example:"{!{image-id.hyperbricks}}"`
	Class            string `mapstructure:"class" description:"CSS class for styling the image" example:"{!{image-class.hyperbricks}}"`
	Quality          int    `mapstructure:"quality" description:"Image quality for optimization" example:"{!{image-quality.hyperbricks}}"`
	Loading          string `mapstructure:"loading" description:"Lazy loading strategy (e.g., 'lazy', 'eager')" example:"{!{image-loading.hyperbricks}}"`
	IsStatic         bool   `mapstructure:"is_static" description:"Flag indicating if the image is static" example:"{!{image-is_static.hyperbricks}}"`
}

func SingleImageConfigGetName() string {
	return "<IMAGE>"
}

type SingleImageRenderer struct {
	ImageProcessorInstance *ImageProcessor
}

var _ shared.ComponentRenderer = (*SingleImageRenderer)(nil)

func (r *SingleImageRenderer) Types() []string {
	return []string{
		SingleImageConfigGetName(),
	}
}

func (config *SingleImageConfig) Validate() []error {
	errors := shared.Validate(config)

	if config.Quality <= 0 {
		config.Quality = 90
	}

	return errors
}

func (sir *SingleImageRenderer) Render(instance interface{}) (string, []error) {
	var errors []error
	var builder strings.Builder

	config, ok := instance.(SingleImageConfig)
	if !ok {
		errors = append(errors, fmt.Errorf("invalid configuration type for SingleImageRenderer"))
		return "", errors
	}

	errors = append(errors, config.Validate()...)

	if sir.ImageProcessorInstance == nil {
		errors = append(errors, fmt.Errorf("ImageProcessorInstance is nil"))
		return builder.String(), errors
	}

	result, err := sir.ImageProcessorInstance.ProcessSingleImage(config)
	if err != nil {
		errors = append(errors, fmt.Errorf("failed to process image: %w", err))
		return builder.String(), errors
	}

	if config.Enclose != "" {
		result = shared.EncloseContent(config.Enclose, result)
	}

	builder.WriteString(result)

	return builder.String(), errors
}
