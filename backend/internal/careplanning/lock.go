package careplanning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LockExceptionDay serializes every plan exception for one child and calendar
// day, including full-day statuses and time-specific excusals.
func LockExceptionDay(ctx context.Context, db *bun.DB, studentID int64, date timezone.Date) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	key := fmt.Sprintf("care-exception-day:%d:%d:%s", tenantID, studentID, date.String())
	if err := base.AcquireXactLock(ctx, db, key); err != nil {
		return fmt.Errorf("lock care exception day: %w", err)
	}
	return nil
}

// LockStudent takes only the student row FOR UPDATE — the first lock every
// care-day writer acquires. Weekly-schedule writers take it BEFORE touching
// their schedule rows so the global order stays student → schedule rows →
// care-day locks and cannot deadlock against exception writers.
//
// Returns sql.ErrNoRows when the student is missing under the tenant.
func LockStudent(ctx context.Context, db *bun.DB, studentID int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if studentID <= 0 {
		return errors.New("student id is required")
	}
	var lockedID int64
	err := base.GetDB(ctx, db).NewRaw(
		`SELECT id FROM users.students WHERE id = ? AND tenant_id = ? FOR UPDATE`,
		studentID, tenantID,
	).Scan(ctx, &lockedID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return fmt.Errorf("lock student for care exception day: %w", err)
	}
	return nil
}

// LockStudentAndExceptionDay takes the student row FOR UPDATE first, then the
// shared care-day advisory lock. Full-day status writers already use this order
// (GetByIDForUpdate → LockExceptionDay). Pickup/arrival exception and partial-
// absence writers must match it: taking the care-day lock first deadlocks when
// a concurrent full-day writer holds the student row and waits for care-day,
// while an exception INSERT's FK check needs a KEY SHARE on the student.
//
// Returns sql.ErrNoRows when the student is missing under the tenant.
func LockStudentAndExceptionDay(ctx context.Context, db *bun.DB, studentID int64, date timezone.Date) error {
	if err := LockStudent(ctx, db, studentID); err != nil {
		return err
	}
	return LockExceptionDay(ctx, db, studentID, date)
}
