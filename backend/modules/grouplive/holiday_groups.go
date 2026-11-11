package grouplive

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// A Feriengruppe before its week (ADR 0120, Nachtrag 2026-11-11). The
// school picks one of its groups for the holiday guests of a hosted week;
// the guests join it only from the first day of the week and wait in no
// group before (Nachtrag 2026-11-03). A group made for the week therefore
// stands empty in the daily lists for weeks, with no explanation. The group
// lists keep such a group back while it waits: it is the Feriengruppe of a
// week that starts after today, of no week running today, and no child of
// it takes part in care today. A group with its own children in care, a
// group whose week has begun and a group the caller asks for by ID stay.
// After its last week the group is an ordinary group again. Group
// management reads the groups elsewhere and always lists it.

// awaitingHolidayGroupIDs names the groups whose Feriengruppe weeks all
// start after today. The weeks end today or later, so a week that has begun
// is running.
func awaitingHolidayGroupIDs(weeks []HolidayGroupWeek, today Date) map[int64]bool {
	awaiting := map[int64]bool{}
	running := map[int64]bool{}
	for _, week := range weeks {
		if week.StartsOn > today {
			awaiting[week.GroupID] = true
		} else if week.EndsOn >= today {
			running[week.GroupID] = true
		}
	}
	for id := range running {
		delete(awaiting, id)
	}
	return awaiting
}

// withoutAwaitingHolidayGroups drops the waiting Feriengruppen from the
// groups, except the requested one. It reads the Feriengruppen once and the
// care of their children only when one of the groups waits, so the list
// costs the same statements for any number of groups.
func (s *service) withoutAwaitingHolidayGroups(ctx context.Context, groups []GroupRecord, requestedGroupID int64) ([]GroupRecord, error) {
	if s.deps.HolidayGroups == nil || len(groups) == 0 {
		return groups, nil
	}
	if s.deps.Roster == nil || s.deps.Calendar == nil {
		return nil, errors.New("OGS group live service is not fully configured")
	}
	weeks, err := s.deps.HolidayGroups.HolidayGroupWeeks(ctx)
	if err != nil {
		return nil, fmt.Errorf("load holiday groups: %w", err)
	}
	today := s.deps.Calendar.Today()
	awaiting := awaitingHolidayGroupIDs(weeks, today)
	candidates := make([]int64, 0, len(awaiting))
	for _, group := range groups {
		if awaiting[group.ID] && group.ID != requestedGroupID {
			candidates = append(candidates, group.ID)
		}
	}
	if len(candidates) == 0 {
		return groups, nil
	}
	inCare, err := s.groupsWithChildrenInCare(ctx, candidates, today)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(slices.Clone(groups), func(group GroupRecord) bool {
		return slices.Contains(candidates, group.ID) && !inCare[group.ID]
	}), nil
}

// groupsWithChildrenInCare names the groups with a child that takes part in
// care on the day, by the rule of the group's own roster.
func (s *service) groupsWithChildrenInCare(ctx context.Context, groupIDs []int64, today Date) (map[int64]bool, error) {
	members, err := s.deps.Roster.GroupMemberIDs(ctx, groupIDs)
	if err != nil {
		return nil, fmt.Errorf("load holiday group members: %w", err)
	}
	var studentIDs []int64
	for _, ids := range members {
		studentIDs = append(studentIDs, ids...)
	}
	inCare := map[int64]bool{}
	if len(studentIDs) == 0 {
		return inCare, nil
	}
	participants, err := s.deps.Roster.CareParticipants(ctx, studentIDs, today)
	if err != nil {
		return nil, fmt.Errorf("load holiday group care: %w", err)
	}
	for groupID, ids := range members {
		inCare[groupID] = slices.ContainsFunc(ids, func(id int64) bool { return participants[id] })
	}
	return inCare, nil
}
