package groups

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

// SupervisionSources are the owner reads behind who supervises a group on
// a day. The teacher and staff rows are School Membership's, the
// substitutions Workforce's (#2667, #2688).
type SupervisionSources struct {
	// TeacherStaffIDs maps every given teacher that is still live to its
	// staff member. A teacher that is gone is absent.
	TeacherStaffIDs func(ctx context.Context, teacherIDs []int64) (map[int64]int64, error)
	// LiveStaffIDs reports the given staff members that are still live.
	LiveStaffIDs func(ctx context.Context, staffIDs []int64) (map[int64]bool, error)
	// GroupSubstitutes lists who substitutes in the groups on the day
	// (YYYY-MM-DD), in query order.
	GroupSubstitutes func(ctx context.Context, groupIDs []int64, on string) ([]domain.StaffGroupID, error)
}

// SupervisingStaff decides who supervises a group on a day: the assigned
// teachers through their live teacher and staff rows, plus the substituting
// staff that are still live. Offboarded teachers and staff drop out, as the
// replaced inner joins did. Without the School Membership reads it resolves
// nothing: the group repository then reports the missing binding.
func SupervisingStaff(sources SupervisionSources) func(context.Context, domain.GroupMembershipPairs) ([]domain.StaffGroupID, error) {
	if sources.TeacherStaffIDs == nil || sources.LiveStaffIDs == nil {
		return nil
	}
	return func(ctx context.Context, raw domain.GroupMembershipPairs) ([]domain.StaffGroupID, error) {
		staffByTeacher, err := teacherStaff(ctx, sources, raw.Assigned)
		if err != nil {
			return nil, err
		}
		substituted, liveSubstitutes, err := liveSubstitutions(ctx, sources, raw)
		if err != nil {
			return nil, err
		}
		return supervisionPairs(raw.Assigned, staffByTeacher, substituted, liveSubstitutes), nil
	}
}

// teacherStaff maps every live teacher of the assignments to their live
// staff member. A teacher or staff row that is gone yields no entry.
func teacherStaff(ctx context.Context, sources SupervisionSources, assigned []domain.TeacherGroupID) (map[int64]int64, error) {
	result := make(map[int64]int64, len(assigned))
	ids := make([]int64, 0, len(assigned))
	for _, pair := range assigned {
		ids = append(ids, pair.TeacherID)
	}
	ids = positiveUniqueIDs(ids)
	if len(ids) == 0 {
		return result, nil
	}
	teachers, err := sources.TeacherStaffIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load teachers of group assignments: %w", err)
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, staffID := range teachers {
		staffIDs = append(staffIDs, staffID)
	}
	live, err := sources.LiveStaffIDs(ctx, staffIDs)
	if err != nil {
		return nil, err
	}
	for teacherID, staffID := range teachers {
		if live[staffID] {
			result[teacherID] = staffID
		}
	}
	return result, nil
}

// liveSubstitutions loads the substitutions of the groups on the day and the
// substituting staff members that are still live.
func liveSubstitutions(ctx context.Context, sources SupervisionSources, raw domain.GroupMembershipPairs) ([]domain.StaffGroupID, map[int64]bool, error) {
	if sources.GroupSubstitutes == nil {
		return nil, nil, fmt.Errorf("group supervision resolves substitutions through Workforce")
	}
	substituted := []domain.StaffGroupID{}
	if len(raw.GroupIDs) > 0 {
		var err error
		substituted, err = sources.GroupSubstitutes(ctx, raw.GroupIDs, raw.On.String())
		if err != nil {
			return nil, nil, fmt.Errorf("load substitutions of group supervision: %w", err)
		}
	}
	substituteIDs := make([]int64, 0, len(substituted))
	for _, pair := range substituted {
		substituteIDs = append(substituteIDs, pair.StaffID)
	}
	live, err := sources.LiveStaffIDs(ctx, substituteIDs)
	if err != nil {
		return nil, nil, err
	}
	return substituted, live, nil
}

// supervisionPairs lists the assignments first, then the substitutions, each
// in query order and without duplicates. An assignment whose teacher has no
// live staff member and a substitute who is gone drop out.
func supervisionPairs(assigned []domain.TeacherGroupID, staffByTeacher map[int64]int64, substituted []domain.StaffGroupID, liveSubstitutes map[int64]bool) []domain.StaffGroupID {
	candidates := make([]domain.StaffGroupID, 0, len(assigned)+len(substituted))
	for _, pair := range assigned {
		if staffID, found := staffByTeacher[pair.TeacherID]; found {
			candidates = append(candidates, domain.StaffGroupID{StaffID: staffID, GroupID: pair.GroupID})
		}
	}
	for _, pair := range substituted {
		if liveSubstitutes[pair.StaffID] {
			candidates = append(candidates, pair)
		}
	}
	result := make([]domain.StaffGroupID, 0, len(candidates))
	seen := make(map[domain.StaffGroupID]struct{}, len(candidates))
	for _, pair := range candidates {
		if _, dup := seen[pair]; dup {
			continue
		}
		seen[pair] = struct{}{}
		result = append(result, pair)
	}
	return result
}

// positiveUniqueIDs keeps every positive ID once, in first-seen order.
func positiveUniqueIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}
