package cmd

import (
	"context"
	"fmt"
	"log"
	"os/signal"
	"syscall"

	appe2e "github.com/moto-nrw/project-phoenix/e2e"
	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	"github.com/spf13/cobra"
)

var e2eCmd = &cobra.Command{
	Use:    "e2e",
	Short:  "E2E harness implementation commands",
	Long:   `Internal implementation commands for the canonical Playwright E2E world. The supported local and CI entrypoint is "cd frontend && pnpm e2e".`,
	Hidden: true,
}

var e2ePrepareCmd = &cobra.Command{
	Use:    "prepare",
	Short:  "internal building block: reset the isolated database and seed the canonical Playwright world",
	Long:   `Internal building block for the canonical E2E harness. Resets the isolated E2E database, re-runs migrations, seeds the canonical multi-tenant Playwright world, and writes backend/.e2e-state.json. Use "cd frontend && pnpm e2e" for the supported local and CI path.`,
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		scenario, _ := cmd.Flags().GetString("scenario")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if err := appe2e.Prepare(ctx, appe2e.PrepareOptions{
			Scenario: scenario,
			Verbose:  verbose,
		}); err != nil {
			log.Fatal(err)
		}
	},
}

var e2eRunCmd = &cobra.Command{
	Use:   "run",
	Short: "run the canonical E2E flow end-to-end",
	Long:  `Starts the isolated E2E infrastructure, prepares the canonical world, boots the dedicated frontend, runs Playwright, and tears everything down again.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		scenario, _ := cmd.Flags().GetString("scenario")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if err := appe2e.Run(ctx, appe2e.RunOptions{
			Scenario: scenario,
			Verbose:  verbose,
		}); err != nil {
			log.Fatal(err)
		}
	},
}

var e2eHostsSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "sync the canonical E2E hostnames into /etc/hosts",
	Long:  `Materialises the canonical localtest.me hostnames for the E2E scenario into /etc/hosts.`,
	Run: func(cmd *cobra.Command, args []string) {
		scenario, _ := cmd.Flags().GetString("scenario")
		if err := appe2e.SyncHostsForScenario(scenario); err != nil {
			log.Fatal(err)
		}
	},
}

var e2eHostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "print the canonical local hostnames for an E2E scenario",
	Long:  `Prints the local hostnames required by the canonical Playwright E2E world, one hostname per line.`,
	Run: func(cmd *cobra.Command, args []string) {
		scenario, _ := cmd.Flags().GetString("scenario")
		hosts, err := appe2e.HostsForScenario(scenario)
		if err != nil {
			log.Fatal(err)
		}
		for _, host := range hosts {
			fmt.Println(host)
		}
	},
}

func init() {
	RootCmd.AddCommand(e2eCmd)
	e2eCmd.AddCommand(e2ePrepareCmd)
	e2eCmd.AddCommand(e2eRunCmd)
	e2eCmd.AddCommand(e2eHostsCmd)
	e2eHostsCmd.AddCommand(e2eHostsSyncCmd)

	e2ePrepareCmd.Flags().String("scenario", scenarios.DefaultPrepareScenario().Name, "Named E2E scenario")
	e2ePrepareCmd.Flags().Bool("verbose", false, "Enable verbose logging")
	e2eRunCmd.Flags().String("scenario", scenarios.DefaultPrepareScenario().Name, "Named E2E scenario")
	e2eRunCmd.Flags().Bool("verbose", false, "Enable verbose logging")

	e2eHostsCmd.Flags().String("scenario", scenarios.DefaultPrepareScenario().Name, "Named E2E scenario")
	e2eHostsSyncCmd.Flags().String("scenario", scenarios.DefaultPrepareScenario().Name, "Named E2E scenario")
}
