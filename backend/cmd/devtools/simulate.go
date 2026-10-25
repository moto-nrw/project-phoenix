package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/devtools/simulate"
	"github.com/spf13/cobra"
)

// simulationCopyError keeps the CLI's displayed prefix and wrapped cause.
type simulationCopyError struct {
	message string
	cause   error
}

func (e *simulationCopyError) Error() string { return e.message }
func (e *simulationCopyError) Unwrap() error { return e.cause }

func simulationCopyErrorf(format string, args ...any) error {
	formatted := fmt.Errorf(format, args...)
	return &simulationCopyError{message: formatted.Error(), cause: errors.Unwrap(formatted)}
}

type simulateRoot struct {
	client  simulate.ClientFactory
	fullDay func(context.Context, simulate.FullDayOptions) error
	status  func(context.Context, simulate.StatusOptions) error
	live    func(context.Context, simulate.LiveOptions) error
}

type simulationClient = simulate.Client
type simulationFullDayOptions = simulate.FullDayOptions
type simulationStatusOptions = simulate.StatusOptions
type simulationLiveOptions = simulate.LiveOptions

func (root simulateRoot) validate() error {
	if root.client == nil {
		return fmt.Errorf("simulation client factory is required")
	}
	return nil
}

func (root simulateRoot) runFullDay(ctx context.Context, options simulate.FullDayOptions) error {
	if err := root.validate(); err != nil {
		return err
	}
	if root.fullDay == nil {
		return fmt.Errorf("full-day simulation runner is required")
	}
	options.Client = root.client
	if err := root.fullDay(ctx, options); err != nil {
		return simulationCopyErrorf("Full-day simulation failed: %w", err)
	}
	return nil
}

func (root simulateRoot) runStatus(ctx context.Context, options simulate.StatusOptions) error {
	if err := root.validate(); err != nil {
		return err
	}
	if root.status == nil {
		return fmt.Errorf("status simulation runner is required")
	}
	options.Client = root.client
	if err := root.status(ctx, options); err != nil {
		return simulationCopyErrorf("Status query failed: %w", err)
	}
	return nil
}

func (root simulateRoot) runLive(ctx context.Context, options simulate.LiveOptions) error {
	if err := root.validate(); err != nil {
		return err
	}
	if root.live == nil {
		return fmt.Errorf("live simulation runner is required")
	}
	options.Client = root.client
	if err := root.live(ctx, options); err != nil {
		return simulationCopyErrorf("Live simulation failed: %w", err)
	}
	return nil
}

var defaultSimulateRoot = simulateRoot{
	client:  newSimulationClient,
	fullDay: simulate.RunFullDay,
	status:  simulate.RunStatus,
	live:    simulate.RunLive,
}

// simulateCmd groups the simulation subcommands (full-day, status, live).
var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Run simulation commands against a seeded local stack",
	Long: `Groups the simulation subcommands. All of them read the versioned profiles in
.seed-state.json (written by the seed command) and drive the API of a seeded local stack.
The default profile is vollbetrieb; --profile selects another semantic profile key.`,
}

var simulateFullDayCmd = &cobra.Command{
	Use:   "full-day",
	Short: "Run a one-shot full-day simulation",
	Long: `Reads .seed-state.json, then exercises all API flows: RFID assignment, sessions, attendance, check-ins, and optionally end-of-day cleanup.

The day runs in waves (early care, staff shift starts, sessions, arrivals class by class).
--clock-command moves the clock the server shares before each wave: the command is run with
the target instant appended as Unix seconds. Without it every wave runs at once.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		statePath, _ := cmd.Flags().GetString("state")
		profile, _ := cmd.Flags().GetString("profile")
		closeSessions, _ := cmd.Flags().GetBool("close")
		verbose, _ := cmd.Flags().GetBool("verbose")
		clockCommand, _ := cmd.Flags().GetString("clock-command")

		opts := simulate.FullDayOptions{
			StatePath: statePath,
			Profile:   profile,
			Close:     closeSessions,
			Verbose:   verbose,
			SetClock:  simulate.CommandClock(clockCommand),
		}
		return defaultSimulateRoot.runFullDay(ctx, opts)
	},
}

var simulateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current simulation state",
	Long:  `Reads .seed-state.json, logs in as admin, and queries the server for active sessions, visits, and prints a summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		statePath, _ := cmd.Flags().GetString("state")
		profile, _ := cmd.Flags().GetString("profile")
		verbose, _ := cmd.Flags().GetBool("verbose")

		opts := simulate.StatusOptions{
			StatePath: statePath,
			Profile:   profile,
			Verbose:   verbose,
		}
		return defaultSimulateRoot.runStatus(ctx, opts)
	},
}

var simulateLiveCmd = &cobra.Command{
	Use:   "live",
	Short: "Run continuous live simulation",
	Long:  `Continuously generates random events (room moves, unterwegs, sick toggles, Schulhof rotations, attendance toggles, supervisor swaps) at a configurable interval. Requires simulate full-day to have run first. Ctrl+C to stop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		statePath, _ := cmd.Flags().GetString("state")
		profile, _ := cmd.Flags().GetString("profile")
		interval, _ := cmd.Flags().GetDuration("interval")
		verbose, _ := cmd.Flags().GetBool("verbose")

		opts := simulate.LiveOptions{
			StatePath: statePath,
			Profile:   profile,
			Interval:  interval,
			Verbose:   verbose,
		}
		return defaultSimulateRoot.runLive(ctx, opts)
	},
}

func init() {
	RootCmd.AddCommand(simulateCmd)
	simulateCmd.AddCommand(simulateFullDayCmd)
	simulateCmd.AddCommand(simulateStatusCmd)
	simulateCmd.AddCommand(simulateLiveCmd)

	// full-day flags
	simulateFullDayCmd.Flags().String("state", ".seed-state.json", "Path to .seed-state.json")
	simulateFullDayCmd.Flags().String("profile", simulate.DefaultProfileKey, "Semantic demo school profile key")
	simulateFullDayCmd.Flags().Bool("close", false, "End sessions and checkout all students at the end")
	simulateFullDayCmd.Flags().Bool("verbose", false, "Verbose output")
	simulateFullDayCmd.Flags().String("clock-command", "", "Command that moves the shared clock before each wave; called with the target as Unix seconds")

	// status flags
	simulateStatusCmd.Flags().String("state", ".seed-state.json", "Path to .seed-state.json")
	simulateStatusCmd.Flags().String("profile", simulate.DefaultProfileKey, "Semantic demo school profile key")
	simulateStatusCmd.Flags().Bool("verbose", false, "Verbose output")

	// live flags
	simulateLiveCmd.Flags().String("state", ".seed-state.json", "Path to .seed-state.json")
	simulateLiveCmd.Flags().String("profile", simulate.DefaultProfileKey, "Semantic demo school profile key")
	simulateLiveCmd.Flags().Duration("interval", 10*time.Second, "Time between actions")
	simulateLiveCmd.Flags().Bool("verbose", false, "Verbose output")
}
