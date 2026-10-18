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
	persons  accountPersonsQuery
	chains   caregiverChainQuery
	schools  operatorSchools
}

// accountPersonsQuery is the People Directory read of the persons behind
// accounts.
type accountPersonsQuery interface {
	ListPersonsByAccount(ctx context.Context, accountIDs []int64) ([]peopledirectory.Person, error)
}

// operatorSchools are the Organisation & Tenancy school reads the listing
// scopes and names its rows with.
type operatorSchools interface {
	ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error)
	ListNonDeletedSchools(ctx context.Context) ([]organizationtenancy.School, error)
	ListSchoolsByID(ctx context.Context, ids []int64) ([]organizationtenancy.School, error)
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
func personsByAccountAndTenant(ctx context.Context, query accountPersonsQuery, accountIDs []int64) (map[peopledirectory.AccountSchool]peopledirectory.Person, error) {
	values, err := query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	return peopledirectory.LatestPersonsByAccountSchool(values), nil
}

// accountEntry is one account row under enrichment together with the
// tenant it belongs to, whether a person backs it, and whether it is the
// owner's pending invitation row.
type accountEntry struct {
	TenantID   int64
	HasPerson  bool
	Info       TenantAccountInfo
	invitation bool
}

func (e accountEntry) pendingInvitation() bool { return e.invitation }

// enrichAccounts binds the person enrichment to the listing order.
func (r operatorAccountDirectory) enrichAccounts(ctx context.Context) func([]accountEntry) error {
	return func(entries []accountEntry) error { return r.attachAccountPersons(ctx, entries) }
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
	entries := make([]accountEntry, 0, len(schoolRows))
	for _, row := range schoolRows {
		entries = append(entries, accountEntry{TenantID: tenantID, Info: row.TenantAccountInfo, invitation: row.invitation})
	}
	ordered, err := identityaccess.OrderSchoolAccountListing(entries, accountEntry.pendingInvitation, r.enrichAccounts(ctx), accountEntry.listingKey)
	if err != nil {
		return nil, err
	}
	result := make([]TenantAccountInfo, 0, len(ordered))
	for _, entry := range ordered {
		result = append(result, entry.Info)
	}
	return result, nil
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
	for _, row := range rows {
		entries = append(entries, accountEntry{TenantID: row.SchoolID, Info: row.TenantAccountInfo, invitation: row.invitation})
	}
	ordered, err := identityaccess.OrderAccountListingBySchoolID(entries, accountEntry.pendingInvitation, r.enrichAccounts(ctx), accountEntry.listingKey)
	if err != nil {
		return nil, err
	}
	result := make([]OrgAccountInfo, 0, len(ordered))
	for _, entry := range ordered {
		result = append(result, OrgAccountInfo{TenantAccountInfo: entry.Info, SchoolID: entry.TenantID})
	}
	return result, nil
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
		if !identityaccess.StaffRecordCountsAtSchool(hasChain, chain.TenantID, entry.TenantID) {
			continue
		}
		entry.Info.PedagogicRole = chain.TeacherRole
		entry.Info.HasCaregiverProfile, entry.Info.IsActiveCaregiver = schoolmembership.CaregiverStatus(chain.TeacherID != 0, entry.Info.HasUserRole)
	}
	return nil
}

// listingKey maps the entry onto the fields Identity & Access orders the
// listing by.
func (e accountEntry) listingKey() identityaccess.ListedAccountKey {
	return identityaccess.ListedAccountKey{
		SchoolID: e.TenantID, HasPerson: e.HasPerson, LastName: e.Info.LastName, FirstName: e.Info.FirstName,
	}
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
		return identityaccess.CompareAccountsBySchoolName(
			identityaccess.ListedAccountKey{SchoolName: left.SchoolName, LastName: left.LastName, FirstName: left.FirstName},
			identityaccess.ListedAccountKey{SchoolName: right.SchoolName, LastName: right.LastName, FirstName: right.FirstName},
		)
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
