package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

type Config struct {
	Port int32 `json:"port"`
}

var (
	StartMode   bool
	StartModule string
	Port        int32
	Production  bool
	Debug       bool
)

func GetModule() string {
	return StartModule
}

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

			fmt.Printf("Starting server with config: %s on port: %d\n", StartModule, config.Port)
		},
	}
	cmd.Flags().StringVarP(&StartModule, "module", "m", "default", "module in the modules dorectory")
	cmd.Flags().Int32VarP(&Port, "port", "p", 8080, "port")
	cmd.Flags().BoolVarP(&Production, "production", "P", false, "set production mode")
	cmd.Flags().BoolVarP(&Debug, "debug", "d", false, "debug")
	return cmd
}
