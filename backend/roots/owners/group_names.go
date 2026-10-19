package owners

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// GroupNamesByID returns the names of the groups the owner query sees for
// ids, keyed by group ID, in one batched read. Duplicates and non-positive
// IDs are dropped first; a group the caller cannot see is absent.
func GroupNamesByID(ctx context.Context, query schoolstructure.Query, ids []int64) (map[int64]string, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	names := make(map[int64]string, len(unique))
	if len(unique) == 0 {
		return names, nil
	}
	groups, err := query.ListGroupsByID(ctx, unique)
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		names[group.ID] = group.Name
	}
	return names, nil
}

// EnrichGroupNames fills a display name on every row from the owner with one
// batched query. A group the owner cannot see in the caller's transaction
// yields the fallback, mirroring the former LEFT JOIN + COALESCE; a query
// failure is returned, never swallowed. op names the read in that error.
func EnrichGroupNames[T any](
	ctx context.Context,
	query schoolstructure.Query,
	rows []T,
	groupID func(T) int64,
	setName func(T, string),
	fallback string,
	op string,
) ([]T, error) {
	if len(rows) == 0 {
		return rows, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, groupID(row))
	}
	names, err := GroupNamesByID(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for %s: %w", op, err)
	}
	for index := range rows {
		name, found := names[groupID(rows[index])]
		if !found {
			name = fallback
		}
		setName(rows[index], name)
	}
	return rows, nil
}

// OptionalGroupID reads an optional group reference, zero when unset.
func OptionalGroupID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}
