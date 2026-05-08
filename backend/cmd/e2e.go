package cmd

import (
	"context"
	"log"

	appe2e "github.com/moto-nrw/project-phoenix/e2e"
	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
	"github.com/spf13/cobra"
)

var e2eCmd = &cobra.Command{
	Use:   "e2e",
	Short: "E2E harness commands",
	Long:  `Commands for preparing and validating the canonical Playwright E2E world.`,
}

var e2ePrepareCmd = &cobra.Command{
	Use:   "prepare",
	Short: "reset the isolated database and seed the canonical Playwright world",
	Long:  `Resets the isolated E2E database, re-runs migrations, seeds the canonical multi-tenant Playwright world, and writes backend/.e2e-manifest.json.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

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

func init() {
	RootCmd.AddCommand(e2eCmd)
	e2eCmd.AddCommand(e2ePrepareCmd)

	e2ePrepareCmd.Flags().String("scenario", scenarios.DefaultPrepareScenario().Name, "Named E2E scenario")
	e2ePrepareCmd.Flags().Bool("verbose", false, "Enable verbose logging")
}
