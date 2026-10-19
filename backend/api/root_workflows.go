package api

import (
	"context"
	"fmt"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"
)

// buildGradeTransition binds the school-year rollover workflow (#2711),
// which re-reconciles the offering-sourced templates' Jahrgang-filtered
// rosters (#2137) through Care Plan's booking materialization.
func buildGradeTransition(
	db transactionruntime.Database, env owners.RootEnvironment, persons peopledirectory.Capability, membership schoolmembership.Capability,
	owner owners.TimetableOwner, resyncOfferingRosters func(context.Context, timezone.Date) error, audit owners.AuditOwner,
) (*gradetransition.Workflow, error) {
	structure, presence, err := NewGradeTransitionOwners(db)
	if err != nil {
		return nil, fmt.Errorf("compose grade transition owners: %w", err)
	}
	workflow, err := gradetransitioncompose.New(gradetransitioncompose.Dependencies{
		DB: db.Pool(), Structure: structure, Presence: presence, Directory: persons, Membership: membership, Rosters: owner.RosterReconciler,
		LockRecurrenceWrites:  owner.RecurrenceLock.LockRecurrenceWrites,
		ResyncOfferingRosters: resyncOfferingRosters,
		Logger:                env.Logger.With("workflow", "grade_transition"), Audit: audit.Command, ClassListEntryRecord: GradeTransitionClassListRecord,
		Clock: env.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("compose grade transition workflow: %w", err)
	}
	return workflow, nil
}
