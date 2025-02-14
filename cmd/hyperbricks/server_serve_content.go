package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hyperbricks/hyperbricks/internal/component"
	"github.com/hyperbricks/hyperbricks/internal/composite"
	"github.com/hyperbricks/hyperbricks/internal/shared"
	"github.com/hyperbricks/hyperbricks/pkg/logging"
	"github.com/yosssi/gohtml"
)

func renderStaticContent(route string) string {
	hbConfig := getHyperBricksConfiguration()

	_config, found := getConfig(route)
	if !found {
		logging.GetLogger().Info("Config not found for route", "route", route)
		return fmt.Sprintf("Expected Hyperbricks '%s' was not found.", route)
	}

	configCopy := make(map[string]interface{})
	for key, value := range _config {
		configCopy[key] = value
	}

	if configCopy["@type"].(string) == composite.FragmentConfigGetName() {
		configCopy["hx_response"] = nil
	}

	htmlContent, renderErrors := rm.Render(configCopy["@type"].(string), configCopy)

	htmlContent = FrontEndErrorRender(renderErrors) + htmlContent

	if hbConfig.Server.Beautify {
		htmlContent = gohtml.Format(htmlContent)
	}

	cacheTime := time.Now().Format(time.RFC3339)
	htmlContent += fmt.Sprintf("\n<!-- Cached at: %s -->", cacheTime)
	return htmlContent
}

func renderContent(w http.ResponseWriter, route string) string {
	hbConfig := getHyperBricksConfiguration()

	_config, found := getConfig(route)
	if !found {
		__config, _found := getConfig("404")
		if _found {
			logging.GetLogger().Info("Redirecting to 404", " from ", route)
			_config = __config
		} else {
			if route == "favicon.ico" {
				return ""
			}
			logging.GetLogger().Info("Config not found for route: ", route)
			return fmt.Sprintf("Expected Hyperbricks '%s' was not found.", route)
		}
	}

	configCopy := make(map[string]interface{})
	for key, value := range _config {
		configCopy[key] = value
	}

	if configCopy["@type"].(string) == composite.FragmentConfigGetName() {
		configCopy["hx_response"] = w
	}

	var htmlContent strings.Builder

	renderOutput, renderErrors := rm.Render(configCopy["@type"].(string), configCopy)

	htmlContent.WriteString(renderOutput)
	var output strings.Builder

	if hbConfig.Server.Beautify {
		output.WriteString(gohtml.Format(htmlContent.String()))
	}

	cacheTime := time.Now().Format(time.RFC3339)
	output.WriteString(fmt.Sprintf("\n<!-- Cached at: %s -->", cacheTime))
	if hbConfig.Development.FrontendErrors {
		output.WriteString(FrontEndErrorRender(renderErrors))
	} else {
		output.WriteString(HandleRenderErrors(renderErrors))
	}
	return output.String()

}

// ComponentErrorTemplate represents the structure for rendering errors
type ComponentErrorTemplate struct {
	Type string
	File string
	Path string
	Key  string
	Err  string
}

// errorTemplate is the embedded Go template as a string
const errorTemplate = `{{if .HasErrors}}
{{safe "<!--  Begin Frontend Errors [development.frontend_errors = true] in package.hyperbricks  -->"}}

<style>

    .error-panel, .succes-panel {
		opacity:0.5;
		font-family: monospace;
    	font-size: 12px;
        position: fixed;
        bottom: 10px;
		right:10px;
		margin:5px;
        width: 190px;
        display: flex;
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
<div class="error-panel">
    <div class="error-header" onclick="toggleErrorPanel()"><span class="error_number">{{len .Errors}}</span> HyperBricks errors</div>
    <div class="error-content">
        <ul class="frontent_errors">
            {{range .Errors}}
                <li><span class="error_message">
				<div class="error_error">{{.Err}}</div>
				type <span class="error_type error_mark">{{.Type}}</span> at file
				<span class="error_file error_mark">{{.File}}.hyperbricks</span> at 
				<span class="error_path error_mark">{{.Path}}.{{.Key}}</span> 
                    
				</span>
                </li>
            {{end}}
        </ul>
    </div>
</div>
<script>
    function toggleErrorPanel() {
        var content = document.querySelector('.error-content');
        content.style.display = (content.style.display === 'block') ? 'none' : 'block';
		var pcontent = document.querySelector('.error-panel');
		pcontent.style.width = (pcontent.style.width === '500px') ? '190px' : '500px';
		pcontent.style.opacity = (pcontent.style.opacity === '1') ? '0.5' : '1';
    }
</script>
{{safe "<!--  End Frontend Errors [development.frontend_errors = true] in package.hyperbricks  -->"}}
{{else}}{{safe "<!-- No Errors -->"}}{{end}}
`

// ErrorData holds the errors and a flag to determine if there are any
type ErrorData struct {
	HasErrors bool
	Errors    []ComponentErrorTemplate
}

// HandleRenderErrors processes errors and returns a string with formatted errors
func FrontEndErrorRender(renderErrors []error) string {
	var errorsList []ComponentErrorTemplate

	for _, err := range renderErrors {
		if componentError, ok := err.(shared.ComponentError); ok {
			errorsList = append(errorsList, ComponentErrorTemplate{
				File: componentError.File,
				Type: componentError.Type,
				Path: componentError.Path,
				Key:  componentError.Key,
				Err:  componentError.Err,
			})
		} else {
			errorsList = append(errorsList, ComponentErrorTemplate{
				File: "Unknown",
				Type: "Unknown",
				Path: "Unknown",
				Key:  "Unknown",
				Err:  fmt.Sprintf("%v", err),
			})
		}
	}

	data := ErrorData{
		HasErrors: len(errorsList) > 0,
		Errors:    errorsList,
	}

	// Parse the embedded template
	tmpl, err := template.New("errorTemplate").Funcs(template.FuncMap{
		"safe": func(s string) template.HTML { return template.HTML(s) },
	}).Parse(errorTemplate)
	if err != nil {
		log.Println("Error parsing template:", err)
		return ""
	}

	// Render the template to a string
	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		log.Println("Error rendering template:", err)
		return ""
	}

	return output.String()
}

func HandleRenderErrors(renderErrors []error) string {
	errors := "\n"
	for e := range renderErrors {

		componentError, ok := renderErrors[e].(shared.ComponentError)
		if ok {
			errors += "<!-- Error " + fmt.Sprintf(`in file: %s at %s.%s|%v`, componentError.File, componentError.Path, componentError.Key, componentError.Err) + " -->\n"
		} else {
			e := error(renderErrors[e])
			errors += "<!-- Error:" + fmt.Sprintf("%v", e) + " -->\n"
		}
	}

	if errors != "" {
		return errors
	}
	return ""
}

func ServeContent(w http.ResponseWriter, r *http.Request) {
	hbConfig := getHyperBricksConfiguration()

	route := strings.TrimPrefix(r.URL.Path, "/")
	if route == "" {
		route = "index"
	}

	rm.GetRenderComponent("<MENU>").(*component.MenuRenderer).CurrentRoute = route
	logging.GetLogger().Debugw("Received request for route", "route", route)

	var htmlContent strings.Builder
	if hbConfig.Mode == shared.LIVE_MODE {
		htmlContent.WriteString(handleLiveMode(w, route))
	} else {
		htmlContent.WriteString(handleDeveloperMode(w, route))
	}

	//w.Header().Set("HX-Trigger", "Deleted")
	w.Header().Set("Content-Type", "text/html")
	if _, err := fmt.Fprint(w, htmlContent.String()); err != nil {
		logging.GetLogger().Errorw("Error writing response", "route", route, "error", err)
	} else {
		logging.GetLogger().Debugw("Served request", "route", route)
	}
}

// RENDER WITHOUT CACHE
func handleDeveloperMode(w http.ResponseWriter, route string) string {
	logging.GetLogger().Debugw("Developer mode active. Rendering fresh content:", route)
	return renderContent(w, route)
}

// RENDER WITH CACHE
func handleLiveMode(w http.ResponseWriter, route string) string {

	hbConfig := getHyperBricksConfiguration()
	cacheDuration := hbConfig.Live.CacheTime

	htmlCacheMutex.RLock()
	cacheEntry, found := htmlCache[route]
	htmlCacheMutex.RUnlock()

	if found && time.Since(cacheEntry.Timestamp) <= cacheDuration.Duration {
		logging.GetLogger().Debugw("Cache hit for route", "route", route)
		return cacheEntry.Content
	}

	if found {
		logging.GetLogger().Infof("Cache expired for route. Re-rendering content. %v", "route", route)
	} else {
		logging.GetLogger().Debugw("Cache miss for route. Rendering content. %v", "route", route)
	}

	htmlContent := renderContent(w, route)
	if htmlContent != "" {
		htmlCacheMutex.Lock()
		htmlCache[route] = CacheEntry{
			Content:   htmlContent,
			Timestamp: time.Now(),
		}
		htmlCacheMutex.Unlock()
		logging.GetLogger().Debugw("Updated cache for route", "route", route)
	}

	return htmlContent
}
