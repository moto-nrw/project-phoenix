package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// MasterDataRequestRows is the slice of Care Plan's own request persistence a
// Stammdaten decision needs: read, lock, decide and re-decide one row.
type MasterDataRequestRows interface {
	FindStudentDataRequest(ctx context.Context, id int64, lock bool) (careplan.StudentDataChangeRequest, error)
	FindPendingStudentDataRequest(ctx context.Context, id int64) (careplan.StudentDataChangeRequest, error)
	DecideStudentDataRequest(ctx context.Context, decision careplan.StudentDataRequestDecision) error
	RedecideStudentDataRequest(ctx context.Context, decision careplan.StudentDataRequestDecision) error
}

// MasterDataStudent is the People Directory row a Stammdaten decision gates
// on and writes: the review projection plus the departure plan.
type MasterDataStudent struct {
	ReviewStudent
	DepartureModes domain.DepartureModes
}

// MasterDataPerson is the person row behind a child. Birthday is the
// canonical YYYY-MM-DD day, empty when none is recorded.
type MasterDataPerson struct {
	ID        int64
	FirstName string
	LastName  string
	Birthday  string
}

// MasterDataStudentWrite is what a decision writes to the locked student
// row. A nil field leaves that column untouched, so a class change never
// rewrites the departure plan.
type MasterDataStudentWrite struct {
	SchoolClass    *string
	DepartureModes domain.DepartureModes
}

// MasterDataStudentChange decides the write from the locked student row. It
// returns an error to refuse the write; the row is then left untouched.
type MasterDataStudentChange func(MasterDataStudent) (MasterDataStudentWrite, error)

// MasterDataPersonChange edits the locked person row in place. It returns an
// error to refuse the write; the row is then left untouched.
type MasterDataPersonChange func(*MasterDataPerson) error

// Steps of a People Directory write, so the decision can label a failure the
// way its callers log and render it.
const (
	MasterDataWriteLoad   = "load"
	MasterDataWriteUpdate = "update"
	MasterDataWriteAudit  = "audit"
)

// MasterDataWriteError reports which step of a People Directory write
// failed. A change function's own error is returned unwrapped.
type MasterDataWriteError struct {
	Step string
	Err  error
}

func (e *MasterDataWriteError) Error() string { return e.Step + ": " + e.Err.Error() }

func (e *MasterDataWriteError) Unwrap() error { return e.Err }

// MasterDataRecords reads, locks and writes the People Directory rows a
// Stammdaten decision applies to. People Directory owns the rows, their
// write path (including the "läuft mit" links a narrowed departure plan
// trims) and the per-child change history; Care Plan owns the decision that
// says what to write. Every call runs in the caller's tenant transaction.
type MasterDataRecords interface {
	// FindStudent reads the child without a lock.
	FindStudent(ctx context.Context, id int64) (MasterDataStudent, error)
	// LockStudent takes the child's row FOR UPDATE, so a concurrent grade
	// transition cannot flip the child underneath a decision.
	LockStudent(ctx context.Context, id int64) (MasterDataStudent, error)
	// FindPerson reads the person behind a child without a lock.
	FindPerson(ctx context.Context, id int64) (MasterDataPerson, error)
	// UpdateStudent locks the child, lets change decide the class or the
	// departure plan to write, writes it and appends the change history under
	// the actor. It reports whether the write trimmed a "läuft mit" link.
	UpdateStudent(ctx context.Context, id, actorAccountID int64, change MasterDataStudentChange) (companionsTrimmed bool, err error)
	// UpdatePerson locks the person, lets change edit the name or birthday
	// and writes it.
	UpdatePerson(ctx context.Context, id int64, change MasterDataPersonChange) error
}

// MasterDataBroadcaster wakes staff tabs after a Stammdaten decision
// committed.
type MasterDataBroadcaster interface {
	// StudentUpdated invalidates child cards and student list/detail caches.
	StudentUpdated(tenantID int64) error
	// StudentCompanionsChanged makes mounted "läuft mit" views refetch.
	StudentCompanionsChanged(tenantID int64) error
}
