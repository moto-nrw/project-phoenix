package repositories

import (
	"context"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// substitutedStaffQuery answers who substitutes in the given groups on a
// day. education.group_substitution belongs to Workforce (#2688); the root
// supplies the owner's query so the group repository never joins the table.
type substitutedStaffQuery = func(ctx context.Context, groupIDs []int64, on string) ([]educationRepo.StaffGroupID, error)

// supervisionStaffResolver resolves who supervises a group on a day: the
// assigned teachers through their live teacher and staff rows, plus the
// substituting staff that are still live. Offboarded teachers and staff drop
// out, as the replaced inner joins did.
func supervisionStaffResolver(membership staffLookup, substitutions substitutedStaffQuery) func(context.Context, educationRepo.GroupMembershipPairs) ([]educationRepo.StaffGroupID, error) {
	return func(ctx context.Context, raw educationRepo.GroupMembershipPairs) ([]educationRepo.StaffGroupID, error) {
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
		result := make([]educationRepo.StaffGroupID, 0, len(raw.Assigned)+len(substituted))
		seen := make(map[educationRepo.StaffGroupID]struct{}, cap(result))
		appendPair := func(pair educationRepo.StaffGroupID) {
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
			appendPair(educationRepo.StaffGroupID{StaffID: staffID, GroupID: pair.GroupID})
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
	return func(ctx context.Context, groupIDs []int64, on string) ([]educationRepo.StaffGroupID, error) {
		if len(groupIDs) == 0 {
			return []educationRepo.StaffGroupID{}, nil
		}
		rows, err := capability.ListGroupSubstitutions(ctx, workforce.GroupSubstitutionFilter{GroupIDs: groupIDs, On: on})
		if err != nil {
			return nil, err
		}
		pairs := make([]educationRepo.StaffGroupID, 0, len(rows))
		for _, row := range rows {
			pairs = append(pairs, educationRepo.StaffGroupID{StaffID: row.SubstituteStaffID, GroupID: row.GroupID})
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

// groupSubstitutions serves the retained group substitution contract over
// the Workforce rows. The reads with relations attach the group through
// School Structure and the staff members through School Membership; the
// Workforce adapter names neither (#3556).
type groupSubstitutions struct {
	GroupSubstitutionRows
	groups     func(ctx context.Context, ids []int64) (map[int64]*educationRepo.Group, error)
	membership staffLookup
}

func newGroupSubstitutions(rows GroupSubstitutionRows, groups func(context.Context, []int64) (map[int64]*educationRepo.Group, error), membership staffLookup) groupSubstitutions {
	return groupSubstitutions{GroupSubstitutionRows: rows, groups: groups, membership: membership}
}

// FindGroupHandovers lists the substitutions of one group as School
// Structure values, the group service's deletion guard.
func (r groupSubstitutions) FindGroupHandovers(ctx context.Context, groupID int64) ([]*educationRepo.GroupSubstitution, error) {
	return educationHandovers(r.FindByGroup(ctx, groupID))
}

// ListWithRelations resolves the groups and then the staff members. Soft-
// deleted staff are included so historical substitutions keep resolving
// after offboarding, as the replaced WhereAllWithDeleted lookup did.
func (r groupSubstitutions) ListWithRelations(ctx context.Context, options *userModels.QueryOptions) ([]*educationRepo.GroupSubstitution, error) {
	rows, err := educationHandovers(r.ListWithOptions(ctx, options))
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	groupIDs := make([]int64, 0, len(rows))
	staffIDs := make([]int64, 0, 2*len(rows))
	for _, row := range rows {
		if row.GroupID > 0 {
			groupIDs = append(groupIDs, row.GroupID)
		}
		staffIDs = append(staffIDs, row.SubstituteStaffID)
		if row.RegularStaffID != nil {
			staffIDs = append(staffIDs, *row.RegularStaffID)
		}
	}
	if groupIDs = uniqueIDs(groupIDs); len(groupIDs) > 0 {
		groups, err := r.groups(ctx, groupIDs)
		if err != nil {
			return nil, &userModels.DatabaseError{Op: "load substitution groups", Err: err}
		}
		for _, row := range rows {
			row.Group = groups[row.GroupID]
		}
	}
	members, err := staffByID(ctx, r.membership, staffIDs, true)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if member, found := members[row.SubstituteStaffID]; found {
			row.SubstituteStaff = substitutionStaff(member)
		}
		if row.RegularStaffID == nil {
			continue
		}
		if member, found := members[*row.RegularStaffID]; found {
			row.RegularStaff = substitutionStaff(member)
		}
	}
	return rows, nil
}

// substitutionStaff is the staff reference a substitution row carries; the
// name follows from the People Directory where a reader needs it.
func substitutionStaff(value schoolmembership.Staff) *educationRepo.SubstitutionStaff {
	return &educationRepo.SubstitutionStaff{ID: value.ID, PersonID: value.PersonID}
}
