package seed

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/seed/fixed"
	"github.com/moto-nrw/project-phoenix/seed/runtime"
	"github.com/uptrace/bun"
)

// Config holds configuration for the seeding process
type Config struct {
	// Reset clears all data before seeding
	Reset bool
	// FixedOnly only seeds fixed data (no runtime state)
	FixedOnly bool
	// RuntimeOnly only creates runtime state (requires fixed data)
	RuntimeOnly bool
	// CreateActiveState creates some active sessions and visits for testing
	CreateActiveState bool
	// Verbose enables detailed logging
	Verbose bool
}

// Result contains IDs of all created entities for reference
type Result struct {
	Fixed   *fixed.Result
	Runtime *runtime.Result
}

// Seeder orchestrates the seeding process
type Seeder struct {
	db     *bun.DB
	config *Config
}

// NewSeeder creates a new seeder instance
func NewSeeder(db *bun.DB, config *Config) *Seeder {
	if config == nil {
		config = &Config{
			CreateActiveState: true,
		}
	}
	return &Seeder{
		db:     db,
		config: config,
	}
}

// Seed executes the seeding process
func (s *Seeder) Seed(ctx context.Context) (*Result, error) {
	result := &Result{}

	// Reset if requested
	if s.config.Reset {
		if err := s.resetData(ctx); err != nil {
			return nil, fmt.Errorf("failed to reset data: %w", err)
		}
		if s.config.Verbose {
			logging.Logger.Info("Data reset completed")
		}
	}

	// Seed fixed data in its own transaction
	if !s.config.RuntimeOnly {
		if s.config.Verbose {
			logging.Logger.Info("Starting fixed data seeding...")
		}

		err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			fixedSeeder := fixed.NewSeeder(tx, s.config.Verbose)
			fixedResult, err := fixedSeeder.SeedAll(ctx)
			if err != nil {
				return fmt.Errorf("failed to seed fixed data: %w", err)
			}
			result.Fixed = fixedResult
			return nil
		})
		if err != nil {
			return nil, err
		}

		if s.config.Verbose {
			logging.Logger.Info("Fixed data seeding completed")
		}
	}

	// Seed runtime state in a separate transaction
	if !s.config.FixedOnly && s.config.CreateActiveState {
		if s.config.Verbose {
			logging.Logger.Info("Starting runtime state creation...")
		}

		err := s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			// Get fixed data if we're in runtime-only mode
			if s.config.RuntimeOnly {
				fixedData, err := fixed.LoadExistingData(ctx, tx)
				if err != nil {
					return fmt.Errorf("failed to load existing fixed data: %w", err)
				}
				result.Fixed = fixedData
			}

			runtimeSeeder := runtime.NewSeeder(tx, result.Fixed, s.config.Verbose)
			runtimeResult, err := runtimeSeeder.CreateInitialState(ctx)
			if err != nil {
				return fmt.Errorf("failed to create runtime state: %w", err)
			}
			result.Runtime = runtimeResult
			return nil
		})
		if err != nil {
			return nil, err
		}

		if s.config.Verbose {
			logging.Logger.Info("Runtime state creation completed")
		}
	}

	// Validate relationships using the main db connection
	if err := s.validateRelationships(ctx, s.db, result); err != nil {
		return nil, fmt.Errorf("relationship validation failed: %w", err)
	}

	var err error

	if err != nil {
		return nil, err
	}

	// Print summary
	s.printSummary(result)

	return result, nil
}

// resetData clears all data from the database
func (s *Seeder) resetData(ctx context.Context) error {
	// Order matters due to foreign key constraints
	tables := []string{
		// Runtime data first
		"active.attendance",
		"active.visits",
		"active.group_supervisors",
		"active.group_mappings",
		"active.combined_groups",
		"active.groups",

		// Activities
		"activities.student_enrollments",
		"activities.supervisors",
		"activities.schedules",
		"activities.groups",
		"activities.categories",

		// Education
		"education.substitutions",
		"education.group_teacher",
		"education.groups",

		// Users
		"users.privacy_consents",
		"users.students_guardians",
		"users.students",
		"users.teachers",
		"users.staff",
		"users.rfid_cards",
		"users.persons",

		// Auth
		"auth.tokens",
		"auth.account_roles",
		"auth.accounts",

		// Schedule
		"schedule.entries",
		"schedule.dateframes",
		"schedule.timeframes",

		// Facilities
		"facilities.rooms",

		// IoT
		"iot.devices",

		// Config & Feedback
		"config.settings",
		"feedback.entries",
	}

	for _, table := range tables {
		var exists bool
		if err := s.db.NewSelect().ColumnExpr("to_regclass(?) IS NOT NULL", table).Scan(ctx, &exists); err != nil {
			if s.config.Verbose {
				logging.Logger.Warnf("Could not check existence for %s: %v", table, err)
			}
			continue
		}
		if !exists {
			continue
		}

		query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			// Some tables might not exist, continue
			if s.config.Verbose {
				logging.Logger.Warnf("Could not truncate %s: %v", table, err)
			}
		}
	}

	return nil
}

// validateRelationships ensures all relationships are properly established
func (s *Seeder) validateRelationships(ctx context.Context, db bun.IDB, result *Result) error {
	if result.Fixed == nil {
		return nil
	}

	// Validate teacher-staff relationships
	orphanedTeachers, err := db.NewSelect().
		Table("users.teachers").
		Where("staff_id NOT IN (SELECT id FROM users.staff)").
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check teacher-staff relationships: %w", err)
	}
	if orphanedTeachers > 0 {
		return fmt.Errorf("found %d teachers without valid staff records", orphanedTeachers)
	}

	// Validate student-person relationships
	orphanedStudents, err := db.NewSelect().
		Table("users.students").
		Where("person_id NOT IN (SELECT id FROM users.persons)").
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check student-person relationships: %w", err)
	}
	if orphanedStudents > 0 {
		return fmt.Errorf("found %d students without valid person records", orphanedStudents)
	}

	// Validate group-room relationships
	groupsWithoutRooms, err := db.NewSelect().
		Table("education.groups").
		Where("room_id IS NOT NULL AND room_id NOT IN (SELECT id FROM facilities.rooms)").
		Count(ctx)
	if err != nil {
		return fmt.Errorf("failed to check group-room relationships: %w", err)
	}
	if groupsWithoutRooms > 0 {
		return fmt.Errorf("found %d groups with invalid room assignments", groupsWithoutRooms)
	}

	// Validate active group relationships if runtime state exists
	if result.Runtime != nil && len(result.Runtime.ActiveGroups) > 0 {
		invalidActiveGroups, err := db.NewSelect().
			Table("active.groups").
			Where("group_id NOT IN (SELECT id FROM activities.groups)").
			Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to check active group relationships: %w", err)
		}
		if invalidActiveGroups > 0 {
			return fmt.Errorf("found %d active groups with invalid group references", invalidActiveGroups)
		}
	}

	return nil
}

// printSummary displays a summary of the seeded data
func (s *Seeder) printSummary(result *Result) {
	fmt.Println("\n=== Seed Data Summary ===")

	if result.Fixed != nil {
		fmt.Println("\nFixed Data:")
		fmt.Printf("  Rooms: %d\n", len(result.Fixed.Rooms))
		fmt.Printf("  Persons: %d (with RFID cards)\n", len(result.Fixed.Persons))
		fmt.Printf("  Staff: %d\n", len(result.Fixed.Staff))
		fmt.Printf("  Teachers: %d\n", len(result.Fixed.Teachers))
		fmt.Printf("  Students: %d\n", len(result.Fixed.Students))
		fmt.Printf("  Education Groups: %d\n", len(result.Fixed.EducationGroups))
		fmt.Printf("  Activity Groups: %d\n", len(result.Fixed.ActivityGroups))
		fmt.Printf("  Devices: %d\n", len(result.Fixed.Devices))
		fmt.Printf("  Accounts: %d\n", len(result.Fixed.Accounts))
	}

	if result.Runtime != nil {
		fmt.Println("\nRuntime State:")
		fmt.Printf("  Active Groups: %d\n", len(result.Runtime.ActiveGroups))
		fmt.Printf("  Active Visits: %d\n", len(result.Runtime.Visits))
		fmt.Printf("  Group Supervisors: %d assignments\n", result.Runtime.SupervisorCount)

		// Count students currently checked in
		checkedIn := 0
		for _, visit := range result.Runtime.Visits {
			if visit.ExitTime == nil {
				checkedIn++
			}
		}
		fmt.Printf("  Students Checked In: %d\n", checkedIn)
	}

	fmt.Println("\nThe database is now ready for testing RFID check-ins/check-outs!")
	fmt.Println("Use the admin account: admin@example.com / Test1234%")
	if result.Fixed != nil && len(result.Fixed.StaffWithPINs) > 0 {
		fmt.Println("\nStaff members with PINs for RFID device authentication:")
		for email, pin := range result.Fixed.StaffWithPINs {
			fmt.Printf("  %s: PIN %s\n", email, pin)
		}
	}
}
