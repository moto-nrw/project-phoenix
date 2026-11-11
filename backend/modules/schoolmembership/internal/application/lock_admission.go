package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// admittingStore admits every lock the School Membership store takes in the
// unit of work's lock order (tenant.OrderLock) before the store waits for it
// (#6702): the class-write gate before the recurrence gate, the merge gate
// before the staff rows, every other row and advisory lock as a leaf after
// them. A gate or staff row requested after a later lock is refused before
// it waits, so the write fails instead of closing a deadlock cycle. The staff
// row itself is admitted by findStaffRow. backend/test/row_lock_admission_test.go
// requires every lock of the store to pass one of these methods.
type admittingStore struct {
	ports.Store
}

func admitLeaf(ctx context.Context, lock bool, key string) error {
	if !lock {
		return nil
	}
	return tenant.OrderLock(ctx, tenant.LockLeaf, key)
}

func admitClassWrites(ctx context.Context) error {
	return tenant.OrderLock(ctx, tenant.LockStudentClassWrites, tenant.LockIDKey(tenant.FromContext(ctx)))
}

func (a admittingStore) EnrollStudent(ctx context.Context, input schoolmembership.StudentEnrollment) (int64, domain.OperationStats, error) {
	if err := admitClassWrites(ctx); err != nil {
		return 0, domain.OperationStats{}, err
	}
	return a.Store.EnrollStudent(ctx, input)
}

func (a admittingStore) LockStudentClassWrites(ctx context.Context, exclusive bool) (domain.OperationStats, error) {
	if err := admitClassWrites(ctx); err != nil {
		return domain.OperationStats{}, err
	}
	return a.Store.LockStudentClassWrites(ctx, exclusive)
}

func (a admittingStore) LockStudentWindow(ctx context.Context, id int64) (schoolmembership.StudentEnrollment, bool, domain.OperationStats, error) {
	if err := admitLeaf(ctx, true, "users.student_school_memberships"); err != nil {
		return schoolmembership.StudentEnrollment{}, false, domain.OperationStats{}, err
	}
	return a.Store.LockStudentWindow(ctx, id)
}

func (a admittingStore) LockChildQuota(ctx context.Context) (domain.OperationStats, error) {
	if err := admitLeaf(ctx, true, "advisory:child-quota"); err != nil {
		return domain.OperationStats{}, err
	}
	return a.Store.LockChildQuota(ctx)
}

// CreateStaff shares the school's staff person merge gate before it inserts
// the membership. The inserted row is held: a caregiver profile or
// employment written next locks it again without waiting.
func (a admittingStore) CreateStaff(ctx context.Context, personID int64) (schoolmembership.Staff, domain.OperationStats, error) {
	if err := tenant.OrderLock(ctx, tenant.LockStaffPersonMerge, tenant.LockIDKey(tenant.FromContext(ctx))); err != nil {
		return schoolmembership.Staff{}, domain.OperationStats{}, err
	}
	staff, stats, err := a.Store.CreateStaff(ctx, personID)
	if err == nil {
		tenant.HoldInserted(ctx, tenant.LockStaffRows, tenant.LockIDKey(staff.ID))
	}
	return staff, stats, err
}

func (a admittingStore) FindTeacher(ctx context.Context, id int64, lock string) (schoolmembership.Teacher, bool, domain.OperationStats, error) {
	if err := admitLeaf(ctx, lock != "", "users.teachers"); err != nil {
		return schoolmembership.Teacher{}, false, domain.OperationStats{}, err
	}
	return a.Store.FindTeacher(ctx, id, lock)
}

func (a admittingStore) FindExternalCaregiverRecord(ctx context.Context, id int64, lock string) (schoolmembership.ExternalCaregiverRecord, bool, domain.OperationStats, error) {
	if err := admitLeaf(ctx, lock != "", "users.external_caregivers"); err != nil {
		return schoolmembership.ExternalCaregiverRecord{}, false, domain.OperationStats{}, err
	}
	return a.Store.FindExternalCaregiverRecord(ctx, id, lock)
}

func (a admittingStore) FindClassListEntry(ctx context.Context, id int64, lock string) (schoolmembership.ClassListEntry, bool, domain.OperationStats, error) {
	if err := admitLeaf(ctx, lock != "", "users.class_list_entries"); err != nil {
		return schoolmembership.ClassListEntry{}, false, domain.OperationStats{}, err
	}
	return a.Store.FindClassListEntry(ctx, id, lock)
}

func (a admittingStore) ListClassAssignments(ctx context.Context, filter schoolmembership.ClassAssignmentFilter, forUpdate bool) ([]schoolmembership.ClassAssignment, domain.OperationStats, error) {
	if err := admitLeaf(ctx, forUpdate, "education.class_teachers"); err != nil {
		return nil, domain.OperationStats{}, err
	}
	return a.Store.ListClassAssignments(ctx, filter, forUpdate)
}

func (a admittingStore) ListGroupAssignments(ctx context.Context, filter schoolmembership.GroupAssignmentFilter, forUpdate bool) ([]schoolmembership.GroupAssignment, domain.OperationStats, error) {
	if err := admitLeaf(ctx, forUpdate, "education.group_teacher"); err != nil {
		return nil, domain.OperationStats{}, err
	}
	return a.Store.ListGroupAssignments(ctx, filter, forUpdate)
}

func (a admittingStore) LockGroupAssignments(ctx context.Context) (domain.OperationStats, error) {
	if err := admitLeaf(ctx, true, "education.group_teacher"); err != nil {
		return domain.OperationStats{}, err
	}
	return a.Store.LockGroupAssignments(ctx)
}

// MarkNextRetiredStaffErased skips a locked tombstone and never waits, so
// the row is a leaf of the nightly retention transaction.
func (a admittingStore) MarkNextRetiredStaffErased(ctx context.Context, before time.Time, afterID int64) (schoolmembership.RetiredStaffErasure, bool, domain.OperationStats, error) {
	if err := admitLeaf(ctx, true, "users.staff_school_memberships"); err != nil {
		return schoolmembership.RetiredStaffErasure{}, false, domain.OperationStats{}, err
	}
	return a.Store.MarkNextRetiredStaffErased(ctx, before, afterID)
}
