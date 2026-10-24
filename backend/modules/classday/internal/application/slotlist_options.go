package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *service) ListOptions(ctx context.Context, requested classday.Date) (*classday.OptionsResult, error) {
	if err := s.requireTimetableEnabled(ctx); err != nil {
		return nil, err
	}
	if !s.configured() {
		return nil, fmt.Errorf("slot list service is not configured")
	}
	date, err := parseDate(requested)
	if err != nil {
		return nil, err
	}
	// Read the day's instances once rather than rebuilding each list kind.
	instances, err := s.loadInstances(ctx, date)
	if err != nil {
		return nil, err
	}
	slots, kindInstanceIDs, err := s.optionSlots(ctx, instances)
	if err != nil {
		return nil, err
	}
	students, studentIDs, readable, err := s.optionStudents(ctx, date)
	if err != nil {
		return nil, err
	}
	careDays := map[int64]ports.CareDay{}
	if len(studentIDs) > 0 {
		careDays, err = s.careDays.ResolveForDate(ctx, studentIDs, date)
		if err != nil {
			return nil, fmt.Errorf("resolve care days: %w", err)
		}
	}
	plannedByInstance, err := s.optionPlannedCounts(ctx, instances, kindInstanceIDs, readable, careDays)
	if err != nil {
		return nil, err
	}
	listKinds := optionListKinds(kindInstanceIDs, plannedByInstance)
	cohorts, err := s.optionPickupCohorts(ctx, date, students, studentIDs, readable, careDays)
	if err != nil {
		return nil, err
	}
	return &classday.OptionsResult{Date: date.String(), Slots: slots, PickupCohorts: cohorts, ListKinds: listKinds}, nil
}

// optionSlots excludes cancelled occurrences before resolving rooms. Room and
// list-kind metadata let clients detect changes even when instance IDs stay put.
func (s *service) optionSlots(ctx context.Context, instances []*slotInstance) ([]classday.Slot, map[classday.ListKind][]int64, error) {
	slots := make([]classday.Slot, 0, len(instances))
	kindInstanceIDs := map[classday.ListKind][]int64{}
	roomCache := map[int64]string{}
	for _, inst := range instances {
		if inst.Status == ports.InstanceStatusCancelled {
			continue
		}
		roomName, err := s.lookupRoomName(ctx, inst.RoomID, roomCache)
		if err != nil {
			return nil, nil, err
		}
		listKind := ""
		if inst.ListKind != nil {
			listKind = *inst.ListKind
		}
		slots = append(slots, classday.Slot{
			InstanceID: inst.ID,
			Title:      inst.Title,
			TimeRange:  fmt.Sprintf("%s\u2013%s", inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout)),
			Status:     inst.Status,
			ListKind:   listKind,
			RoomName:   roomName,
		})
		if inst.ListKind == nil {
			continue
		}
		kind := classday.ListKind(*inst.ListKind)
		if kind != classday.ListKindNone && kind.Valid() {
			kindInstanceIDs[kind] = append(kindInstanceIDs[kind], inst.ID)
		}
	}
	return slots, kindInstanceIDs, nil
}

// Availability counts use the same GDPR scope and date-effective enrollment as
// preview. Care-day and pickup reads still use all eligible IDs, not just readable ones.
func (s *service) optionStudents(ctx context.Context, date calendar.Date) ([]peopledirectory.Student, []int64, map[int64]struct{}, error) {
	access, err := s.resolveStudentReadAccess(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	students, err := s.listEligibleStudents(ctx, date, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	studentIDs := make([]int64, 0, len(students))
	readable := make(map[int64]struct{}, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		if access.canReadGroup(student.GroupID) {
			readable[student.ID] = struct{}{}
		}
	}
	return students, studentIDs, readable, nil
}

func (s *service) optionPlannedCounts(ctx context.Context, instances []*slotInstance, kindInstanceIDs map[classday.ListKind][]int64, readable map[int64]struct{}, careDays map[int64]ports.CareDay) (map[int64]int, error) {
	completedByInstance := make(map[int64]bool, len(instances))
	activeGroupByInstance := make(map[int64]*int64, len(instances))
	for _, inst := range instances {
		completedByInstance[inst.ID] = inst.Status == ports.InstanceStatusCompleted
		activeGroupByInstance[inst.ID] = inst.ActiveGroupID
	}
	classifiedIDs := make([]int64, 0)
	for _, ids := range kindInstanceIDs {
		classifiedIDs = append(classifiedIDs, ids...)
	}
	plannedByInstance := map[int64]int{}
	if len(classifiedIDs) == 0 {
		return plannedByInstance, nil
	}
	rosterRows, err := s.loadRoster(ctx, classifiedIDs)
	if err != nil {
		return nil, err
	}
	presentByGroup, err := s.optionVisitPresence(ctx, classifiedIDs, activeGroupByInstance)
	if err != nil {
		return nil, err
	}
	for _, row := range rosterRows {
		if _, ok := readable[row.StudentID]; !ok {
			continue
		}
		activeGroupID := activeGroupByInstance[row.InstanceID]
		presentViaVisit := activeGroupID != nil && presentByGroup[*activeGroupID][row.StudentID]
		if s.optionRowPlanned(row, completedByInstance[row.InstanceID], careDays[row.StudentID], presentViaVisit) {
			plannedByInstance[row.InstanceID]++
		}
	}
	return plannedByInstance, nil
}

// Visits retain presence evidence when best-effort attendance sync did not mark
// the roster row present. Load the same started groups as loadSlotPresence.
func (s *service) optionVisitPresence(ctx context.Context, classifiedIDs []int64, activeGroupByInstance map[int64]*int64) (map[int64]map[int64]bool, error) {
	presentByGroup := map[int64]map[int64]bool{}
	if s.presence == nil {
		return presentByGroup, nil
	}
	activeGroupIDs := make([]int64, 0, len(classifiedIDs))
	for _, id := range classifiedIDs {
		if activeGroupID := activeGroupByInstance[id]; activeGroupID != nil {
			activeGroupIDs = append(activeGroupIDs, *activeGroupID)
		}
	}
	if len(activeGroupIDs) == 0 {
		return presentByGroup, nil
	}
	visits, err := s.presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: activeGroupIDs})
	if err != nil {
		return nil, err
	}
	for _, visit := range visits {
		byStudent := presentByGroup[visit.ActiveGroupID]
		if byStudent == nil {
			byStudent = map[int64]bool{}
			presentByGroup[visit.ActiveGroupID] = byStudent
		}
		byStudent[visit.StudentID] = true
	}
	return presentByGroup, nil
}

// Mirror classifyPlannedRow: walk-ins and non-bookings are not planned. On an
// unfinished occurrence the raw care-day verdict excludes unscheduled presence;
// manual overrides win and completed occurrences trust their frozen markers.
// A cancelled child who attended is unplanned, including visit-only presence.
// A cancelled no-show remains a planned signed-off row, unlike a non-booking.
func (s *service) optionRowPlanned(row ports.InstanceStudent, completed bool, careDay ports.CareDay, presentViaVisit bool) bool {
	if row.IsUnplanned || row.NotScheduled {
		return false
	}
	if !completed && row.ManualStatusAt == nil && careDay == ports.CareDayNotScheduled {
		return false
	}
	if row.ManualStatusAt == nil && careDay == ports.CareDayCancelled &&
		(row.Status == ports.AttendancePresent || presentViaVisit) {
		return false
	}
	return s.rules.RowCareDay(completed, rosterFacts(row), careDay) != ports.CareDayNotScheduled
}

func optionListKinds(kindInstanceIDs map[classday.ListKind][]int64, plannedByInstance map[int64]int) []classday.ListKindOption {
	listKinds := make([]classday.ListKindOption, 0, len(classday.AllListKinds))
	for _, kind := range classday.AllListKinds {
		ids := kindInstanceIDs[kind]
		rowCount := 0
		for _, id := range ids {
			rowCount += plannedByInstance[id]
		}
		listKinds = append(listKinds, classday.ListKindOption{
			Kind:      kind,
			Label:     kind.Label(),
			Available: len(ids) > 0,
			SlotCount: len(ids),
			RowCount:  rowCount,
		})
	}
	return listKinds
}

func (s *service) optionPickupCohorts(ctx context.Context, date calendar.Date, students []peopledirectory.Student, studentIDs []int64, readable map[int64]struct{}, careDays map[int64]ports.CareDay) ([]classday.PickupCohortOption, error) {
	buckets, err := s.pickupBuckets(ctx)
	if err != nil {
		return nil, err
	}
	cohortCounts := map[classday.PickupCohort]int{}
	// Past pickup lists are unavailable because the schedule has no history.
	if !date.Before(s.todayDate()) && len(studentIDs) > 0 {
		pickupTimes, err := s.effectiveTimes.PickupTimes(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		// A cancelled care day keeps its weekly bucket even if a cleared same-day
		// exception removed the effective time, matching collectPickupEntries.
		regularBucket, err := s.regularPickupBucket(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		cohortCounts = countOptionPickupCohorts(students, readable, careDays, pickupTimes, regularBucket, buckets)
	}
	return pickupCohortOptions(buckets, cohortCounts), nil
}

func countOptionPickupCohorts(students []peopledirectory.Student, readable map[int64]struct{}, careDays map[int64]ports.CareDay, pickupTimes map[int64]*time.Time, regularBucket map[int64]string, buckets pickupBucketConfig) map[classday.PickupCohort]int {
	cohortCounts := map[classday.PickupCohort]int{}
	for _, student := range students {
		if _, ok := readable[student.ID]; !ok {
			continue
		}
		if careDays[student.ID] == ports.CareDayNotScheduled {
			continue
		}
		cancelled := careDays[student.ID] == ports.CareDayCancelled
		hhmm := cohortPickupTime(cancelled, pickupTimes[student.ID], regularBucket[student.ID])
		if hhmm == "" {
			continue
		}
		for _, cohort := range []classday.PickupCohort{classday.PickupCohortShortDay, classday.PickupCohortLongDay} {
			if pickupMatchesCohort(cohort, hhmm, buckets) {
				cohortCounts[cohort]++
			}
		}
	}
	return cohortCounts
}

func pickupCohortOptions(buckets pickupBucketConfig, cohortCounts map[classday.PickupCohort]int) []classday.PickupCohortOption {
	cohortLabels := map[classday.PickupCohort]string{
		classday.PickupCohortShortDay: "Ganztag bis " + buckets.ShortCutoff,
		classday.PickupCohortLongDay:  "Ganztag bis " + buckets.LongCutoff,
	}
	cohorts := make([]classday.PickupCohortOption, 0, 2)
	for _, cohort := range []classday.PickupCohort{classday.PickupCohortShortDay, classday.PickupCohortLongDay} {
		cohorts = append(cohorts, classday.PickupCohortOption{
			Cohort:    cohort,
			Label:     cohortLabels[cohort],
			Available: cohortCounts[cohort] > 0,
			RowCount:  cohortCounts[cohort],
		})
	}
	return cohorts
}
