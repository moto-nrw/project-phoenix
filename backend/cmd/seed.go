package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/database"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/seed"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/spf13/cobra"
)

// seedCmd represents the seed command
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with test data",
	Long: `Seed the database with comprehensive test data for development and testing.

Two seeding modes are available:

1. DIRECT DATABASE MODE (default):
   Seeds via direct database writes. Creates large dataset for development.

2. API MODE (--api flag):
   Seeds via HTTP API calls. Ensures all data passes through validation.
   Creates smaller, focused demo dataset for demonstrations.

API MODE DEMO DATA:
- 6 rooms (OGS rooms, gym, schoolyard, cafeteria)
- 7 staff members
- 45 students (3 classes × 15)
- 10 activities (homework, sports, crafts, etc.)
- 4 IoT devices for RFID scanning
- 4 active sessions with ~21 checked-in students

Usage:
  # Direct DB seeding (large dataset)
  go run main.go seed                  # Create all data with runtime state
  go run main.go seed --reset          # Clear all data first, then seed
  go run main.go seed --fixed-only     # Only create fixed data (no sessions)

  # API-based seeding (demo dataset, requires running server)
  go run main.go seed --api --email admin@example.com --password 'Test1234%' --pin 1234
  go run main.go seed --api --email admin@example.com --password 'Test1234%' --pin 1234 --verbose`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// Parse flags
		reset, _ := cmd.Flags().GetBool("reset")
		fixedOnly, _ := cmd.Flags().GetBool("fixed-only")
		runtimeOnly, _ := cmd.Flags().GetBool("runtime-only")
		verbose, _ := cmd.Flags().GetBool("verbose")

		// Parse API flags
		useAPI, _ := cmd.Flags().GetBool("api")
		apiEmail, _ := cmd.Flags().GetString("email")
		apiPassword, _ := cmd.Flags().GetString("password")
		apiPIN, _ := cmd.Flags().GetString("pin")
		apiURL, _ := cmd.Flags().GetString("url")

		if useAPI {
			if apiEmail == "" || apiPassword == "" || apiPIN == "" {
				logger.Logger.Fatal("--email, --password, and --pin are required when using --api")
			}
			// Resolve API URL: flag takes precedence, then env var
			if apiURL == "" {
				apiURL = os.Getenv("SEED_API_URL")
			}
			if apiURL == "" {
				logger.Logger.Fatal("API URL required: use --url flag or set SEED_API_URL env var")
			}
			runAPISeeding(ctx, apiURL, apiEmail, apiPassword, apiPIN, verbose)
			return
		}

		if reset {
			fmt.Println("WARNING: --reset flag is set. This will delete ALL existing data!")
			fmt.Print("Are you sure you want to continue? (y/N): ")
			var response string
			_, err := fmt.Scanln(&response)
			if err != nil || (response != "y" && response != "Y") {
				fmt.Println("Seed operation cancelled.")
				return
			}
		}

		// Validate flag combinations
		if fixedOnly && runtimeOnly {
			logger.Logger.Fatal("Cannot use --fixed-only and --runtime-only together")
		}

		// Run seeding
		runSeeding(ctx, reset, fixedOnly, runtimeOnly, verbose)
	},
}

func init() {
	RootCmd.AddCommand(seedCmd)
	seedCmd.Flags().Bool("reset", false, "Reset all data before seeding")
	seedCmd.Flags().Bool("fixed-only", false, "Only seed fixed data (no runtime state)")
	seedCmd.Flags().Bool("runtime-only", false, "Only create runtime state (requires existing fixed data)")
	seedCmd.Flags().Bool("verbose", false, "Enable verbose logging")
	// New flags for API-based seeding
	seedCmd.Flags().Bool("api", false, "Use API-based seeding instead of direct DB writes")
	seedCmd.Flags().String("email", "", "Admin email for API authentication (required with --api)")
	seedCmd.Flags().String("password", "", "Admin password for API authentication (required with --api)")
	seedCmd.Flags().String("pin", "", "Staff PIN for IoT authentication (required with --api)")
	seedCmd.Flags().String("url", "", "Backend API URL (or set SEED_API_URL env var)")
}

func runSeeding(ctx context.Context, reset, fixedOnly, runtimeOnly, verbose bool) {
	// Initialize database connection
	db, err := database.DBConn()
	if err != nil {
		logger.Logger.WithError(err).Fatal("Failed to initialize database")
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Logger.WithError(err).Warn("Failed to close database connection")
		}
	}()

	// Configure seeder
	config := &seed.Config{
		Reset:             reset,
		FixedOnly:         fixedOnly,
		RuntimeOnly:       runtimeOnly,
		CreateActiveState: !fixedOnly, // Create active state unless fixed-only
		Verbose:           verbose,
	}

	// Create and run seeder
	seeder := seed.NewSeeder(db, config)
	result, err := seeder.Seed(ctx)
	if err != nil {
		logger.Logger.WithError(err).Fatal("Seeding failed")
	}

	// Print additional instructions if runtime state was created
	if result.Runtime != nil && len(result.Runtime.ActiveGroups) > 0 {
		fmt.Println("\n=== Testing RFID Check-ins ===")
		fmt.Println("The database now has active sessions ready for testing:")
		fmt.Println("1. Use an RFID device API key from the seeded devices")
		fmt.Println("2. Authenticate with a staff PIN (see above)")
		fmt.Println("3. Test student check-ins/check-outs with RFID tags")
		fmt.Println("\nExample RFID check-in:")
		fmt.Println("POST /iot/checkin")
		fmt.Println(`{
  "tag_uid": "<student_rfid_tag>",
  "device_id": "RFID-MAIN-001",
  "room_id": <room_id>
}`)
	}
}

func runAPISeeding(ctx context.Context, baseURL, email, password, staffPIN string, verbose bool) {
	seeder := seedapi.NewSeeder(baseURL, verbose)

	result, err := seeder.Seed(ctx, email, password, staffPIN)
	if err != nil {
		logger.Logger.WithError(err).Fatal("API seeding failed")
	}

	// Result summary is printed by seeder itself
	_ = result
}
