package api

import (
	"context"

	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"

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
func NewOperatorAccountDirectory(accounts identityaccess.SchoolAccountListings, persons peopledirectory.Query, membership schoolMembershipCompose.StaffLookup, schools organizationtenancy.Query) OperatorAccountDirectory {
	return operatorAccountDirectory{accounts: accounts, persons: persons, chains: caregiverChainsFromMembership(membership), schools: schools}
}

// listedAccount is one identity row of the listing; invitation marks the
// owner's pending invitation rows.
type listedAccount struct {
	OrgAccountInfo
	invitation bool
}

func (r operatorAccountDirectory) identityRows(ctx context.Context, schoolIDs []int64) ([]listedAccount, error) {
	rows, err := r.accounts.ListSchoolAccountListings(ctx, schoolIDs)
	if err != nil {
		return nil, err
	}
	result := make([]listedAccount, 0, len(rows))
	for _, row := range rows {
		result = append(result, listedAccount{invitation: row.PendingInvitation(), OrgAccountInfo: OrgAccountInfo{SchoolID: row.SchoolID, TenantAccountInfo: TenantAccountInfo{
			AccountID: row.AccountID, Email: row.Email, Active: row.Active,
			FirstName: row.FirstName, LastName: row.LastName, RoleName: row.RoleName,
			Status: row.Status, HasAdminRole: row.HasAdminRole, HasUserRole: row.HasUserRole,
		}}})
	}
	return result, nil
}

// caregiverChainQuery supplies the School Membership facts used by the
// operator listing projection.
type caregiverChainQuery interface {
	CaregiverChainByPersonIDs(context.Context, []int64) (map[int64]CaregiverChain, error)
}

// personsByAccountAndTenant resolves the People Directory's person of each
// account at each school.
func personsByAccountAndTenant(ctx context.Context, query peopledirectory.Query, accountIDs []int64) (map[peopledirectory.AccountSchool]peopledirectory.Person, error) {
	values, err := query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	return peopledirectory.LatestPersonsByAccountSchool(values), nil
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
	for _, row := range schoolRows {
		if row.invitation {
			invitations = append(invitations, row.TenantAccountInfo)
			continue
		}
		entries = append(entries, accountEntry{TenantID: tenantID, Info: row.TenantAccountInfo})
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
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []OrgAccountInfo{}, nil
	}
	entries := make([]accountEntry, 0, len(rows))
	invitations := make([]OrgAccountInfo, 0)
	for _, row := range rows {
		if row.invitation {
			invitations = append(invitations, row.OrgAccountInfo)
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
		person, found := persons[peopledirectory.AccountSchool{AccountID: entry.Info.AccountID, TenantID: entry.TenantID}]
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
		entry.Info.HasCaregiverProfile, entry.Info.IsActiveCaregiver = schoolmembership.CaregiverStatus(chain.TeacherID != 0, entry.Info.HasUserRole)
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

func (r operatorAccountDirectory) caregiverChains(ctx context.Context, persons map[peopledirectory.AccountSchool]peopledirectory.Person) (map[int64]CaregiverChain, error) {
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

func (r operatorAccountDirectory) ListAccountsByOrganizationID(ctx context.Context, organizationID int64) ([]OrgAccountInfo, error) {
	schools, err := r.schools.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("load organization schools for accounts: %w", err)
	}
	return r.listAccounts(ctx, nonDeletedSchoolIDs(schools))
}

func (r operatorAccountDirectory) ListAllAccounts(ctx context.Context) ([]OrgAccountInfo, error) {
	schools, err := r.schools.ListNonDeletedSchools(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for accounts: %w", err)
	}
	return r.listAccounts(ctx, nonDeletedSchoolIDs(schools))
}

func (r operatorAccountDirectory) listAccounts(ctx context.Context, schoolIDs []int64) ([]OrgAccountInfo, error) {
	rows, err := r.accountsBySchoolIDs(ctx, schoolIDs)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	schools, err := schoolsByID(ctx, r.schools, schoolIDs)
	if err != nil {
		return nil, fmt.Errorf("load school names for accounts: %w", err)
	}
	for index := range rows {
		school, found := schools[rows[index].SchoolID]
		if !found {
			return nil, fmt.Errorf("school %d missing for account row", rows[index].SchoolID)
		}
		rows[index].SchoolName = school.Name
	}
	slices.SortStableFunc(rows, func(left, right OrgAccountInfo) int {
		if order := compareStrings(left.SchoolName, right.SchoolName); order != 0 {
			return order
		}
		if order := compareStrings(left.LastName, right.LastName); order != 0 {
			return order
		}
		return compareStrings(left.FirstName, right.FirstName)
	})
	return rows, nil
}

func nonDeletedSchoolIDs(schools []organizationtenancy.School) []int64 {
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		if school.IsDeleted() {
			continue
		}
		ids = append(ids, school.ID)
	}
	return ids
}

func compareStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
