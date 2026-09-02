package repositories

import (
	"context"
	"fmt"
	"slices"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// caregiverChainQuery is the staff/teacher half of the caregiver facts the
// account listings derive; the auth repository implements it.
type caregiverChainQuery interface {
	CaregiverChainByPersonIDs(context.Context, []int64) (map[int64]authModels.CaregiverChain, error)
}

type accountTenantSchoolRowsQuery interface {
	ListAccountsBySchoolIDs(context.Context, []int64) ([]authModels.OrgAccountInfo, error)
}

// personAccountTenantRepository attaches person names and the caregiver
// facts to the account listings. It sits below the school projection, which
// keeps sorting by the names this layer fills in.
type personAccountTenantRepository struct {
	authModels.AccountTenantRepository
	chains  caregiverChainQuery
	rows    accountTenantSchoolRowsQuery
	persons peopledirectory.Query
}

func newPersonAccountTenantRepository(inner authModels.AccountTenantRepository, persons peopledirectory.Query) authModels.AccountTenantRepository {
	chains, _ := inner.(caregiverChainQuery)
	rows, _ := inner.(accountTenantSchoolRowsQuery)
	return personAccountTenantRepository{AccountTenantRepository: inner, chains: chains, rows: rows, persons: persons}
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

func (r personAccountTenantRepository) ListTenantAccessByAccountID(ctx context.Context, accountID int64) ([]authModels.AccountTenantAccessInfo, error) {
	rows, err := r.AccountTenantRepository.ListTenantAccessByAccountID(ctx, accountID)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	persons, err := personsByAccountAndTenant(ctx, r.persons, []int64{accountID})
	if err != nil {
		return nil, err
	}
	chains, err := r.caregiverChains(ctx, persons)
	if err != nil {
		return nil, err
	}
	for index := range rows {
		person, found := persons[tenantAccountKey{AccountID: accountID, TenantID: rows[index].TenantID}]
		rows[index].HasPerson = found
		if !found {
			continue
		}
		chain, hasChain := chains[person.ID]
		rows[index].HasStaff = hasChain && chain.TenantID == rows[index].TenantID
	}
	return rows, nil
}

// accountEntry is one account row under enrichment together with the
// tenant it belongs to and whether a person backs it.
type accountEntry struct {
	TenantID  int64
	HasPerson bool
	Info      authModels.TenantAccountInfo
}

func (r personAccountTenantRepository) ListAccountsByTenantID(ctx context.Context, tenantID int64) ([]authModels.TenantAccountInfo, error) {
	rows, err := r.AccountTenantRepository.ListAccountsByTenantID(ctx, tenantID)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	// Pending invitations (no account yet) carry no person and keep their
	// place after the accounts; only the account rows take part in the
	// name order.
	entries := make([]accountEntry, 0, len(rows))
	invitations := make([]authModels.TenantAccountInfo, 0)
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
	result := make([]authModels.TenantAccountInfo, 0, len(rows))
	for _, entry := range entries {
		result = append(result, entry.Info)
	}
	return append(result, invitations...), nil
}

// ListAccountsBySchoolIDs keeps the raw school-set listing reachable for the
// school projection above it, with the person facts already attached.
func (r personAccountTenantRepository) ListAccountsBySchoolIDs(ctx context.Context, schoolIDs []int64) ([]authModels.OrgAccountInfo, error) {
	if r.rows == nil {
		return nil, fmt.Errorf("account tenant repository does not list accounts by school")
	}
	rows, err := r.rows.ListAccountsBySchoolIDs(ctx, schoolIDs)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	entries := make([]accountEntry, 0, len(rows))
	invitations := make([]authModels.OrgAccountInfo, 0)
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
	result := make([]authModels.OrgAccountInfo, 0, len(rows))
	for _, entry := range entries {
		result = append(result, authModels.OrgAccountInfo{TenantAccountInfo: entry.Info, SchoolID: entry.TenantID})
	}
	return append(result, invitations...), nil
}

// isInvitationRow recognises the synthetic rows the repository builds for
// pending invitations: no account id yet and the fixed "invited" status.
func isInvitationRow(row authModels.TenantAccountInfo) bool {
	return row.AccountID == 0 && row.Status == "invited"
}

// attachAccountPersons fills names, pedagogic role and the caregiver facts
// of the entries from the People Directory and the staff/teacher chain.
func (r personAccountTenantRepository) attachAccountPersons(ctx context.Context, entries []accountEntry) error {
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

func (r personAccountTenantRepository) caregiverChains(ctx context.Context, persons map[tenantAccountKey]peopledirectory.Person) (map[int64]authModels.CaregiverChain, error) {
	if len(persons) == 0 {
		return map[int64]authModels.CaregiverChain{}, nil
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
