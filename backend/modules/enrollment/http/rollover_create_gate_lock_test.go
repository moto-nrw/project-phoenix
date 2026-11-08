package enrollmenthttp_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A rollover takes the approval gates before it inserts the follow-up phase.
// The insert's foreign key holds KEY SHARE on the source phase; a rollover
// that inserted first and waited for the recurrence gate in the catalog clone
// later closed a 40P01 cycle with a phase delete that holds the gate and
// waits to delete the source phase (#6168). Now the rollover waits for the
// gate and then finds the source phase gone.
func TestRolloverService_CreatePhaseFromSource_WaitsForPhaseDelete(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	lock := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, db)).LockRecurrenceWrites
	env, decision := setupAutoApproveIntegrationEnvWithDecision(t, nil, lock)
	gatedCatalog := testCareOfferingCatalog(t, env.db, api.WithCareOfferingSettings(env.settings), api.WithCareOfferingRecurrenceLock(lock))
	env.rolloverSvc = newTestRolloverService(owners.EnrollmentRolloverSources{
		Bookings:   requestTestBookingCommands(),
		Phases:     env.repos.Enrollment(),
		Requests:   env.repos.Enrollment(),
		Children:   env.repos.Enrollment(),
		Catalog:    gatedCatalog,
		Outbox:     env.outbox,
		Settings:   env.settings,
		Decisions:  decision,
		ParentsURL: "http://parents.localhost:3000",
		Logger:     slog.Default(),
	})
	phases := NewTestPhases(TestPhaseDependencies{
		Records:                env.repos.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(env.reposCare.CarePlan)},
		LockTemplateRecurrence: lock,
		Resyncer:               &recordingSourcedTemplateResyncer{},
		Logger:                 slog.Default(),
	})
	req := validRolloverRequest(env, enrollmentNative.PhaseRolloverModeOptOut, false)
	req.Name = "rollover-create-gate-order-target"

	deleteErr, rolloverErr := whileHoldingGates(t, env.db,
		lock,
		func(writeCtx context.Context) error {
			_, err := env.rolloverSvc.CreatePhaseFromSource(writeCtx, req)
			return err
		},
		func(holdCtx context.Context) error {
			return phases.DeletePhase(holdCtx, env.sourcePhase.ID)
		},
	)
	require.NoError(t, deleteErr, "the phase delete must not deadlock")
	assert.ErrorIs(t, rolloverErr, enrollmentNative.ErrRolloverSourceNotFound,
		"the rollover must wait for the gate and then miss the deleted source, without 40P01")
}
