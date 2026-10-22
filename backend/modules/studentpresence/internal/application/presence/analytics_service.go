package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Analytics and statistics

func (s *service) GetDashboardAnalytics(ctx context.Context) (*DashboardAnalytics, error) {
	analytics := &DashboardAnalytics{
		LastUpdated: s.now(),
	}

	today := s.todayDate()

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

	s.applyDashboardAbsences(ctx, analytics, today, baseData.studentsPresent)

	// Phase 3: Build room lookup maps
	roomData := s.buildRoomLookupMaps(baseData.allRooms)

	// Phase 4: Load students with groups for home room calculation
	studentIDs := extractUniqueStudentIDs(baseData.activeVisits)
	studentGroups, err := s.EducationGroupRepo.StudentGroupIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ActiveError{Op: "GetDashboardAnalytics", Err: ErrDatabaseOperation}
	}

	// Phase 5: Build group-related maps
	groupData := s.buildEducationGroupMaps(baseData.allEducationGroups, studentGroups)

	// Phase 6: Process active groups and calculate group metrics
	activeGroupsCount, ogsGroupsCount, uniqueStudentsInRoomsOverall := processActiveGroups(
		baseData.activeGroups, baseData.visitsByGroupID, baseData.activityGroupsByID, roomData,
	)
	analytics.ActiveActivities = activeGroupsCount
	analytics.ActiveOGSGroups = ogsGroupsCount
	analytics.FreeRooms = analytics.TotalRooms - len(roomData.occupiedRooms)

	// Phase 7: Calculate capacity utilization
	if roomData.roomCapacityTotal > 0 {
		analytics.CapacityUtilization = float64(len(uniqueStudentsInRoomsOverall)) / float64(roomData.roomCapacityTotal)
	}

	// Phase 8: Calculate location-based metrics
	applyLocationMetrics(analytics, s.calculateLocationMetrics(roomData, groupData, baseData.activeVisits, baseData.activeGroups))

	// Phase 9: Build summary lists (using pre-loaded maps for O(1) name lookups)
	analytics.RecentActivity = buildRecentActivity(baseData.activeGroups, baseData.activityGroupsByID, roomData)
	analytics.CurrentActivities = buildCurrentActivities(baseData.allActivityGroups, baseData.activeGroups, baseData.visitsByGroupID)
	analytics.ActiveGroupsSummary = buildActiveGroupsSummary(baseData.activeGroups, baseData.activityGroupsByID, roomData, baseData.visitsByGroupID)

	return analytics, nil
}

func applyLocationMetrics(analytics *DashboardAnalytics, locationData *locationMetrics) {
	analytics.StudentsOnPlayground = locationData.studentsOnPlayground
	analytics.StudentsInRooms = locationData.studentsInIndoorRooms
	analytics.StudentsInGroupRooms = locationData.studentsInGroupRooms
	analytics.StudentsInHomeRoom = locationData.studentsInHomeRoom
}

// applyDashboardAbsences fills the sick, excused and at-home counts. The
// absences are optional: a failed count is logged and leaves them empty.
func (s *service) applyDashboardAbsences(ctx context.Context, analytics *DashboardAnalytics, today calendar.Date, studentsPresent map[int64]bool) {
	statusCounts, err := s.countEffectiveAbsencesForDate(ctx, today)
	if err != nil {
		s.getLogger().Warn("failed to count effective student absences for dashboard",
			"error", err.Error(),
		)
		return
	}
	analytics.StudentsSick = statusCounts.Sick
	analytics.StudentsExcused = statusCounts.Excused
	analytics.HomeCandidateIDs = homeCandidateIDs(statusCounts.UnaccountedIDs, studentsPresent)
	analytics.StudentsHome = len(analytics.HomeCandidateIDs)
}

func (s *service) countEffectiveAbsencesForDate(ctx context.Context, date calendar.Date) (studentStatusCounts, error) {
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
		Sick:           counts.Sick,
		Excused:        counts.Excused,
		UnaccountedIDs: counts.UnaccountedIDs,
	}, nil
}

type studentStatusCounts struct {
	Sick           int
	Excused        int
	UnaccountedIDs []int64
}

// homeCandidateIDs keeps the unaccounted students that are not present, in
// the repository's order. They are the "Zuhause" figure before the "Schule"
// split (#3260): every active student who is neither checked in nor sick,
// excused or on a class trip.
//
// The figure is counted from this list, not derived as total minus present
// minus sick minus excused. A child checked in despite a sick or excused day
// sits in two of those buckets, so the subtraction removed it twice and the
// tile showed one child fewer than "Alle Kinder" (#4588).
func homeCandidateIDs(unaccounted []int64, present map[int64]bool) []int64 {
	candidates := make([]int64, 0, len(unaccounted))
	for _, id := range unaccounted {
		if !present[id] {
			candidates = append(candidates, id)
		}
	}
	return candidates
}
