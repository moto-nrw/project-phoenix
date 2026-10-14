package services

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
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

// caregiverChainQuery supplies the School Membership facts used by the
// operator listing projection.
type caregiverChainQuery interface {
	CaregiverChainByPersonIDs(context.Context, []int64) (map[int64]CaregiverChain, error)
}

type tenantAccountKey struct {
	AccountID int64
	TenantID  int64
}

// personsByAccountAndTenant resolves persons for account ids keyed by
// (account, tenant), keeping the most recently updated person per pair.
func personsByAccountAndTenant(ctx context.Context, query peopledirectory.Query, accountIDs []int64) (map[tenantAccountKey]peopledirectory.Person, error) {
	values, err := query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	result := make(map[tenantAccountKey]peopledirectory.Person, len(values))
	for _, value := range values {
		if value.AccountID == nil {
			continue
		}
		key := tenantAccountKey{AccountID: *value.AccountID, TenantID: value.TenantID}
		current, found := result[key]
		if !found || value.UpdatedAt.After(current.UpdatedAt) {
			result[key] = value
		}
	}
	return result, nil
}

// accountEntry is one account row under enrichment together with the
// tenant it belongs to and whether a person backs it.
type accountEntry struct {
	TenantID  int64
	HasPerson bool
	Info      TenantAccountInfo
}

func (r operatorAccountDirectory) ListAccountsByTenantID(ctx context.Context, tenantID int64) ([]TenantAccountInfo, error) {
	schoolRows, err := r.identityRows(ctx, []int64{tenantID})
	rows := make([]TenantAccountInfo, 0, len(schoolRows))
	for _, row := range schoolRows {
		rows = append(rows, row.TenantAccountInfo)
	}
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	// Pending invitations (no account yet) carry no person and keep their
	// place after the accounts; only the account rows take part in the
	// name order.
	entries := make([]accountEntry, 0, len(rows))
	invitations := make([]TenantAccountInfo, 0)
	for _, row := range rows {
		if isInvitationRow(row) {
			invitations = append(invitations, row)
			continue
		}
		entries = append(entries, accountEntry{TenantID: tenantID, Info: row})
	}
	if err := r.attachAccountPersons(ctx, entries); err != nil {
		return nil, err
	}
	slices.SortStableFunc(entries, compareAccountEntries)
	result := make([]TenantAccountInfo, 0, len(rows))
	for _, entry := range entries {
		result = append(result, entry.Info)
	}
	return append(result, invitations...), nil
}

// accountsBySchoolIDs enriches the bounded identity rows with person facts.
func (r operatorAccountDirectory) accountsBySchoolIDs(ctx context.Context, schoolIDs []int64) ([]OrgAccountInfo, error) {
	rows, err := r.identityRows(ctx, schoolIDs)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	entries := make([]accountEntry, 0, len(rows))
	invitations := make([]OrgAccountInfo, 0)
	for _, row := range rows {
		if isInvitationRow(row.TenantAccountInfo) {
			invitations = append(invitations, row)
			continue
		}
		entries = append(entries, accountEntry{TenantID: row.SchoolID, Info: row.TenantAccountInfo})
	}
	if err := r.attachAccountPersons(ctx, entries); err != nil {
		return nil, err
	}
	slices.SortStableFunc(entries, func(left, right accountEntry) int {
		if left.TenantID != right.TenantID {
			if left.TenantID < right.TenantID {
				return -1
			}
			return 1
		}
		return compareAccountEntries(left, right)
	})
	result := make([]OrgAccountInfo, 0, len(rows))
	for _, entry := range entries {
		result = append(result, OrgAccountInfo{TenantAccountInfo: entry.Info, SchoolID: entry.TenantID})
	}
	return append(result, invitations...), nil
}

// isInvitationRow recognises the synthetic rows the repository builds for
// pending invitations: no account id yet and the fixed "invited" status.
func isInvitationRow(row TenantAccountInfo) bool {
	return row.AccountID == 0 && row.Status == "invited"
}

// attachAccountPersons fills names, pedagogic role and the caregiver facts
// of the entries from the People Directory and the staff/teacher chain.
func (r operatorAccountDirectory) attachAccountPersons(ctx context.Context, entries []accountEntry) error {
	if len(entries) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.Info.AccountID)
	}
	persons, err := personsByAccountAndTenant(ctx, r.persons, ids)
	if err != nil {
		return err
	}
	chains, err := r.caregiverChains(ctx, persons)
	if err != nil {
		return err
	}
	for index := range entries {
		entry := &entries[index]
		person, found := persons[tenantAccountKey{AccountID: entry.Info.AccountID, TenantID: entry.TenantID}]
		if !found {
			continue
		}
		entry.HasPerson = true
		entry.Info.FirstName = person.FirstName
		entry.Info.LastName = person.LastName
		chain, hasChain := chains[person.ID]
		if !hasChain || chain.TenantID != entry.TenantID {
			continue
		}
		entry.Info.PedagogicRole = chain.TeacherRole
		hasTeacher := chain.TeacherID != 0
		entry.Info.HasCaregiverProfile = hasTeacher
		entry.Info.IsActiveCaregiver = hasTeacher && entry.Info.HasUserRole
	}
	return nil
}

// compareAccountEntries orders by last and first name with rows lacking a
// person last, as the previous SQL order (NULLs last) did.
func compareAccountEntries(left, right accountEntry) int {
	if left.HasPerson != right.HasPerson {
		if left.HasPerson {
			return -1
		}
		return 1
	}
	if order := compareStrings(left.Info.LastName, right.Info.LastName); order != 0 {
		return order
	}
	return compareStrings(left.Info.FirstName, right.Info.FirstName)
}

func (r operatorAccountDirectory) caregiverChains(ctx context.Context, persons map[tenantAccountKey]peopledirectory.Person) (map[int64]CaregiverChain, error) {
	if len(persons) == 0 {
		return map[int64]CaregiverChain{}, nil
	}
	if r.chains == nil {
		return nil, fmt.Errorf("account tenant repository does not resolve caregiver chains")
	}
	ids := make([]int64, 0, len(persons))
	for _, person := range persons {
		ids = append(ids, person.ID)
	}
	chains, err := r.chains.CaregiverChainByPersonIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load caregiver chains: %w", err)
	}
	return chains, nil
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
