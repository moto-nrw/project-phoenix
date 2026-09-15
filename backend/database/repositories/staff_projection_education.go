package repositories

import (
	"context"
	"fmt"

	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// supervisionStaffResolverSetter is the seam the group repository exposes
// for its staff lookup. It is declared with a plain function type so this
// package needs neither the calendar-date nor the query-option vocabulary
// of the repository layer.
type supervisionStaffResolverSetter interface {
	SetSupervisionStaffResolver(func(context.Context, educationRepo.GroupMembershipPairs) ([]educationModels.StaffGroupID, error))
}

// substitutedStaffQuery answers who substitutes in the given groups on a
// day. education.group_substitution belongs to Workforce (#2688); the root
// supplies the owner's query so the group repository never joins the table.
type substitutedStaffQuery = func(ctx context.Context, groupIDs []int64, on string) ([]educationModels.StaffGroupID, error)

// supervisionStaffResolver resolves who supervises a group on a day: the
// assigned teachers through their live teacher and staff rows, plus the
// substituting staff that are still live. Offboarded teachers and staff drop
// out, as the replaced inner joins did.
func supervisionStaffResolver(membership staffLookup, substitutions substitutedStaffQuery) func(context.Context, educationRepo.GroupMembershipPairs) ([]educationModels.StaffGroupID, error) {
	return func(ctx context.Context, raw educationRepo.GroupMembershipPairs) ([]educationModels.StaffGroupID, error) {
		staffByTeacher, err := resolveTeacherStaff(ctx, membership, raw.Assigned)
		if err != nil {
			return nil, err
		}
		if substitutions == nil {
			return nil, fmt.Errorf("group supervision resolves substitutions through Workforce")
		}
		substituted, err := substitutions(ctx, raw.GroupIDs, raw.On.String())
		if err != nil {
			return nil, fmt.Errorf("load substitutions of group supervision: %w", err)
		}
		substituteIDs := make([]int64, 0, len(substituted))
		for _, pair := range substituted {
			substituteIDs = append(substituteIDs, pair.StaffID)
		}
		liveSubstitutes, err := staffByID(ctx, membership, substituteIDs, false)
		if err != nil {
			return nil, err
		}

		// Assignments first, then substitutions, each in query order.
		result := make([]educationModels.StaffGroupID, 0, len(raw.Assigned)+len(substituted))
		seen := make(map[educationModels.StaffGroupID]struct{}, cap(result))
		appendPair := func(pair educationModels.StaffGroupID) {
			if _, dup := seen[pair]; dup {
				return
			}
			seen[pair] = struct{}{}
			result = append(result, pair)
		}
		for _, pair := range raw.Assigned {
			staffID, found := staffByTeacher[pair.TeacherID]
			if !found {
				continue
			}
			appendPair(educationModels.StaffGroupID{StaffID: staffID, GroupID: pair.GroupID})
		}
		for _, pair := range substituted {
			if _, found := liveSubstitutes[pair.StaffID]; !found {
				continue
			}
			appendPair(pair)
		}
		return result, nil
	}
}

// workforceSubstitutedStaff adapts the Workforce capability to the
// supervision resolver's substitution query.
func workforceSubstitutedStaff(capability workforce.Capability) substitutedStaffQuery {
	if capability == nil {
		return nil
	}
	return func(ctx context.Context, groupIDs []int64, on string) ([]educationModels.StaffGroupID, error) {
		if len(groupIDs) == 0 {
			return []educationModels.StaffGroupID{}, nil
		}
		rows, err := capability.ListGroupSubstitutions(ctx, workforce.GroupSubstitutionFilter{GroupIDs: groupIDs, On: on})
		if err != nil {
			return nil, err
		}
		pairs := make([]educationModels.StaffGroupID, 0, len(rows))
		for _, row := range rows {
			pairs = append(pairs, educationModels.StaffGroupID{StaffID: row.SubstituteStaffID, GroupID: row.GroupID})
		}
		return pairs, nil
	}
}

// resolveTeacherStaff maps every live teacher of the assignments to their
// live staff member. A teacher or staff row that is gone yields no entry.
func resolveTeacherStaff(ctx context.Context, membership staffLookup, assigned []educationRepo.TeacherGroupID) (map[int64]int64, error) {
	result := make(map[int64]int64, len(assigned))
	ids := make([]int64, 0, len(assigned))
	for _, pair := range assigned {
		ids = append(ids, pair.TeacherID)
	}
	ids = uniqueIDs(ids)
	if len(ids) == 0 {
		return result, nil
	}
	teachers, err := membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("load teachers of group assignments: %w", err)
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	live, err := staffByID(ctx, membership, staffIDs, false)
	if err != nil {
		return nil, err
	}
	for _, teacher := range teachers {
		if _, found := live[teacher.StaffID]; found {
			result[teacher.ID] = teacher.StaffID
		}
	}
	return result, nil
}

// substitutionStaffResolver attaches the regular and substituting staff
// members to substitutions. Soft-deleted staff are included so historical
// substitutions keep resolving after offboarding, as the replaced
// WhereAllWithDeleted lookup did.
func substitutionStaffResolver(membership staffLookup) func(context.Context, []*educationModels.GroupSubstitution) error {
	return func(ctx context.Context, rows []*educationModels.GroupSubstitution) error {
		ids := make([]int64, 0, 2*len(rows))
		for _, row := range rows {
			if row == nil {
				continue
			}
			ids = append(ids, row.SubstituteStaffID)
			if row.RegularStaffID != nil {
				ids = append(ids, *row.RegularStaffID)
			}
		}
		members, err := staffByID(ctx, membership, ids, true)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row == nil {
				continue
			}
			if member, found := members[row.SubstituteStaffID]; found {
				row.SubstituteStaff = toLegacyStaff(member)
			}
			if row.RegularStaffID == nil {
				continue
			}
			if member, found := members[*row.RegularStaffID]; found {
				row.RegularStaff = toLegacyStaff(member)
			}
		}
		return nil
	}
}
