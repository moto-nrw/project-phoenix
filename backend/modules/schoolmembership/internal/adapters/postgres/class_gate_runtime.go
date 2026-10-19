package postgres

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

func ResolveStudentClassWriteGate(ctx context.Context, db *bun.DB, transaction func(context.Context) (any, bool), school func(context.Context) int64) (*bun.SelectQuery, error) {
	if _, ok := transaction(ctx); !ok {
		return nil, errors.New("school membership: transaction is required")
	}
	return StudentClassWriteGateQuery(db, school(ctx))
}
