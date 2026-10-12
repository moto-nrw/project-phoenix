package repositories

import (
	"context"
	"fmt"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

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
