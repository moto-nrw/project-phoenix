package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// Store is the persistence port over the School Membership tables in the
// public vocabulary; it returns the public sentinels. Reads honour the
// tenant in context when one is present.
type Store interface {
	TransitionStudentStatus(context.Context, int64, string, string) (bool, domain.OperationStats, error)
	EnrollStudent(context.Context, schoolmembership.StudentEnrollment) (int64, domain.OperationStats, error)
	RenewStudentEnrollment(context.Context, schoolmembership.StudentEnrollment) (int64, domain.OperationStats, error)
	AssignStudentGroup(context.Context, int64, *int64) (bool, domain.OperationStats, error)
	LockStudentClassWrites(context.Context, bool) (domain.OperationStats, error)
	EndStudentCare(context.Context, []int64, string) (int64, domain.OperationStats, error)
	ResumeStudentCare(context.Context, int64, string, string, string) (bool, domain.OperationStats, error)
	GraduateStudents(context.Context, []int64) (int64, domain.OperationStats, error)
	ReactivateStudents(context.Context, []int64, string) ([]int64, domain.OperationStats, error)
	ChangeStudentClass(context.Context, []int64, string, string) (int64, domain.OperationStats, error)
	// LockChildQuota serializes the counting writes of the tenant in
	// context. The lock is exclusive and held until the transaction ends.
	LockChildQuota(context.Context) (domain.OperationStats, error)
	// CountChildQuota is the Kontingentzahl of the tenant in context on the
	// given Berlin calendar day: the actively managed children of the
	// Stichtagszahl plus the pending ones.
	CountChildQuota(ctx context.Context, on string) (int, domain.OperationStats, error)
	FindStaff(ctx context.Context, id int64, lock string, includeDeleted bool) (schoolmembership.Staff, bool, domain.OperationStats, error)
	FindStaffByPerson(context.Context, int64) (schoolmembership.Staff, bool, domain.OperationStats, error)
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, domain.OperationStats, error)
	// CreateStaff and UpdateStaff write the membership row only; the
	// employment half belongs to Workforce (StaffEmployment).
	CreateStaff(ctx context.Context, personID int64) (schoolmembership.Staff, domain.OperationStats, error)
	UpdateStaff(ctx context.Context, id, personID int64) (schoolmembership.Staff, domain.OperationStats, error)
	SoftDeleteStaff(context.Context, int64) (domain.OperationStats, error)
	// ReinstateStaff lifts the soft delete of a retired, not erased
	// membership; ErrStaffNotFound when there is none to reinstate.
	ReinstateStaff(context.Context, int64) (schoolmembership.Staff, domain.OperationStats, error)

	FindTeacher(ctx context.Context, id int64, lock string) (schoolmembership.Teacher, bool, domain.OperationStats, error)
	FindTeacherByStaff(context.Context, int64) (schoolmembership.Teacher, bool, domain.OperationStats, error)
	ListTeachers(context.Context, schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, domain.OperationStats, error)
	CreateTeacher(context.Context, schoolmembership.TeacherFields) (schoolmembership.Teacher, domain.OperationStats, error)
	UpdateTeacher(context.Context, int64, schoolmembership.TeacherFields) (schoolmembership.Teacher, domain.OperationStats, error)
	SoftDeleteTeacher(context.Context, int64) (domain.OperationStats, error)
	// ReinstateTeacher lifts the soft delete of a retired caregiver profile;
	// ErrTeacherNotFound when there is none to reinstate.
	ReinstateTeacher(context.Context, int64) (schoolmembership.Teacher, domain.OperationStats, error)

	FindGuest(ctx context.Context, id int64, lock string) (schoolmembership.Guest, bool, domain.OperationStats, error)
	FindGuestByStaff(context.Context, int64) (schoolmembership.Guest, bool, domain.OperationStats, error)
	ListGuests(context.Context, schoolmembership.GuestFilter) ([]schoolmembership.Guest, domain.OperationStats, error)
	CreateGuest(context.Context, schoolmembership.GuestFields) (schoolmembership.Guest, domain.OperationStats, error)
	UpdateGuest(context.Context, int64, schoolmembership.GuestFields) (schoolmembership.Guest, domain.OperationStats, error)
	DeleteGuest(context.Context, int64) (domain.OperationStats, error)

	FindClassListEntry(ctx context.Context, id int64, lock string) (schoolmembership.ClassListEntry, bool, domain.OperationStats, error)
	ListClassListEntries(context.Context, schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, domain.OperationStats, error)
	// ListClassListEntriesInDisplayOrder returns the same rows in the
	// class-then-name order a class list is read in.
	ListClassListEntriesInDisplayOrder(context.Context, schoolmembership.ClassListEntryFilter) ([]schoolmembership.ClassListEntry, domain.OperationStats, error)
	CreateClassListEntry(ctx context.Context, fields schoolmembership.ClassListEntryFields, createdBy *int64) (schoolmembership.ClassListEntry, domain.OperationStats, error)
	UpdateClassListEntry(context.Context, int64, schoolmembership.ClassListEntryFields) (schoolmembership.ClassListEntry, domain.OperationStats, error)
	DeleteClassListEntry(context.Context, int64) (domain.OperationStats, error)

	ListClassAssignments(ctx context.Context, filter schoolmembership.ClassAssignmentFilter, forUpdate bool) ([]schoolmembership.ClassAssignment, domain.OperationStats, error)
	CreateClassAssignment(context.Context, int64, string) (schoolmembership.ClassAssignment, domain.OperationStats, error)
	UpdateClassAssignment(context.Context, int64, int64, string) (schoolmembership.ClassAssignment, domain.OperationStats, error)
	DeleteClassAssignment(context.Context, int64) (domain.OperationStats, error)
	DeleteClassAssignmentsByStaff(context.Context, int64) (domain.OperationStats, error)

	ListGroupAssignments(ctx context.Context, filter schoolmembership.GroupAssignmentFilter, forUpdate bool) ([]schoolmembership.GroupAssignment, domain.OperationStats, error)
	CreateGroupAssignment(context.Context, int64, int64) (schoolmembership.GroupAssignment, domain.OperationStats, error)
	UpdateGroupAssignment(context.Context, int64, int64, int64) (schoolmembership.GroupAssignment, domain.OperationStats, error)
	DeleteGroupAssignment(context.Context, int64) (domain.OperationStats, error)
	DeleteGroupAssignmentsByTeacher(context.Context, int64) (domain.OperationStats, error)
	LockGroupAssignments(context.Context) (domain.OperationStats, error)

	// The erasure of retired staff (#3963).
	MarkNextRetiredStaffErased(ctx context.Context, before time.Time, afterID int64) (schoolmembership.RetiredStaffErasure, bool, domain.OperationStats, error)
	DeleteGroupAssignmentsByStaff(context.Context, int64) (domain.OperationStats, error)
	DeleteTeachersByStaff(context.Context, int64) (domain.OperationStats, error)
	DeleteGuestsByStaff(context.Context, int64) (domain.OperationStats, error)
}

// StaffEmployment is the consumer-owned Workforce port over the employment
// profile of a staff membership (#2753). A staff member is the membership row
// plus this profile; the module composes the two and never stores the profile
// itself. Calls join the caller's transaction.
type StaffEmployment interface {
	// StaffEmployments returns the profiles of the given memberships; a
	// membership without one is absent from the map.
	StaffEmployments(context.Context, []int64) (map[int64]domain.StaffEmployment, error)
	SaveStaffEmployment(context.Context, domain.StaffEmployment) error
}

// ChildQuota is the consumer-owned port over Organisation & Tenancy's
// Kinderkontingent (#3567). It answers for the tenant in context on the
// caller's transaction; limited is false when the school has none.
type ChildQuota interface {
	ChildQuotaLimit(ctx context.Context) (limit int, limited bool, err error)
}

// Clock returns the Berlin calendar day (YYYY-MM-DD) the Kontingentzahl is
// counted on.
type Clock func() string

type Transaction interface {
	// RunWrite joins the caller's transaction or opens one for the tenant
	// in context.
	RunWrite(context.Context, func(context.Context) error) error
	// RunSavepoint runs a multi-owner write inside a savepoint of the
	// current transaction, so a failed owner write undoes the earlier ones
	// even when the caller catches the error and commits.
	RunSavepoint(context.Context, func(context.Context) error) error
	// RunRead joins the caller's transaction, else opens a tenant
	// transaction, else an admin transaction for cross-tenant readers.
	RunRead(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
