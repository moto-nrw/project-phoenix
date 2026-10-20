package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

type Store interface {
	List(context.Context, int64, int) ([]domain.Group, domain.OperationStats, error)
	FindByID(context.Context, int64) (domain.Group, bool, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]domain.Group, domain.OperationStats, error)
	// CountStudentTransitionHistory counts the grade-transition ledger rows
	// of the tenant that still carry the child's name.
	CountStudentTransitionHistory(ctx context.Context, tenantID, studentID int64) (int, domain.OperationStats, error)
	// AnonymizeStudentTransitionHistory replaces the child's name and clears
	// the tag on the tenant's ledger rows; returns the rows changed.
	AnonymizeStudentTransitionHistory(ctx context.Context, tenantID, studentID int64) (int64, domain.OperationStats, error)
	TransitionStore
}

// TransitionStore persists the grade-transition drafts and ledgers
// (education.grade_transitions, _mappings, _history, _class_teachers,
// _class_list_entries). Every operation runs on the caller's ambient tenant
// transaction and takes the tenant explicitly.
type TransitionStore interface {
	// FindTransition returns the transition with its mappings; found is
	// false when the tenant has no such row. lock "update" takes the row
	// FOR UPDATE.
	FindTransition(ctx context.Context, tenantID, id int64, lock string) (schoolstructure.Transition, bool, domain.OperationStats, error)
	ListTransitions(ctx context.Context, tenantID int64, filter schoolstructure.TransitionFilter) ([]schoolstructure.Transition, int, domain.OperationStats, error)
	InsertTransition(ctx context.Context, tenantID int64, draft schoolstructure.TransitionDraft) (schoolstructure.Transition, domain.OperationStats, error)
	// UpdateTransitionFields rewrites academic year and notes of a DRAFT;
	// rows is 0 when the transition does not exist or is no longer a draft.
	UpdateTransitionFields(ctx context.Context, tenantID, id int64, academicYear string, notes *string) (int64, domain.OperationStats, error)
	ReplaceMappings(ctx context.Context, tenantID, transitionID int64, mappings []schoolstructure.TransitionMappingInput) ([]schoolstructure.TransitionMapping, domain.OperationStats, error)
	// DeleteTransition removes a DRAFT with its mappings; rows is 0 when the
	// transition does not exist or is no longer a draft.
	DeleteTransition(ctx context.Context, tenantID, id int64) (int64, domain.OperationStats, error)
	// LockTransitions takes the tenant-wide transaction-scoped transition
	// gate shared with the timetable materializer.
	LockTransitions(ctx context.Context, tenantID int64) (domain.OperationStats, error)
	// LockLatestApplied returns the most recently applied transition FOR
	// UPDATE; found is false when none is applied.
	LockLatestApplied(ctx context.Context, tenantID int64) (schoolstructure.Transition, bool, domain.OperationStats, error)
	// MarkApplied flips a draft to applied and records the roster baseline;
	// rows is 0 when the row is no longer a draft.
	MarkApplied(ctx context.Context, tenantID, id, accountID int64, at time.Time, rosterBaselineInstanceID *int64) (int64, domain.OperationStats, error)
	// MarkReverted flips an applied transition to reverted; rows is 0 when
	// the row is not applied.
	MarkReverted(ctx context.Context, tenantID, id, accountID int64, at time.Time) (int64, domain.OperationStats, error)
	InsertHistory(ctx context.Context, tenantID int64, entries []schoolstructure.TransitionHistoryEntry) (domain.OperationStats, error)
	ListHistory(ctx context.Context, tenantID, transitionID int64) ([]schoolstructure.TransitionHistoryEntry, domain.OperationStats, error)
	InsertClassTeacherLedger(ctx context.Context, tenantID int64, entries []schoolstructure.TransitionClassTeacherEntry) (domain.OperationStats, error)
	ListClassTeacherLedger(ctx context.Context, tenantID, transitionID int64) ([]schoolstructure.TransitionClassTeacherEntry, domain.OperationStats, error)
	InsertClassListLedger(ctx context.Context, tenantID int64, entries []schoolstructure.TransitionClassListEntry) (domain.OperationStats, error)
	ListClassListLedger(ctx context.Context, tenantID, transitionID int64) ([]schoolstructure.TransitionClassListEntry, domain.OperationStats, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
