package api

import (
	"context"
	"fmt"
	"time"
)

// Seeder orchestrates the complete API-based seeding process
type Seeder struct {
	client  *Client
	verbose bool
}

// SeedResult contains counts of created entities
type SeedResult struct {
	Fixed *FixedResult
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
	fmt.Printf("Connecting to %s...\n", s.client.baseURL)
	if err := s.client.CheckHealth(); err != nil {
		return nil, s.formatError("Server health check", err)
	}

	// 2. Authenticate
	fmt.Printf("Logging in as %s...\n", email)
	if err := s.client.Login(email, password); err != nil {
		return nil, s.formatError("Login", err)
	}
	fmt.Println("Authenticated")
	fmt.Println()

	// 3. Create Stammdaten (fixed/master data)
	fixedSeeder := NewFixedSeeder(s.client, s.verbose)
	fixedResult, err := fixedSeeder.Seed(ctx)
	if err != nil {
		return nil, s.formatError("Stammdaten seeding", err)
	}
	result.Fixed = fixedResult
	fmt.Println()

	// 4. Mark some students as sick
	if err := fixedSeeder.MarkStudentsSick(ctx, fixedResult); err != nil {
		return nil, s.formatError("Marking students sick", err)
	}

	// 5. Collect seed state and write .seed-state.json
	state := s.collectSeedState(fixedSeeder, staffPIN)
	statePath := DefaultSeedStatePath
	if err := WriteSeedState(state, statePath); err != nil {
		return nil, s.formatError("Writing seed state", err)
	}
	fmt.Printf("Seed state written to %s\n", statePath)

	// 6. Generate simulator.yaml
	simPath := "simulator/iot/simulator.yaml"
	if err := WriteSimulatorConfig(state, simPath); err != nil {
		return nil, s.formatError("Generating simulator config", err)
	}
	fmt.Printf("Simulator config written to %s\n", simPath)
	fmt.Println()

	// 7. Print success summary
	s.printSuccessSummary(email, result)

	return result, nil
}

// collectSeedState builds SeedState from the FixedSeeder's internal maps
func (s *Seeder) collectSeedState(fs *FixedSeeder, staffPIN string) *SeedState {
	state := &SeedState{
		CreatedAt:  time.Now().UTC(),
		BaseURL:    s.client.baseURL,
		DevicePIN:  staffPIN,
		Devices:    make(map[string]SeedDevice),
		Rooms:      make(map[string]int64),
		Activities: make(map[string]int64),
		Groups:     make(map[string]int64),
	}

	// Group keys in order, matching seedGroups in stammdaten.go
	groupKeys := []string{
		"sternengruppe", "bärengruppe", "sonnengruppe", "mondgruppe",
		"regenbogengruppe", "blumengruppe", "schmetterlingsgruppe",
		"waldgruppe", "meeresgruppe", "wiesengruppe",
	}

	// Split staff credentials into admin and betreuer
	betreuerIndex := 0
	for _, cred := range fs.staffCredentials {
		staffKey := cred.Name
		ac := AccountCredentials{
			Email:    cred.Email,
			Password: cred.Password,
			PIN:      cred.PIN,
			Name:     cred.Name,
			StaffID:  fs.staffIDs[staffKey],
		}
		if tid, ok := fs.teacherIDs[staffKey]; ok {
			ac.TeacherID = tid
		}

		switch cred.Position {
		case "OGS-Büro":
			state.Accounts.Admin = append(state.Accounts.Admin, ac)
		case "Pädagogische Fachkraft":
			if betreuerIndex < len(groupKeys) {
				ac.Group = groupKeys[betreuerIndex]
			}
			state.Accounts.Betreuer = append(state.Accounts.Betreuer, ac)
			betreuerIndex++
		}
	}

	// Devices
	for _, device := range DemoDevices {
		if apiKey, ok := fs.deviceKeys[device.DeviceID]; ok {
			state.Devices[device.DeviceID] = SeedDevice{
				APIKey: apiKey,
				Name:   device.Name,
			}
		}
	}

	// Students
	for i, student := range DemoStudents {
		if id, ok := fs.studentIDByIndex[i]; ok {
			state.Students = append(state.Students, SeedStudent{
				ID:        id,
				FirstName: student.FirstName,
				LastName:  student.LastName,
				GroupKey:  student.GroupKey,
				Class:     student.Class,
			})
		}
	}

	// Rooms
	for name, id := range fs.roomIDs {
		state.Rooms[name] = id
	}

	// Activities
	for name, id := range fs.activityIDs {
		state.Activities[name] = id
	}

	// Groups
	for key, id := range fs.groupIDs {
		state.Groups[key] = id
	}

	return state
}

// formatError creates a user-friendly error message
func (s *Seeder) formatError(stage string, err error) error {
	fmt.Printf("\nFailed at: %s\n", stage)
	fmt.Printf("  Error: %v\n\n", err)
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

	fmt.Println("OUTPUT FILES:")
	fmt.Printf("  %s   (seed state with credentials & IDs)\n", DefaultSeedStatePath)
	fmt.Println("  simulator.yaml  (simulator configuration)")
	fmt.Println()
}
