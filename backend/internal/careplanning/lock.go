package careplanning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"

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

// StudentLock takes the student row FOR UPDATE inside the caller's tenant
// transaction. The row belongs to the People Directory (#2662), so the
// composition root binds the owner command here.
type StudentLock func(ctx context.Context, studentID int64) error

var studentLock atomic.Pointer[StudentLock]

// BindStudentLock installs the owner-backed student row lock. notFound is
// the error the lock reports for a child the tenant does not have; every
// care-day writer keeps seeing it as sql.ErrNoRows. The root calls this
// once before any writer runs; the lock fails fast while unbound instead of
// falling back to a foreign query.
func BindStudentLock(lock StudentLock, notFound error) {
	if lock == nil || notFound == nil {
		panic("careplanning: student lock and its not-found sentinel are required")
	}
	mapped := StudentLock(func(ctx context.Context, studentID int64) error {
		err := lock(ctx, studentID)
		if errors.Is(err, notFound) {
			return sql.ErrNoRows
		}
		return err
	})
	studentLock.Store(&mapped)
}

// LockStudent takes only the student row FOR UPDATE — the first lock every
// care-day writer acquires. Weekly-schedule writers take it BEFORE touching
// their schedule rows so the global order stays student → schedule rows →
// care-day locks and cannot deadlock against exception writers.
//
// Returns sql.ErrNoRows when the student is missing under the tenant.
func LockStudent(ctx context.Context, _ *bun.DB, studentID int64) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	if studentID <= 0 {
		return errors.New("student id is required")
	}
	lock := studentLock.Load()
	if lock == nil {
		return errors.New("careplanning: student lock is not bound")
	}
	err := (*lock)(ctx, studentID)
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
