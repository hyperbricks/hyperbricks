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

var SupportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".svg":  true,
}

type ImageProcessor struct{}

type RenderImageConfig struct {
	Single   *SingleImageConfig    `mapstructure:"single"`
	Multiple *MultipleImagesConfig `mapstructure:"multiple"`
}

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
				ExtraAttributes: config.ExtraAttributes,
				Enclose:         config.Enclose,
			},
			Width:   config.Width,
			Height:  config.Height,
			Loading: config.Loading,
			Alt:     config.Alt,
			Title:   config.Title,
			Id:      config.Id + fmt.Sprintf("%d", imgcount),
			Class:   config.Class,
			Quality: config.Quality,
		}

		logging.GetLogger().Debugf("Creating new image file", "source", srcFilePath, "destination", destDir)
		err := ir.processAndBuildImgTag(srcFilePath, destDir, fileConfig, builder)
		if err != nil {
			logging.GetLogger().Errorw("Error processing image", "file", srcFilePath, "error", err)
			continue
		}
		imgcount++
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
	if config.IsStatic {
		return srcPath, nil
	}

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

	if config.Id != "" {
		builder.WriteString(fmt.Sprintf(` id="%s"`, config.Id))
	}

	if config.Loading == "lazy" {
		builder.WriteString(` loading="lazy"`)
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
	}

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
