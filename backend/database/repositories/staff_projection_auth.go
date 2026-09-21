package repositories

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// caregiverChainsFromMembership is the caregiver half of the account
// listings, served from the School Membership capability instead of the auth
// repository's former staff/teacher join.
func caregiverChainsFromMembership(membership staffLookup) caregiverChainQuery {
	return membershipCaregiverChains{membership: membership}
}

type membershipCaregiverChains struct {
	membership staffLookup
}

// CaregiverChainByPersonIDs returns the live staff record (and its live
// teacher record, when present) behind each person, keyed by person ID.
// Persons without a live staff record are absent from the result; a person
// with several staff rows keeps the one with the lowest staff ID, as the
// former "ORDER BY person_id ASC, staff id ASC" query did.
func (r membershipCaregiverChains) CaregiverChainByPersonIDs(ctx context.Context, personIDs []int64) (map[int64]CaregiverChain, error) {
	result := make(map[int64]CaregiverChain, len(personIDs))
	if len(personIDs) == 0 {
		return result, nil
	}
	members, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs, MembershipOnly: true})
	if err != nil {
		return nil, fmt.Errorf("load staff for caregiver chains: %w", err)
	}
	if len(members) == 0 {
		return result, nil
	}
	staffIDs := make([]int64, 0, len(members))
	for _, member := range members {
		staffIDs = append(staffIDs, member.ID)
	}
	teachers, err := r.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{StaffIDs: staffIDs})
	if err != nil {
		return nil, fmt.Errorf("load teachers for caregiver chains: %w", err)
	}
	teacherByStaff := make(map[int64]schoolmembership.Teacher, len(teachers))
	for _, teacher := range teachers {
		if current, found := teacherByStaff[teacher.StaffID]; found && current.ID <= teacher.ID {
			continue
		}
		teacherByStaff[teacher.StaffID] = teacher
	}
	// ListStaff sorts by ID, so the first row per person is the lowest one.
	for _, member := range members {
		if _, found := result[member.PersonID]; found {
			continue
		}
		chain := CaregiverChain{
			PersonID: member.PersonID,
			TenantID: member.TenantID,
			StaffID:  member.ID,
		}
		if teacher, found := teacherByStaff[member.ID]; found {
			chain.TeacherID = teacher.ID
			chain.TeacherRole = teacher.Role
		}
		result[member.PersonID] = chain
	}
	return result, nil
}
