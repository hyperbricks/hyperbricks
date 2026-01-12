package composite

import (
	"context"
	"fmt"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/renderer"
	"github.com/hyperbricks/hyperbricks/pkg/shared"
	"github.com/mitchellh/mapstructure"
)

// HyperMediaConfig represents configuration hypermedia.
type HyperMediaConfig struct {
	shared.Composite   `mapstructure:",squash"`
	MetaDocDescription string                 `mapstructure:"@doc" description:"HYPERMEDIA description" example:"{!{hypermedia-@doc.hyperbricks}}"`
	Beautify           *bool                  `mapstructure:"beautify" json:"Beautify,omitempty" description:"Override server.beautify for this object when rendered directly"`
	Title              string                 `mapstructure:"title" description:"The title of the hypermedia site" example:"{!{hypermedia-title.hyperbricks}}"`
	Route              string                 `mapstructure:"route" description:"The route (URL-friendly identifier) for the hypermedia" example:"{!{hypermedia-route.hyperbricks}}"`
	Section            string                 `mapstructure:"section" description:"The section the hypermedia belongs to. This can be used with the component <MENU> for example." example:"{!{hypermedia-section.hyperbricks}}"`
	Items              map[string]interface{} `mapstructure:",remain"`
	BodyTag            string                 `mapstructure:"bodytag" description:"Special body enclose with use of |. Please note that this will not work when a <HYPERMEDIA>.template is configured. In that case, you have to add the bodytag in the template." example:"{!{hypermedia-bodytag.hyperbricks}}"`
	Enclose            string                 `mapstructure:"enclose" description:"Enclosure of the property for the hypermedia" example:"{!{hypermedia-enclose.hyperbricks}}"`
	Favicon            string                 `mapstructure:"favicon" description:"Path to the favicon for the hypermedia" example:"{!{hypermedia-favicon.hyperbricks}}"`
	Template           map[string]interface{} `mapstructure:"template" description:"Template configurations for rendering the hypermedia. See <TEMPLATE> for field descriptions." example:"{!{hypermedia-template.hyperbricks}}"`
	Cache              string                 `mapstructure:"cache" description:"Cache expire string" example:"{!{hypermedia-cache.hyperbricks}}"`
	NoCache            bool                   `mapstructure:"nocache" description:"Explicitly deisable cache" example:"{!{hypermedia-nocache.hyperbricks}}"`
	Static             string                 `mapstructure:"static" description:"Static file path associated with the hypermedia, for rendering out the hypermedia to static files." example:"{!{hypermedia-static.hyperbricks}}"`
	Index              int                    `mapstructure:"index" description:"Index number is a sort order option for the hypermedia defined in the section field. See <MENU> for further explanation and field options" example:"{!{hypermedia-index.hyperbricks}}"`
	Doctype            string                 `mapstructure:"doctype" description:"Alternative Doctype for the HTML document" example:"{!{hypermedia-doctype.hyperbricks}}"`
	HtmlTag            string                 `mapstructure:"htmltag" description:"The opening HTML tag with attributes" example:"{!{hypermedia-htmltag.hyperbricks}}"`
	Head               map[string]interface{} `mapstructure:"head" description:"Configurations for the head section of the hypermedia" example:"{!{hypermedia-head.hyperbricks}}"`
	Headers            map[string]string      `mapstructure:"headers" description:"HTTP response headers to include when serving this hypermedia" example:"{!{hypermedia-headers.hyperbricks}}"`
	ContentType        string                 `mapstructure:"content_type" description:"content type header definition"`
}

// HyperMediaConfigGetName returns the HyperBricks type associated with the HyperMediaConfig.
func HyperMediaConfigGetName() string {
	return "<HYPERMEDIA>"
}

// Validate ensures that the page has valid data.
func (hm *HyperMediaConfig) Validate() []error {
	if hm.Doctype == "" {
		hm.Doctype = "<!DOCTYPE html>"
	}

	if hm.HtmlTag == "" {
		hm.HtmlTag = "<html>"
	}

	var warnings []error
	return warnings
}

// HyperMediaRenderer handles rendering of PAGE content.
type HyperMediaRenderer struct {
	renderer.CompositeRenderer
}

// Ensure HyperMediaRenderer implements renderer.RenderComponent interface.
var _ shared.CompositeRenderer = (*HyperMediaRenderer)(nil)

func (r *HyperMediaRenderer) Types() []string {
	return []string{
		HyperMediaConfigGetName(),
	}
}

// errorTemplate is the embedded Go template as a string
const ErrorPanelTemplate = `
<style>

    .error-panel, .succes-panel {
		display: none;
		opacity:0.5;
		font-family: monospace;
    	font-size: 12px;
        position: fixed;
		right:10px;
		bottom:10px;
		margin:5px;
        width: 50%;
        flex-direction: column;
        border-radius: 5px;
        box-shadow: 2px 2px 10px rgba(0, 0, 0, 0.3);
		
        z-index: 9999;
        overflow: hidden;
    }

    .error-panel {
        border: 1px solid rgb(255, 98, 98);
        background: rgba(255, 230, 230, 0.9);
    }

    .succes-panel {
        border: 1px solid  rgb(98, 255, 161);
        background: rgba(230, 255, 230, 0.9);
        padding: 10px;
        text-align: center;
        font-weight: bold;
        color: green;
    }

    .error-header {
        background:  rgb(255, 98, 98);
        color: white;
        padding: 10px;
        cursor: pointer;
        font-weight: bold;
        text-align: center;
    }

    .error-content {
        display: none;
        overflow-y: auto;
        max-height: 300px;
        padding: 10px;
    }

    .frontent_errors {
        list-style: none;
        padding: 0;
        margin: 0;
    }

	.frontent_errors li {
		padding: 10px;
		margin-bottom: 6px;
		border-radius: 7px;
		border-bottom: 1px solid #ddd;
		background: rgb(255 255 255);
	}

    .frontent_errors .error_message {
        color: #000000;
        font-weight: bold;
		
    }
	.error_mark {
		background-color: #ffebeb;
    	padding: 0px;
	}
	.error_type {color: #b00; overflow-wrap: break-word;}
	.error_file {color: #b00; overflow-wrap: break-word;}
	.error_path {color: #b00; overflow-wrap: break-word;}
	.error_error {    
		color: #0600ade8;
    	overflow-wrap: break-word;
    	padding-bottom: 15px;
	}
	.error_number {
		background-color: #ffa8a9;
   		color: #fff7f7;
		position: relative;
		border-radius: 50%; /* Makes it round */
		padding: 0; /* Adjust padding to make it a proper circle */
		display: inline-flex; /* Ensures proper alignment */
		align-items: center; /* Centers text vertically */
		justify-content: center; /* Centers text horizontally */
		min-width: 24px;
    	min-height: 24px;
		
	}
	
</style>
<div id="error_panel" class="error-panel">
    <div class="error-header" onclick="toggleErrorPanel()">HyperBricks errors</div>
    <div class="error-content">
        <ul id="error_list" class="frontent_errors">
            
        </ul>
    </div>
</div>
<script>
    function toggleErrorPanel() {
        var content = document.querySelector('.error-content');
        content.style.display = (content.style.display === 'block') ? 'none' : 'block';
		var pcontent = document.querySelector('.error-panel');
		pcontent.style.width = (pcontent.style.width === '50%') ? '30%' : '50%';
		pcontent.style.opacity = (pcontent.style.opacity === '1') ? '0.5' : '1';
    }
</script>
`

// Render implements the RenderComponent interface.
func (pr *HyperMediaRenderer) Render(instance interface{}, ctx context.Context) (string, []error) {

	var errors []error
	var config HyperMediaConfig

	err := mapstructure.Decode(instance, &config)
	if err != nil {
		return "", append(errors, shared.ComponentError{
			Hash: shared.GenerateHash(),
			Key:  config.Composite.Meta.HyperBricksKey,
			Path: config.Composite.Meta.HyperBricksPath,
			File: config.Composite.Meta.HyperBricksFile,
			Type: "<HYPERMEDIA>",
			Err:  fmt.Errorf("failed to decode instance into HeadConfig: %w", err).Error(),
		})
	}

	if config.ConfigType != "<HYPERMEDIA>" {
		errors = append(errors, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Key:      config.Composite.Meta.HyperBricksKey,
			Path:     config.Composite.Meta.HyperBricksPath,
			File:     config.Composite.Meta.HyperBricksFile,
			Type:     "<HYPERMEDIA>",
			Err:      fmt.Errorf("invalid type").Error(),
			Rejected: true,
		})
	}

	// appending validation errors
	errors = append(errors, config.Validate()...)

	// HEAD?
	var headbuilder strings.Builder
	var templatebuilder strings.Builder
	var treebuilder strings.Builder

	if config.BodyTag == "" {
		// emty bodyenclose fallback
		config.BodyTag = "<body>|</body>"
	}

	// Not sure how to handle this situation....
	// if no <HEAD> is defined create it
	//if config.Head == nil {
	//	config.Head = make(map[string]interface{})
	//}

	// If a main header config is present, render add it to the string builder
	if config.Head != nil || config.Title != "" || config.Favicon != "" {

		if config.Head == nil {
			config.Head = make(map[string]interface{})
		}

		//head := shared.StructToMap(config.Head)
		config.Head["@type"] = HeadConfigGetName()
		config.Head["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
		config.Head["hyperbrickspath"] = config.Composite.Meta.HyperBricksPath + config.Composite.Meta.HyperBricksKey

		if config.Title != "" {
			config.Head["title"] = config.Title
		}

		if config.Favicon != "" {
			config.Head["favicon"] = config.Favicon
		}

		result, errr := pr.RenderManager.Render(HeadConfigGetName(), config.Head, ctx)
		errors = append(errors, errr...)
		headbuilder.WriteString(result)
	}
	outputHtml := ""
	// TEMPLATE?
	if config.Template != nil {
		config.Template["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
		config.Template["hyperbrickspath"] = config.Composite.Meta.HyperBricksKey + ".template"

		// INSERT HEAD to TEMPLATE VALUES....
		// Ensure 'values' exists inside Template
		if _, exists := config.Template["values"]; !exists {
			config.Template["values"] = make(map[string]interface{})
		}

		// Set 'head' inside 'values'
		if config.Head != nil {
			config.Template["values"].(map[string]interface{})["head"] = config.Head
		}

		result, errr := pr.RenderManager.Render("<TEMPLATE>", config.Template, ctx)
		errors = append(errors, errr...)
		templatebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, templatebuilder.String())
	} else {

		// TREE
		if config.Composite.Items != nil {
			config.Composite.Items["hyperbricksfile"] = config.Composite.Meta.HyperBricksFile
			config.Composite.Items["hyperbrickspath"] = config.Composite.Meta.HyperBricksPath + config.Composite.Meta.HyperBricksKey
		}

		result, errr := pr.RenderManager.Render(TreeRendererConfigGetName(), config.Composite.Items, ctx)
		errors = append(errors, errr...)
		treebuilder.WriteString(result)
		outputHtml = shared.EncloseContent(config.Enclose, treebuilder.String())
	}
	finalHTML := ""
	errorPanelTemplateHtml := ""

	hbconfig := shared.GetHyperBricksConfiguration()
	if hbconfig.Development.FrontendErrors && hbconfig.Mode != shared.LIVE_MODE {
		errorPanelTemplateHtml = ErrorPanelTemplate
	}

	// errorPanelTemplate
	if config.Template != nil {
		finalHTML = outputHtml + errorPanelTemplateHtml
	} else {
		headHtml := headbuilder.String()
		// Wrap the content with the HTML structure
		finalHTML = fmt.Sprintf("%s%s%s%s%s</html>", config.Doctype, config.HtmlTag, headHtml, shared.EncloseContent(config.BodyTag, outputHtml), errorPanelTemplateHtml)

	}

	return finalHTML, errors
}
