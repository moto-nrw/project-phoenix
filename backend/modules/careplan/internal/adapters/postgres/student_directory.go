package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// StudentDirectory is Care Plan's reader port onto the named student
// directory projection (ADR 0058): People Directory's profiles, School
// Membership's live memberships and Care Plan's own care profiles joined in
// one tenant-safe source. The root binds it. Every read takes the caller's
// transaction, so the projection runs inside Care Plan's statement or unit of
// work and keeps its statement budget.
type StudentDirectory interface {
	AllSchoolsStudentRows(db bun.IDB) *bun.SelectQuery
	// StudentRows is the directory row source, aliased "student", that Care Plan
	// embeds in its own statements. It requires a school and executes no SQL.
	// Correlated reads use AllSchoolsStudentRows with their outer school predicate.
	StudentRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	// StudentDisplayRows is the display row of every student, alumni
	// included, embedded the same way.
	StudentDisplayRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	// ListEndedCare reads the archive's directory half: every child whose
	// enrolment interval ran out before asOf, newest last care day first,
	// narrowed by search and classes and paged. It hands each row to each and
	// returns the total.
	ListEndedCare(ctx context.Context, db bun.IDB, tenantID int64, asOf, search string, classes []string, page, pageSize int,
		each func(studentID int64, firstName, lastName, schoolClass string, lastCareDay calendar.Date)) (int, error)
	// LockCareExitPeople locks the person rows a care-exit preview quotes.
	LockCareExitPeople(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64) error
	// ListCareStudents reads the given children, or every child in care on
	// onDay, for the booking evaluation. A child without enrolment interval
	// in inactiveStatus is no candidate; activeStatus keeps a child with a
	// future enrolment start one.
	ListCareStudents(ctx context.Context, db bun.IDB, tenantID int64, onDay string, studentIDs []int64, inactiveStatus, activeStatus string,
		each func(studentID int64, firstName, lastName, schoolClass string, enrolledUntil *calendar.Date)) error
}

// DirectoryStore is the Care Plan store with the student directory bound: the
// reads and writes that resolve a child through the directory.
type DirectoryStore struct {
	*Store
	directory StudentDirectory
}

// NewDirectoryStore binds the student directory to store.
func NewDirectoryStore(store *Store, directory StudentDirectory) *DirectoryStore {
	if store == nil || directory == nil {
		panic("care plan postgres: store and student directory are required")
	}
	return &DirectoryStore{Store: store, directory: directory}
}
