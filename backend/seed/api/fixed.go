package api

import (
	"context"
	"fmt"
)

// StaffCredentials stores login credentials for a staff member
type StaffCredentials struct {
	Email    string
	Password string
	PIN      string
	Name     string
	Position string
}

// FixedSeeder seeds fixed demo data via API calls
type FixedSeeder struct {
	client           *Client
	verbose          bool
	roomIDs          map[string]int64   // room name -> id
	personIDs        map[string]int64   // "firstName lastName" -> id (staff only)
	staffIDs         map[string]int64   // "firstName lastName" -> staff id
	teacherIDs       map[string]int64   // "firstName lastName" -> teacher id (for group assignment)
	studentIDs       map[string]int64   // "firstName lastName" -> id (student IDs for enrollment)
	studentIDByIndex map[int]int64      // student index -> student ID (for guardian linking)
	studentRFID      map[int64]string   // student ID -> RFID tag
	groupIDs         map[string]int64   // class name -> id
	activityIDs      map[string]int64   // activity name -> id
	activityRoomIDs  map[int64]int64    // activity ID -> room ID (for runtime seeder)
	categoryIDs      map[string]int64   // category name -> id
	deviceKeys       map[string]string  // device ID -> API key
	roleIDs          map[string]int64   // role name -> id
	guardianIDs      map[string]int64   // guardian "firstName lastName" -> id
	staffCredentials []StaffCredentials // created staff credentials for summary
}

// FixedResult contains counts of created entities
type FixedResult struct {
	RoomCount        int
	PersonCount      int
	StaffCount       int
	AccountCount     int
	GroupCount       int
	StudentCount     int
	SickStudentCount int // Students marked as sick for demo badges
	GuardianCount    int
	ActivityCount    int
	DeviceCount      int
	StaffCredentials []StaffCredentials // Login credentials for demo
}

// NewFixedSeeder creates a new fixed data seeder
func NewFixedSeeder(client *Client, verbose bool) *FixedSeeder {
	return &FixedSeeder{
		client:           client,
		verbose:          verbose,
		roomIDs:          make(map[string]int64),
		personIDs:        make(map[string]int64),
		staffIDs:         make(map[string]int64),
		teacherIDs:       make(map[string]int64),
		studentIDs:       make(map[string]int64),
		studentIDByIndex: make(map[int]int64),
		studentRFID:      make(map[int64]string),
		groupIDs:         make(map[string]int64),
		activityIDs:      make(map[string]int64),
		activityRoomIDs:  make(map[int64]int64),
		categoryIDs:      make(map[string]int64),
		deviceKeys:       make(map[string]string),
		roleIDs:          make(map[string]int64),
		guardianIDs:      make(map[string]int64),
		staffCredentials: make([]StaffCredentials, 0),
	}
}

// Seed creates all fixed demo data
func (s *FixedSeeder) Seed(ctx context.Context) (*FixedResult, error) {
	result := &FixedResult{}

	fmt.Println("📦 Creating Fixed Data...")

	// 1. Fetch available roles (needed for account creation)
	if err := s.fetchRoles(ctx); err != nil {
		return nil, fmt.Errorf("failed to fetch roles: %w", err)
	}

	// 2. Create rooms
	if err := s.seedRooms(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed rooms: %w", err)
	}

	// 3. Create persons (staff only, students created via student API)
	if err := s.seedPersons(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed persons: %w", err)
	}

	// 4. Create auth accounts for staff and link to persons
	if err := s.seedStaffAccounts(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed staff accounts: %w", err)
	}

	// 5. Create staff records
	if err := s.seedStaff(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed staff: %w", err)
	}

	// 6. Create education groups
	if err := s.seedGroups(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed groups: %w", err)
	}

	// 7. Create students
	if err := s.seedStudents(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed students: %w", err)
	}

	// 8. Create guardians and link to students
	if err := s.seedGuardians(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed guardians: %w", err)
	}

	// 9. Fetch or create activity categories
	if err := s.fetchCategories(); err != nil {
		return nil, fmt.Errorf("failed to fetch categories: %w", err)
	}

	// 10. Create activities
	if err := s.seedActivities(result); err != nil {
		return nil, fmt.Errorf("failed to seed activities: %w", err)
	}

	// 11. Assign supervisors to activities
	if err := s.assignSupervisors(); err != nil {
		return nil, fmt.Errorf("failed to assign supervisors: %w", err)
	}

	// 12. Enroll students in activities
	if err := s.enrollStudents(); err != nil {
		return nil, fmt.Errorf("failed to enroll students: %w", err)
	}

	// 13. Create IoT devices
	if err := s.seedDevices(ctx, result); err != nil {
		return nil, fmt.Errorf("failed to seed devices: %w", err)
	}

	// Store credentials in result for summary
	result.StaffCredentials = s.staffCredentials

	fmt.Println("✅ Fixed data creation complete!")
	return result, nil
}
