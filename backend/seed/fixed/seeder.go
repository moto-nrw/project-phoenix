package fixed

import (
	"context"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

// Seeder handles creation of all fixed data
type Seeder struct {
	tx      bun.Tx
	verbose bool
	result  *Result
}

// NewSeeder creates a new fixed data seeder
func NewSeeder(tx bun.Tx, verbose bool) *Seeder {
	return &Seeder{
		tx:      tx,
		verbose: verbose,
		result:  NewResult(),
	}
}

// SeedAll creates all fixed data in the correct order
func (s *Seeder) SeedAll(ctx context.Context) (*Result, error) {
	// 1. Facilities (no dependencies)
	if err := s.seedRooms(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed rooms: %w", err)
	}

	// 2. Auth roles and permissions (needed for accounts)
	if err := s.seedRolesAndPermissions(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed roles and permissions: %w", err)
	}

	// 3. Create admin account first
	if err := s.seedAdminAccount(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed admin account: %w", err)
	}

	// 4. Persons with RFID cards and accounts
	if err := s.seedPersonsWithAccounts(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed persons: %w", err)
	}

	// 5. Staff (depends on persons)
	if err := s.seedStaff(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed staff: %w", err)
	}

	// 6. Teachers (depends on staff)
	if err := s.seedTeachers(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed teachers: %w", err)
	}

	// 7. Education groups (depends on rooms)
	if err := s.seedEducationGroups(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed education groups: %w", err)
	}

	// 8. Assign teachers to groups
	if err := s.assignTeachersToGroups(ctx); err != nil {
		return nil, fmt.Errorf("failed to assign teachers to groups: %w", err)
	}

	// 9. Students (depends on persons and groups)
	if err := s.seedStudents(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed students: %w", err)
	}

	// 10. Privacy consents for students
	if err := s.seedPrivacyConsents(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed privacy consents: %w", err)
	}

	// 11. Activity categories and groups
	if err := s.seedActivities(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed activities: %w", err)
	}

	// 12. Schedule timeframes
	if err := s.seedTimeframes(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed timeframes: %w", err)
	}

	// 13. Activity schedules
	if err := s.seedActivitySchedules(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed activity schedules: %w", err)
	}

	// 14. Student enrollments in activities
	if err := s.seedStudentEnrollments(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed student enrollments: %w", err)
	}

	// 15. IoT devices (depends on persons for registrar)
	if err := s.seedIoTDevices(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed IoT devices: %w", err)
	}

	// 16. Create some guardian relationships
	if err := s.seedGuardianRelationships(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed guardian relationships: %w", err)
	}

	if s.verbose {
		log.Printf("Fixed data seeding completed successfully")
	}

	return s.result, nil
}

// LoadExistingData loads existing fixed data from the database
func LoadExistingData(ctx context.Context, tx bun.Tx) (*Result, error) {
	result := NewResult()

	// Load rooms
	err := tx.NewSelect().Model(&result.Rooms).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load rooms: %w", err)
	}
	for _, room := range result.Rooms {
		result.RoomByID[room.ID] = room
	}

	// Load persons
	err = tx.NewSelect().Model(&result.Persons).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load persons: %w", err)
	}
	for _, person := range result.Persons {
		result.PersonByID[person.ID] = person
	}

	// Load staff
	err = tx.NewSelect().Model(&result.Staff).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load staff: %w", err)
	}

	// Load teachers
	err = tx.NewSelect().Model(&result.Teachers).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load teachers: %w", err)
	}
	for _, teacher := range result.Teachers {
		result.TeacherByStaffID[teacher.StaffID] = teacher
	}

	// Load students
	err = tx.NewSelect().Model(&result.Students).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load students: %w", err)
	}
	for _, student := range result.Students {
		result.StudentByPersonID[student.PersonID] = student
	}

	// Load education groups
	err = tx.NewSelect().Model(&result.EducationGroups).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load education groups: %w", err)
	}

	// Separate class groups and supervision groups
	for _, group := range result.EducationGroups {
		result.GroupByID[group.ID] = group
		if isClassGroup(group.Name) {
			result.ClassGroups = append(result.ClassGroups, group)
		} else {
			result.SupervisionGroups = append(result.SupervisionGroups, group)
		}
	}

	// Load activity groups
	err = tx.NewSelect().Model(&result.ActivityGroups).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load activity groups: %w", err)
	}
	for _, activity := range result.ActivityGroups {
		result.ActivityByID[activity.ID] = activity
	}

	// Load devices
	err = tx.NewSelect().Model(&result.Devices).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load devices: %w", err)
	}

	// Load admin account
	err = tx.NewSelect().
		Model(&result.AdminAccount).
		Where("email = ?", "admin@example.com").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin account: %w", err)
	}

	return result, nil
}

// Helper function to determine if a group is a class group
func isClassGroup(name string) bool {
	// Class groups follow pattern: 1A, 1B, 2A, etc.
	if len(name) == 2 {
		grade := name[0]
		section := name[1]
		return grade >= '1' && grade <= '5' && (section == 'A' || section == 'B')
	}
	return false
}