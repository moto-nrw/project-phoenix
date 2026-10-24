package users

import (
	"context"

	"github.com/uptrace/bun"
)

// FindOperatorPersonStudentMembership is the batched student-membership
// projection for the operator directory composition. It does not read
// users.persons, and it no longer answers the staff half: staff rows belong to
// School Membership, so the caller resolves those through that owner.
func FindOperatorPersonStudentMembership(ctx context.Context, runtime Runtime, directory StudentDirectoryRows, personIDs []int64) (map[int64]bool, error) {
	if len(personIDs) == 0 {
		return map[int64]bool{}, nil
	}
	var rows []struct {
		PersonID int64 `bun:"person_id"`
	}
	ambientDB1, databaseErr1 := runtime.DB(ctx)
	if databaseErr1 != nil {
		return *new(map[int64]bool), databaseErr1
	}
	query := studentRows(ctx, runtime, directory, ambientDB1).
		ColumnExpr(`"student".person_id`).
		Where(`"student".person_id IN (?)`, bun.List(personIDs))
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	students := make(map[int64]bool, len(rows))
	for _, row := range rows {
		students[row.PersonID] = true
	}
	return students, nil
}
