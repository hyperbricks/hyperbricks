package ui

import _ "embed"

// Stylesheet is the compiled DaisyUI stylesheet shared by HyperBricks web tools.
//
//go:embed web/hyperbricks.css
var Stylesheet []byte

// ThemeScript applies the shared light/dark preference before the UI is painted.
//
//go:embed web/theme.js
var ThemeScript []byte

//go:embed web/brandmark.svg
var LogoMark []byte

//go:embed web/favicon.svg
var Favicon []byte

//go:embed web/lucide.js
var IconsScript []byte
