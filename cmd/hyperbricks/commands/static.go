package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	RenderStatic bool
)

// NewHelloCommand creates the "hello" subcommand
func NewMakeStaticCommand() *cobra.Command {
	var static bool

	cmd := &cobra.Command{
		Use:   "static",
		Short: "Render static content",
		Run: func(cmd *cobra.Command, args []string) {
			RenderStatic = true

			config := Config{
				Port: Port,
			}
			if StartModule != "default" {
				StartModule := fmt.Sprintf("modules/%s/package.hyperbricks", StartModule)
				data, err := os.ReadFile(StartModule)
				if err != nil {
					fmt.Printf("Error reading config file: %v\n", err)
					return
				}
				if err := json.Unmarshal(data, &config); err != nil {
					//fmt.Printf("Error parsing config file: %v\n", err)
					return
				}
			}
		},
	}

	// Add flags
	cmd.Flags().BoolVarP(&static, "static", "s", true, "Render static content")
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module in the ./modules directory")

	return cmd
}
