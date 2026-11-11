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
// The order spans owners: a staff exit's account row comes first (Konto →
// Schule → Profil), then School Membership's exclusive class-write gate,
// Workforce's payroll-rules gate and per-staff work-time gates (month close
// and offboarding hold them while they take staff-shift gates), then the
// staff-shift gate before Timetable's recurrence gate, the grade transition
// gate, the reference rows a writer pins and the day locks: a sick report
// holds the gate while its cascade writes the days, and plan writers that
// read sick reports hold days, never the gate after a day. The staff person
// merge gate and the staff rows come after the days and before every other
// row (#6583): offboarding holds a staff row while it locks that person's
// blocks, so a day writer takes the staff rows of every person it plans or
// references before its first block row. Every other row lock and every
// leaf advisory lock of the ordered owners is the last class, LockLeaf
// (#6702); backend/test/row_lock_admission_test.go holds the inventory and
// refuses an acquisition that skips OrderLock.
type LockClass uint8

const (
	// LockAccount is an Identity & Access account row a staff exit locks
	// before every gate (Konto → Schule → Profil); keys are account ids
	// (LockIDKey), ascending.
	LockAccount LockClass = iota + 1
	// LockStudentClassWrites is School Membership's tenant-wide class-write
	// gate held exclusively: a grade transition takes it as its first lock,
	// before the recurrence gate. Shared holders read the gate at different
	// points of their own protocols (student writes, attendance, reopen) and
	// stay out of the ledger.
	LockStudentClassWrites
	// LockPayrollRules is Workforce's tenant-wide payroll-rules gate; month
	// close freezes the rules before it locks the staff balances.
	LockPayrollRules
	// LockStaffWorkTime is Workforce's per-staff balance and absence gates;
	// keys are the staff id with the gate (StaffWorkTimeKey), so a writer
	// takes one person's balance before that person's absence gate and the
	// people ascending.
	LockStaffWorkTime
	// LockStaffShifts is Workforce's per-staff shift gate; keys are staff
	// ids (LockIDKey).
	LockStaffShifts
	// LockRecurrence is Timetable's tenant-wide recurrence gate.
	LockRecurrence
	// LockGradeTransitions is School Structure's tenant-wide transition
	// gate, taken after recurrence.
	LockGradeTransitions
	// LockReference is a Timetable row a writer validates and pins before it
	// plans: catalogue rows (activities.categories, schedule.planning_tracks),
	// the template or activity row it edits (activities.groups) and the
	// spontaneous start's gates. Keys name the table or gate and are not
	// ordered among themselves. Writers take them after the gates and before
	// their days and staff rows; a writer that updates a template row after
	// its staff rows locks that row first (template update).
	LockReference
	// LockScheduleDay is Timetable's day-wide staffing lock; keys are
	// calendar days in YYYY-MM-DD form, which sort like the days.
	LockScheduleDay
	// LockStaffPersonMerge is the school's staff person merge gate: the
	// merge holds it exclusively, staff and person creation share it.
	LockStaffPersonMerge
	// LockStaffRows is School Membership's row lock on a staff member
	// (users.staff_school_memberships); keys are staff ids (LockIDKey),
	// ascending across the whole transaction (#6702). A writer that locks
	// several people determines all of them first and takes them ascending
	// in one batch; a staff row below one it already holds is refused. The
	// lock is FOR NO KEY UPDATE: the foreign-key key share a row referencing
	// the staff member takes (instance_staff, supervisors, shifts, absences,
	// actor columns, tombstone triggers) never waits for it, so those
	// implicit locks are no part of the order. Only the erasure of a whole
	// school deletes staff rows.
	LockStaffRows
	// LockLeaf is every other row lock (FOR UPDATE, FOR SHARE and their KEY
	// variants) and every leaf advisory lock of Timetable, Workforce, School
	// Membership and their workflows (#6702). A FOR ... SKIP LOCKED
	// acquisition never waits and is a leaf whatever row it takes.
	// Keys name the locked table or advisory scope; they are not ordered
	// among themselves (each statement locks its rows by ascending id), but
	// once a transaction holds a leaf it takes no gate, day or staff row.
	LockLeaf
)

// StaffWorkTimeKey is the ordered key of one staff member's work-time gate:
// the balance gate sorts before the absence gate of the same person.
func StaffWorkTimeKey(staffID int64, absence bool) string {
	if absence {
		return LockIDKey(staffID) + "/2-absence"
	}
	return LockIDKey(staffID) + "/1-balance"
}

func (c LockClass) String() string {
	switch c {
	case LockAccount:
		return "account row"
	case LockPayrollRules:
		return "payroll-rules gate"
	case LockStaffWorkTime:
		return "staff work-time gate"
	case LockStaffShifts:
		return "staff-shift gate"
	case LockStudentClassWrites:
		return "class-write gate"
	case LockRecurrence:
		return "recurrence gate"
	case LockGradeTransitions:
		return "grade-transition gate"
	case LockReference:
		return "catalogue row"
	case LockScheduleDay:
		return "schedule day"
	case LockStaffPersonMerge:
		return "staff person merge gate"
	case LockStaffRows:
		return "staff row"
	case LockLeaf:
		return "leaf lock"
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
// holds a lock of a later class, or a later key of the same class (except
// LockReference and LockLeaf), and does not hold this one. Outside a transaction opened by this runtime there is
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
	if ledger.started && (class < ledger.last.class || (class == ledger.last.class && class != LockLeaf && class != LockReference && key < ledger.last.key)) {
		return fmt.Errorf("%w: %s %q after %s %q", ErrLockOrder, class, key, ledger.last.class, ledger.last.key)
	}
	ledger.held[request] = struct{}{}
	ledger.last, ledger.started = request, true
	return nil
}

// AdmitBefore binds an advisory acquisition behind the admission of the
// ordered lock (class, key): every call admits before it takes lockKey
// (#6702), so a composition binds an ordered gate in one expression.
func AdmitBefore(class LockClass, key string, acquire func(context.Context, string, bool) error) func(context.Context, string, bool) error {
	return func(ctx context.Context, lockKey string, shared bool) error {
		if err := OrderLock(ctx, class, key); err != nil {
			return err
		}
		return acquire(ctx, lockKey, shared)
	}
}

// HoldInserted records a row the ambient transaction has just inserted as a
// held lock of the class: no other transaction can lock an uncommitted row,
// so locking it again never waits, whatever the order (#6702). It moves no
// order position. Outside a transaction opened by this runtime it does
// nothing.
func HoldInserted(ctx context.Context, class LockClass, key string) {
	ledger, ok := ctx.Value(lockLedgerKey{}).(*lockLedger)
	if !ok || ledger == nil {
		return
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	ledger.held[heldLock{class: class, key: key}] = struct{}{}
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
