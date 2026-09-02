package careplanning

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

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
// transaction. The row belongs to the People Directory (#2662).
type StudentLock func(ctx context.Context, studentID int64) error

var studentLocks sync.Map // map[*bun.DB]StudentLock

// BindStudentLock maps an owner-backed lock to the SQL contract retained by
// care-day writers. Callers can keep the returned value in their own graph.
func BindStudentLock(lock StudentLock, notFound error) StudentLock {
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
	return mapped
}

// BindStudentLockForDB binds an owner-backed lock to exactly one repository
// graph. Independent factories may use different databases or capabilities
// without replacing each other's lock.
func BindStudentLockForDB(db *bun.DB, lock StudentLock, notFound error) {
	if db == nil {
		panic("careplanning: database is required")
	}
	studentLocks.Store(db, BindStudentLock(lock, notFound))
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
	if db == nil {
		return errors.New("careplanning: database is required")
	}
	value, ok := studentLocks.Load(db)
	if !ok {
		return errors.New("careplanning: student lock is not bound for database")
	}
	lock, ok := value.(StudentLock)
	if !ok {
		return errors.New("careplanning: invalid student lock binding")
	}
	err := lock(ctx, studentID)
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
