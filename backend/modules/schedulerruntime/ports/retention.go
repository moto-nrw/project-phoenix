package ports

import (
	"context"
	"time"
)

// CleanupRunLedger keeps when a nightly cleanup job last succeeded for a
// school (#3950). Both methods join the tenant transaction in ctx, so a
// recorded success commits or rolls back together with the cleanup itself.
type CleanupRunLedger interface {
	LastCleanupSucceeded(ctx context.Context, jobID string) (time.Time, bool, error)
	RecordCleanupSucceeded(ctx context.Context, jobID string, at time.Time) error
}

// RetentionStep is one owner's deletion in the nightly cleanup of a school
// (#3950). The owner resolves its own retention window and deletes in the
// tenant transaction of ctx. New data kinds join the nightly cleanup as a
// step, so the scheduler needs no further dependency per owner.
type RetentionStep struct {
	// Kind names the data kind in the logs and in the deletion log.
	Kind string
	Run  func(context.Context) (RetentionOutcome, error)
	// LogsSubjects marks a step that writes one deletion-log entry per
	// deleted subject itself, like the automatic deletion of a child
	// (#3962); the run then adds no entry for the data kind.
	LogsSubjects bool
}

// RetentionOutcome reports how many rows one step deleted and the retention
// window it applied.
type RetentionOutcome struct {
	Deleted       int
	RetentionDays int
}

// RetentionLog records one step's deletion in the deletion log of the school
// in ctx: the data kind and the number of rows, never their content.
type RetentionLog func(ctx context.Context, kind string, outcome RetentionOutcome) error

// SubjectRetention is the automatic deletion of whole subjects of a school,
// children (#3962) or staff members (#3963), which writes one deletion-log
// entry per deleted subject itself. A nil Run leaves the deletion out.
type SubjectRetention struct {
	Kind string
	Run  func(context.Context) (RetentionOutcome, error)
}

// RetentionPlan orders the deletions of one school's nightly cleanup. The
// automatic deletion of children runs first, the deletion of staff members
// with it, and the data kinds after them: their shorter windows then apply
// to the rows of the children and staff members that stay.
func RetentionPlan(children, staff SubjectRetention, dataKinds []RetentionStep) []RetentionStep {
	var steps []RetentionStep
	for _, subjects := range []SubjectRetention{children, staff} {
		if subjects.Run != nil {
			steps = append(steps, RetentionStep{Kind: subjects.Kind, Run: subjects.Run, LogsSubjects: true})
		}
	}
	return append(steps, dataKinds...)
}
