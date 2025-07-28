package core

type ModuleConfiguredDirectories struct {
	// relative to ./modules/<mymodule> (ModuleDir)
	ResourcesDir   string
	StaticDir      string
	TemplateDir    string
	HyperbricksDir string
	RenderedDir    string

	// relative to the
	Root        string // the current working directory ./
	PluginsDir  string // ./bin/plugins or other location when configured
	ModulesRoot string // the root directory: ./modules
	ModuleDir   string // ./modules/<mymodule>
}

var (
	ModuleDirectories = ModuleConfiguredDirectories{}
)
