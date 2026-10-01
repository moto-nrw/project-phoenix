package cmd

import (
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/spf13/cobra"
)

// workerCmd runs the background jobs outside the HTTP server (#2726).
var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "run the background jobs as a standalone worker",
	Long: `Runs the background jobs without the HTTP API. The worker waits in
standby until it holds the database lease; only the holder runs jobs, so it
may start beside a serve process that still runs the embedded worker.
It needs the same configuration as serve and listens on PORT for /health
(alive), /ready (holds the lease) and /internal/metrics.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServeRoot(cmd, api.ServeWorkerOnly)
	},
}

func init() {
	RootCmd.AddCommand(workerCmd)
}
