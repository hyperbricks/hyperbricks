package assets

import (
	_ "embed"
)

//go:embed logo.png
var Logo []byte

//go:embed version.md
var VersionMD string

//go:embed dashboard.html
var Dashboard string

//go:embed dashboard.css
var DashboardCSS string

//go:embed deploy_dashboard.html
var DeployDashboard string
