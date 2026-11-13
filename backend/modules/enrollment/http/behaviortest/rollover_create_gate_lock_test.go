package enrollmenthttp_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

// lockingCollectionSettings resolves the rollover test settings and takes the
// real class-collection advisory lock, the way the settings service does.
type lockingCollectionSettings struct {
	collectionSettings
	db *bun.DB
}

func (s lockingCollectionSettings) LockClassCollectionPair(ctx context.Context) error {
	key := fmt.Sprintf("enrollment-class-collection:%d", testpkg.ConfigRuntime(s.db).TenantID(ctx))
	return testpkg.ConfigRuntime(s.db).AcquireLock(ctx, key, false)
}

// A class-restricted rollover checks eligibility under the class-collection
// lock before it takes the recurrence gate, in the order a phase update takes
// them. Taking the gates first and the collection lock in the eligibility
// check closed a 40P01 cycle with a phase update that holds the collection
// lock and waits for the recurrence gate (#6168 review).
func TestRolloverService_CreatePhaseFromSource_TakesCollectionLockBeforeGates(t *testing.T) {
	t.Parallel()

	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	lock := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, db)).LockRecurrenceWrites
	env, decision := setupAutoApproveIntegrationEnvWithDecision(t, nil, lock)
	ctx := testpkg.Ctx(t)
	env.sourcePhase.AvailableSchoolClasses = []string{"2a", "2b"}
	env.sourcePhase.EligibleSchoolClasses = []string{"2a", "2b"}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, env.db, env.repos.Enrollment().UpdatePhase, OwnerPhaseForTest(env.sourcePhase)))
	env.settings.boolValues[settingKeyCollectSchoolClass] = true
	settings := lockingCollectionSettings{collectionSettings: collectionSettings{settings: env.settings}, db: env.db}
	phases := NewTestPhases(TestPhaseDependencies{
		Records:                env.repos.Enrollment(),
		Offerings:              phaseOfferings{rows: newCareOfferingFixtures(env.reposCare.CarePlan)},
		LockTemplateRecurrence: lock,
		Resyncer:               &recordingSourcedTemplateResyncer{},
		Settings:               settings,
		Logger:                 slog.Default(),
	})
	gatedCatalog := testCareOfferingCatalog(t, env.db, api.WithCareOfferingSettings(env.settings), api.WithCareOfferingRecurrenceLock(lock))
	env.rolloverSvc = newTestRolloverService(owners.EnrollmentRolloverSources{
		Bookings:         requestTestBookingCommands(),
		Phases:           env.repos.Enrollment(),
		Requests:         env.repos.Enrollment(),
		Children:         env.repos.Enrollment(),
		Catalog:          gatedCatalog,
		PhaseEligibility: phases,
		Outbox:           env.outbox,
		Settings:         env.settings,
		Decisions:        decision,
		ParentsURL:       "http://parents.localhost:3000",
		Logger:           slog.Default(),
	})
	req := validRolloverRequest(env, enrollmentNative.PhaseRolloverModeOptOut, false)
	req.Name = "rollover-collection-lock-order-target"

	updateErr, rolloverErr := whileHoldingGates(t, env.db,
		settings.LockClassCollectionPair,
		func(writeCtx context.Context) error {
			_, err := env.rolloverSvc.CreatePhaseFromSource(writeCtx, req)
			return err
		},
		lock,
	)
	require.NoError(t, updateErr, "the phase update's recurrence gate must not deadlock")
	require.NoError(t, rolloverErr, "the rollover must finish after the phase update")
}
