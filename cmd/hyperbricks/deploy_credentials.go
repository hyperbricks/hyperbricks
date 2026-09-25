package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
)

const maxDeployCredentialsRequestBytes = 16 * 1024

type deployCredentialValue struct {
	Source string `json:"source"`
	Value  string `json:"value"`
}

type deployCredentialsUpdate struct {
	ExpectedSHA256   string  `json:"expected_sha256"`
	User             *string `json:"user,omitempty"`
	Password         *string `json:"password,omitempty"`
	DashboardEnabled *bool   `json:"dashboard_enabled,omitempty"`
	SpacesEnabled    *bool   `json:"spaces_enabled,omitempty"`
}

var errCredentialSourceEdit = errors.New("this YAML structure cannot be changed safely by this form; use Edit package config")

func parseDeployCredentialDocument(content []byte) (*yaml.Node, error) {
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	if err := decoder.Decode(&document); err != nil {
		return nil, errors.New("cannot read developer settings from invalid YAML; use Edit package config")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("developer settings require a single YAML document; use Edit package config")
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, errCredentialSourceEdit
	}
	if err := validateDeployCredentialNodes(document.Content[0]); err != nil {
		return nil, err
	}
	return document.Content[0], nil
}

// Inspect source nodes, never resolved configuration: an env/file/conf resolver
// must not turn this administrative form into an endpoint for reading secrets.
func validateDeployCredentialNodes(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return errors.New("YAML aliases are unsupported; use Edit package config")
	}
	if node.Kind == yaml.MappingNode {
		keys := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			name := strings.TrimSpace(key.Value)
			if key.Kind != yaml.ScalarNode || name == "" || keys[name] {
				return errors.New("YAML contains an unsupported or duplicate mapping key; use Edit package config")
			}
			keys[name] = true
		}
	}
	for _, child := range node.Content {
		if err := validateDeployCredentialNodes(child); err != nil {
			return err
		}
	}
	return nil
}

func deployCredentialField(mapping *yaml.Node, name string) (*yaml.Node, *yaml.Node) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i < len(mapping.Content); i += 2 {
		if strings.TrimSpace(mapping.Content[i].Value) == name {
			return mapping.Content[i], mapping.Content[i+1]
		}
	}
	return nil, nil
}

func deployCredentialsMapping(root *yaml.Node) (*yaml.Node, error) {
	current := root
	for _, name := range []string{"hyperbricks", "development", "dashboard", "credentials"} {
		_, child := deployCredentialField(current, name)
		if child == nil || (child.Kind == yaml.ScalarNode && child.Tag == "!!null") {
			return nil, nil
		}
		if child.Kind != yaml.MappingNode {
			return nil, errCredentialSourceEdit
		}
		current = child
	}
	return current, nil
}

func deployCredentialBoolean(root *yaml.Node, path []string, defaultValue bool) (bool, error) {
	current := root
	for index, name := range path {
		_, child := deployCredentialField(current, name)
		if child == nil || (child.Kind == yaml.ScalarNode && child.Tag == "!!null") {
			return defaultValue, nil
		}
		if index == len(path)-1 {
			if child.Kind != yaml.ScalarNode {
				return false, errCredentialSourceEdit
			}
			switch child.Value {
			case "true":
				return true, nil
			case "false":
				return false, nil
			default:
				return false, errCredentialSourceEdit
			}
		}
		if child.Kind != yaml.MappingNode {
			return false, errCredentialSourceEdit
		}
		current = child
	}
	return defaultValue, nil
}

func describeDeployCredential(node *yaml.Node, username bool) (deployCredentialValue, error) {
	if node == nil || (node.Kind == yaml.ScalarNode && (node.Tag == "!!null" || node.Value == "" || (username && strings.TrimSpace(node.Value) == ""))) {
		return deployCredentialValue{Source: "missing"}, nil
	}
	if node.Kind == yaml.ScalarNode {
		return deployCredentialValue{Source: "literal", Value: node.Value}, nil
	}
	// Serialization is only for displaying the unresolved reference, never for
	// rewriting the package file. The original node and all resolvers stay intact.
	encoded, err := yaml.Marshal(node)
	if err != nil {
		return deployCredentialValue{}, errCredentialSourceEdit
	}
	return deployCredentialValue{Source: "reference", Value: strings.TrimSpace(string(encoded))}, nil
}

func writeDeployCredentialsResponse(w http.ResponseWriter, module, buildID string, location deployPackageConfigLocation, content []byte) error {
	root, err := parseDeployCredentialDocument(content)
	if err != nil {
		return err
	}
	credentials, err := deployCredentialsMapping(root)
	if err != nil {
		return err
	}
	_, userNode := deployCredentialField(credentials, "user")
	_, passwordNode := deployCredentialField(credentials, "password")
	user, err := describeDeployCredential(userNode, true)
	if err != nil {
		return err
	}
	password, err := describeDeployCredential(passwordNode, false)
	if err != nil {
		return err
	}
	dashboardEnabled, err := deployCredentialBoolean(root, []string{"hyperbricks", "development", "dashboard", "enabled"}, false)
	if err != nil {
		return err
	}
	spacesEnabled, err := deployCredentialBoolean(root, []string{"hyperbricks", "development", "frontend_editing", "spaces", "enabled"}, true)
	if err != nil {
		return err
	}
	frontendEditingEnabled, err := deployCredentialBoolean(root, []string{"hyperbricks", "development", "frontend_editing", "enabled"}, true)
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"module": module, "build_id": buildID, "scope": location.scope,
		"restart_required": location.running, "sha256": packageConfigSHA256(content),
		"user": user, "password": password,
		"dashboard_enabled": dashboardEnabled, "spaces_enabled": spacesEnabled,
		"frontend_editing_enabled": frontendEditingEnabled,
	})
	return nil
}

func decodeDeployCredentialsUpdate(w http.ResponseWriter, r *http.Request) (deployCredentialsUpdate, error) {
	var update deployCredentialsUpdate
	r.Body = http.MaxBytesReader(w, r.Body, maxDeployCredentialsRequestBytes)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return update, errors.New("invalid credentials request")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&update); err != nil {
		return update, errors.New("invalid credentials request")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return update, errors.New("credentials request must contain one JSON object")
	}
	if err := rejectNullDeployVisibilityFields(payload); err != nil {
		return update, err
	}
	update.ExpectedSHA256 = strings.ToLower(strings.TrimSpace(update.ExpectedSHA256))
	if len(update.ExpectedSHA256) != sha256.Size*2 || !isHex(update.ExpectedSHA256) {
		return update, errors.New("expected_sha256 is required; reopen the credentials dialog")
	}
	if update.User == nil && update.Password == nil && update.DashboardEnabled == nil && update.SpacesEnabled == nil {
		return update, errors.New("change credentials or a developer interface setting")
	}
	for _, value := range []*string{update.User, update.Password} {
		if value != nil && (*value == "" || !utf8.ValidString(*value) || strings.ContainsFunc(*value, unicode.IsControl)) {
			return update, errors.New("credentials must be non-empty UTF-8 text without control characters")
		}
	}
	if update.User != nil && (strings.TrimSpace(*update.User) == "" || strings.Contains(*update.User, ":")) {
		return update, errors.New("username must not be blank or contain a colon")
	}
	return update, nil
}

func rejectNullDeployVisibilityFields(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if _, err := decoder.Token(); err != nil {
		return errors.New("invalid credentials request")
	}
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return errors.New("invalid credentials request")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return errors.New("invalid credentials request")
		}
		if (name == "dashboard_enabled" || name == "spaces_enabled") && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return errors.New("developer interface toggles must be booleans")
		}
	}
	return nil
}

func handleDeployCredentialsRequest(w http.ResponseWriter, r *http.Request, module, buildID string, location deployPackageConfigLocation) {
	w.Header().Set("Cache-Control", "no-store")
	content, _, err := readRegularConfinedFile(location.moduleRoot, location.path)
	if err != nil || len(content) > maxPackageConfigBytes {
		writeError(w, http.StatusBadRequest, errors.New("package configuration is unavailable or too large"))
		return
	}
	if r.Method == http.MethodPut {
		update, err := decodeDeployCredentialsUpdate(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if update.ExpectedSHA256 != packageConfigSHA256(content) {
			writeError(w, http.StatusConflict, errors.New("package configuration changed; reopen the credentials dialog before saving"))
			return
		}
		patched, err := patchDeployCredentials(content, update)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err)
			return
		}
		if len(patched) > maxPackageConfigBytes {
			writeError(w, http.StatusRequestEntityTooLarge, errors.New("updated package configuration is too large"))
			return
		}
		var status int
		content, status, err = savePackageConfig(location, packageConfigUpdateRequest{Content: string(patched), ExpectedSHA256: update.ExpectedSHA256})
		if err != nil {
			// Validation errors can quote values from the package. Do not reflect
			// either saved or submitted credential values in an error or log.
			message := "developer settings could not be saved; check the package configuration with Edit package config"
			if status == http.StatusConflict {
				message = "package configuration changed; reopen the credentials dialog before saving"
			}
			writeError(w, status, errors.New(message))
			return
		}
	}
	if err := writeDeployCredentialsResponse(w, module, buildID, location, content); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err)
	}
}

func (api *deployLocalServer) handleBuildCredentials(w http.ResponseWriter, r *http.Request, module, buildID string) {
	w.Header().Set("Cache-Control", "no-store")
	location, err := api.localPackageConfig(module, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("package configuration is unavailable"))
		return
	}
	handleDeployCredentialsRequest(w, r, module, buildID, location)
}

func (api *deployAPI) handleBuildCredentials(w http.ResponseWriter, r *http.Request, module, buildID string) {
	w.Header().Set("Cache-Control", "no-store")
	location, err := api.remotePackageConfig(module, buildID)
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("package configuration is unavailable"))
		return
	}
	handleDeployCredentialsRequest(w, r, module, buildID, location)
}
