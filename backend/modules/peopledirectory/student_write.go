package peopledirectory

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// The two bounds of the enrolment interval a lifecycle tick compares against.
const (
	StudentBoundCareStart = "care_start"
	StudentBoundCareEnd   = "care_end"
)

var (
	// ErrCompanionWouldLoseDeparture refuses a plan change that would leave a
	// linked child with an accompanied weekday and no remaining "mit wem"
	// detail — neither a free-text note nor another link on that day.
	ErrCompanionWouldLoseDeparture = errors.New("companion would lose its departure detail")
	// ErrCompanionLockBusy reports a companion row another transaction holds.
	// It is retriable: the alternative to refusing is a deadlock abort.
	ErrCompanionLockBusy = errors.New("companion row is locked by another write")
)

// StudentPlan is the departure plan a write carries: the unified per-weekday
// mode set plus the three legacy projections clients still send. A nil field
// means "not supplied", which is what keeps a write that touches unrelated
// columns from rewriting the stored plan.
type StudentPlan = departure.Plan

// StudentWrite is one create or update of a child.
type StudentWrite struct {
	// Record is the owned row. On a create its ID is ignored.
	Record StudentRecord
	// Plan is the departure plan this write carries, if any.
	Plan StudentPlan
	// Baseline is the plan the caller's read hydrated, when it had one. It is
	// what distinguishes a field the caller really changed from one that merely
	// rode along on that read, so an unrelated write cannot revert a companion
	// edit that committed in between.
	Baseline *StudentPlan
	// CompanionNote is the free-text "mit wem" detail.
	CompanionNote *string
	// NoteSupplied says the caller spoke about the note at all, which a nil
	// pointer alone cannot express.
	NoteSupplied bool
}

// StudentWriteCommand is the child row's own lifecycle. Each call is one flow:
// it takes the gates and locks in the owner's order, reconciles what the
// departure plan implies, and commits inside the caller's transaction.
type StudentWriteCommand interface {
	// CreateStudent inserts a child and writes its departure plan.
	CreateStudent(context.Context, StudentWrite) (StudentRecord, error)
	// UpdateStudent rewrites a child, trims the companion links its plan no
	// longer allows, and refuses with ErrCompanionWouldLoseDeparture when that
	// would strand a linked child.
	UpdateStudent(context.Context, StudentWrite) (StudentRecord, error)
	// DeleteStudentRecord removes one child row. The permanent-deletion
	// workflow is a different flow and keeps the DeleteStudent name.
	DeleteStudentRecord(context.Context, int64) error
	// VerifyStudentStrandingBatch decides the verdicts a coordinated
	// multi-child write deferred, against the state the whole batch leaves
	// behind. A no-op without an open batch.
	VerifyStudentStrandingBatch(context.Context) error
	// ListStudentCareEnds projects the enrolment upper bound of the given
	// children; a child without one is absent from the result.
	ListStudentCareEnds(context.Context, []int64) (map[int64]string, error)
}

func (m *Module) CreateStudent(ctx context.Context, write StudentWrite) (StudentRecord, error) {
	if write.Record.PersonID <= 0 {
		return StudentRecord{}, invalidStudent("person ID is required")
	}
	return m.engine.CreateStudent(ctx, write)
}

func (m *Module) UpdateStudent(ctx context.Context, write StudentWrite) (StudentRecord, error) {
	if write.Record.ID <= 0 {
		return StudentRecord{}, invalidStudent("student ID is required")
	}
	return m.engine.UpdateStudent(ctx, write)
}

func (m *Module) DeleteStudentRecord(ctx context.Context, studentID int64) error {
	if studentID <= 0 {
		return invalidStudent("student ID is required")
	}
	return m.engine.DeleteStudentRecord(ctx, studentID)
}

func (m *Module) VerifyStudentStrandingBatch(ctx context.Context) error {
	return m.engine.VerifyStudentStrandingBatch(ctx)
}

func (m *Module) ListStudentCareEnds(ctx context.Context, ids []int64) (map[int64]string, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	return m.engine.ListStudentCareEnds(ctx, ids)
}
