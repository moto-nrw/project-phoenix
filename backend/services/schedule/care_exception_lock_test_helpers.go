package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// BindCareStudentLockForDB installs a graph-scoped owner lock for tests that
// compose schedule services without the application root.
func BindCareStudentLockForDB(db *bun.DB, lock func(context.Context, int64) error, notFound error) {
	studentLock := func(ctx context.Context, studentID int64) error {
		err := lock(ctx, studentID)
		if errors.Is(err, notFound) {
			return sql.ErrNoRows
		}
		return err
	}
	careplanning.BindStudentLockForDB(db, studentLock, sql.ErrNoRows)
	careplanning.BindExceptionDayLockForDB(db, func(ctx context.Context, studentID int64, date string) error {
		tenantID := tenant.FromContext(ctx)
		if tenantID <= 0 {
			return errors.New("tenant id is required")
		}
		return tenant.AcquireLock(ctx, fmt.Sprintf("care-exception-day:%d:%d:%s", tenantID, studentID, date), false)
	})
}
