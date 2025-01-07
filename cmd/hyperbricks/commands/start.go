// commands/start.go
package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type Config struct {
	Port int32 `json:"port"`
	// Add other configuration fields as needed
}

var (
	StartMode   bool
	StartModule string
	Port        int32
)

// NewStartCommand creates the "start" subcommand
func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start server",
		Run: func(cmd *cobra.Command, args []string) {
			StartMode = true

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

			fmt.Printf("Starting server with config: %s on port: %d\n", StartModule, config.Port)
			// Add your server startup logic here, using config.Port and other parameters
		},
	}
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module in the modules dorectory")
	cmd.Flags().Int32VarP(&Port, "port", "p", 8080, "port")
	return cmd
}
