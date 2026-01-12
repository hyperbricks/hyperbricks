package commands

import (
	"github.com/spf13/cobra"
)

var (
	RenderStatic    bool
	ServeStatic     bool
	ForceStatic     bool
	ExportZip       bool
	ExportOutDir    string
	ExportExclude   string
	StaticWizard    bool
	StaticServePort int
)

func NewMakeStaticCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "static",
		Short: "Render static content",
		Run: func(cmd *cobra.Command, args []string) {
			if cmd.Flags().NFlag() == 0 {
				RunStaticWizard()
				return
			}
			RenderStatic = true
		},
	}

	// Add flags
	cmd.Flags().BoolVar(&ServeStatic, "serve", false, "Serve rendered static files")
	cmd.Flags().BoolVar(&ForceStatic, "force", false, "Overwrite rendered output without confirmation")
	cmd.Flags().BoolVar(&ExportZip, "zip", false, "Export rendered output as a zip file")
	cmd.Flags().StringVar(&ExportOutDir, "out", "", "Output directory for zip export (default ./exports/<module>)")
	cmd.Flags().StringVar(&ExportExclude, "exclude", "", "Comma-separated paths to exclude, relative to render root")
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module in the ./modules directory")

	return cmd
}
