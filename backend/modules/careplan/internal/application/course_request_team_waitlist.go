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

// courseWaitlistHorizonDays bounds the occupancy window of the team's view.
// The peak over the coming year counts every seat already promised to a
// child, also one that starts later, so a free place is never shown while
// an approval would still find the AG full.
const courseWaitlistHorizonDays = 366

// teamCourseQueue is every open request with the facts the rank rule reads:
// the request children, what they already hold and the AGs each requested
// offering feeds.
type teamCourseQueue struct {
	rows             []careplan.OfferingChangeRequest
	childrenByID     map[int64]*ports.OfferingChangeChild
	bookedByChild    map[int64]map[int64]bool
	groupsByOffering map[int64][]ports.CourseGroup
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
	if activityGroupID > 0 && entries[activityGroupID] == nil {
		groupIDs = append(groupIDs, activityGroupID)
	}
	slices.Sort(groupIDs)
	return s.courseWaitlistsFor(ctx, groupIDs, entries, today)
}

func (s *OfferingChanges) loadTeamCourseQueue(ctx context.Context, rows []careplan.OfferingChangeRequest, today calendar.Date) (*teamCourseQueue, error) {
	queue := &teamCourseQueue{
		rows: rows, childrenByID: map[int64]*ports.OfferingChangeChild{},
		bookedByChild: map[int64]map[int64]bool{}, groupsByOffering: map[int64][]ports.CourseGroup{},
	}
	if len(rows) == 0 {
		return queue, nil
	}
	dates := make(map[int64]calendar.Date, len(rows))
	childIDs := make([]int64, 0, len(rows))
	offeringIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.RequestChildID <= 0 {
			continue
		}
		dates[row.RequestChildID] = offeringChangeEffectiveFrom(row)
		childIDs = append(childIDs, row.RequestChildID)
		selections, err := requestSelections(row)
		if err != nil {
			continue
		}
		for _, selected := range selections {
			offeringIDs = append(offeringIDs, selected.OfferingID)
		}
	}
	if err := s.loadTeamQueueChildren(ctx, queue, childIDs, dates); err != nil {
		return nil, err
	}
	slices.Sort(offeringIDs)
	if err := s.loadTeamQueueGroups(ctx, queue, slices.Compact(offeringIDs), today); err != nil {
		return nil, err
	}
	return queue, nil
}

func (s *OfferingChanges) loadTeamQueueChildren(ctx context.Context, queue *teamCourseQueue, childIDs []int64, dates map[int64]calendar.Date) error {
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
	return nil
}

func (s *OfferingChanges) loadTeamQueueGroups(ctx context.Context, queue *teamCourseQueue, offeringIDs []int64, today calendar.Date) error {
	if len(offeringIDs) == 0 {
		return nil
	}
	offerings, err := s.offeringsByIDs(ctx, offeringIDs)
	if err != nil {
		return fmt.Errorf("course waitlist: load requested offerings: %w", err)
	}
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
			if group.Active && (activityGroupID <= 0 || group.ID == activityGroupID) {
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

// courseWaitlistsFor reads the AGs' names, limits and occupancy and joins
// them with their queues, sorted by name. An id that names no active AG of
// the school is left out.
func (s *OfferingChanges) courseWaitlistsFor(
	ctx context.Context,
	groupIDs []int64,
	entries map[int64][]careplan.CourseWaitlistEntry,
	today calendar.Date,
) ([]careplan.CourseWaitlist, error) {
	result := []careplan.CourseWaitlist{}
	if len(groupIDs) == 0 {
		return result, nil
	}
	groups, err := s.deps.Planning.CourseGroupsByID(ctx, groupIDs, today)
	if err != nil {
		return nil, fmt.Errorf("course waitlist: read course groups: %w", err)
	}
	activeIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group.Active {
			activeIDs = append(activeIDs, group.ID)
		}
	}
	taken, err := s.courseOccupancy(ctx, activeIDs, today, today.AddDays(courseWaitlistHorizonDays), 0)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(groups))
	for _, group := range groups {
		if !group.Active || seen[group.ID] {
			continue
		}
		seen[group.ID] = true
		capacity, free := effectiveCourseCapacity(group.ParticipantLimit, taken[group.ID], nil, nil)
		queue := entries[group.ID]
		if queue == nil {
			queue = []careplan.CourseWaitlistEntry{}
		}
		result = append(result, careplan.CourseWaitlist{
			ActivityGroupID: group.ID, Name: group.Name, ParticipantLimit: capacity,
			Participants: taken[group.ID], FreeSlots: free, Entries: queue,
		})
	}
	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}
