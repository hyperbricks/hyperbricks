package component

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type MultipleImagesConfig struct {
	shared.Component   `mapstructure:",squash"`
	MetaDocDescription string `mapstructure:"@doc" description:"Processes JPEG, PNG, and GIF files from a local directory into fingerprinted static/images copies with root-relative URLs and escaped attributes. Uses the base id plus an index only when id is set. Reports unreadable or invalid images as render errors." example:"{!{images-@doc.hyperbricks.yaml}}"`
	Directory          string `mapstructure:"directory" validate:"required" description:"Local filesystem directory containing JPEG, PNG, or GIF images. Reads files in filename order without descending into subdirectories; other extensions are skipped." example:"{!{images-directory.hyperbricks.yaml}}"`
	Width              int    `mapstructure:"width" validate:"min=0" description:"Output width in integer pixels; omit or use 0 to preserve aspect ratio from height. Omit both dimensions to keep each source size." example:"{!{images-width.hyperbricks.yaml}}"`
	Height             int    `mapstructure:"height" validate:"min=0" description:"Output height in integer pixels; omit or use 0 to preserve aspect ratio from width. Setting both dimensions resizes to that exact size." example:"{!{images-height.hyperbricks.yaml}}"`
	Id                 string `mapstructure:"id" description:"Id of images with a index added to it" example:"{!{images-id.hyperbricks.yaml}}"`
	Class              string `mapstructure:"class" description:"CSS class for styling the image" example:"{!{images-class.hyperbricks.yaml}}"`
	IsStatic           bool   `mapstructure:"is_static" exclude:"true" description:"Flag indicating if the images are static" example:"{!{images-is_static.hyperbricks.yaml}}"`
	Alt                string `mapstructure:"alt" description:"Alternative text, automatically HTML-escaped. An empty value renders an empty alt attribute for decorative images; supply meaningful text for informative images." example:"{!{images-alt.hyperbricks.yaml}}"`
	Title              string `mapstructure:"title" description:"The title attribute of the image" example:"{!{images-title.hyperbricks.yaml}}"`
	Quality            int    `mapstructure:"quality" description:"JPEG encoding quality from 1 to 100; omit or use 0 for 90. Does not affect PNG or GIF encoding." example:"{!{images-quality.hyperbricks.yaml}}"`
	Loading            string `mapstructure:"loading" description:"Lazy loading strategy (e.g., 'lazy', 'eager')" example:"{!{images-loading.hyperbricks.yaml}}"`
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
			Hash: shared.GenerateHash(),
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
			Hash:     shared.GenerateHash(),
			Key:      config.Component.Meta.HyperBricksKey,
			Path:     config.Component.Meta.HyperBricksPath,
			File:     config.Component.Meta.HyperBricksFile,
			Type:     MultipleImagesConfigGetName(),
			Err:      fmt.Errorf("failed to process multiple images: %w", err).Error(),
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
