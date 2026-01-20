package api

import (
	"context"
	"fmt"
)

// Seeder orchestrates the complete API-based seeding process
type Seeder struct {
	client  *Client
	verbose bool
}

// SeedResult combines results from fixed and runtime seeding
type SeedResult struct {
	Fixed   *FixedResult
	Runtime *RuntimeResult
}

// NewSeeder creates a new API seeder
func NewSeeder(baseURL string, verbose bool) *Seeder {
	return &Seeder{
		client:  NewClient(baseURL, verbose),
		verbose: verbose,
	}
}

// Seed executes the complete seeding workflow
func (s *Seeder) Seed(ctx context.Context, email, password, staffPIN string) (*SeedResult, error) {
	result := &SeedResult{}

	// 1. Check server health
	fmt.Printf("🔌 Connecting to %s...\n", s.client.baseURL)
	if err := s.client.CheckHealth(); err != nil {
		return nil, s.formatError("Server health check", err)
	}

	// 2. Authenticate
	fmt.Printf("🔐 Logging in as %s...\n", email)
	if err := s.client.Login(email, password); err != nil {
		return nil, s.formatError("Login", err)
	}
	fmt.Println("✓ Authenticated")
	fmt.Println()

	// 3. Create fixed data
	fixedSeeder := NewFixedSeeder(s.client, s.verbose)
	fixedResult, err := fixedSeeder.Seed(ctx)
	if err != nil {
		return nil, s.formatError("Fixed data seeding", err)
	}
	result.Fixed = fixedResult
	fmt.Println()

	// 4. Create runtime state
	runtimeSeeder := NewRuntimeSeeder(s.client, fixedSeeder, s.verbose, staffPIN)
	runtimeResult, err := runtimeSeeder.Seed(ctx, DefaultRuntimeConfig)
	if err != nil {
		return nil, s.formatError("Runtime seeding", err)
	}
	result.Runtime = runtimeResult
	fmt.Println()

	// 5. Mark some students as sick (AFTER check-in to avoid auto-clear)
	if err := fixedSeeder.MarkStudentsSick(ctx, fixedResult); err != nil {
		return nil, s.formatError("Marking students sick", err)
	}

	// 6. Print success summary
	s.printSuccessSummary(email, result)

	return result, nil
}

// formatError creates a user-friendly error message
func (s *Seeder) formatError(stage string, err error) error {
	fmt.Printf("\n❌ Failed at: %s\n", stage)
	fmt.Printf("   Error: %v\n\n", err)
	fmt.Println("Run './main migrate reset' and try again.")
	return fmt.Errorf("%s failed: %w", stage, err)
}

// printSuccessSummary prints the final demo-ready status with all created data
func (s *Seeder) printSuccessSummary(email string, result *SeedResult) {
	fmt.Println()
	fmt.Println("=== DEMO READY ===")
	fmt.Println()

	// Admin account used for seeding
	fmt.Println("ADMIN ACCOUNT (used for seeding):")
	fmt.Printf("  Email:    %s\n", email)
	fmt.Printf("  Password: Test1234%%\n")
	fmt.Println()

	// Staff accounts with correct individual passwords
	fmt.Println("STAFF ACCOUNTS:")
	fmt.Println("  Name                 | Position                | Email              | Password   | PIN")
	fmt.Println("  " + "--------------------" + " | " + "-----------------------" + " | " + "------------------" + " | " + "----------" + " | " + "----")
	for _, cred := range result.Fixed.StaffCredentials {
		fmt.Printf("  %-20s | %-23s | %-18s | %-10s | %s\n",
			cred.Name, cred.Position, cred.Email, cred.Password, cred.PIN)
	}
	fmt.Println()

	// Statistics
	fmt.Println("CREATED DATA:")
	fmt.Printf("  Rooms:       %d\n", result.Fixed.RoomCount)
	fmt.Printf("  Staff:       %d\n", result.Fixed.StaffCount)
	fmt.Printf("  Accounts:    %d\n", result.Fixed.AccountCount)
	fmt.Printf("  Groups:      %d\n", result.Fixed.GroupCount)
	fmt.Printf("  Students:    %d\n", result.Fixed.StudentCount)
	fmt.Printf("  Sick:        %d\n", result.Fixed.SickStudentCount)
	fmt.Printf("  Guardians:   %d\n", result.Fixed.GuardianCount)
	fmt.Printf("  Activities:  %d\n", result.Fixed.ActivityCount)
	fmt.Printf("  IoT Devices: %d\n", result.Fixed.DeviceCount)
	fmt.Println()

	// Runtime state
	fmt.Println("RUNTIME STATE:")
	fmt.Printf("  Active Sessions: %d\n", result.Runtime.ActiveSessions)
	fmt.Printf("  Checked In:      %d / %d students\n", result.Runtime.CheckedInStudents, result.Fixed.StudentCount)
	fmt.Printf("  RFIDs Assigned:  %d\n", result.Runtime.RFIDsAssigned)
	fmt.Println()
}
