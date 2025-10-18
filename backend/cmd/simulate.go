package cmd

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"

	iotSimulator "github.com/moto-nrw/project-phoenix/simulator/iot"
	"github.com/spf13/cobra"
)

const (
	defaultSimulatorConfig = "simulator/iot/simulator.yaml"
	envSimulatorConfig     = "SIMULATOR_CONFIG"
)

// simulateCmd bootstraps the IoT simulator in its minimal form.
var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Run the IoT runtime simulator bootstrap",
	Long: `Starts the IoT simulator bootstrap process. The bootstrap currently authenticates all
configured devices against the IoT API so you can verify connectivity before enabling
dynamic event generation in follow-up iterations.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		configPath := resolveSimulatorConfigPath()
		cfg, err := iotSimulator.LoadConfig(configPath)
		if err != nil {
			log.Fatalf("Failed to load simulator config: %v", err)
		}

		if err := iotSimulator.Run(ctx, cfg); err != nil {
			if errors.Is(err, iotSimulator.ErrPartialAuthentication) {
				log.Fatalf("Simulator completed with authentication errors: %v", err)
			}
			log.Fatalf("Simulator failed: %v", err)
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
