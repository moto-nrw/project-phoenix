package cmd

import (
	api "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"

	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "start http server with configured api",
	Long:  `Starts a http server and serves the configured api`,
	Run: func(cmd *cobra.Command, args []string) {
		server, err := api.NewServer()
		if err != nil {
			logger.Logger.WithError(err).Fatal("Failed to create server")
		}
		server.Start()
	},
}

func init() {
	RootCmd.AddCommand(serveCmd)

	// JWT configuration must be explicitly provided via environment variables.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
