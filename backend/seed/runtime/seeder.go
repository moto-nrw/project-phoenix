package runtime

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/seed/fixed"
	"github.com/uptrace/bun"
)

// Seeder creates runtime state for testing
type Seeder struct {
	tx        bun.Tx
	fixedData *fixed.Result
	verbose   bool
	result    *Result
}

// NewSeeder creates a new runtime seeder
func NewSeeder(tx bun.Tx, fixedData *fixed.Result, verbose bool) *Seeder {
	return &Seeder{
		tx:        tx,
		fixedData: fixedData,
		verbose:   verbose,
		result:    NewResult(),
	}
}

// CreateInitialState creates a realistic runtime state for testing
func (s *Seeder) CreateInitialState(ctx context.Context) (*Result, error) {
	// Set the time context - let's say it's 2:30 PM on a Wednesday
	now := time.Now()
	currentTime := time.Date(now.Year(), now.Month(), now.Day(), 14, 30, 0, 0, time.Local)
	if currentTime.After(now) {
		currentTime = now
	}

	// 1. Create active sessions for some groups
	if err := s.createActiveSessions(ctx, currentTime); err != nil {
		return nil, fmt.Errorf("failed to create active sessions: %w", err)
	}

	// 2. Check in some students to active groups
	if err := s.checkInStudents(ctx, currentTime); err != nil {
		return nil, fmt.Errorf("failed to check in students: %w", err)
	}

	// 3. Create some attendance records for today
	if err := s.createAttendanceRecords(ctx, currentTime); err != nil {
		return nil, fmt.Errorf("failed to create attendance records: %w", err)
	}

	// 4. Create a combined group scenario
	if err := s.createCombinedGroup(ctx, currentTime); err != nil {
		return nil, fmt.Errorf("failed to create combined group: %w", err)
	}

	// Calculate statistics
	s.calculateStatistics()

	if s.verbose {
		log.Printf("Created runtime state: %d active groups, %d visits, %d students checked in",
			len(s.result.ActiveGroups), len(s.result.Visits), s.result.StudentsCheckedIn)
	}

	return s.result, nil
}

// createActiveSessions creates active group sessions
func (s *Seeder) createActiveSessions(ctx context.Context, currentTime time.Time) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Create active sessions for:
	// 1. Some class groups (during school hours)
	// 2. Lunch supervision
	// 3. Some afternoon activities

	// Active class sessions (3-4 classes)
	activeClasses := 4
	for i := 0; i < activeClasses && i < len(s.fixedData.ClassGroups); i++ {
		classGroup := s.fixedData.ClassGroups[i]

		// Skip if room already has an active session
		if classGroup.RoomID != nil {
			if _, exists := s.result.ActiveGroupsByRoom[*classGroup.RoomID]; exists {
				continue
			}
		}

		// Find a device for this room
		var deviceID *int64
		if classGroup.RoomID != nil {
			if device, ok := s.fixedData.DevicesByRoom[*classGroup.RoomID]; ok {
				deviceID = &device.ID
			}
		}

		activeGroup := &active.Group{
			StartTime:      currentTime.Add(-90 * time.Minute), // Started 90 minutes ago
			LastActivity:   currentTime.Add(-5 * time.Minute),  // Last activity 5 minutes ago
			TimeoutMinutes: 30,
			GroupID:        classGroup.ID,
			DeviceID:       deviceID,
			RoomID:         *classGroup.RoomID,
		}
		activeGroup.CreatedAt = time.Now()
		activeGroup.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(activeGroup).ModelTableExpr("active.groups").Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create active group for %s: %w", classGroup.Name, err)
		}

		s.result.ActiveGroups = append(s.result.ActiveGroups, activeGroup)
		if classGroup.RoomID != nil {
			s.result.ActiveGroupsByRoom[*classGroup.RoomID] = activeGroup
		}

		// Skip supervisor assignment - no representative in education groups anymore
	}

	// Lunch supervision in Mensa
	var mensaID int64
	for _, room := range s.fixedData.Rooms {
		if room.Name == "Mensa" {
			mensaID = room.ID
			break
		}
	}

	if mensaID > 0 {
		// Find lunch supervision group
		var lunchGroup *education.Group
		for _, group := range s.fixedData.SupervisionGroups {
			if group.Name == "OGS-Mittag-1" {
				lunchGroup = group
				break
			}
		}

		if lunchGroup != nil {
			var deviceID *int64
			if device, ok := s.fixedData.DevicesByRoom[mensaID]; ok {
				deviceID = &device.ID
			}

			activeGroup := &active.Group{
				StartTime:      currentTime.Add(-30 * time.Minute), // Started 30 minutes ago
				LastActivity:   currentTime.Add(-2 * time.Minute),  // Recent activity
				TimeoutMinutes: 60,                                 // Longer timeout for lunch
				GroupID:        lunchGroup.ID,
				DeviceID:       deviceID,
				RoomID:         mensaID,
			}
			activeGroup.CreatedAt = time.Now()
			activeGroup.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().Model(activeGroup).ModelTableExpr("active.groups").Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create lunch supervision: %w", err)
			}

			s.result.ActiveGroups = append(s.result.ActiveGroups, activeGroup)
			s.result.ActiveGroupsByRoom[mensaID] = activeGroup

			// Assign 2 supervisors for lunch
			supervisorStaff := s.fixedData.Staff[20:22] // Non-teacher staff
			for i, staff := range supervisorStaff {
				supervisor := &active.GroupSupervisor{
					GroupID:   activeGroup.ID,
					StaffID:   staff.ID,
					Role:      "supervisor",
					StartDate: time.Now(),
				}
				if i == 0 {
					supervisor.Role = "lead_supervisor"
				}
				supervisor.CreatedAt = time.Now()
				supervisor.UpdatedAt = time.Now()

				_, err = s.tx.NewInsert().Model(supervisor).ModelTableExpr("active.group_supervisors").Exec(ctx)
				if err == nil {
					s.result.Supervisors = append(s.result.Supervisors, supervisor)
					s.result.SupervisorCount++
				}
			}
		}
	}

	// Some afternoon activities
	activityCount := 0
	for _, activity := range s.fixedData.ActivityGroups {
		if activityCount >= 3 {
			break
		}

		// Skip if no preferred room or room already occupied
		if activity.PlannedRoomID == nil {
			continue
		}
		if _, exists := s.result.ActiveGroupsByRoom[*activity.PlannedRoomID]; exists {
			continue
		}

		// 50% chance to be active
		if rng.Float32() > 0.5 {
			continue
		}

		var deviceID *int64
		if device, ok := s.fixedData.DevicesByRoom[*activity.PlannedRoomID]; ok {
			deviceID = &device.ID
		}

		activeGroup := &active.Group{
			StartTime:      currentTime.Add(-15 * time.Minute), // Just started
			LastActivity:   currentTime,
			TimeoutMinutes: 30,
			GroupID:        activity.ID,
			DeviceID:       deviceID,
			RoomID:         *activity.PlannedRoomID,
		}
		activeGroup.CreatedAt = time.Now()
		activeGroup.UpdatedAt = time.Now()

		_, err := s.tx.NewInsert().Model(activeGroup).ModelTableExpr("active.groups").Exec(ctx)
		if err != nil {
			continue // Skip on error
		}

		s.result.ActiveGroups = append(s.result.ActiveGroups, activeGroup)
		s.result.ActiveGroupsByRoom[*activity.PlannedRoomID] = activeGroup
		activityCount++

		// Assign activity supervisors
		var supervisors []struct {
			StaffID   int64 `bun:"staff_id"`
			IsPrimary bool  `bun:"is_primary"`
		}
		err = s.tx.NewSelect().
			Table("activities.supervisors").
			Column("staff_id", "is_primary").
			Where("group_id = ?", activity.ID).
			Scan(ctx, &supervisors)

		if err == nil {
			for _, sup := range supervisors {
				role := "supervisor"
				if sup.IsPrimary {
					role = "lead_supervisor"
				}
				supervisor := &active.GroupSupervisor{
					GroupID: activeGroup.ID,
					StaffID: sup.StaffID,
					Role:    role,
				}
				supervisor.CreatedAt = time.Now()
				supervisor.UpdatedAt = time.Now()

				_, err = s.tx.NewInsert().Model(supervisor).ModelTableExpr("active.group_supervisors").Exec(ctx)
				if err == nil {
					s.result.Supervisors = append(s.result.Supervisors, supervisor)
					s.result.SupervisorCount++
				}
			}
		}
	}

	if s.verbose {
		log.Printf("Created %d active group sessions", len(s.result.ActiveGroups))
	}

	return nil
}

// checkInStudents creates visit records for students
func (s *Seeder) checkInStudents(ctx context.Context, currentTime time.Time) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Track students that already have an active visit so we don't double-book them.
	activeVisitStudents := make(map[int64]struct{})
	var existingActive []int64
	if err := s.tx.NewSelect().Table("active.visits").Column("student_id").Where("exit_time IS NULL").Scan(ctx, &existingActive); err == nil {
		for _, id := range existingActive {
			activeVisitStudents[id] = struct{}{}
		}
	}

	// For each active group, check in some students
	for _, activeGroup := range s.result.ActiveGroups {
		// Determine the type of group
		var groupType string
		var studentIDs []int64

		// Check if it's a class group
		isClassGroup := false
		for _, classGroup := range s.fixedData.ClassGroups {
			if classGroup.ID == activeGroup.GroupID {
				isClassGroup = true
				// Get students from this class
				err := s.tx.NewSelect().
					Table("users.students").
					Column("id").
					Where("group_id = ?", classGroup.ID).
					Scan(ctx, &studentIDs)
				if err != nil {
					continue
				}
				groupType = "class"
				break
			}
		}

		// If not a class group, check if it's an activity
		if !isClassGroup {
			// Get enrolled students for this activity
			err := s.tx.NewSelect().
				Table("activities.student_enrollments").
				Column("student_id").
				Where("activity_group_id = ?", activeGroup.GroupID).
				Scan(ctx, &studentIDs)
			if err != nil {
				continue
			}
			groupType = "activity"
		}

		// Check in 60-90% of students
		checkInRate := 0.6 + rng.Float64()*0.3
		numToCheckIn := int(float64(len(studentIDs)) * checkInRate)

		// Shuffle student IDs
		for i := len(studentIDs) - 1; i > 0; i-- {
			j := rng.Intn(i + 1)
			studentIDs[i], studentIDs[j] = studentIDs[j], studentIDs[i]
		}

		// Check in students
		for i := 0; i < numToCheckIn && i < len(studentIDs); i++ {
			studentID := studentIDs[i]
			if _, alreadyActive := activeVisitStudents[studentID]; alreadyActive {
				continue
			}

			// Vary entry times to be realistic
			entryOffset := rng.Intn(20) // 0-20 minutes after group start
			entryTime := activeGroup.StartTime.Add(time.Duration(entryOffset) * time.Minute)
			if entryTime.After(currentTime) {
				entryTime = currentTime
			}

			visit := &active.Visit{
				StudentID:     studentID,
				ActiveGroupID: activeGroup.ID,
				EntryTime:     entryTime,
			}
			visit.CreatedAt = time.Now()
			visit.UpdatedAt = time.Now()

			// Some students might have already left (10% chance)
			if rng.Float32() < 0.1 && groupType == "activity" {
				exitTime := entryTime.Add(time.Duration(rng.Intn(30)+10) * time.Minute)
				if exitTime.After(currentTime) {
					exitTime = currentTime
				}
				visit.ExitTime = &exitTime
			}

			_, err := s.tx.NewInsert().Model(visit).ModelTableExpr("active.visits").Exec(ctx)
			if err != nil {
				continue // Skip on duplicate
			}

			s.result.Visits = append(s.result.Visits, visit)
			if visit.ExitTime == nil {
				activeVisitStudents[studentID] = struct{}{}
				s.result.StudentsCheckedIn++
				s.result.StudentsInRooms[activeGroup.RoomID]++
			}
		}
	}

	if s.verbose {
		log.Printf("Created %d student visits (%d currently checked in)",
			len(s.result.Visits), s.result.StudentsCheckedIn)
	}

	return nil
}

// createAttendanceRecords creates daily attendance records
func (s *Seeder) createAttendanceRecords(ctx context.Context, currentTime time.Time) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	today := currentTime.Truncate(24 * time.Hour)

	// Create attendance records for all students
	attendanceCount := 0
	for _, student := range s.fixedData.Students {
		// 90% attendance rate
		if rng.Float32() < 0.9 {
			checkInTime := today.Add(7*time.Hour + 30*time.Minute) // 7:30 AM

			// Add some variation to arrival times
			arrivalOffset := rng.Intn(60) - 10 // -10 to +50 minutes
			checkInTime = checkInTime.Add(time.Duration(arrivalOffset) * time.Minute)

			// Find a device to check in with
			var deviceID int64 = 1 // Default device
			if len(s.fixedData.Devices) > 0 {
				deviceID = s.fixedData.Devices[0].ID
			}

			attendance := &active.Attendance{
				StudentID:   student.ID,
				Date:        today,
				CheckInTime: checkInTime,
				CheckedInBy: 1, // Admin user
				DeviceID:    deviceID,
			}

			attendance.CreatedAt = time.Now()
			attendance.UpdatedAt = time.Now()

			_, err := s.tx.NewInsert().Model(attendance).ModelTableExpr("active.attendance").
				Exec(ctx)
			if err == nil {
				s.result.Attendance = append(s.result.Attendance, attendance)
				attendanceCount++
			}
		}
	}

	if s.verbose {
		log.Printf("Created %d attendance records for today", attendanceCount)
	}

	return nil
}

// createCombinedGroup creates a combined group scenario
func (s *Seeder) createCombinedGroup(ctx context.Context, currentTime time.Time) error {
	// Find two small active groups that could be combined
	if len(s.result.ActiveGroups) < 2 {
		return nil // Not enough groups to combine
	}

	// Create a combined group
	combined := &active.CombinedGroup{
		StartTime: currentTime,
	}
	combined.CreatedAt = time.Now()
	combined.UpdatedAt = time.Now()

	_, err := s.tx.NewInsert().Model(combined).ModelTableExpr("active.combined_groups").Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create combined group: %w", err)
	}

	s.result.CombinedGroups = append(s.result.CombinedGroups, combined)

	// Link first two active groups to the combined group
	for i := 0; i < 2 && i < len(s.result.ActiveGroups); i++ {
		mapping := struct {
			CombinedGroupID int64 `bun:"active_combined_group_id"`
			ActiveGroupID   int64 `bun:"active_group_id"`
		}{
			CombinedGroupID: combined.ID,
			ActiveGroupID:   s.result.ActiveGroups[i].ID,
		}

		_, err = s.tx.NewInsert().
			Model(&mapping).
			ModelTableExpr("active.group_mappings").
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create group mapping: %w", err)
		}

		s.result.GroupMappings = append(s.result.GroupMappings, GroupMapping{
			CombinedGroupID: mapping.CombinedGroupID,
			ActiveGroupID:   mapping.ActiveGroupID,
		})
	}

	if s.verbose {
		log.Printf("Created combined group with %d linked active groups", len(s.result.GroupMappings))
	}

	return nil
}

// calculateStatistics calculates summary statistics
func (s *Seeder) calculateStatistics() {
	// Already tracked during creation, but we can add more stats here if needed
}
