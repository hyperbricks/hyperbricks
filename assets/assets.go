package assets

import (
	_ "embed"
)

//go:embed version.md
var VersionMD string

//go:embed dashboard.html
var Dashboard string

//go:embed dashboard.css
var DashboardCSS string

//go:embed deploy_dashboard.html
var DeployDashboard string

// DeployYAMLEditorScript is the generated, lazy-loaded deployment YAML editor.
//
//go:embed deploy_yaml_editor.js
var DeployYAMLEditorScript []byte

//go:embed errors.html
var ErrorsPage string

//go:embed errors.css
var ErrorsCSS []byte

//go:embed errors.js
var ErrorsScript []byte

//go:embed errors-model.mjs
var ErrorsModel []byte
