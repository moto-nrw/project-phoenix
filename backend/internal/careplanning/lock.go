package careplanning

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrStudentNotFound = errors.New("careplanning: student not found")

// StudentLock takes the student row FOR UPDATE inside the caller's tenant
// transaction. The row belongs to the People Directory (#2662).
type StudentLock func(ctx context.Context, studentID int64) error

// ExceptionDayLock takes the shared transaction-scoped advisory lock for one
// student's care day. The tenant runtime implementation is supplied by the
// composition layer, keeping this coordinator independent of SQL and Bun.
type ExceptionDayLock func(ctx context.Context, studentID int64, date string) error

type locks struct {
	student      StudentLock
	exceptionDay ExceptionDayLock
}

var graphLocks sync.Map // map[database identity]locks
var graphLocksMu sync.Mutex

// BindStudentLockForDB binds an owner-backed lock to exactly one repository
// graph. Independent factories may use different databases or capabilities
// without replacing each other's lock.
func BindStudentLockForDB(db any, lock StudentLock, notFound error) {
	if db == nil {
		panic("careplanning: database is required")
	}
	if lock == nil || notFound == nil {
		panic("careplanning: student lock and not-found sentinel are required")
	}
	mapped := StudentLock(func(ctx context.Context, studentID int64) error {
		err := lock(ctx, studentID)
		if errors.Is(err, notFound) {
			return ErrStudentNotFound
		}
		return err
	})
	graphLocksMu.Lock()
	defer graphLocksMu.Unlock()
	bound, _ := graphLocks.Load(db)
	current, _ := bound.(locks)
	current.student = mapped
	graphLocks.Store(db, current)
}

// BindExceptionDayLockForDB binds the Care Plan day lock to one repository graph.
func BindExceptionDayLockForDB(db any, lock ExceptionDayLock) {
	if db == nil || lock == nil {
		panic("careplanning: database and exception-day lock are required")
	}
	graphLocksMu.Lock()
	defer graphLocksMu.Unlock()
	bound, _ := graphLocks.Load(db)
	current, _ := bound.(locks)
	current.exceptionDay = lock
	graphLocks.Store(db, current)
}

// LockStudent takes only the student row FOR UPDATE — the first lock every
// care-day writer acquires. Weekly-schedule writers take it BEFORE touching
// their schedule rows so the global order stays student → schedule rows →
// care-day locks and cannot deadlock against exception writers.
//
// Returns ErrStudentNotFound when the student is missing under the tenant.
func LockStudent(ctx context.Context, db any, studentID int64) error {
	if studentID <= 0 {
		return errors.New("student id is required")
	}
	if db == nil {
		return errors.New("careplanning: database is required")
	}
	value, ok := graphLocks.Load(db)
	if !ok {
		return errors.New("careplanning: student lock is not bound for database")
	}
	bound, ok := value.(locks)
	if !ok {
		return errors.New("careplanning: invalid student lock binding")
	}
	if bound.student == nil {
		return errors.New("careplanning: student lock is not bound for database")
	}
	err := bound.student(ctx, studentID)
	if err != nil {
		return fmt.Errorf("lock student for care exception day: %w", err)
	}
	return nil
}

// LockExceptionDay serializes every plan exception for one child and calendar
// day, including full-day statuses and time-specific excusals.
func LockExceptionDay(ctx context.Context, db any, studentID int64, date string) error {
	value, ok := graphLocks.Load(db)
	if !ok {
		return errors.New("careplanning: exception-day lock is not bound for database")
	}
	bound, ok := value.(locks)
	if !ok {
		return errors.New("careplanning: invalid exception-day lock binding")
	}
	if bound.exceptionDay == nil {
		return errors.New("careplanning: exception-day lock is not bound for database")
	}
	if err := bound.exceptionDay(ctx, studentID, date); err != nil {
		return fmt.Errorf("lock care exception day: %w", err)
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
// Returns ErrStudentNotFound when the student is missing under the tenant.
func LockStudentAndExceptionDay(ctx context.Context, db any, studentID int64, date string) error {
	if err := LockStudent(ctx, db, studentID); err != nil {
		return err
	}
	return LockExceptionDay(ctx, db, studentID, date)
}
