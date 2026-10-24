package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// PersonsByID resolves the non-deleted persons for ids through the owner
// query, keyed by person ID. Duplicates and non-positive IDs are dropped
// before the lookup.
func PersonsByID(ctx context.Context, query peopledirectory.Query, ids []int64) (map[int64]peopledirectory.Person, error) {
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
