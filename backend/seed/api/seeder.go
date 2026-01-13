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

	// 5. Print success summary
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

// printSuccessSummary prints the final demo-ready status
func (s *Seeder) printSuccessSummary(email string, result *SeedResult) {
	fmt.Println("=== Demo Ready ===")
	fmt.Printf("Login: %s / Test1234%%\n", email)
	fmt.Printf("Active Sessions: %d\n", result.Runtime.ActiveSessions)
	fmt.Printf("Students Checked In: %d/%d\n",
		result.Runtime.CheckedInStudents,
		result.Fixed.StudentCount)
}
