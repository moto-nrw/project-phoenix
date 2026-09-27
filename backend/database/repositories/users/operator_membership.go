package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"

	"github.com/uptrace/bun"
)

// FindOperatorPersonStudentMembership is the batched student-membership
// projection for the operator directory composition. It does not read
// users.persons, and it no longer answers the staff half: staff rows belong to
// School Membership, so the caller resolves those through that owner.
func FindOperatorPersonStudentMembership(ctx context.Context, runtime Runtime, personIDs []int64) (map[int64]bool, error) {
	if len(personIDs) == 0 {
		return map[int64]bool{}, nil
	}
	var rows []struct {
		PersonID int64 `bun:"person_id"`
	}
	query := studentdirectoryview.Query(runtime.DB(ctx), runtime.TenantID(ctx)).
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
