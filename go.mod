module github.com/hyperbricks/hyperbricks

go 1.26.1

require (
	github.com/Masterminds/semver/v3 v3.5.0
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/charmbracelet/bubbles v0.20.0
	github.com/charmbracelet/bubbletea v1.2.4
	github.com/charmbracelet/lipgloss v1.0.0
	github.com/disintegration/imaging v1.6.2
	github.com/dop251/goja v0.0.0-20260903201622-f87b40ad7341
	github.com/eiannone/keyboard v0.0.0-20220611211555-0d226195f203
	github.com/emirpasic/gods v1.18.1
	github.com/evanw/esbuild v0.25.7
	github.com/fsnotify/fsnotify v1.8.0
	github.com/go-playground/validator/v10 v10.23.0
	github.com/golang-jwt/jwt/v4 v4.5.2
	github.com/mitchellh/mapstructure v1.5.0
	github.com/otiai10/copy v1.14.1
	github.com/shirou/gopsutil v3.21.11+incompatible
	github.com/spf13/cobra v1.8.1
	github.com/tetratelabs/wazero v1.12.0
	github.com/yosssi/gohtml v0.0.0-20201013000340-ee4748c638f4
	go.uber.org/zap v1.27.0
	go.yaml.in/yaml/v4 v4.0.0-rc.4
	golang.org/x/time v0.11.0
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/x/ansi v0.4.5 // indirect
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.15.2 // indirect
	github.com/otiai10/mint v1.6.3 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.11.0 // indirect
	github.com/sahilm/fuzzy v0.1.1 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8 // indirect
	golang.org/x/net v0.33.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.22.0 // indirect
)

// Early alpha versions predate the runtime cleanup and are unsupported.
retract [v0.1.0-alpha, v0.8.4-alpha]
