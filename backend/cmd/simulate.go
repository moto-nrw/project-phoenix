package cmd

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	iotSimulator "github.com/moto-nrw/project-phoenix/simulator/iot"
	"github.com/spf13/cobra"
)

const (
	envSimulatorConfig = "SIMULATOR_CONFIG"
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

		configFlag, err := cmd.Flags().GetString("config")
		if err != nil {
			logger.Logger.WithError(err).Fatal("Failed to read simulator config flag")
		}

		configPath, err := resolveSimulatorConfigPath(configFlag)
		if err != nil {
			logger.Logger.WithError(err).Fatal("Simulator config path is required")
		}

		cfg, err := iotSimulator.LoadConfig(configPath)
		if err != nil {
			logger.Logger.WithError(err).Fatal("Failed to load simulator config")
		}

		if err := iotSimulator.Run(ctx, cfg); err != nil {
			if errors.Is(err, iotSimulator.ErrPartialAuthentication) {
				logger.Logger.WithError(err).Fatal("Simulator completed with authentication errors")
			}
			logger.Logger.WithError(err).Fatal("Simulator failed")
		}
	},
}

func init() {
	RootCmd.AddCommand(simulateCmd)
	simulateCmd.Flags().String("config", "", "Path to simulator config (or set SIMULATOR_CONFIG env var)")
}

func resolveSimulatorConfigPath(configFlag string) (string, error) {
	configPath := strings.TrimSpace(configFlag)
	if configPath == "" {
		configPath = strings.TrimSpace(os.Getenv(envSimulatorConfig))
	}
	if configPath == "" {
		return "", errors.New("SIMULATOR_CONFIG environment variable or --config flag is required")
	}
	return filepath.Clean(configPath), nil
}
