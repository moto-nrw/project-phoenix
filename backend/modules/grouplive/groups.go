package grouplive

import (
	"context"
	"fmt"
	"slices"
)

func (s *service) ListGroups(ctx context.Context) ([]Group, error) {
	if err := s.validateGroupDependencies(); err != nil {
		return nil, err
	}
	caller, err := s.deps.Access.Caller(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve caller access: %w", err)
	}
	groups, _, err := s.resolveGroups(ctx, caller, 0, true)
	if err != nil {
		return nil, err
	}
	s.attachLeaders(ctx, groups)
	return groups, nil
}

// attachLeaders names each group's Gruppenleitung with one bulk lookup. Like
// the room names, a failed lookup degrades to groups without leadership: the
// navigation must not fail over a display line.
func (s *service) attachLeaders(ctx context.Context, groups []Group) {
	if len(groups) == 0 {
		return
	}
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	leaders, err := s.deps.Groups.GroupLeaderNames(ctx, ids)
	if err != nil {
		s.deps.Logger.Warn("load group leaders failed", "error", err)
		return
	}
	for i := range groups {
		names := slices.Clone(leaders[groups[i].ID])
		if len(names) == 0 {
			continue
		}
		if s.deps.Collate != nil {
			slices.SortStableFunc(names, s.deps.Collate)
		}
		groups[i].Leaders = names
	}
}

// sortGroups orders groups by name with the injected collation; the sort is
// stable, so groups with equal names keep the directory order.
func (s *service) sortGroups(groups []GroupRecord) {
	if s.deps.Collate == nil {
		return
	}
	slices.SortStableFunc(groups, func(a, b GroupRecord) int { return s.deps.Collate(a.Name, b.Name) })
}

func (s *service) resolveGroups(ctx context.Context, caller Caller, requestedGroupID int64, allowMissingRoomNames bool) ([]Group, *GroupRecord, error) {
	myGroups, err := s.deps.Groups.SupervisedGroups(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load supervised groups: %w", err)
	}
	s.sortGroups(myGroups)
	personalIDs := make(map[int64]bool, len(myGroups))
	for _, group := range myGroups {
		personalIDs[group.ID] = true
	}

	visibleGroups := myGroups
	if caller.OperationalOverview && caller.CanReadGroups {
		visibleGroups, err = s.deps.Groups.TenantGroups(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("load tenant groups: %w", err)
		}
		s.sortGroups(visibleGroups)
	}

	if len(visibleGroups) == 0 {
		if requestedGroupID > 0 {
			return nil, nil, ErrForbiddenGroup
		}
		return nil, nil, nil
	}

	selected := selectGroup(visibleGroups, personalIDs, requestedGroupID)
	if selected == nil {
		return nil, nil, ErrForbiddenGroup
	}

	substituted, err := s.deps.Groups.SubstitutedGroupIDs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load substitution metadata: %w", err)
	}
	groups, err := s.mapGroups(ctx, visibleGroups, personalIDs, substituted, allowMissingRoomNames)
	if err != nil {
		return nil, nil, err
	}
	return groups, selected, nil
}

// selectGroup picks the requested group from the directory order, or the
// first personal group (falling back to the first visible one) when no group
// was requested. The directory delivers groups in German dictionary order.
func selectGroup(groups []GroupRecord, personalIDs map[int64]bool, requestedGroupID int64) *GroupRecord {
	if requestedGroupID <= 0 {
		for i := range groups {
			if personalIDs[groups[i].ID] {
				return &groups[i]
			}
		}
		return &groups[0]
	}
	for i := range groups {
		if groups[i].ID == requestedGroupID {
			return &groups[i]
		}
	}
	return nil
}

func (s *service) mapGroups(ctx context.Context, groups []GroupRecord, personalIDs, substituted map[int64]bool, allowMissingRoomNames bool) ([]Group, error) {
	// One bulk lookup resolves every room name; a per-group lookup loop would
	// run on every aggregate refresh including SSE revalidations.
	roomGroupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group.RoomID != nil {
			roomGroupIDs = append(roomGroupIDs, group.ID)
		}
	}
	roomNames := map[int64]string{}
	if len(roomGroupIDs) > 0 {
		loaded, err := s.deps.Groups.GroupRoomNames(ctx, roomGroupIDs)
		if err != nil {
			if !allowMissingRoomNames {
				return nil, fmt.Errorf("load group rooms: %w", err)
			}
			s.deps.Logger.Warn("load group rooms failed",
				"error", err,
			)
		} else {
			roomNames = loaded
		}
	}

	result := make([]Group, 0, len(groups))
	for _, group := range groups {
		result = append(result, Group{
			ID: group.ID, Name: group.Name, RoomID: group.RoomID, RoomName: roomNames[group.ID],
			ViaSubstitution: substituted[group.ID], IsPersonal: personalIDs[group.ID],
		})
	}
	return result, nil
}
