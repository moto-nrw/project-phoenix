package enrollmenthttp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-ogs/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-ogs/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// assertPhaseKept checks that a refused delete left the phase and its
// approved request in place.
func assertPhaseKept(t *testing.T, env *decisionTestEnv, requests int) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	_, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().Phase, env.sourcePhase.ID)
	require.NoError(t, err, "the phase stays")
	count, err := testpkg.TenantQuery1(t, ctx, env.db, env.repos.Enrollment().CountPhaseRequests, env.sourcePhase.ID)
	require.NoError(t, err)
	assert.Equal(t, requests, count, "the requests and their bookings stay")
}

// A phase whose bookings still apply today or later is not deleted: the
// delete would end the children's care without the care-end or booking
// process and leave them without a booking and without a completion task
// (#6505, EC-6). The refusal comes before any write. A phase whose bookings
// have ended, or that has ended itself, is still refused for its requests
// (#6836): they stay as evidence.
func TestPhaseService_Delete_RefusesEffectiveBookings(t *testing.T) {
	t.Parallel()

	t.Run("running booking", func(t *testing.T) {
		testpkg.OwnTenant(t)
		env := setupDecisionTest(t)
		offering := createAdjustmentCareOffering(t, env, uniqueSchemaName("Betreuung Loeschen"))
		submitApprovedAdjustmentChild(t, env, "loeschen@example.com", "Loeschen", []*offeringContract.CareOfferingRecord{offering})

		err := testpkg.TenantCommand1(t, testpkg.Ctx(t), env.db, followingPhases(t, env, decisionTestToday).DeletePhase, env.sourcePhase.ID)
		require.ErrorIs(t, err, enrollmentNative.ErrPhaseHasEffectiveBookings)
		assertPhaseKept(t, env, 1)
	})

	t.Run("application not yet approved", func(t *testing.T) {
		testpkg.OwnTenant(t)
		env := setupDecisionTest(t)
		offering := createAdjustmentCareOffering(t, env, uniqueSchemaName("Betreuung Offen"))
		submitWithOffering(t, env, offering, "Offen")

		err := testpkg.TenantCommand1(t, testpkg.Ctx(t), env.db, followingPhases(t, env, decisionTestToday).DeletePhase, env.sourcePhase.ID)
		require.ErrorIs(t, err, enrollmentNative.ErrPhaseHasRequests, "an open application is no care yet, but evidence")
		assertPhaseKept(t, env, 1)
	})

	t.Run("ended phase", func(t *testing.T) {
		testpkg.OwnTenant(t)
		env := setupDecisionTest(t)
		offering := createAdjustmentCareOffering(t, env, uniqueSchemaName("Betreuung Vorbei"))
		submitApprovedAdjustmentChild(t, env, "vorbei@example.com", "Vorbei", []*offeringContract.CareOfferingRecord{offering})
		afterEnd := calendar.Date(env.sourcePhase.ServiceEndDate).AddDays(1)

		err := testpkg.TenantCommand1(t, testpkg.Ctx(t), env.db, followingPhases(t, env, afterEnd).DeletePhase, env.sourcePhase.ID)
		require.ErrorIs(t, err, enrollmentNative.ErrPhaseHasRequests, "an ended phase keeps its requests")
		assertPhaseKept(t, env, 1)
	})

	t.Run("ended bookings", func(t *testing.T) {
		testpkg.OwnTenant(t)
		env := setupDecisionTest(t)
		offering := createAdjustmentCareOffering(t, env, uniqueSchemaName("Betreuung Beendet"))
		_, childID, _ := submitApprovedAdjustmentChild(t, env, "beendet@example.com", "Beendet", []*offeringContract.CareOfferingRecord{offering})
		_, err := testpkg.TenantQuery2(t, testpkg.Ctx(t), env.db, env.reposCare.CarePlan.EndCareOfferingBookings, []int64{childID}, careplan.Date(decisionTestToday))
		require.NoError(t, err)

		err = testpkg.TenantCommand1(t, testpkg.Ctx(t), env.db, followingPhases(t, env, decisionTestToday).DeletePhase, env.sourcePhase.ID)
		require.ErrorIs(t, err, enrollmentNative.ErrPhaseHasRequests, "a booking ending today no longer applies, the request stays")
		assertPhaseKept(t, env, 1)
	})
}

// submitWithOffering submits a one-child request for the offering without
// deciding it.
func submitWithOffering(t *testing.T, env *decisionTestEnv, offering *offeringContract.CareOfferingRecord, lastName string) *enrollmentAPI.SubmitResult {
	t.Helper()
	submitted, err := env.requestSvc.Submit(testpkg.Ctx(t), enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: lastName, GuardianEmail: lastName + "@example.com",
		ConsentFlags: testFormValues(map[string]any{"agb": true, "data_processing": true, "email_contact": true, "photo": true}),
		Children: []enrollmentAPI.SubmitChild{{
			FirstName: "Kind", LastName: lastName, DateOfBirth: calendar.NewDate(2018, 4, 15), TargetGradeLevel: testpkg.Int16Ptr(2),
			OfferingIDs:  []int64{offering.ID},
			OfferingDays: []enrollmentAPI.SubmitOfferingDays{{OfferingID: offering.ID, SelectedDays: []string{"mon"}}},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	return submitted
}

// A delete that waits for an approval of the phase sees the bookings that
// approval wrote. The approval holds the recurrence gate while it books; the
// delete checks only once it holds the gate itself (#6505).
func TestPhaseService_Delete_WaitsForConcurrentApproval(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	lock := owners.MustNewTimetableRecurrenceLock(testpkg.NewDatabase(t, env.db)).LockRecurrenceWrites
	bookings := newBookingsForTest(env.rolloverTestEnv, nil, lock, nil)
	decision := newDecisionServiceForTestWithBookings(t, env.rolloverTestEnv, bookings, nil, lock, nil, nil)
	offering := createAdjustmentCareOffering(t, env, uniqueSchemaName("Betreuung Gleichzeitig"))
	submitted := submitWithOffering(t, env, offering, "Gleichzeitig")
	phases := followingPhases(t, env, decisionTestToday)

	// The approval takes the gate first and books only once the delete
	// waits, so a delete that checked before it held the gate would miss
	// the booking.
	approveErr, deleteErr := whileHolding(t, env,
		lock,
		func(ctx context.Context) error { return phases.DeletePhase(ctx, env.sourcePhase.ID) },
		func(ctx context.Context) error {
			_, err := decision.Decide(ctx, enrollmentAPI.DecideInput{
				RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID,
				Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
			})
			return err
		})
	require.NoError(t, approveErr)
	require.ErrorIs(t, deleteErr, enrollmentNative.ErrPhaseHasEffectiveBookings)
	assertPhaseKept(t, env, 1)
}
