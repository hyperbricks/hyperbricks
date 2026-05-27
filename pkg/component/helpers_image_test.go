package component

import (
	"strings"
	"testing"
)

func TestAddDimensionsUsesFinalGeneratedSuffix(t *testing.T) {
	var builder strings.Builder

	addDimensions("cute_cat_w800_h800_w100_h100.jpg", &builder)

	if got, want := builder.String(), ` width="100" height="100"`; got != want {
		t.Fatalf("addDimensions() = %q, want %q", got, want)
	}
}

func TestAddDimensionsKeepsStaticGeneratedFileSupport(t *testing.T) {
	var builder strings.Builder

	addDimensions("static/images/cute_cat_w320_h240.jpg", &builder)

	if got, want := builder.String(), ` width="320" height="240"`; got != want {
		t.Fatalf("addDimensions() = %q, want %q", got, want)
	}
}

func TestImageDimensionsFromFileNameRejectsNonNumericSuffix(t *testing.T) {
	if width, height, ok := imageDimensionsFromFileName("hero_wide_hall.jpg"); ok {
		t.Fatalf("imageDimensionsFromFileName() = %q, %q, true; want false", width, height)
	}
}
