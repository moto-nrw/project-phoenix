package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// School Membership admits every lock its store takes in the unit of work's
// lock order (tenant.OrderLock) right before the store call that waits for it
// (#6702): the class-write gate before the recurrence gate, the merge gate
// before the staff rows, every other row and advisory lock as a leaf after
// them. A gate or staff row requested after a later lock is refused before it
// waits, so the write fails instead of closing a deadlock cycle. The staff
// row itself is admitted by findStaffRow. backend/test/row_lock_admission_test.go
// requires an admission before every locking store call of this package.

// admitLeaf admits a leaf row or advisory lock the next store call takes.
func admitLeaf(ctx context.Context, lock bool, key string) error {
	if !lock {
		return nil
	}
	return tenant.OrderLock(ctx, tenant.LockLeaf, key)
}

// admitClassWrites admits the school's class-write gate.
func admitClassWrites(ctx context.Context) error {
	return tenant.OrderLock(ctx, tenant.LockStudentClassWrites, tenant.LockIDKey(tenant.FromContext(ctx)))
}

// admitStaffPersonMerge admits the school's staff person merge gate a staff
// creation shares.
func admitStaffPersonMerge(ctx context.Context) error {
	return tenant.OrderLock(ctx, tenant.LockStaffPersonMerge, tenant.LockIDKey(tenant.FromContext(ctx)))
}
