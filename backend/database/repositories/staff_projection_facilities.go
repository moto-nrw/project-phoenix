package repositories

import (
	"context"
)

// supervisorPersonsResolverSetter is the seam the room repository exposes for
// its staff-to-person lookup. The plain function type keeps the query-option
// vocabulary of the repository layer out of this package.
type supervisorPersonsResolverSetter interface {
	SetSupervisorPersonsResolver(func(context.Context, []int64) (map[int64]int64, error))
}

// supervisorPersonsResolver maps the supervising staff of a room to the
// persons behind them. The replaced INNER JOIN carried no soft-delete filter,
// so an offboarded supervisor still counts while their supervision is open.
func supervisorPersonsResolver(membership staffLookup) func(context.Context, []int64) (map[int64]int64, error) {
	return func(ctx context.Context, staffIDs []int64) (map[int64]int64, error) {
		members, err := staffByID(ctx, membership, staffIDs, true)
		if err != nil {
			return nil, err
		}
		persons := make(map[int64]int64, len(members))
		for id, member := range members {
			persons[id] = member.PersonID
		}
		return persons, nil
	}
}
