package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func BindCareLocks(db *bun.DB, studentLock careplanning.StudentLock, studentNotFound error, ambientDB func(context.Context) (bun.IDB, error)) {
	careplanning.BindStudentLockForDB(db, studentLock, studentNotFound)
	careplanning.BindExceptionDayLockForDB(db, func(ctx context.Context, studentID int64, date string) error {
		tenantID := tenant.FromContext(ctx)
		if tenantID <= 0 {
			return errors.New("care plan: tenant is required for exception-day lock")
		}
		db, err := ambientDB(ctx)
		if err != nil {
			return err
		}
		return LockExceptionDay(ctx, db, tenantID, studentID, date)
	})
}

func CarePlanDatabase(db *bun.DB) Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		raw, present := tenant.TransactionFromContext(ctx)
		return ResolveDatabase(raw, present, tenant.FromContext(ctx))
	}
}
