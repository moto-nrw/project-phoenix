package cmd

import (
	"context"
	"log"

	appe2e "github.com/moto-nrw/project-phoenix/e2e"
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

		url, _ := cmd.Flags().GetString("url")
		verbose, _ := cmd.Flags().GetBool("verbose")

		if err := assertNonProductionURL(url); err != nil {
			log.Fatal(err)
		}

		if err := appe2e.Prepare(ctx, appe2e.PrepareOptions{
			URL:     url,
			Verbose: verbose,
		}); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	RootCmd.AddCommand(e2eCmd)
	e2eCmd.AddCommand(e2ePrepareCmd)

	e2ePrepareCmd.Flags().String("url", appe2e.DefaultSeedURL, "Backend API URL")
	e2ePrepareCmd.Flags().Bool("verbose", false, "Enable verbose logging")
}
