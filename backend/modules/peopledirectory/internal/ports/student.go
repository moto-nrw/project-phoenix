package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentStore is the persistence port over users.students. Reads honour the
// tenant in context when one is present; inside an admin transaction they
// span every tenant. Writes always require a tenant.
type StudentStore interface {
	ListDepartureModes(context.Context, []int64) (map[int64]map[string][]string, domain.OperationStats, error)
	CurrentFamilyProtection(context.Context, []int64) (map[int64]bool, domain.OperationStats, error)
	// AppendFamilyProtection inserts one immutable ledger row. The caller
	// holds the student's row lock.
	AppendFamilyProtection(context.Context, domain.FamilyProtectionChange) (domain.OperationStats, error)
	// LockLifecycle takes the student row FOR UPDATE and reports its
	// lifecycle status; found is false when the tenant has no such row.
	LockLifecycle(ctx context.Context, id int64) (status string, found bool, stats domain.OperationStats, err error)
	ReadEnrollment(context.Context, int64, string) (domain.EnrollmentRecord, domain.OperationStats, error)
	LockEnrollmentClassWrites(context.Context) (domain.OperationStats, error)
	// LockEnrollmentClassWritesExclusive takes the same gate exclusively.
	LockEnrollmentClassWritesExclusive(context.Context) (domain.OperationStats, error)
	ApplyEnrollmentProfile(context.Context, int64, domain.EnrollmentProfilePatch) (domain.OperationStats, error)
	CreateEnrollment(context.Context, domain.EnrollmentStudent) (domain.Student, domain.OperationStats, error)
	RenewEnrollment(context.Context, int64, domain.EnrollmentStudent) (domain.OperationStats, error)
	// ListByIDs returns the rows for ids, alumni included.
	ListByIDs(context.Context, []int64) ([]domain.Student, domain.OperationStats, error)
	ListNamesByIDs(context.Context, []int64) ([]domain.StudentName, domain.OperationStats, error)
	// ListByClasses returns the non-alumni rows of the classes, ordered by
	// class then id.
	ListByClasses(context.Context, []string) ([]domain.Student, domain.OperationStats, error)
	// ListByPersonIDs returns the rows whose person is one of the ids, alumni
	// included, ordered by id.
	ListByPersonIDs(context.Context, []int64) ([]domain.Student, domain.OperationStats, error)
	// ListDirectory returns one page of the staff directory, ordered by id so
	// paging over a selection cannot repeat or skip a child.
	ListDirectory(context.Context, domain.StudentDirectoryFilter) ([]domain.StudentRecord, domain.OperationStats, error)
	// CountDirectory counts the same selection without its page window.
	CountDirectory(context.Context, domain.StudentDirectoryFilter) (int, domain.OperationStats, error)
	// ListDirectoryIDs returns the ids of every non-alumni row, the
	// lightweight candidate set of the dated participation rule.
	ListDirectoryIDs(context.Context) ([]int64, domain.OperationStats, error)
	// ListEnrolled returns every non-alumni row of the current tenant.
	ListEnrolled(context.Context) ([]domain.Student, domain.OperationStats, error)
	// ListClasses returns the distinct non-empty classes of non-alumni rows.
	ListClasses(context.Context) ([]string, domain.OperationStats, error)
	ListByStatusFlag(context.Context, string) ([]domain.Student, domain.OperationStats, error)
	// Lock takes the row FOR UPDATE and reports whether it exists.
	Lock(context.Context, int64) (bool, domain.OperationStats, error)
	Promote(ctx context.Context, ids []int64, fromClass, toClass string) (int64, domain.OperationStats, error)
	RevertClass(ctx context.Context, id int64, fromClass, toClass string) (int64, domain.OperationStats, error)
	GraduateByClasses(context.Context, []string) (int64, domain.OperationStats, error)
	GraduateByIDs(context.Context, []int64) (int64, domain.OperationStats, error)
	// Reactivate moves alumni back to status and returns the ids it changed.
	Reactivate(ctx context.Context, ids []int64, status string) ([]int64, domain.OperationStats, error)
	ClearStatusFlags(ctx context.Context, ids []int64, status string) (int64, domain.OperationStats, error)
	// CountGuardianLinks counts the current (students_guardians) and legacy
	// (persons_guardians) links a permanent deletion removes.
	CountGuardianLinks(ctx context.Context, studentID, personID int64) (int, domain.OperationStats, error)
	// DeleteLegacyGuardianLinks removes the person-based guardian links that
	// do not cascade from the student row.
	DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, domain.OperationStats, error)
	// Delete hard-deletes the student row; dependent rows cascade or unlink
	// per their foreign keys. Returns the number of deleted rows (0 or 1).
	Delete(ctx context.Context, id int64) (int64, domain.OperationStats, error)
}
