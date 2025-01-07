package component

import (
	"fmt"

	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
)

// SupportedExtensions contains the image formats supported for processing
var SupportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".svg":  true,
}

// ImageProcessor handles rendering and processing of images
type ImageProcessor struct{}

// RenderImageConfig represents the configuration for rendering a single or multiple images
type RenderImageConfig struct {
	Single   *SingleImageConfig    `mapstructure:"single"`
	Multiple *MultipleImagesConfig `mapstructure:"multiple"`
}

// Render processes the image rendering based on the instance configuration
func (ir *ImageProcessor) Render(instance interface{}) (string, error) {
	config, ok := instance.(RenderImageConfig)
	if !ok {
		return "", fmt.Errorf("invalid configuration type for ImageProcessor")
	}

	var builder strings.Builder

	if config.Single != nil {
		output, err := ir.ProcessSingleImage(*config.Single)
		if err != nil {
			return "", err
		}
		builder.WriteString(output)
	}

	if config.Multiple != nil {
		output, err := ir.ProcessMultipleImages(*config.Multiple)
		if err != nil {
			return "", err
		}
		builder.WriteString(output)
	}

	return builder.String(), nil
}

// ProcessSingleImage processes a single image and generates the HTML
func (ir *ImageProcessor) ProcessSingleImage(config SingleImageConfig) (string, error) {
	builder := &strings.Builder{}

	hbConfig := shared.GetHyperBricksConfiguration()
	destDir := hbConfig.Directories["static"] + "/images/"
	if config.IsStatic {
		destDir = hbConfig.Directories["render"] + "/images/"
	}

	err := ir.processAndBuildImgTag(config.Src, destDir, config, builder)
	if err != nil {
		logging.GetLogger().Errorw("Error processing image", "file", config.Path, "error", err)
		return "", err
	}

	return builder.String(), nil
}

// ProcessMultipleImages processes multiple images and adjusts their dimensions if width/height is specified
func (ir *ImageProcessor) ProcessMultipleImages(config MultipleImagesConfig) (string, error) {
	builder := &strings.Builder{}

	files, err := os.ReadDir(config.Directory)
	if err != nil {
		logging.GetLogger().Errorw("Error reading directory", "directory", config.Directory, "error", err)
		return "", err
	}
	hbConfig := shared.GetHyperBricksConfiguration()
	destDir := hbConfig.Directories["static"] + "/images/"
	if config.IsStatic {
		destDir = hbConfig.Directories["render"] + "/images/"
	}

	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if !SupportedExtensions[ext] || ext == ".svg" {
			continue
		}

		srcFilePath := filepath.Join(config.Directory, file.Name())

		fileConfig := SingleImageConfig{
			Component: shared.Component{
				Meta: shared.Meta{
					Path: srcFilePath,
				},
			},
			Width:    config.Width,
			Height:   config.Height,
			IsStatic: config.IsStatic,
		}
		logging.GetLogger().Debugf("Creating new image file", "source", srcFilePath, "destination", destDir)
		err := ir.processAndBuildImgTag(srcFilePath, destDir, fileConfig, builder)
		if err != nil {
			logging.GetLogger().Errorw("Error processing image", "file", srcFilePath, "error", err)
			continue
		}
	}

	return builder.String(), nil
}

func (ir *ImageProcessor) processAndBuildImgTag(srcPath, destDir string, config SingleImageConfig, builder *strings.Builder) error {
	newFileName, err := ir.processImage(srcPath, destDir, config)
	if err != nil {
		return err
	}

	builder.WriteString("<img src=\"")
	builder.WriteString(fmt.Sprintf("static/images/%s\"", newFileName))

	addDimensions(newFileName, builder)
	addOptionalAttributes(config, builder)

	builder.WriteString(" />")
	return nil
}

func (ir *ImageProcessor) processImage(srcPath, destDir string, config SingleImageConfig) (string, error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source image: %v", err)
	}
	defer srcFile.Close()

	srcImage, format, err := image.Decode(srcFile)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %v", err)
	}

	baseName := filepath.Base(srcPath)
	ext := strings.ToLower(filepath.Ext(baseName))
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	width, err := getInt(config.Width)
	if err != nil && config.Width > 0 {
		return "", err
	}

	height, err := getInt(config.Height)
	if err != nil && config.Height > 0 {
		return "", err
	}

	if width == 0 && height == 0 {
		width = srcImage.Bounds().Dx()
		height = srcImage.Bounds().Dy()
	} else if width == 0 {
		width = (height * srcImage.Bounds().Dx()) / srcImage.Bounds().Dy()
	} else if height == 0 {
		height = (width * srcImage.Bounds().Dy()) / srcImage.Bounds().Dx()
	}

	resizedImage := imaging.Resize(srcImage, width, height, imaging.Lanczos)

	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %v", err)
	}

	newFileName := fmt.Sprintf("%s_w%d_h%d%s", nameWithoutExt, width, height, ext)
	destPath := filepath.Join(destDir, newFileName)

	destFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %v", err)
	}
	defer destFile.Close()

	switch format {
	case "jpeg":
		quality := 90
		if config.Quality > 0 {
			quality, err = getInt(config.Quality)
			if err != nil {
				return "", err
			}
		}
		err = jpeg.Encode(destFile, resizedImage, &jpeg.Options{Quality: quality})
	case "png":
		err = png.Encode(destFile, resizedImage)
	case "gif":
		err = gif.Encode(destFile, resizedImage, nil)
	default:
		return "", fmt.Errorf("unsupported image format: %s", format)
	}

	if err != nil {
		return "", fmt.Errorf("failed to encode and save image: %v", err)
	}

	return newFileName, nil
}

func addDimensions(fileName string, builder *strings.Builder) {
	parts := strings.Split(fileName, "_w")
	if len(parts) > 1 {
		sizeParts := strings.Split(parts[1], "_h")
		if len(sizeParts) > 1 {
			width := sizeParts[0]
			height := strings.TrimSuffix(sizeParts[1], filepath.Ext(sizeParts[1]))
			builder.WriteString(fmt.Sprintf(" width=\"%s\" height=\"%s\"", width, height))
		}
	}
}

// addOptionalAttributes adds optional and extra attributes to the image tag.
func addOptionalAttributes(config SingleImageConfig, builder *strings.Builder) {
	if config.Alt != "" {
		builder.WriteString(fmt.Sprintf(` alt="%s"`, config.Alt))
	}

	if config.Title != "" {
		builder.WriteString(fmt.Sprintf(` title="%s"`, config.Title))
	}

	if config.Class != "" {
		builder.WriteString(fmt.Sprintf(` class="%s"`, config.Class))
	}

	if config.Loading == "lazy" {
		builder.WriteString(` loading="lazy"`)
	}

	// Define allowed extra attributes for the image component
	allowedAttributes := []string{"id", "data-role", "data-action", "aria-label", "role", "style"}

	// Render extra attributes
	extraAttributes := shared.RenderAllowedAttributes(config.ExtraAttributes, allowedAttributes)
	builder.WriteString(extraAttributes)
}

func getInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("unsupported type for integer conversion: %T", v)
	}
}
