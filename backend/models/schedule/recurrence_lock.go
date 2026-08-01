package schedule

import "fmt"

// TenantRecurrenceLockKey is the advisory-lock key that serializes recurrence
// mutation, re-planning, and materialization inside one tenant. It lives on the
// model because it has more than one holder and they must agree on the exact
// string:
//
//   - services/schedule (lockTenantRecurrenceWrites) — template edits, splits,
//     re-plans, and materialization passes.
//   - services/education Apply/Revert, via
//     GradeTransitionRepository.LockTenantRecurrenceWrites — a grade transition
//     deletes and re-inserts instance_students rows, which is recurrence-derived
//     state.
//
// LOCK ORDER: this key is always taken FIRST, before
// education.TenantTransitionsLockKey. A re-plan holds this gate, deletes planned
// instances (locking their roster rows), and then asks for the transition gate;
// an apply that held the transition gate and only then waited on one of those
// rows would deadlock (#405 review).
func TenantRecurrenceLockKey(tenantID int64) string {
	return fmt.Sprintf("template-recurrence:%d", tenantID)
}
