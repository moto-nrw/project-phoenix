package tenant

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"
)

// LockClass names one class of transaction-scoped locks in the canonical
// order of a unit of work (#6232). A transaction acquires classes in
// ascending LockClass order and, within one class, keys in ascending order.
// Re-acquiring a lock it already holds is always allowed. Two units of work
// that follow this order can wait for each other but never close a cycle,
// so PostgreSQL never has to abort one of them with 40P01.
//
// The order spans owners: Workforce's staff-shift gate comes before
// Timetable's recurrence gate and day locks, because a sick report holds the
// gate while its cascade writes the days, and plan writers that read sick
// reports hold days, never the gate after a day. The staff rows come after
// the days and before every other row (#6583): offboarding holds a staff row
// while it locks that person's blocks, so a day writer takes the staff rows
// of every person it plans or references before its first block row. Other
// row locks (block rows by ascending id, assignment rows, the remaining
// foreign-key key shares) come after every class listed here.
type LockClass uint8

const (
	// LockStaffShifts is Workforce's per-staff shift gate; keys are staff
	// ids (LockIDKey).
	LockStaffShifts LockClass = iota + 1
	// LockRecurrence is Timetable's tenant-wide recurrence gate.
	LockRecurrence
	// LockGradeTransitions is School Structure's tenant-wide transition
	// gate, taken after recurrence.
	LockGradeTransitions
	// LockScheduleDay is Timetable's day-wide staffing lock; keys are
	// calendar days in YYYY-MM-DD form, which sort like the days.
	LockScheduleDay
	// LockStaffRows is School Membership's row lock on a staff member
	// (users.staff_school_memberships); keys are staff ids (LockIDKey). The
	// foreign-key key share a row referencing the staff member takes
	// (instance_staff, supervisors, shifts, deviation events) belongs to this
	// class too: the writer holds the row lock before it inserts, so the key
	// share is never the first lock on that row. The class position is
	// enforced; its keys are not ordered among themselves yet, because
	// composed writers (a re-plan and its materialization, a template write
	// and its supervisors) learn their people one by one. Writers that know
	// their roster take it ascending (Timetable's LockStaffAssignments).
	LockStaffRows
)

// unorderedKeys are the classes whose keys may come in any order.
var unorderedKeys = map[LockClass]bool{LockStaffRows: true}

func (c LockClass) String() string {
	switch c {
	case LockStaffShifts:
		return "staff-shift gate"
	case LockRecurrence:
		return "recurrence gate"
	case LockGradeTransitions:
		return "grade-transition gate"
	case LockScheduleDay:
		return "schedule day"
	case LockStaffRows:
		return "staff row"
	default:
		return fmt.Sprintf("lock class %d", c)
	}
}

// ErrLockOrder reports a lock request against the canonical order. The
// request is refused before it waits, so the transaction fails instead of
// risking a deadlock against a writer that follows the order.
var ErrLockOrder = errors.New("tenant: lock requested against the canonical lock order")

// LockIDKey is the ordered key of a numeric id: zero-padded, so the keys
// sort like the ids.
func LockIDKey(id int64) string {
	return fmt.Sprintf("%020d", id)
}

type lockLedgerKey struct{}

type heldLock struct {
	class LockClass
	key   string
}

// lockLedger records the ordered locks one transaction holds.
type lockLedger struct {
	mu      sync.Mutex
	held    map[heldLock]struct{}
	last    heldLock
	started bool
}

func withLockLedger(ctx context.Context) context.Context {
	return context.WithValue(ctx, lockLedgerKey{}, &lockLedger{held: make(map[heldLock]struct{})})
}

// lockCheckpoint records the locks PostgreSQL will keep on savepoint rollback.
func lockCheckpoint(ctx context.Context) func() {
	ledger, _ := ctx.Value(lockLedgerKey{}).(*lockLedger)
	if ledger == nil {
		return func() {}
	}
	ledger.mu.Lock()
	held, last, started := maps.Clone(ledger.held), ledger.last, ledger.started
	ledger.mu.Unlock()
	return func() {
		ledger.mu.Lock()
		defer ledger.mu.Unlock()
		ledger.held, ledger.last, ledger.started = held, last, started
	}
}

// OrderLock admits one ordered lock of the ambient transaction before the
// caller acquires it. It returns ErrLockOrder when the transaction already
// holds a lock of a later class, or a later key of the same ordered class,
// and does not hold this one. Outside a transaction opened by this runtime there is
// nothing to compare against, so every request is admitted.
func OrderLock(ctx context.Context, class LockClass, key string) error {
	ledger, ok := ctx.Value(lockLedgerKey{}).(*lockLedger)
	if !ok || ledger == nil {
		return nil
	}
	request := heldLock{class: class, key: key}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if _, held := ledger.held[request]; held {
		return nil
	}
	if ledger.started && (class < ledger.last.class || (class == ledger.last.class && key < ledger.last.key && !unorderedKeys[class])) {
		return fmt.Errorf("%w: %s %q after %s %q", ErrLockOrder, class, key, ledger.last.class, ledger.last.key)
	}
	ledger.held[request] = struct{}{}
	ledger.last, ledger.started = request, true
	return nil
}

// HoldsLock reports whether the ambient transaction already holds the
// ordered lock. A caller that would only take the same lock again skips the
// statement (#6583). Outside a transaction opened by this runtime nothing is
// held.
func HoldsLock(ctx context.Context, class LockClass, key string) bool {
	ledger, ok := ctx.Value(lockLedgerKey{}).(*lockLedger)
	if !ok || ledger == nil {
		return false
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	_, held := ledger.held[heldLock{class: class, key: key}]
	return held
}
