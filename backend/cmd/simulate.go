package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/moto-nrw/project-phoenix/logging"
	iotSimulator "github.com/moto-nrw/project-phoenix/simulator/iot"
	"github.com/spf13/cobra"
)

const (
	defaultSimulatorConfig = "simulator/iot/simulator.yaml"
	envSimulatorConfig     = "SIMULATOR_CONFIG"
)

// simulateCmd runs the IoT simulator state discovery loop.
var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Run the IoT simulator discovery loop",
	Long: `Starts the IoT simulator discovery loop. The simulator authenticates every configured device,
collects session/room/student/activity information, and keeps that snapshot fresh on the configured interval.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		configPath := resolveSimulatorConfigPath()
		cfg, err := iotSimulator.LoadConfig(configPath)
		if err != nil {
			logging.Logger.WithError(err).Fatal("Failed to load simulator config")
		}

		if err := iotSimulator.Run(ctx, cfg); err != nil {
			if errors.Is(err, iotSimulator.ErrPartialAuthentication) {
				logging.Logger.WithError(err).Fatal("Simulator completed with authentication errors")
			}
			logging.Logger.WithError(err).Fatal("Simulator failed")
		}
	},
}

func init() {
	RootCmd.AddCommand(simulateCmd)
}

func resolveSimulatorConfigPath() string {
	// Priority order: environment variable -> default path
	if envPath := os.Getenv(envSimulatorConfig); envPath != "" {
		return envPath
	}

	// Use default relative to current working directory
	return filepath.Clean(defaultSimulatorConfig)
}
