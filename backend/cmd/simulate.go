package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/moto-nrw/project-phoenix/simulate"
	"github.com/spf13/cobra"
)

// simulateCmd groups the simulation subcommands (full-day, status, live).
var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Run simulation commands against a seeded local stack",
	Long: `Groups the simulation subcommands. All of them read .seed-state.json (written by the seed command)
and drive the API of a seeded local stack: full-day runs a one-shot day, status prints the current
state, and live generates continuous random events.`,
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

var simulateLiveCmd = &cobra.Command{
	Use:   "live",
	Short: "Run continuous live simulation",
	Long:  `Continuously generates random events (room moves, unterwegs, sick toggles, Schulhof rotations, attendance toggles, supervisor swaps) at a configurable interval. Requires simulate full-day to have run first. Ctrl+C to stop.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		statePath, _ := cmd.Flags().GetString("state")
		interval, _ := cmd.Flags().GetDuration("interval")
		verbose, _ := cmd.Flags().GetBool("verbose")

		state, err := seedapi.LoadSeedState(statePath)
		if err != nil {
			log.Fatalf("Failed to load seed state: %v", err)
		}
		if err := assertNonProductionURL(state.BaseURL); err != nil {
			log.Fatal(err)
		}

		opts := simulate.LiveOptions{
			StatePath: statePath,
			Interval:  interval,
			Verbose:   verbose,
		}

		if err := simulate.RunLive(ctx, opts); err != nil {
			log.Fatalf("Live simulation failed: %v", err)
		}
	},
}

func init() {
	RootCmd.AddCommand(simulateCmd)
	simulateCmd.AddCommand(simulateFullDayCmd)
	simulateCmd.AddCommand(simulateStatusCmd)
	simulateCmd.AddCommand(simulateLiveCmd)

	// full-day flags
	simulateFullDayCmd.Flags().String("state", seedapi.DefaultSeedStatePath, "Path to .seed-state.json")
	simulateFullDayCmd.Flags().Bool("close", false, "End sessions and checkout all students at the end")
	simulateFullDayCmd.Flags().Bool("verbose", false, "Verbose output")

	// status flags
	simulateStatusCmd.Flags().String("state", seedapi.DefaultSeedStatePath, "Path to .seed-state.json")
	simulateStatusCmd.Flags().Bool("verbose", false, "Verbose output")

	// live flags
	simulateLiveCmd.Flags().String("state", seedapi.DefaultSeedStatePath, "Path to .seed-state.json")
	simulateLiveCmd.Flags().Duration("interval", 10*time.Second, "Time between actions")
	simulateLiveCmd.Flags().Bool("verbose", false, "Verbose output")
}
