package cmd

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/moto-nrw/project-phoenix/simulate"
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
	Short: "Run the IoT simulator and simulation commands",
	Long: `Starts the IoT simulator discovery loop. The simulator authenticates every configured device,
collects session/room/student/activity information, and keeps that snapshot fresh on the configured interval.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		configPath := resolveSimulatorConfigPath()
		cfg, err := iotSimulator.LoadConfig(configPath)
		if err != nil {
			log.Fatalf("Failed to load simulator config: %v", err)
		}

		if err := assertNonProductionURL(cfg.BaseURL); err != nil {
			log.Fatal(err)
		}

		if err := iotSimulator.Run(ctx, cfg); err != nil {
			if errors.Is(err, iotSimulator.ErrPartialAuthentication) {
				log.Fatalf("Simulator completed with authentication errors: %v", err)
			}
			log.Fatalf("Simulator failed: %v", err)
		}
	},
}

var simulateFullDayCmd = &cobra.Command{
	Use:   "full-day",
	Short: "Run a one-shot full-day simulation",
	Long:  `Reads .seed-state.json, then exercises all API flows: RFID assignment, sessions, attendance, check-ins, and optionally end-of-day cleanup.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		statePath, _ := cmd.Flags().GetString("state")
		closeSessions, _ := cmd.Flags().GetBool("close")
		verbose, _ := cmd.Flags().GetBool("verbose")

		state, err := seedapi.LoadSeedState(statePath)
		if err != nil {
			log.Fatalf("Failed to load seed state: %v", err)
		}
		if err := assertNonProductionURL(state.BaseURL); err != nil {
			log.Fatal(err)
		}

		opts := simulate.FullDayOptions{
			StatePath: statePath,
			Close:     closeSessions,
			Verbose:   verbose,
		}

		if err := simulate.RunFullDay(ctx, opts); err != nil {
			log.Fatalf("Full-day simulation failed: %v", err)
		}
	},
}

var simulateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current simulation state",
	Long:  `Reads .seed-state.json, logs in as admin, and queries the server for active sessions, visits, and prints a summary.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		statePath, _ := cmd.Flags().GetString("state")
		verbose, _ := cmd.Flags().GetBool("verbose")

		state, err := seedapi.LoadSeedState(statePath)
		if err != nil {
			log.Fatalf("Failed to load seed state: %v", err)
		}
		if err := assertNonProductionURL(state.BaseURL); err != nil {
			log.Fatal(err)
		}

		opts := simulate.StatusOptions{
			StatePath: statePath,
			Verbose:   verbose,
		}

		if err := simulate.RunStatus(ctx, opts); err != nil {
			log.Fatalf("Status query failed: %v", err)
		}
	},
}

func init() {
	RootCmd.AddCommand(simulateCmd)
	simulateCmd.AddCommand(simulateFullDayCmd)
	simulateCmd.AddCommand(simulateStatusCmd)

	// full-day flags
	simulateFullDayCmd.Flags().String("state", seedapi.DefaultSeedStatePath, "Path to .seed-state.json")
	simulateFullDayCmd.Flags().Bool("close", false, "End sessions and checkout all students at the end")
	simulateFullDayCmd.Flags().Bool("verbose", false, "Verbose output")

	// status flags
	simulateStatusCmd.Flags().String("state", seedapi.DefaultSeedStatePath, "Path to .seed-state.json")
	simulateStatusCmd.Flags().Bool("verbose", false, "Verbose output")
}

func resolveSimulatorConfigPath() string {
	// Priority order: environment variable -> default path
	if envPath := os.Getenv(envSimulatorConfig); envPath != "" {
		return envPath
	}

	// Use default relative to current working directory
	return filepath.Clean(defaultSimulatorConfig)
}
