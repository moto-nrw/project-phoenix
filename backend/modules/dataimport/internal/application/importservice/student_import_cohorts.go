package importpkg

import (
	"context"
	"fmt"

	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
)

// A Klasse or Gruppe an update row changes moves the child between the
// Regeltermine in the attempt's transaction, as every other writer of the
// child's Klasse and Gruppe does (#6951). The attempt takes the gates before
// its first row locks a child and follows all changed children at once
// after its last row.

var _ ports.RowWriteFollowUp = (*StudentImportConfig)(nil)

// cohortChangeLog collects the changes of one attempt. A failed row write
// ends the attempt and its transaction, so every entry belongs to a write
// that will commit with the follow-up.
type cohortChangeLog struct {
	changes []ports.CohortChange
}

type cohortChangeLogKey struct{}

// BeginRowWrites takes the class-writes and recurrence gates for an attempt
// that may update children. A create import changes no child's Klasse.
func (c *StudentImportConfig) BeginRowWrites(ctx context.Context) (context.Context, func(context.Context) error, error) {
	done := func(context.Context) error { return nil }
	if c.FollowCohortChanges == nil || c.LockCohortWrites == nil || importModeFromContext(ctx) == importModels.ImportModeCreate {
		return ctx, done, nil
	}
	if err := c.LockCohortWrites(ctx); err != nil {
		return ctx, done, fmt.Errorf("lock class and recurrence gates: %w", err)
	}
	log := &cohortChangeLog{}
	return context.WithValue(ctx, cohortChangeLogKey{}, log), func(ctx context.Context) error {
		if len(log.changes) == 0 {
			return nil
		}
		if err := c.FollowCohortChanges(ctx, log.changes, c.todayDate()); err != nil {
			return fmt.Errorf("follow class and group changes: %w", err)
		}
		return nil
	}, nil
}

// noteCohortChange records the child when the row changed its Klasse or
// Gruppe; a write outside a follow-up attempt records nothing.
func noteCohortChange(ctx context.Context, record ports.EnrollmentRecord, class string, patch ports.EnrollmentProfilePatch) {
	log, ok := ctx.Value(cohortChangeLogKey{}).(*cohortChangeLog)
	if !ok {
		return
	}
	change := ports.CohortChange{
		StudentID: record.ID, SchoolClassChanged: class != record.SchoolClass, PreviousSchoolClass: record.SchoolClass,
		GroupChanged: patch.GroupIDSet && !sameGroup(record.GroupID, patch.GroupID), PreviousGroupID: record.GroupID,
	}
	if change.SchoolClassChanged || change.GroupChanged {
		log.changes = append(log.changes, change)
	}
}

func sameGroup(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
