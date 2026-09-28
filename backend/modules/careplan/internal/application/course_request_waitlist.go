package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// courseWaitlist is the queue of open requests a course's waiting position
// is counted in: the requests submitted up to the child's own, the children
// behind them and what those children already hold.
type courseWaitlist struct {
	rows          []careplan.OfferingChangeRequest
	pending       careplan.OfferingChangeRequest
	childrenByID  map[int64]*ports.OfferingChangeChild
	bookedByChild map[int64]map[int64]bool
}

// courseWaitlistPosition is the child's rank in the queue for one course:
// how many open requests for the same target group were submitted before
// this one. The child's own request counts, so the family that asked first
// reads "Platz 1".
func (s *OfferingChanges) courseWaitlistPosition(
	ctx context.Context,
	catalog *careplan.OfferingChangeCatalog,
	targetGroups []ports.CourseGroup,
	groupsByOffering map[int64][]ports.CourseGroup,
	pending careplan.OfferingChangeRequest,
) (int, error) {
	rows, err := s.deps.Rows.ListPending(ctx)
	if err != nil {
		return 0, fmt.Errorf("course request: list pending for waitlist: %w", err)
	}
	queue := &courseWaitlist{rows: rows, pending: pending}
	dates, childIDs := queue.earlierRequestDates()
	if err := s.loadWaitlistChildren(ctx, queue, targetGroups, childIDs); err != nil {
		return 0, err
	}
	if err := s.loadWaitlistBookings(ctx, queue, dates); err != nil {
		return 0, err
	}
	competing, err := s.courseGroupsForCompetingRequests(ctx, queue, catalog, groupsByOffering)
	if err != nil {
		return 0, err
	}
	return queue.position(targetGroups, competing), nil
}

// earlierRequestDates lists the request children queued ahead of the
// child's own request with their effective dates, plus the child's own.
func (q *courseWaitlist) earlierRequestDates() (map[int64]calendar.Date, []int64) {
	dates := make(map[int64]calendar.Date, len(q.rows))
	childIDs := make([]int64, 0, len(q.rows)+1)
	for _, row := range q.rows {
		if row.ID != q.pending.ID && !courseRequestAfter(row, q.pending) && row.RequestChildID > 0 {
			dates[row.RequestChildID] = offeringChangeEffectiveFrom(row)
			childIDs = append(childIDs, row.RequestChildID)
		}
	}
	if q.pending.RequestChildID > 0 {
		childIDs = append(childIDs, q.pending.RequestChildID)
	}
	slices.Sort(childIDs)
	return dates, slices.Compact(childIDs)
}

// loadWaitlistChildren reads the queued children only when the course
// narrows its audience to a grade or class.
func (s *OfferingChanges) loadWaitlistChildren(ctx context.Context, queue *courseWaitlist, targetGroups []ports.CourseGroup, childIDs []int64) error {
	queue.childrenByID = make(map[int64]*ports.OfferingChangeChild)
	if !courseGroupsHaveTargets(targetGroups) {
		return nil
	}
	children, err := s.deps.Enrollment.Children(ctx, childIDs)
	if err != nil {
		return fmt.Errorf("course request: load waitlist children: %w", err)
	}
	for i := range children {
		queue.childrenByID[children[i].ID] = &children[i]
	}
	return nil
}

func (s *OfferingChanges) loadWaitlistBookings(ctx context.Context, queue *courseWaitlist, dates map[int64]calendar.Date) error {
	queue.bookedByChild = make(map[int64]map[int64]bool, len(dates))
	if len(dates) == 0 {
		return nil
	}
	current, err := s.deps.Enrollment.EffectiveSelectionsAt(ctx, dates)
	if err != nil {
		return fmt.Errorf("course request: load current offerings for waitlist: %w", err)
	}
	for _, link := range current {
		if queue.bookedByChild[link.RequestChildID] == nil {
			queue.bookedByChild[link.RequestChildID] = make(map[int64]bool)
		}
		queue.bookedByChild[link.RequestChildID][link.CareOfferingID] = true
	}
	return nil
}

// courseGroupsForCompetingRequests fills the gaps the current child's catalog
// leaves: an earlier request can name an offering that has since become
// unavailable or is filtered for this child, while still feeding the AG.
func (s *OfferingChanges) courseGroupsForCompetingRequests(
	ctx context.Context,
	queue *courseWaitlist,
	catalog *careplan.OfferingChangeCatalog,
	knownGroups map[int64][]ports.CourseGroup,
) (map[int64][]ports.CourseGroup, error) {
	groupsByOffering := make(map[int64][]ports.CourseGroup, len(knownGroups))
	for offeringID, groups := range knownGroups {
		groupsByOffering[offeringID] = groups
	}
	if catalog != nil {
		for _, item := range catalog.Items {
			if _, known := groupsByOffering[item.OfferingID]; !known {
				// The catalog query already established that this offering
				// does not reach a course of the child's target.
				groupsByOffering[item.OfferingID] = nil
			}
		}
	}
	missing := queue.unknownOfferingIDs(groupsByOffering)
	if len(missing) == 0 {
		return groupsByOffering, nil
	}
	offerings, err := s.offeringsByIDs(ctx, missing)
	if err != nil {
		return nil, fmt.Errorf("course request: load competing waitlist offerings: %w", err)
	}
	references := make([]ports.CourseOfferingReference, 0, len(offerings))
	for _, offering := range offerings {
		references = append(references, ports.CourseOfferingReference{OfferingID: offering.ID, ActivityGroupID: offering.ActivityGroupID})
	}
	additionalGroups, err := s.deps.Planning.CourseGroupsForOfferings(ctx, references, calendar.TodayDate())
	if err != nil {
		return nil, fmt.Errorf("course request: load competing waitlist course groups: %w", err)
	}
	for offeringID, groups := range additionalGroups {
		groupsByOffering[offeringID] = groups
	}
	return groupsByOffering, nil
}

// unknownOfferingIDs lists the offerings the queued requests ask for whose
// course groups are not known yet. A malformed payload is surfaced by the
// review queue; it cannot establish a waiting position here.
func (q *courseWaitlist) unknownOfferingIDs(groupsByOffering map[int64][]ports.CourseGroup) []int64 {
	missing := make([]int64, 0)
	for _, row := range q.rows {
		if courseRequestAfter(row, q.pending) {
			continue
		}
		selections, err := requestSelections(row)
		if err != nil {
			continue
		}
		for _, selected := range selections {
			if _, known := groupsByOffering[selected.OfferingID]; !known {
				missing = append(missing, selected.OfferingID)
			}
		}
	}
	slices.Sort(missing)
	return slices.Compact(missing)
}

// position counts the queued course requests for the target groups. A
// payload that cannot be read is no evidence of a competing request.
func (q *courseWaitlist) position(targetGroups []ports.CourseGroup, groupsByOffering map[int64][]ports.CourseGroup) int {
	targetGroupIDs := make(map[int64]bool, len(targetGroups))
	for _, group := range targetGroups {
		if group.Active {
			targetGroupIDs[group.ID] = true
		}
	}
	position := 0
	for _, row := range q.rows {
		if courseRequestAfter(row, q.pending) || !courseGroupsMatchTarget(targetGroups, q.childrenByID[row.RequestChildID]) {
			continue
		}
		added, err := courseWasAddedForGroups(row, targetGroupIDs, groupsByOffering, q.bookedByChild[row.RequestChildID])
		if err == nil && added {
			position++
		}
	}
	return max(position, 1)
}

func courseRequestAfter(row, pending careplan.OfferingChangeRequest) bool {
	return row.CreatedAt.After(pending.CreatedAt) || (row.CreatedAt.Equal(pending.CreatedAt) && row.ID > pending.ID)
}

func courseGroupsMatchTarget(groups []ports.CourseGroup, child *ports.OfferingChangeChild) bool {
	if !courseGroupsHaveTargets(groups) {
		return true
	}
	if child == nil {
		return false
	}
	catalog := courseTargetCatalog(child)
	return slices.ContainsFunc(groups, func(group ports.CourseGroup) bool {
		return group.Active && courseGroupMatchesTarget(group, catalog)
	})
}

func courseGroupsHaveTargets(groups []ports.CourseGroup) bool {
	return slices.ContainsFunc(groups, func(group ports.CourseGroup) bool {
		return len(group.SourceGradeLevels) > 0 || len(group.SourceSchoolClasses) > 0
	})
}

// courseWasAddedForGroups tells a course request from another offering
// change whose complete selection merely retains a booked course. The same
// course group can be reached through more than one offering.
func courseWasAddedForGroups(
	row careplan.OfferingChangeRequest,
	targetGroupIDs map[int64]bool,
	groupsByOffering map[int64][]ports.CourseGroup,
	booked map[int64]bool,
) (bool, error) {
	selections, err := requestSelections(row)
	if err != nil {
		return false, err
	}
	for _, selected := range selections {
		if booked[selected.OfferingID] {
			continue
		}
		if slices.ContainsFunc(groupsByOffering[selected.OfferingID], func(group ports.CourseGroup) bool {
			return group.Active && targetGroupIDs[group.ID]
		}) {
			return true, nil
		}
	}
	return false, nil
}
