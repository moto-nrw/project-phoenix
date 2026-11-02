package contracttest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A booking change ends or adds the bookings the booking-leftover hints
// (#5102) compare the hand-planned Regeltermine with. Other open tabs refetch
// those hints on staffing_deviation_changed, so the change announces it once
// the transaction committed (#5498).
func TestUpdateChildOfferings_AnnouncesBookingChangeToStaffingViews(t *testing.T) {
	t.Parallel()

	f := setupUnperiodizedTemplateFixture(t, "buchungswechsel-live@example.com")
	broadcaster := testpkg.NewRecordingBroadcaster()
	module, err := api.NewAnnouncingBookingMaterialization(testpkg.NewDatabase(t, f.env.db), testpkg.TenantRuntime(t, f.env.db), broadcaster,
		api.WithBookingCatalog(f.env.offeringCatalog),
		api.WithBookingToday(func() calendar.Date { return decisionTestToday }),
		api.WithBookingSettings(registryDefaultDecisionSettings{}),
		api.WithBookingWithdrawals(newTestCareLifecycle(f.env.db, api.CareLifecycleTestConfig{
			BookingsAuthoritative: testBookingsAuthority(nil),
		})),
	)
	require.NoError(t, err)
	bookings := module.Bookings
	decision := enrollmentTest.WithDecisionUnit(enrollmentTest.NewPublicDecisions(composeDecisions(
		t, f.env.db, f.env.reposEnrollment, f.env.repos, f.env.reposPeople, f.env.reposSchedules, bookings, nil, nil, nil,
	)), fixtureEnrollmentCommand)

	// The child keeps only the template's source offering; the other booking ends.
	_, err = testpkg.TenantQuery1(t, testpkg.Ctx(t), f.env.db, decision.UpdateChildOfferings, enrollmentTest.UpdateChildOfferingsInput{
		RequestID:      f.requestID,
		ChildID:        f.childID,
		ActorAccountID: f.env.creatorID,
		ActorRole:      "admin",
		Reason:         "Spätbetreuung abgemeldet",
		Offerings: []enrollmentTest.OfferingAdjustmentSelection{
			{OfferingID: f.source.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	var announced []testpkg.BroadcastCall
	for _, call := range broadcaster.CallsByMethod("tenant") {
		if call.Event.Type == realtime.EventStaffingDeviationChanged {
			announced = append(announced, call)
		}
	}
	require.Len(t, announced, 1, "a committed booking change wakes the staffing views once")
	assert.Equal(t, testpkg.Tenant(t), announced[0].TenantID)
	require.NotNil(t, announced[0].Event.Data.Source)
	assert.Equal(t, "booking_change", *announced[0].Event.Data.Source)
	assert.Nil(t, announced[0].Event.Data.StudentID, "the event names no child")
}
