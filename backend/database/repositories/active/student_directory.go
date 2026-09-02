package active

import (
	"context"
	"errors"
	"time"
)

// DirectoryStudent is the People Directory projection this package reads.
// users.students belongs to that owner (#2662); the composition root binds
// the directory behind StudentDirectory instead of the former SQL joins.
type DirectoryStudent struct {
	ID           int64
	TenantID     int64
	PersonID     int64
	SchoolClass  string
	GroupID      *int64
	Status       string
	Sick         *bool
	SickSince    *time.Time
	Excused      *bool
	ExcusedSince *time.Time
	PhotoPath    *string
}

// StudentDirectory is the owner query the active repositories read students
// through. Every method fails while unbound; there is no fallback join.
type StudentDirectory interface {
	// ListStudentsByID returns the tenant's rows for ids, alumni included.
	ListStudentsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error)
	// ListStudentsAcrossTenantsByID resolves visiting students from their
	// home school in a separate admin read.
	ListStudentsAcrossTenantsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error)
	// ListActiveStudents returns every student of the current tenant whose
	// lifecycle status is active.
	ListActiveStudents(ctx context.Context) ([]DirectoryStudent, error)
	// ListStudentsWithStatusFlag returns every student with the requested
	// legacy live absence flag in the current tenant.
	ListStudentsWithStatusFlag(ctx context.Context, status string) ([]DirectoryStudent, error)
	// ClearStudentStatusFlags clears one legacy live absence flag for the
	// supplied student ids. The People Directory owns this write.
	ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, error)
	// LockStudent takes a student row FOR UPDATE in the caller's transaction.
	// Status archival locks every candidate before it re-reads and clears flags.
	LockStudent(ctx context.Context, studentID int64) error
}

var errStudentDirectoryRequired = errors.New("active repositories: student directory is not bound")
