package component

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"html"

	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/disintegration/imaging"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

var SupportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
}

type ImageProcessor struct{}

type RenderImageConfig struct {
	Single   *SingleImageConfig    `mapstructure:"single"`
	Multiple *MultipleImagesConfig `mapstructure:"multiple"`
}

func (ir *ImageProcessor) Render(instance interface{}, ctx context.Context) (string, error) {
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

func (ir *ImageProcessor) ProcessSingleImage(config SingleImageConfig) (string, error) {
	builder := &strings.Builder{}

	hbConfig := shared.GetHyperBricksConfiguration()
	destDir := hbConfig.Directories["static"] + "/images/"
	if config.IsStatic {
		destDir = hbConfig.Directories["render"] + "/images/"
	}

	err := ir.processAndBuildImgTag(config.Src, destDir, config, builder)
	if err != nil {
		logging.GetLogger().Errorw("Error processing image", "file", config.HyperBricksPath, "error", err)
		return "", err
	}

	return builder.String(), nil
}

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
	imgcount := 0
	var processingErrors []error
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Name()))
		if file.IsDir() || !SupportedExtensions[ext] {
			continue
		}

		srcFilePath := filepath.Join(config.Directory, file.Name())

		fileConfig := SingleImageConfig{
			Component: shared.Component{
				Meta: shared.Meta{
					HyperBricksPath: srcFilePath,
				},
				ExtraAttributes: config.ExtraAttributes,
				Enclose:         config.Enclose,
			},
			Width:   config.Width,
			Height:  config.Height,
			Loading: config.Loading,
			Alt:     config.Alt,
			Title:   config.Title,
			Class:   config.Class,
			Quality: config.Quality,
		}
		if config.Id != "" {
			fileConfig.Id = config.Id + strconv.Itoa(imgcount)
		}

		logging.GetLogger().Debugf("Creating new image file", "source", srcFilePath, "destination", destDir)
		err := ir.processAndBuildImgTag(srcFilePath, destDir, fileConfig, builder)
		if err != nil {
			logging.GetLogger().Errorw("Error processing image", "file", srcFilePath, "error", err)
			processingErrors = append(processingErrors, fmt.Errorf("%s: %w", srcFilePath, err))
			continue
		}
		imgcount++
	}

	return builder.String(), errors.Join(processingErrors...)
}

func (ir *ImageProcessor) processAndBuildImgTag(srcPath, destDir string, config SingleImageConfig, builder *strings.Builder) error {
	newFileName, err := ir.processImage(srcPath, destDir, config)
	if err != nil {
		return err
	}

	builder.WriteString("<img src=\"")
	publicURL := &url.URL{Path: "/static/images/" + newFileName}
	builder.WriteString(html.EscapeString(publicURL.EscapedPath()))
	builder.WriteString("\"")

	addDimensions(newFileName, builder)
	addOptionalAttributes(config, builder)

	hbConfig := shared.GetHyperBricksConfiguration()
	if hbConfig.Server.SelfClosingTags {
		builder.WriteString(" />")
	} else {
		builder.WriteString(">")
	}
	return nil
}

func (ir *ImageProcessor) processImage(srcPath, destDir string, config SingleImageConfig) (string, error) {
	if config.Width < 0 || config.Height < 0 {
		return "", fmt.Errorf("image width and height must be non-negative pixel counts")
	}
	if config.Quality < 0 || config.Quality > 100 {
		return "", fmt.Errorf("image quality must be between 1 and 100, or 0 for the default")
	}
	if config.IsStatic {
		return srcPath, nil
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source image: %w", err)
	}
	defer srcFile.Close()

	// Include the source bytes, not just its basename, in the generated asset name.
	fingerprint := sha256.New()
	if _, err := io.Copy(fingerprint, srcFile); err != nil {
		return "", fmt.Errorf("failed to read source image: %w", err)
	}
	if _, err := srcFile.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("failed to rewind source image: %w", err)
	}

	srcImage, format, err := image.Decode(srcFile)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	baseName := filepath.Base(srcPath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	ext = strings.ToLower(ext)
	width, height := config.Width, config.Height

	if width == 0 && height == 0 {
		width = srcImage.Bounds().Dx()
		height = srcImage.Bounds().Dy()
	} else if width == 0 {
		width = max(1, (height*srcImage.Bounds().Dx())/srcImage.Bounds().Dy())
	} else if height == 0 {
		height = max(1, (width*srcImage.Bounds().Dy())/srcImage.Bounds().Dx())
	}

	resizedImage := imaging.Resize(srcImage, width, height, imaging.Lanczos)

	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %v", err)
	}

	quality := 0
	if format == "jpeg" {
		quality = config.Quality
		if quality == 0 {
			quality = 90
		}
	}
	fmt.Fprintf(fingerprint, "\x00resize-v1:%s:%d:%d:%d", format, width, height, quality)
	// Leave space for the fingerprint and dimensions within common 255-byte limits.
	if len(nameWithoutExt) > 160 {
		nameWithoutExt = nameWithoutExt[:160]
		for !utf8.ValidString(nameWithoutExt) {
			nameWithoutExt = nameWithoutExt[:len(nameWithoutExt)-1]
		}
	}
	newFileName := fmt.Sprintf("%s_%x_w%d_h%d%s", nameWithoutExt, fingerprint.Sum(nil)[:16], width, height, ext)
	destPath := filepath.Join(destDir, newFileName)

	// Publish only a complete image, even when concurrent renders share this name.
	destFile, err := os.CreateTemp(destDir, ".image-*")
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %v", err)
	}
	defer os.Remove(destFile.Name())
	defer destFile.Close()

	switch format {
	case "jpeg":
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
	if err := destFile.Chmod(0o644); err != nil {
		return "", fmt.Errorf("failed to set image permissions: %w", err)
	}
	if err := destFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close generated image: %w", err)
	}
	if err := os.Rename(destFile.Name(), destPath); err != nil {
		return "", fmt.Errorf("failed to publish generated image: %w", err)
	}

	return newFileName, nil
}

func addDimensions(fileName string, builder *strings.Builder) {
	width, height, ok := imageDimensionsFromFileName(fileName)
	if ok {
		builder.WriteString(fmt.Sprintf(" width=\"%s\" height=\"%s\"", width, height))
	}
}

func imageDimensionsFromFileName(fileName string) (string, string, bool) {
	baseName := filepath.Base(fileName)
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)
	widthIndex := strings.LastIndex(nameWithoutExt, "_w")
	if widthIndex < 0 {
		return "", "", false
	}

	dimensions := nameWithoutExt[widthIndex+len("_w"):]
	heightIndex := strings.Index(dimensions, "_h")
	if heightIndex < 0 {
		return "", "", false
	}

	width := dimensions[:heightIndex]
	height := dimensions[heightIndex+len("_h"):]
	if width == "" || height == "" {
		return "", "", false
	}
	if _, err := strconv.Atoi(width); err != nil {
		return "", "", false
	}
	if _, err := strconv.Atoi(height); err != nil {
		return "", "", false
	}

	return width, height, true
}

func addOptionalAttributes(config SingleImageConfig, builder *strings.Builder) {
	builder.WriteString(fmt.Sprintf(` alt="%s"`, html.EscapeString(config.Alt)))

	if config.Title != "" {
		builder.WriteString(fmt.Sprintf(` title="%s"`, html.EscapeString(config.Title)))
	}

	if config.Class != "" {
		builder.WriteString(fmt.Sprintf(` class="%s"`, html.EscapeString(config.Class)))
	}

	if config.Id != "" {
		builder.WriteString(fmt.Sprintf(` id="%s"`, html.EscapeString(config.Id)))
	}

	if config.Loading == "lazy" || config.Loading == "eager" {
		builder.WriteString(fmt.Sprintf(` loading="%s"`, config.Loading))
	}

	allowedAttributes := []string{
		"loading",
		"decoding",
		"srcset",
		"sizes",
		"crossorigin",
		"usemap",
		"longdesc",
		"referrerpolicy",
		"ismap",
		"class",
		"tabindex",
	}

	// A first-class field takes precedence over the same extra attribute.
	attributes := make(map[string]interface{}, len(config.ExtraAttributes))
	for key, value := range config.ExtraAttributes {
		if (key == "class" && config.Class != "") || (key == "loading" && config.Loading != "") {
			continue
		}
		attributes[key] = value
	}
	extraAttributes := shared.RenderAllowedAttributes(attributes, allowedAttributes)

	builder.WriteString(extraAttributes)
}
