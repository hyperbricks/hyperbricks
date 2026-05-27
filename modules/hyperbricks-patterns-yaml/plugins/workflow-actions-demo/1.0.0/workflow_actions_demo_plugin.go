package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hyperbricks/hyperbricks/pkg/shared"
)

type workflowActionsDemoFields struct {
	Action   string `mapstructure:"action"`
	Template string `mapstructure:"template"`
}

type workflowActionsDemoConfig struct {
	shared.Component `mapstructure:",squash"`
	PluginName       string                    `mapstructure:"plugin"`
	Fields           workflowActionsDemoFields `mapstructure:"data"`
}

type workflowActionsDemoPlugin struct{}

var _ shared.PluginRenderer = (*workflowActionsDemoPlugin)(nil)

func (p *workflowActionsDemoPlugin) Render(instance interface{}, ctx context.Context) (any, []error) {
	var errs []error

	var cfg workflowActionsDemoConfig
	if err := shared.DecodeWithBasicHooks(instance, &cfg); err != nil {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      fmt.Sprintf("failed to decode workflow actions demo config: %v", err),
		})
		return "<!-- failed to decode workflow actions demo config -->", errs
	}

	req, _ := ctx.Value(shared.Request).(*http.Request)
	if req == nil {
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      "workflow actions demo plugin requires request context",
		})
		return "<!-- workflow actions demo plugin requires request context -->", errs
	}

	switch strings.TrimSpace(cfg.Fields.Action) {
	case "landing":
		return renderStage(cfg, lookupStageValues("", "")), errs
	case "lookup":
		return handleLookup(cfg, req), errs
	case "signup":
		return handleSignup(cfg, req), errs
	case "complete":
		return handleComplete(cfg, req), errs
	default:
		errs = append(errs, shared.ComponentError{
			Hash:     shared.GenerateHash(),
			Path:     cfg.HyperBricksPath,
			Key:      cfg.HyperBricksKey,
			Rejected: true,
			Err:      "unknown workflow actions demo action",
		})
		return "<!-- unknown workflow actions demo action -->", errs
	}
}

func handleLookup(cfg workflowActionsDemoConfig, req *http.Request) map[string]interface{} {
	if err := req.ParseForm(); err != nil {
		return renderStage(cfg, lookupStageValues("", "Could not parse the lookup request."))
	}

	email := strings.TrimSpace(req.FormValue("email"))
	if email == "" {
		return renderStage(cfg, lookupStageValues("", "Enter an email address first."))
	}

	if strings.HasSuffix(strings.ToLower(email), "@member.test") {
		return renderStage(cfg, existingMemberStageValues(email))
	}

	return renderStage(cfg, signupStageValues(email, "", ""))
}

func handleSignup(cfg workflowActionsDemoConfig, req *http.Request) map[string]interface{} {
	if err := req.ParseForm(); err != nil {
		return renderStage(cfg, signupStageValues("", "", "Could not parse the signup request."))
	}

	email := strings.TrimSpace(req.FormValue("email"))
	name := strings.TrimSpace(req.FormValue("name"))
	if email == "" || name == "" {
		return renderStage(cfg, signupStageValues(email, name, "Both email and display name are required."))
	}

	return renderStage(cfg, signupConfirmStageValues(email, name))
}

func handleComplete(cfg workflowActionsDemoConfig, req *http.Request) map[string]interface{} {
	if err := req.ParseForm(); err != nil {
		return renderStage(cfg, lookupStageValues("", "Could not complete the workflow."))
	}

	email := strings.TrimSpace(req.FormValue("email"))
	name := strings.TrimSpace(req.FormValue("name"))
	flow := strings.TrimSpace(req.FormValue("flow"))

	switch flow {
	case "existing":
		return renderStage(cfg, successStageValues(
			"Existing member confirmed",
			fmt.Sprintf("%s was found and attached to the workflow using the same plugin owner.", email),
		))
	case "signup":
		if email == "" || name == "" {
			return renderStage(cfg, signupStageValues(email, name, "The signup confirmation was incomplete."))
		}
		return renderStage(cfg, successStageValues(
			"New member created",
			fmt.Sprintf("%s is now registered as %s through the same plugin workflow.", email, name),
		))
	default:
		return renderStage(cfg, lookupStageValues("", "Unknown workflow completion state."))
	}
}

func renderStage(cfg workflowActionsDemoConfig, values map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"@type":    "<TEMPLATE>",
		"template": cfg.Fields.Template,
		"values":   values,
	}
}

func lookupStageValues(email string, message string) map[string]interface{} {
	return map[string]interface{}{
		"stage":         "lookup",
		"stage_title":   "Lookup step",
		"stage_intro":   "One explicit route calls one plugin with data.action = lookup. The plugin decides whether the workflow continues to an existing-member confirm or a signup step.",
		"email":         email,
		"error_message": strings.TrimSpace(message),
	}
}

func existingMemberStageValues(email string) map[string]interface{} {
	return map[string]interface{}{
		"stage":       "existing",
		"stage_title": "Existing member path",
		"stage_intro": "The lookup action found a known member. The next explicit route is still owned by the same plugin, but it runs a different branch of the workflow.",
		"email":       email,
	}
}

func signupStageValues(email string, name string, message string) map[string]interface{} {
	return map[string]interface{}{
		"stage":         "signup",
		"stage_title":   "Signup step",
		"stage_intro":   "The same plugin now handles a different explicit route with data.action = signup. That keeps the workflow cohesive without hiding route intent.",
		"email":         email,
		"name":          name,
		"error_message": strings.TrimSpace(message),
	}
}

func signupConfirmStageValues(email string, name string) map[string]interface{} {
	return map[string]interface{}{
		"stage":       "signup_confirm",
		"stage_title": "Signup confirmation",
		"stage_intro": "This route is still handled by the same plugin owner. Only the action and submitted fields changed.",
		"email":       email,
		"name":        name,
	}
}

func successStageValues(title string, message string) map[string]interface{} {
	return map[string]interface{}{
		"stage":           "success",
		"success_title":   strings.TrimSpace(title),
		"success_message": strings.TrimSpace(message),
	}
}

func Plugin() (shared.PluginRenderer, error) {
	return &workflowActionsDemoPlugin{}, nil
}
