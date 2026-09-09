package component

import (
	"context"
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type SingleImageConfig struct {
	shared.Component   `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"Processes a local image and writes a copy to the configured static/images directory. The filename includes a fingerprint of the source content and resize settings; the img tag uses a root-relative /static/images/ URL and escaped attribute values." example:"{!{image-@doc.hyperbricks.yaml}}"`
	Src                string `mapstructure:"src" validate:"required" description:"Local filesystem path to a JPEG, PNG, or GIF image, relative to the working directory unless absolute. Use a path resolver for module resources. Remote URLs and SVG processing are not supported." example:"{!{image-src.hyperbricks.yaml}}"`
	Width              int    `mapstructure:"width" validate:"min=0" description:"Output width in integer pixels; omit or use 0 to preserve aspect ratio from height. Omit both dimensions to keep the source size." example:"{!{image-width.hyperbricks.yaml}}"`
	Height             int    `mapstructure:"height" validate:"min=0" description:"Output height in integer pixels; omit or use 0 to preserve aspect ratio from width. Setting both dimensions resizes to that exact size." example:"{!{image-height.hyperbricks.yaml}}"`
	Alt                string `mapstructure:"alt" description:"Alternative text, automatically HTML-escaped. An empty value renders an empty alt attribute for decorative images; supply meaningful text for informative images." example:"{!{image-alt.hyperbricks.yaml}}"`
	Title              string `mapstructure:"title" description:"The title attribute of the image" example:"{!{image-title.hyperbricks.yaml}}"`
	Id                 string `mapstructure:"id" description:"Id of image" example:"{!{image-id.hyperbricks.yaml}}"`
	Class              string `mapstructure:"class" description:"CSS class for styling the image" example:"{!{image-class.hyperbricks.yaml}}"`
	Quality            int    `mapstructure:"quality" description:"JPEG encoding quality from 1 to 100; omit or use 0 for 90. Does not affect PNG or GIF encoding." example:"{!{image-quality.hyperbricks.yaml}}"`
	Loading            string `mapstructure:"loading" description:"Lazy loading strategy (e.g., 'lazy', 'eager')" example:"{!{image-loading.hyperbricks.yaml}}"`
	IsStatic           bool   `mapstructure:"is_static" exclude:"true" description:"Flag indicating if the image is static" example:"{!{image-is_static.hyperbricks.yaml}}"`
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

	if config.Quality == 0 {
		config.Quality = 90
	}

	return errors
}

func (sir *SingleImageRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {
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
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			File:     config.Component.Meta.HyperBricksFile,
			Key:      config.HyperBricksKey,
			Path:     config.HyperBricksPath,
			Err:      fmt.Errorf("failed to process image: %w", err).Error(),
			Rejected: true,
		})
		return builder.String(), errors
	}

	if config.Enclose != "" {
		result = shared.EncloseContent(config.Enclose, result)
	}

	builder.WriteString(result)

	return builder.String(), errors
}
