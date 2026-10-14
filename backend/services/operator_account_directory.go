package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// TenantAccountInfo holds flattened account data for a given tenant, used by operator dashboard.
type TenantAccountInfo struct {
	AccountID           int64
	Email               string
	Active              bool
	FirstName           string
	LastName            string
	RoleName            string
	PedagogicRole       string
	Status              string
	HasAdminRole        bool
	HasUserRole         bool
	HasCaregiverProfile bool
	IsActiveCaregiver   bool
}

// OrgAccountInfo extends TenantAccountInfo with school context for org-level listings.
type OrgAccountInfo struct {
	TenantAccountInfo
	SchoolID   int64
	SchoolName string
}

// CaregiverChain is the staff and teacher record behind one person at one
// school. The account listings combine it with the People Directory's
// person rows to derive the caregiver facts (#2661).
type CaregiverChain struct {
	PersonID    int64
	TenantID    int64
	StaffID     int64
	TeacherID   int64
	TeacherRole string
}

// OperatorAccountDirectory is the operator listing projection composed from
// identity, person, membership and school facts, with no persistence rows.
type OperatorAccountDirectory interface {
	ListAccountsByTenantID(context.Context, int64) ([]TenantAccountInfo, error)
	ListAccountsByOrganizationID(context.Context, int64) ([]OrgAccountInfo, error)
	ListAllAccounts(context.Context) ([]OrgAccountInfo, error)
}

type operatorAccountDirectory struct {
	accounts identityaccess.SchoolAccountListings
	persons  peopledirectory.Query
	chains   caregiverChainQuery
	schools  organizationtenancy.Query
}

// NewOperatorAccountDirectory reuses the serving owners; it constructs no
// repositories or modules and does not own a database connection.
func NewOperatorAccountDirectory(accounts identityaccess.SchoolAccountListings, persons peopledirectory.Query, membership staffLookup, schools organizationtenancy.Query) OperatorAccountDirectory {
	return operatorAccountDirectory{accounts: accounts, persons: persons, chains: caregiverChainsFromMembership(membership), schools: schools}
}

func (r operatorAccountDirectory) identityRows(ctx context.Context, schoolIDs []int64) ([]OrgAccountInfo, error) {
	rows, err := r.accounts.ListSchoolAccountListings(ctx, schoolIDs)
	if err != nil {
		return nil, err
	}
	result := make([]OrgAccountInfo, 0, len(rows))
	for _, row := range rows {
		result = append(result, OrgAccountInfo{SchoolID: row.SchoolID, TenantAccountInfo: TenantAccountInfo{
			AccountID: row.AccountID, Email: row.Email, Active: row.Active,
			FirstName: row.FirstName, LastName: row.LastName, RoleName: row.RoleName,
			Status: row.Status, HasAdminRole: row.HasAdminRole, HasUserRole: row.HasUserRole,
		}})
	}
	return result, nil
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
