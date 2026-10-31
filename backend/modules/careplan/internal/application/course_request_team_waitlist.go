package application

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// teamCourseQueue is every open request with the facts the rank rule reads:
// the request children, what they already hold and the AGs each requested
// offering feeds.
type teamCourseQueue struct {
	rows             []careplan.OfferingChangeRequest
	childrenByID     map[int64]*ports.OfferingChangeChild
	bookedByChild    map[int64]map[int64]bool
	groupsByOffering map[int64][]ports.CourseGroup
	offeringsByID    map[int64]*careplan.CareOffering
	periods          []ports.OfferingCarePeriod
}

// CourseWaitlists is the team's view of the AG queues (#4767). The queue is
// the open requests, ranked by the same rule the parents portal shows, so
// the family's "Platz 2" is the team's second row. Nothing is stored: the
// team confirms the first child through the existing request decision. The
// caller names the day, so the view and its clock stay at the request edge.
func (s *OfferingChanges) CourseWaitlists(ctx context.Context, activityGroupID int64, today calendar.Date) ([]careplan.CourseWaitlist, error) {
	rows, err := s.deps.Rows.ListPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("course waitlist: list pending: %w", err)
	}
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
	}
	students, err := s.deps.Students.FindStudents(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("course waitlist: load students: %w", err)
	}
	scope, err := s.deps.Scope(ctx)
	if err != nil {
		return nil, fmt.Errorf("course waitlist: resolve review scope: %w", err)
	}
	rows, _ = visibleOfferingRows(rows, students, scope, careplan.Date(today))
	slices.SortStableFunc(rows, func(a, b careplan.OfferingChangeRequest) int {
		if courseRequestAfter(a, b) {
			return 1
		}
		if courseRequestAfter(b, a) {
			return -1
		}
		return 0
	})
	queue, err := s.loadTeamCourseQueue(ctx, rows, today)
	if err != nil {
		return nil, err
	}
	entries := queue.entriesByGroup(activityGroupID)
	groupIDs := make([]int64, 0, len(entries)+1)
	for groupID := range entries {
		groupIDs = append(groupIDs, groupID)
	}
	if activityGroupID > 0 && entries[activityGroupID] == nil && (scope.SchoolWide || queue.hasVisibleCourseGroup(activityGroupID)) {
		groupIDs = append(groupIDs, activityGroupID)
	}
	slices.Sort(groupIDs)
	return s.courseWaitlistsFor(ctx, groupIDs, entries, today, queue)
}

func (s *OfferingChanges) loadTeamCourseQueue(ctx context.Context, rows []careplan.OfferingChangeRequest, today calendar.Date) (*teamCourseQueue, error) {
	queue := &teamCourseQueue{
		rows: rows, childrenByID: map[int64]*ports.OfferingChangeChild{},
		bookedByChild: map[int64]map[int64]bool{}, groupsByOffering: map[int64][]ports.CourseGroup{},
		offeringsByID: map[int64]*careplan.CareOffering{},
	}
	if len(rows) == 0 {
		return queue, nil
	}
	dates := make(map[int64]calendar.Date, len(rows))
	childIDs := make([]int64, 0, len(rows))
	studentIDs := make([]int64, 0, len(rows))
	offeringIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.RequestChildID <= 0 {
			continue
		}
		dates[row.RequestChildID] = offeringChangeEffectiveFrom(row)
		childIDs = append(childIDs, row.RequestChildID)
		if row.StudentID > 0 {
			studentIDs = append(studentIDs, row.StudentID)
		}
		selections, err := requestSelections(row)
		if err != nil {
			continue
		}
		for _, selected := range selections {
			offeringIDs = append(offeringIDs, selected.OfferingID)
		}
	}
	if err := s.loadTeamQueueChildren(ctx, queue, childIDs, studentIDs, dates, today); err != nil {
		return nil, err
	}
	slices.Sort(offeringIDs)
	if err := s.loadTeamQueueGroups(ctx, queue, slices.Compact(offeringIDs), today); err != nil {
		return nil, err
	}
	return queue, nil
}

// hasVisibleCourseGroup reports whether an in-scope request references the
// limited, active AG. A group-scoped reviewer may only query those AGs by id;
// a school-wide reviewer may also inspect an empty AG.
func (q *teamCourseQueue) hasVisibleCourseGroup(activityGroupID int64) bool {
	for _, groups := range q.groupsByOffering {
		for _, group := range groups {
			if group.ID == activityGroupID && group.Active && group.ParticipantLimit != nil {
				return true
			}
		}
	}
	return false
}

func (s *OfferingChanges) loadTeamQueueChildren(
	ctx context.Context,
	queue *teamCourseQueue,
	childIDs, studentIDs []int64,
	dates map[int64]calendar.Date,
	today calendar.Date,
) error {
	slices.Sort(childIDs)
	children, err := s.deps.Enrollment.Children(ctx, slices.Compact(childIDs))
	if err != nil {
		return fmt.Errorf("course waitlist: load request children: %w", err)
	}
	for i := range children {
		queue.childrenByID[children[i].ID] = &children[i]
	}
	current, err := s.deps.Enrollment.EffectiveSelectionsAt(ctx, dates)
	if err != nil {
		return fmt.Errorf("course waitlist: load current offerings: %w", err)
	}
	for _, link := range current {
		if queue.bookedByChild[link.RequestChildID] == nil {
			queue.bookedByChild[link.RequestChildID] = map[int64]bool{}
		}
		queue.bookedByChild[link.RequestChildID][link.CareOfferingID] = true
	}
	slices.Sort(studentIDs)
	periods, err := s.deps.Enrollment.StudentCarePeriodsByStudentIDs(ctx, slices.Compact(studentIDs))
	if err != nil {
		return fmt.Errorf("course waitlist: load care periods: %w", err)
	}
	queue.periods = periods
	return nil
}

// courseWaitlistCapacityUntil covers the current phase of the queued child.
// The caller passes the next request for an AG, so its window matches the
// complete phase check that approval performs for that child.
func courseWaitlistCapacityUntil(
	rows []careplan.OfferingChangeRequest,
	periods []ports.OfferingCarePeriod,
	today calendar.Date,
) calendar.Date {
	effectiveDates := make(map[int64]calendar.Date, len(rows))
	for _, row := range rows {
		if row.StudentID > 0 {
			effectiveDates[row.StudentID] = offeringChangeEffectiveFrom(row)
		}
	}
	until := today.AddDays(1)
	for _, period := range periods {
		effectiveOn, ok := effectiveDates[period.StudentID]
		if !ok || !carePeriodCovers(period, effectiveOn) {
			continue
		}
		if until.Before(period.ServiceEnd.AddDays(1)) {
			until = period.ServiceEnd.AddDays(1)
		}
	}
	return until
}

func (s *OfferingChanges) loadTeamQueueGroups(ctx context.Context, queue *teamCourseQueue, offeringIDs []int64, today calendar.Date) error {
	if len(offeringIDs) == 0 {
		return nil
	}
	offerings, err := s.offeringsByIDs(ctx, offeringIDs)
	if err != nil {
		return fmt.Errorf("course waitlist: load requested offerings: %w", err)
	}
	queue.offeringsByID = offeringsByID(offerings)
	references := make([]ports.CourseOfferingReference, 0, len(offerings))
	for _, offering := range offerings {
		references = append(references, ports.CourseOfferingReference{OfferingID: offering.ID, ActivityGroupID: offering.ActivityGroupID})
	}
	groups, err := s.deps.Planning.CourseGroupsForOfferings(ctx, references, today)
	if err != nil {
		return fmt.Errorf("course waitlist: load course groups: %w", err)
	}
	queue.groupsByOffering = groups
	return nil
}

// entriesByGroup ranks the open requests per active AG. A positive filter
// keeps that one AG.
func (q *teamCourseQueue) entriesByGroup(activityGroupID int64) map[int64][]careplan.CourseWaitlistEntry {
	targets := make(map[int64]ports.CourseGroup)
	for _, groups := range q.groupsByOffering {
		for _, group := range groups {
			if group.Active && group.ParticipantLimit != nil && (activityGroupID <= 0 || group.ID == activityGroupID) {
				targets[group.ID] = group
			}
		}
	}
	entries := make(map[int64][]careplan.CourseWaitlistEntry, len(targets))
	for groupID, group := range targets {
		target := []ports.CourseGroup{group}
		for _, row := range q.rows {
			if !waitsForCourseGroups(row, target, q.childrenByID[row.RequestChildID], q.groupsByOffering, q.bookedByChild[row.RequestChildID]) {
				continue
			}
			entries[groupID] = append(entries[groupID], careplan.CourseWaitlistEntry{
				RequestID: row.ID, StudentID: row.StudentID, Rank: len(entries[groupID]) + 1, SubmittedAt: row.CreatedAt,
			})
		}
	}
	return entries
}

// capacityWindow is the interval the next child in one AG would reserve on
// approval. The waiting list's free-place hint must therefore ignore a seat
// that ends before that child's request becomes effective.
func (q *teamCourseQueue) capacityWindow(entries []careplan.CourseWaitlistEntry, today calendar.Date) (calendar.Date, calendar.Date) {
	if len(entries) == 0 {
		return today, today.AddDays(1)
	}
	for _, row := range q.rows {
		if row.ID != entries[0].RequestID {
			continue
		}
		from := offeringChangeEffectiveFrom(row)
		return from, courseWaitlistCapacityUntil([]careplan.OfferingChangeRequest{row}, q.periods, from)
	}
	return today, today.AddDays(1)
}

type waitlistOccupancyWindow struct {
	from, until calendar.Date
	groupIDs    []int64
	offeringIDs []int64
}

// nextOfferingsForGroup returns the newly requested course offerings that
// make the first queued child wait for this AG. Their own limits are part of
// the same approval and must narrow the free-place hint as well.
func (q *teamCourseQueue) nextOfferingsForGroup(groupID int64, entries []careplan.CourseWaitlistEntry) []*careplan.CareOffering {
	if len(entries) == 0 {
		return nil
	}
	row := q.requestByID(entries[0].RequestID)
	if row == nil {
		return nil
	}
	selections, err := requestSelections(*row)
	if err != nil {
		return nil
	}
	result := make([]*careplan.CareOffering, 0, len(selections))
	seen := make(map[int64]bool, len(selections))
	for _, selected := range selections {
		if seen[selected.OfferingID] {
			continue
		}
		seen[selected.OfferingID] = true
		if offering := q.nextOfferingForGroup(groupID, row.RequestChildID, selected.OfferingID); offering != nil {
			result = append(result, offering)
		}
	}
	return result
}

func (q *teamCourseQueue) requestByID(requestID int64) *careplan.OfferingChangeRequest {
	for _, row := range q.rows {
		if row.ID != requestID {
			continue
		}
		return &row
	}
	return nil
}

func (q *teamCourseQueue) nextOfferingForGroup(groupID, requestChildID, offeringID int64) *careplan.CareOffering {
	if q.bookedByChild[requestChildID][offeringID] {
		return nil
	}
	if !slices.ContainsFunc(q.groupsByOffering[offeringID], func(group ports.CourseGroup) bool {
		return group.Active && group.ID == groupID
	}) {
		return nil
	}
	return q.offeringsByID[offeringID]
}

func waitlistOccupancyWindows(
	groups []ports.CourseGroup,
	entries map[int64][]careplan.CourseWaitlistEntry,
	today calendar.Date,
	queue *teamCourseQueue,
) map[string]*waitlistOccupancyWindow {
	windows := map[string]*waitlistOccupancyWindow{}
	for _, group := range groups {
		if !group.Active {
			continue
		}
		from, until := queue.capacityWindow(entries[group.ID], today)
		key := from.String() + "|" + until.String()
		if windows[key] == nil {
			windows[key] = &waitlistOccupancyWindow{from: from, until: until}
		}
		windows[key].groupIDs = append(windows[key].groupIDs, group.ID)
		for _, offering := range queue.nextOfferingsForGroup(group.ID, entries[group.ID]) {
			if offering.Capacity != nil {
				windows[key].offeringIDs = append(windows[key].offeringIDs, offering.ID)
			}
		}
	}
	return windows
}

func (s *OfferingChanges) waitlistOccupancy(
	ctx context.Context,
	groups []ports.CourseGroup,
	entries map[int64][]careplan.CourseWaitlistEntry,
	today calendar.Date,
	queue *teamCourseQueue,
) (map[int64]int, map[int64]map[int64]int, error) {
	taken := make(map[int64]int, len(groups))
	offeringTaken := make(map[int64]map[int64]int, len(groups))
	for _, window := range waitlistOccupancyWindows(groups, entries, today, queue) {
		counts, err := s.courseOccupancy(ctx, window.groupIDs, window.from, window.until, 0)
		if err != nil {
			return nil, nil, err
		}
		for groupID, count := range counts {
			taken[groupID] = count
		}
		slices.Sort(window.offeringIDs)
		peaks, err := s.deps.Enrollment.CapacityPeaks(ctx, slices.Compact(window.offeringIDs), window.from, window.until)
		if err != nil {
			return nil, nil, fmt.Errorf("course waitlist: count offering occupancy: %w", err)
		}
		for _, groupID := range window.groupIDs {
			offeringTaken[groupID] = peaks
		}
	}
	return taken, offeringTaken, nil
}

// courseWaitlistsFor reads the AGs' names, limits and occupancy and joins
// them with their queues, sorted by name. An id that names no active AG of
// the school is left out.
func (s *OfferingChanges) courseWaitlistsFor(
	ctx context.Context,
	groupIDs []int64,
	entries map[int64][]careplan.CourseWaitlistEntry,
	today calendar.Date,
	queue *teamCourseQueue,
) ([]careplan.CourseWaitlist, error) {
	if len(groupIDs) == 0 {
		return []careplan.CourseWaitlist{}, nil
	}
	groups, err := s.waitlistGroups(ctx, groupIDs, today, queue)
	if err != nil {
		return nil, err
	}
	taken, offeringTaken, err := s.waitlistOccupancy(ctx, groups, entries, today, queue)
	if err != nil {
		return nil, err
	}
	return queue.waitlists(groups, entries, taken, offeringTaken), nil
}

func (s *OfferingChanges) waitlistGroups(
	ctx context.Context,
	groupIDs []int64,
	today calendar.Date,
	queue *teamCourseQueue,
) ([]ports.CourseGroup, error) {
	groupsByID := queue.linkedGroupsByID(groupIDs)
	missing := missingWaitlistGroups(groupIDs, groupsByID)
	if len(missing) > 0 {
		groups, err := s.deps.Planning.CourseGroupsByID(ctx, missing, today)
		if err != nil {
			return nil, fmt.Errorf("course waitlist: read course groups: %w", err)
		}
		for _, group := range groups {
			groupsByID[group.ID] = group
		}
	}
	return orderedWaitlistGroups(groupIDs, groupsByID), nil
}

func (q *teamCourseQueue) linkedGroupsByID(groupIDs []int64) map[int64]ports.CourseGroup {
	wanted := make(map[int64]bool, len(groupIDs))
	groupsByID := make(map[int64]ports.CourseGroup, len(groupIDs))
	for _, groupID := range groupIDs {
		wanted[groupID] = true
	}
	for _, linkedGroups := range q.groupsByOffering {
		for _, group := range linkedGroups {
			if wanted[group.ID] {
				groupsByID[group.ID] = group
			}
		}
	}
	return groupsByID
}

func missingWaitlistGroups(groupIDs []int64, groupsByID map[int64]ports.CourseGroup) []int64 {
	missing := make([]int64, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		if _, ok := groupsByID[groupID]; !ok {
			missing = append(missing, groupID)
		}
	}
	return missing
}

func orderedWaitlistGroups(groupIDs []int64, groupsByID map[int64]ports.CourseGroup) []ports.CourseGroup {
	groups := make([]ports.CourseGroup, 0, len(groupsByID))
	for _, groupID := range groupIDs {
		if group, ok := groupsByID[groupID]; ok {
			groups = append(groups, group)
		}
	}
	return groups
}

func (q *teamCourseQueue) waitlists(
	groups []ports.CourseGroup,
	entries map[int64][]careplan.CourseWaitlistEntry,
	taken map[int64]int,
	offeringTaken map[int64]map[int64]int,
) []careplan.CourseWaitlist {
	result := make([]careplan.CourseWaitlist, 0, len(groups))
	seen := make(map[int64]bool, len(groups))
	for _, group := range groups {
		if !group.Active || seen[group.ID] {
			continue
		}
		seen[group.ID] = true
		groupQueue := entries[group.ID]
		capacity, free := q.waitlistCapacity(group, groupQueue, taken[group.ID], offeringTaken[group.ID])
		if groupQueue == nil {
			groupQueue = []careplan.CourseWaitlistEntry{}
		}
		result = append(result, careplan.CourseWaitlist{
			ActivityGroupID: group.ID, Name: group.Name, ParticipantLimit: capacity,
			Participants: taken[group.ID], FreeSlots: free, Entries: groupQueue,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result
}

func (q *teamCourseQueue) waitlistCapacity(
	group ports.CourseGroup,
	entries []careplan.CourseWaitlistEntry,
	groupTaken int,
	offeringTaken map[int64]int,
) (*int, *int) {
	capacity, free := effectiveCourseCapacity(group.ParticipantLimit, groupTaken, nil, nil)
	for _, offering := range q.nextOfferingsForGroup(group.ID, entries) {
		if offering.Capacity == nil {
			continue
		}
		remaining := max(*offering.Capacity-offeringTaken[offering.ID], 0)
		capacity, free = narrowCourseCapacity(capacity, free, offering.Capacity, &remaining)
	}
	return capacity, free
}
