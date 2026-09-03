package absence

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// BindCareStudentLockForDB installs the People Directory's student row lock
// behind the excused-request writers (#2662) for test graphs. Production binds it
// through the timetable services; hand-wired graphs bind their own database.
func BindCareStudentLockForDB(db *bun.DB, lock func(ctx context.Context, studentID int64) error, notFound error) {
	careplanning.BindStudentLockForDB(db, lock, notFound)
	careplanning.BindExceptionDayLockForDB(db, func(ctx context.Context, studentID int64, date string) error {
		tenantID := tenant.FromContext(ctx)
		if tenantID <= 0 {
			return errors.New("tenant id is required")
		}
		return tenant.AcquireLock(ctx, fmt.Sprintf("care-exception-day:%d:%d:%s", tenantID, studentID, date), false)
	})
}
