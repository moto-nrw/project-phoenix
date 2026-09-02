package repositories

import (
	"context"
	"fmt"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// staffAccountTenantRepository serves the caregiver half of the account
// listings from School Membership. The auth repository used to answer it
// with a staff/teacher join of its own (#2667); the person projection above
// still type-asserts caregiverChainQuery on whatever it wraps, so the
// capability is attached here, below it.
type staffAccountTenantRepository struct {
	authModels.AccountTenantRepository
	rows   accountTenantSchoolRowsQuery
	chains caregiverChainQuery
}

func newStaffAccountTenantRepository(inner authModels.AccountTenantRepository, membership staffLookup) authModels.AccountTenantRepository {
	rows, _ := inner.(accountTenantSchoolRowsQuery)
	return staffAccountTenantRepository{
		AccountTenantRepository: inner,
		rows:                    rows,
		chains:                  caregiverChainsFromMembership(membership),
	}
}

func (r staffAccountTenantRepository) CaregiverChainByPersonIDs(ctx context.Context, personIDs []int64) (map[int64]authModels.CaregiverChain, error) {
	return r.chains.CaregiverChainByPersonIDs(ctx, personIDs)
}

// ListAccountsBySchoolIDs keeps the raw school-set listing reachable for the
// person and school projections stacked above this decorator.
func (r staffAccountTenantRepository) ListAccountsBySchoolIDs(ctx context.Context, schoolIDs []int64) ([]authModels.OrgAccountInfo, error) {
	if r.rows == nil {
		return nil, fmt.Errorf("account tenant repository does not list accounts by school")
	}
	return r.rows.ListAccountsBySchoolIDs(ctx, schoolIDs)
}

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
func (r membershipCaregiverChains) CaregiverChainByPersonIDs(ctx context.Context, personIDs []int64) (map[int64]authModels.CaregiverChain, error) {
	result := make(map[int64]authModels.CaregiverChain, len(personIDs))
	if len(personIDs) == 0 {
		return result, nil
	}
	members, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs})
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
		chain := authModels.CaregiverChain{
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
