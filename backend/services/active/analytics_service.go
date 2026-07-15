package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// Analytics and statistics

func (s *service) GetDashboardAnalytics(ctx context.Context) (*DashboardAnalytics, error) {
	analytics := &DashboardAnalytics{
		LastUpdated: time.Now(),
	}

	today := timezone.TodayDate()

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

	if statusCounts, err := s.countEffectiveAbsencesForDate(ctx, today); err == nil {
		analytics.StudentsSick = statusCounts.Sick
		analytics.StudentsExcused = statusCounts.Excused
	} else {
		s.getLogger().Warn("failed to count effective student absences for dashboard",
			"error", err.Error(),
		)
	}

	// Phase 3: Build room lookup maps
	roomData := s.buildRoomLookupMaps(baseData.allRooms)

	// Phase 4: Load students with groups for home room calculation
	studentIDs := extractUniqueStudentIDs(baseData.activeVisits)
	studentsWithGroups, err := s.loadStudentsWithGroups(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Phase 5: Build group-related maps
	groupData := s.buildEducationGroupMaps(baseData.allEducationGroups, studentsWithGroups)

	// Phase 6: Process active groups and calculate group metrics
	activeGroupsCount, ogsGroupsCount, uniqueStudentsInRoomsOverall := processActiveGroups(
		baseData.activeGroups, baseData.visitsByGroupID, baseData.educationGroupsByID, roomData,
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

	// Phase 9: Build summary lists (using pre-loaded maps for O(1) name lookups)
	analytics.RecentActivity = buildRecentActivity(baseData.activeGroups, baseData.activityGroupsByID, baseData.educationGroupsByID, roomData)
	analytics.CurrentActivities = buildCurrentActivities(baseData.allActivityGroups, baseData.activeGroups, roomData)
	analytics.ActiveGroupsSummary = buildActiveGroupsSummary(baseData.activeGroups, baseData.activityGroupsByID, baseData.educationGroupsByID, roomData)

	return analytics, nil
}

func (s *service) countEffectiveAbsencesForDate(ctx context.Context, date timezone.Date) (studentStatusCounts, error) {
	if s.StudentStatusRepo == nil {
		return studentStatusCounts{}, nil
	}
	counts, err := s.StudentStatusRepo.CountEffectiveDashboardAbsences(ctx, date)
	if err != nil {
		return studentStatusCounts{}, err
	}
	if counts == nil {
		return studentStatusCounts{}, nil
	}
	return studentStatusCounts{
		Sick:    counts.Sick,
		Excused: counts.Excused,
	}, nil
}

type studentStatusCounts struct {
	Sick    int
	Excused int
}
