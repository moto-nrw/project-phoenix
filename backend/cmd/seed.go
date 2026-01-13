package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/seed"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/spf13/cobra"
)

// seedCmd represents the seed command
var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed the database with test data",
	Long: `Seed the database with comprehensive test data for development and testing.

This command creates:
FIXED DATA (always created):
- 24 rooms across multiple buildings
- 150 persons with RFID cards (30 staff, 120 students)
- 20 teachers with specializations
- 25 education groups (10 classes, 15 supervision groups)
- 19 activity groups with schedules
- 7 IoT devices with room assignments
- Privacy consents for 90% of students

RUNTIME STATE (optional, for testing):
- Active group sessions with supervisors
- Students checked into rooms
- Visit tracking records
- Attendance records for today
- Combined group scenarios

The data includes proper relationships:
- Teachers → Staff → Persons → RFID Cards
- Students → Groups with guardian information
- Groups → Rooms with teacher assignments
- Activities → Schedules with enrollments
- Devices → Rooms for RFID testing

Usage:
  go run main.go seed                  # Create all data with initial runtime state
  go run main.go seed --reset          # Clear all data first, then seed
  go run main.go seed --fixed-only     # Only create fixed data (no active sessions)
  go run main.go seed --runtime-only   # Only create runtime state (requires fixed data)`,
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
				log.Fatal("--email, --password, and --pin are required when using --api")
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
			log.Fatal("Cannot use --fixed-only and --runtime-only together")
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
	seedCmd.Flags().String("url", "http://localhost:8080", "Backend API URL")
}

func runSeeding(ctx context.Context, reset, fixedOnly, runtimeOnly, verbose bool) {
	// Initialize database connection
	db, err := database.DBConn()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
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
		log.Fatal("Seeding failed:", err)
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
		log.Fatal(err)
	}

	// Result summary is printed by seeder itself
	_ = result
}
