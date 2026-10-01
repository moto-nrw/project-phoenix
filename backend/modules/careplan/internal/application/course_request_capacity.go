package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// pendingCourseRequest returns the child's open request and whether it is a
// course request. A pending request that changes only care offerings blocks
// a new one (one open request per child) but is not withdrawable from the
// Kurse section.
func (s *OfferingChanges) pendingCourseRequest(
	ctx context.Context,
	studentID int64,
	catalog *careplan.OfferingChangeCatalog,
	courses []careplan.CourseCatalogItem,
) (*careplan.OfferingChangeRequest, bool, error) {
	pending, err := s.deps.Rows.PendingForStudent(ctx, studentID)
	if err != nil {
		return nil, false, fmt.Errorf("course request: get pending: %w", err)
	}
	if pending == nil {
		return nil, false, nil
	}
	added, err := addedCourseIDs(*pending, courses)
	if err != nil {
		return nil, false, err
	}
	if len(added) == 0 {
		return pending, false, nil
	}
	pure, err := isCourseOnlyRequest(*pending, catalog, added)
	if err != nil {
		return nil, false, err
	}
	if !pure {
		return pending, false, nil
	}
	for i := range courses {
		courses[i].Requested = slices.Contains(added, courses[i].OfferingID)
	}
	return pending, true, nil
}

// isCourseOnlyRequest verifies that a complete selection changes nothing
// but adding courses. A course withdrawal acts on the whole row, so mixed
// care and course requests stay in the care flow.
func isCourseOnlyRequest(request careplan.OfferingChangeRequest, catalog *careplan.OfferingChangeCatalog, addedCourseIDs []int64) (bool, error) {
	if catalog == nil || len(addedCourseIDs) == 0 {
		return false, nil
	}
	selections, err := requestSelections(request)
	if err != nil {
		return false, err
	}
	selected := make(map[int64]careplan.OfferingChangeSelection, len(selections))
	for _, wanted := range selections {
		selected[wanted.OfferingID] = wanted
	}
	known := make(map[int64]bool, len(catalog.Items))
	for _, item := range catalog.Items {
		known[item.OfferingID] = true
		if !catalogItemKeptByRequest(item, selected, addedCourseIDs) {
			return false, nil
		}
	}
	for offeringID := range selected {
		if !known[offeringID] {
			return false, nil
		}
	}
	return true, nil
}

// catalogItemKeptByRequest reports whether the request leaves the item as
// it is, or adds it as one of the requested courses.
func catalogItemKeptByRequest(item careplan.OfferingChangeCatalogItem, selected map[int64]careplan.OfferingChangeSelection, addedCourseIDs []int64) bool {
	selection, requested := selected[item.OfferingID]
	if item.Automatic {
		return true
	}
	if !item.Selected && requested && slices.Contains(addedCourseIDs, item.OfferingID) {
		return true
	}
	if item.Selected != requested {
		return false
	}
	return !requested || item.DaysOfWeekMode != daysOfWeekModeParentChoice ||
		slices.Equal(canonicalDays(item.SelectedDays), canonicalDays(selection.SelectedDays))
}

// addedCourseIDs is the set of courses a request adds on top of what the
// child already holds: the definition of a course request.
func addedCourseIDs(request careplan.OfferingChangeRequest, courses []careplan.CourseCatalogItem) ([]int64, error) {
	selections, err := requestSelections(request)
	if err != nil {
		return nil, err
	}
	selected := make(map[int64]bool, len(selections))
	for _, wanted := range selections {
		selected[wanted.OfferingID] = true
	}
	added := make([]int64, 0, len(courses))
	for _, course := range courses {
		if selected[course.OfferingID] && !course.Booked {
			added = append(added, course.OfferingID)
		}
	}
	return added, nil
}

// applyCourseCapacity fills the effective limit, the free slots and, for a
// requested course without a free slot, the waiting position.
func (s *OfferingChanges) applyCourseCapacity(
	ctx context.Context,
	catalog *careplan.OfferingChangeCatalog,
	courses []careplan.CourseCatalogItem,
	groupsByOffering map[int64][]ports.CourseGroup,
	pending *careplan.OfferingChangeRequest,
) error {
	groupIDs := make([]int64, 0, len(courses))
	for _, course := range courses {
		for _, group := range groupsByOffering[course.OfferingID] {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	taken, err := s.courseOccupancy(ctx, groupIDs, catalog.EarliestEffectiveFrom, catalog.CourseCapacityUntil, 0)
	if err != nil {
		return err
	}
	itemsByID := make(map[int64]careplan.OfferingChangeCatalogItem, len(catalog.Items))
	for _, item := range catalog.Items {
		itemsByID[item.OfferingID] = item
	}
	for i := range courses {
		course := &courses[i]
		item := itemsByID[course.OfferingID]
		capacity, free := item.Capacity, item.FreeSlots
		// One offering can feed several Regeltermine (a Jahrgang split). The
		// child ends up in one of them, and nobody knows which before the
		// decision, so the tightest one decides.
		for _, group := range groupsByOffering[course.OfferingID] {
			capacity, free = effectiveCourseCapacity(group.ParticipantLimit, taken[group.ID], capacity, free)
		}
		course.Capacity, course.FreeSlots = capacity, free
		if !course.Requested || course.Booked || free == nil || *free > 0 {
			continue
		}
		course.Waitlisted = true
		if pending == nil {
			continue
		}
		position, err := s.courseWaitlistPosition(ctx, catalog, groupsByOffering[course.OfferingID], groupsByOffering, *pending)
		if err != nil {
			return err
		}
		course.WaitlistPosition = position
	}
	return nil
}

// effectiveCourseCapacity takes the stricter of the two limits a school can
// maintain. An unset limit never narrows the other one.
func effectiveCourseCapacity(groupLimit *int, groupTaken int, offeringCapacity, offeringFree *int) (capacity, free *int) {
	if groupLimit != nil {
		limit := *groupLimit
		capacity = &limit
		remaining := max(limit-groupTaken, 0)
		free = &remaining
	}
	if offeringCapacity == nil {
		return capacity, free
	}
	if capacity == nil || *offeringCapacity < *capacity {
		limit := *offeringCapacity
		capacity = &limit
	}
	if offeringFree != nil && (free == nil || *offeringFree < *free) {
		remaining := *offeringFree
		free = &remaining
	}
	return capacity, free
}

func (s *OfferingChanges) courseOccupancy(ctx context.Context, groupIDs []int64, from, until calendar.Date, excludeStudentID int64) (map[int64]int, error) {
	if !from.Before(until) {
		until = from.AddDays(1)
	}
	counts, err := s.deps.Planning.CountActiveCourseEnrollments(ctx, groupIDs, from, until, excludeStudentID)
	if err != nil {
		return nil, fmt.Errorf("course request: count course rosters: %w", err)
	}
	return counts, nil
}

// courseCapacityCheck is one approval's claim on the AGs its newly booked
// offerings feed, in [from, until).
type courseCapacityCheck struct {
	studentID      int64
	requestChildID int64
	offerings      []*careplan.CareOffering
	from, until    calendar.Date
	pending        *careplan.OfferingChangeRequest
}

// assertCourseCapacitiesAvailable refuses an approval that would put one
// more child into a full AG. It locks every relevant group in one globally
// ordered batch before reading occupancy, because several offerings can
// share an activity group.
func (s *OfferingChanges) assertCourseCapacitiesAvailable(ctx context.Context, check courseCapacityCheck) error {
	if len(check.offerings) == 0 {
		return nil
	}
	groupsByOffering, allGroups, err := s.targetCourseGroups(ctx, check)
	if err != nil || len(allGroups) == 0 {
		return err
	}
	locked, err := s.lockCourseGroups(ctx, allGroups)
	if err != nil {
		return err
	}
	if check.pending != nil {
		position, err := s.courseWaitlistPosition(ctx, nil, allGroups, groupsByOffering, *check.pending)
		if err != nil {
			return err
		}
		if err := assertCourseRequestQueuePosition(position); err != nil {
			return err
		}
	}
	return s.assertCourseGroupLimits(ctx, check, groupsByOffering, locked)
}

// targetCourseGroups resolves the active groups each offering feeds for the
// request child. An offering whose only matching groups are inactive can no
// longer be booked.
func (s *OfferingChanges) targetCourseGroups(ctx context.Context, check courseCapacityCheck) (map[int64][]ports.CourseGroup, []ports.CourseGroup, error) {
	child, err := s.deps.Enrollment.Child(ctx, check.requestChildID)
	if err != nil {
		return nil, nil, fmt.Errorf("course request: load request child: %w", err)
	}
	if child == nil {
		return nil, nil, fmt.Errorf("course request: request child %d not found", check.requestChildID)
	}
	references := make([]ports.CourseOfferingReference, 0, len(check.offerings))
	for _, offering := range check.offerings {
		references = append(references, ports.CourseOfferingReference{OfferingID: offering.ID, ActivityGroupID: offering.ActivityGroupID})
	}
	projected, err := s.deps.Planning.CourseGroupsForOfferings(ctx, references, check.from)
	if err != nil {
		return nil, nil, fmt.Errorf("course request: list course groups: %w", err)
	}
	catalog := courseTargetCatalog(child)
	groupsByOffering := make(map[int64][]ports.CourseGroup, len(check.offerings))
	allGroups := make([]ports.CourseGroup, 0)
	for _, offering := range check.offerings {
		groups, hadCourseTarget := activeCourseGroupsForTarget(projected[offering.ID], catalog)
		if len(groups) == 0 {
			if hadCourseTarget {
				return nil, nil, fmt.Errorf("%w: %s", careplan.ErrOfferingChangeInvalid, offering.Name)
			}
			continue
		}
		groupsByOffering[offering.ID] = groups
		allGroups = append(allGroups, groups...)
	}
	return groupsByOffering, allGroups, nil
}

func (s *OfferingChanges) assertCourseGroupLimits(
	ctx context.Context,
	check courseCapacityCheck,
	groupsByOffering map[int64][]ports.CourseGroup,
	locked []ports.CourseGroup,
) error {
	lockedByID := make(map[int64]ports.CourseGroup, len(locked))
	groupIDs := make([]int64, 0, len(locked))
	for _, group := range locked {
		lockedByID[group.ID] = group
		groupIDs = append(groupIDs, group.ID)
	}
	taken, err := s.courseOccupancy(ctx, groupIDs, check.from, check.until, check.studentID)
	if err != nil {
		return err
	}
	for _, offering := range check.offerings {
		for _, group := range groupsByOffering[offering.ID] {
			lockedGroup, ok := lockedByID[group.ID]
			if !ok {
				return fmt.Errorf("%w: %s", careplan.ErrOfferingChangeInvalid, offering.Name)
			}
			if lockedGroup.ParticipantLimit != nil && taken[lockedGroup.ID] >= *lockedGroup.ParticipantLimit {
				return fmt.Errorf("%w: %s", careplan.ErrOfferingChangeCapacityFull, offering.Name)
			}
		}
	}
	return nil
}

// assertCourseRequestQueuePosition keeps a seat for the oldest pending
// request. It runs while the shared course-group locks are held, so another
// approval cannot overtake the queue between the check and the booking.
func assertCourseRequestQueuePosition(position int) error {
	if position <= 1 {
		return nil
	}
	return fmt.Errorf("%w: an older course request is pending", careplan.ErrOfferingChangeCapacityFull)
}

// lockCourseGroups serializes every approval that can consume the same AG
// capacity. Offering locks alone do not suffice: several offerings may feed
// one group through their source templates.
func (s *OfferingChanges) lockCourseGroups(ctx context.Context, groups []ports.CourseGroup) ([]ports.CourseGroup, error) {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	slices.Sort(ids)
	locked, err := s.deps.Planning.LockCourseGroups(ctx, slices.Compact(ids))
	if err != nil {
		return nil, fmt.Errorf("course request: lock course capacity: %w", err)
	}
	return locked, nil
}

func activeCourseGroupsForTarget(projected []ports.CourseGroup, catalog *careplan.OfferingChangeCatalog) ([]ports.CourseGroup, bool) {
	groups := make([]ports.CourseGroup, 0, len(projected))
	hadCourseTarget := false
	seen := make(map[int64]bool, len(projected))
	for _, group := range projected {
		if seen[group.ID] || !courseGroupMatchesTarget(group, catalog) {
			continue
		}
		seen[group.ID] = true
		hadCourseTarget = true
		if group.Active {
			groups = append(groups, group)
		}
	}
	return groups, hadCourseTarget
}
