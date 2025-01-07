package shared

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/internal/typefactory"
	"github.com/mitchellh/mapstructure"
)

const (
	tpswarningformat string = "<!-- tps-warn:[%s] %s-->"
)

// returns a anchor with error
func HyperbricksDecodeError(err string, path string, key string) (string, []ComponentError) {
	errHash := HyperScriptErrorHash(path)
	anchor := fmt.Sprintf("<a data-hyperbricks-error=\"%s\">", errHash)
	return anchor, []ComponentError{
		{
			Key:  key,
			Path: path,
			Err:  fmt.Errorf("%s: %s (hash:%s)", err, path, errHash).Error(),
		},
	}
}

func HyperScriptErrorHash(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func RenderValidationWarningsWithPath(warnings []string, hyperbrickspath []string) string {

	var html string = ""
	for _, warning := range warnings {
		path := strings.Join(hyperbrickspath, ".")
		html += fmt.Sprintf(tpswarningformat, path, warning)
	}
	return html
}

func RenderValidationWarnings(warnings []string) string {

	var html string = ""
	for _, warning := range warnings {
		html += fmt.Sprintf(tpswarningformat, "", warning)
	}
	return html
}

func RenderWarning(warning string) string {
	return fmt.Sprintf(tpswarningformat, "", warning)
}

func DecodeWithBasicHooks(instance interface{}, config interface{}) []error {
	var errors []error
	combinedHook := mapstructure.ComposeDecodeHookFunc(
		typefactory.StringToSliceHookFunc(),
		typefactory.StringToIntHookFunc(),
		typefactory.StringToMapStringHookFunc(),
	)

	// Set up the decoder with appropriate configuration
	decoderConfig := &mapstructure.DecoderConfig{
		Metadata:         nil,
		DecodeHook:       combinedHook,
		Result:           config,
		TagName:          "mapstructure",
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decoderConfig)
	if err != nil {
		errors = append(errors, ComponentError{
			Err: err.Error(),
		})
	}

	err = decoder.Decode(instance)
	if err != nil {
		errors = append(errors, ComponentError{
			Err: err.Error(),
		})
	}
	return errors
}
