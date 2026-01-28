package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
)

// Analytics and statistics

func (s *service) GetActiveGroupsCount(ctx context.Context) (int, error) {
	// Implementation would count active groups without end time
	// This is a simplified implementation
	groups, err := s.groupRepo.List(ctx, nil)
	if err != nil {
		return 0, &ActiveError{Op: "GetActiveGroupsCount", Err: ErrDatabaseOperation}
	}

	count := 0
	for _, group := range groups {
		if group.IsActive() {
			count++
		}
	}

	return count, nil
}

func (s *service) GetTotalVisitsCount(ctx context.Context) (int, error) {
	visits, err := s.visitRepo.List(ctx, nil)
	if err != nil {
		return 0, &ActiveError{Op: "GetTotalVisitsCount", Err: ErrDatabaseOperation}
	}
	return len(visits), nil
}

func (s *service) GetActiveVisitsCount(ctx context.Context) (int, error) {
	visits, err := s.visitRepo.List(ctx, nil)
	if err != nil {
		return 0, &ActiveError{Op: "GetActiveVisitsCount", Err: ErrDatabaseOperation}
	}

	count := 0
	for _, visit := range visits {
		if visit.IsActive() {
			count++
		}
	}

	return count, nil
}

// GetRoomUtilization returns the current occupancy ratio for a room.
//
// Deprecated: This method is not used by any frontend UI components and provides
// limited value in its current form. The dashboard uses GetDashboardAnalytics instead.
// Consider using facilities.Service.GetRoomUtilization or removing this endpoint entirely.
//
// Current behavior:
// - Returns real-time occupancy ratio: (active students) / (room capacity)
// - Example: 15 students in a 20-capacity room = 0.75 (75%)
// - Does NOT calculate historical time-based utilization
//
// API endpoint: GET /api/active/analytics/room/{roomId}/utilization
// Exposed but unused by frontend. May be removed in a future version.
func (s *service) GetRoomUtilization(ctx context.Context, roomID int64) (float64, error) {
	capacity, err := s.getRoomCapacityOrZero(ctx, roomID)
	if err != nil {
		return 0.0, err
	}
	if capacity == 0 {
		return 0.0, nil
	}

	activeGroups, err := s.groupRepo.FindActiveByRoomID(ctx, roomID)
	if err != nil {
		return 0.0, &ActiveError{Op: "GetRoomUtilization", Err: err}
	}

	currentOccupancy := s.countActiveOccupancyInRoom(ctx, activeGroups)
	return float64(currentOccupancy) / float64(capacity), nil
}

// getRoomCapacityOrZero retrieves room capacity, returning 0 if room not found or has no capacity
func (s *service) getRoomCapacityOrZero(ctx context.Context, roomID int64) (int, error) {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil {
		return 0, &ActiveError{Op: "GetRoomUtilization", Err: err}
	}

	if room.Capacity == nil || *room.Capacity <= 0 {
		return 0, nil
	}

	return *room.Capacity, nil
}

// countActiveOccupancyInRoom counts the number of active visits across all active groups
func (s *service) countActiveOccupancyInRoom(ctx context.Context, activeGroups []*active.Group) int {
	currentOccupancy := 0
	for _, group := range activeGroups {
		if !group.IsActive() {
			continue
		}

		visits, err := s.visitRepo.FindByActiveGroupID(ctx, group.ID)
		if err != nil {
			continue
		}

		for _, visit := range visits {
			if visit.IsActive() {
				currentOccupancy++
			}
		}
	}
	return currentOccupancy
}

// GetStudentAttendanceRate returns a binary presence indicator for a student.
//
// Deprecated: This method is not used by any frontend UI components and provides
// misleading semantics. Despite the name "AttendanceRate", it only returns binary
// presence (1.0 if present, 0.0 if not), not a historical attendance rate.
// The dashboard uses GetDashboardAnalytics or GetStudentAttendanceStatus instead.
//
// Current behavior:
// - Returns 1.0 if student currently has an active visit (present)
// - Returns 0.0 if student has no active visit (not present)
// - Does NOT calculate historical attendance rates or activity participation
//
// API endpoint: GET /api/active/analytics/student/{studentId}/attendance
// Exposed but unused by frontend. May be removed in a future version.
// For actual attendance tracking, use GetStudentAttendanceStatus instead.
func (s *service) GetStudentAttendanceRate(ctx context.Context, studentID int64) (float64, error) {
	visit, err := s.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		// If error, assume student not present
		return 0.0, nil
	}

	if visit != nil && visit.IsActive() {
		return 1.0, nil // Student is present
	}

	return 0.0, nil // Student is not present
}

func (s *service) GetDashboardAnalytics(ctx context.Context) (*DashboardAnalytics, error) {
	analytics := &DashboardAnalytics{
		LastUpdated: time.Now(),
	}

	// Use timezone.Today() for consistent Europe/Berlin timezone handling
	today := timezone.Today()

	// Phase 1: Fetch all base data
	baseData, err := s.fetchDashboardBaseData(ctx, today)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Phase 2: Calculate presence metrics
	analytics.StudentsPresent = len(baseData.studentsPresent)
	analytics.StudentsInTransit = calculateStudentsInTransit(baseData.studentsWithAttendance, baseData.studentsWithActiveVisits)
	analytics.TotalRooms = len(baseData.allRooms)
	analytics.ActivityCategories = baseData.activityCategories
	analytics.SupervisorsToday = baseData.supervisorsToday

	// Phase 3: Build room lookup maps
	roomData := s.buildRoomLookupMaps(baseData.allRooms)

	// Phase 4: Load students with groups for home room calculation
	studentIDs := extractUniqueStudentIDs(baseData.activeVisits)
	studentsWithGroups, err := s.loadStudentsWithGroups(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Phase 5: Build group-related maps
	groupData, err := s.buildEducationGroupMaps(ctx, baseData.activeGroups, baseData.allEducationGroups, studentsWithGroups)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Phase 6: Process active groups and calculate group metrics
	activeGroupsCount, ogsGroupsCount, uniqueStudentsInRoomsOverall := s.processActiveGroups(
		baseData.activeGroups, baseData.visitsByGroupID, groupData, roomData,
	)
	analytics.ActiveActivities = activeGroupsCount
	analytics.ActiveOGSGroups = ogsGroupsCount
	analytics.FreeRooms = analytics.TotalRooms - len(roomData.occupiedRooms)

	// Phase 7: Calculate capacity utilization
	if roomData.roomCapacityTotal > 0 {
		analytics.CapacityUtilization = float64(len(uniqueStudentsInRoomsOverall)) / float64(roomData.roomCapacityTotal)
	}

	// Phase 8: Calculate location-based metrics
	locationData := s.calculateLocationMetrics(roomData, groupData, baseData.activeVisits, baseData.activeGroups)
	analytics.StudentsOnPlayground = locationData.studentsOnPlayground
	analytics.StudentsInRooms = locationData.studentsInIndoorRooms
	analytics.StudentsInGroupRooms = locationData.studentsInGroupRooms
	analytics.StudentsInHomeRoom = locationData.studentsInHomeRoom

	// Phase 9: Build summary lists
	analytics.RecentActivity = s.buildRecentActivity(ctx, baseData.activeGroups, roomData)
	analytics.CurrentActivities = s.buildCurrentActivities(ctx, baseData.activeGroups, roomData)
	analytics.ActiveGroupsSummary = s.buildActiveGroupsSummary(ctx, baseData.activeGroups, roomData)

	return analytics, nil
}
