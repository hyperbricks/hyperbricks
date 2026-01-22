package assets

import (
	_ "embed"
)

//go:embed logo.png
var Logo []byte

//go:embed hyperbricks_logo_h_black_on_transparent.png
var Logo_Black []byte

//go:embed hyperbricks_logo_h_blue_on_transparent.png
var Logo_Blue []byte

//go:embed version.md
var VersionMD string

//go:embed dashboard.html
var Dashboard string

//go:embed dashboard.css
var DashboardCSS string

//go:embed deploy_dashboard.html
var DeployDashboard string
