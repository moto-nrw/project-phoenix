package repositories

import (
	"context"
	"fmt"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// bindPersonProjections wraps every legacy repository that used to read or
// write users.persons through a foreign join. The wrapped repositories keep
// their interfaces; the person columns are resolved through the owner query
// afterwards and the tag writes go through the owner command.
func (f *Factory) bindPersonProjections(persons peopledirectory.Capability) {
	_ = persons
}

// personsByID resolves the non-deleted persons for ids through the owner
// query, keyed by person ID. Duplicates are collapsed before the lookup.
func personsByID(ctx context.Context, query peopledirectory.Query, ids []int64) (map[int64]peopledirectory.Person, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return map[int64]peopledirectory.Person{}, nil
	}
	values, err := query.ListPersonsByID(ctx, unique)
	if err != nil {
		return nil, fmt.Errorf("load persons: %w", err)
	}
	result := make(map[int64]peopledirectory.Person, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}

// personsByAccount resolves persons for account ids, keyed by account ID.
// An account with several persons (staff at more than one school inside an
// admin transaction) keeps the most recently updated one, matching the
// DISTINCT ON ordering the legacy joins used.
func personsByAccount(ctx context.Context, query peopledirectory.Query, accountIDs []int64) (map[int64]peopledirectory.Person, error) {
	values, err := query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	result := make(map[int64]peopledirectory.Person, len(values))
	for _, value := range values {
		if value.AccountID == nil {
			continue
		}
		current, found := result[*value.AccountID]
		if !found || value.UpdatedAt.After(current.UpdatedAt) {
			result[*value.AccountID] = value
		}
	}
	return result, nil
}

// toLegacyPerson projects an owner value onto the legacy model that the
// wrapped repositories still hand to their callers. The projection carries
// the display and link fields those callers read; the birthday stays with
// the owner (no wrapped caller renders it).
func toLegacyPerson(value peopledirectory.Person) *usersModels.Person {
	person := &usersModels.Person{
		FirstName: value.FirstName, LastName: value.LastName,
		TagID: value.TagID, AccountID: value.AccountID, DeletedAt: value.DeletedAt,
	}
	person.ID = value.ID
	person.CreatedAt = value.CreatedAt
	person.UpdatedAt = value.UpdatedAt
	person.SetTenantID(value.TenantID)
	return person
}
