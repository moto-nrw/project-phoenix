package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	educationModels "github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	facilityModels "github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// dashboardBaseData holds the raw data fetched for dashboard analytics
type dashboardBaseData struct {
	activeVisits             []*active.Visit
	todaysAttendance         []*active.Attendance
	allRooms                 []*facilityModels.Room
	activeGroups             []*active.Group
	allEducationGroups       []*educationModels.Group
	activityCategories       int
	supervisorsToday         int
	visitsByGroupID          map[int64][]*active.Visit
	studentsWithActiveVisits map[int64]bool
	studentsWithAttendance   map[int64]bool
	studentsPresent          map[int64]bool
}

// fetchDashboardBaseData retrieves all raw data needed for dashboard analytics
func (s *service) fetchDashboardBaseData(ctx context.Context, today time.Time) (*dashboardBaseData, error) {
	data := &dashboardBaseData{
		studentsWithActiveVisits: make(map[int64]bool),
		studentsWithAttendance:   make(map[int64]bool),
		studentsPresent:          make(map[int64]bool),
		visitsByGroupID:          make(map[int64][]*active.Visit),
	}

	// Get active visits
	activeVisits, err := s.visitRepo.FindActiveVisits(ctx)
	if err != nil {
		return nil, err
	}
	data.activeVisits = activeVisits

	// Build student-visit maps
	for _, visit := range activeVisits {
		data.studentsWithActiveVisits[visit.StudentID] = true
		data.visitsByGroupID[visit.ActiveGroupID] = append(data.visitsByGroupID[visit.ActiveGroupID], visit)
	}

	// Get today's attendance
	todaysAttendance, err := s.attendanceRepo.FindForDate(ctx, today)
	if err != nil {
		return nil, err
	}
	data.todaysAttendance = todaysAttendance

	// Build attendance maps
	for _, record := range todaysAttendance {
		if record.CheckOutTime == nil {
			data.studentsWithAttendance[record.StudentID] = true
			data.studentsPresent[record.StudentID] = true
		}
	}

	// Add students with active visits to present set
	for studentID := range data.studentsWithActiveVisits {
		data.studentsPresent[studentID] = true
	}

	// Get all rooms
	allRooms, err := s.roomRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.allRooms = allRooms

	// Get active groups
	activeGroups, err := s.groupReadRepo.FindActiveGroups(ctx)
	if err != nil {
		return nil, err
	}
	data.activeGroups = activeGroups

	// Get education groups
	allEducationGroups, err := s.educationGroupRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.allEducationGroups = allEducationGroups

	// Get activity categories count
	activityCategories, err := s.activityCatRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.activityCategories = len(activityCategories)

	// Get supervisors count
	supervisorsCount, err := s.countSupervisorsToday(ctx, today)
	if err != nil {
		return nil, err
	}
	data.supervisorsToday = supervisorsCount

	return data, nil
}

// countSupervisorsToday counts unique supervisors active today
func (s *service) countSupervisorsToday(ctx context.Context, today time.Time) (int, error) {
	supervisors, err := s.supervisorRepo.List(ctx, nil)
	if err != nil {
		return 0, err
	}

	supervisorMap := make(map[int64]bool)
	now := time.Now()
	for _, supervisor := range supervisors {
		if supervisor.IsActive() || (supervisor.StartDate.After(today) && supervisor.StartDate.Before(now)) {
			supervisorMap[supervisor.StaffID] = true
		}
	}
	return len(supervisorMap), nil
}

// loadStudentsWithGroups batch loads students for the given IDs
func (s *service) loadStudentsWithGroups(ctx context.Context, studentIDs []int64) ([]*userModels.Student, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	return s.studentRepo.FindByIDs(ctx, studentIDs)
}
