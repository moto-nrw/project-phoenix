package contracttest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/realtime"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A care end ends the child's bookings and removes the planned rows after the
// last care day; its withdrawal puts both back. Other open tabs (Betreuungsplan,
// „Heute geplant“, Aktuelle Aufsicht, the hints for children without a
// matching booking) refetch on staffing_deviation_changed, so each committed
// change announces it once (#5554).
func newAnnouncingCareLifecycleAt(t *testing.T, db *bun.DB, today calendar.Date, bookingsAuthoritative bool) (careplan.CareLifecycle, *testpkg.RecordingBroadcaster) {
	t.Helper()
	broadcaster := testpkg.NewRecordingBroadcaster()
	clock := func() calendar.Date { return today }
	repos, err := api.NewCareLifecycleTestRepositories(testpkg.NewDatabase(t, db), nil)
	require.NoError(t, err)
	svc, err := repos.NewCareLifecycle(api.CareLifecycleTestConfig{
		Audit:                 api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAudit(testpkg.NewDatabase(t, db))),
		BookingsAuthoritative: func(context.Context) (bool, error) { return bookingsAuthoritative, nil },
		Today:                 clock,
		PlanningStaffing:      api.NewStudentTemplateDaysTestModule(testpkg.NewDatabase(t, db), clock, owners.NewStaffingAnnouncer(broadcaster)),
	})
	require.NoError(t, err)
	return svc, broadcaster
}

func staffingAnnouncements(t *testing.T, broadcaster *testpkg.RecordingBroadcaster, tenantID int64) []testpkg.BroadcastCall {
	t.Helper()
	var announced []testpkg.BroadcastCall
	for _, call := range broadcaster.CallsByMethod("tenant") {
		if call.Event.Type == realtime.EventStaffingDeviationChanged {
			require.Equal(t, tenantID, call.TenantID)
			require.NotNil(t, call.Event.Data.Source)
			assert.Nil(t, call.Event.Data.StudentID, "the event names no child")
			announced = append(announced, call)
		}
	}
	return announced
}

func TestCareLifecycle_ConfirmAndCancelAnnounceToStaffingViews(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	today := calendar.NewDate(2026, 8, 24)
	svc, broadcaster := newAnnouncingCareLifecycleAt(t, db, today, false)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Lina", "Brandt", "2b")
	input := careplan.CareExitInput{StudentIDs: []int64{student.ID}, LastCareDay: today.AddDays(10), Reason: careplan.CareExitReasonMovedAway}

	// A refused confirmation rolls back and wakes nobody.
	_, err := testpkg.TenantQuery3(t, ctx, db, svc.Confirm, "stale-token", input, actorID)
	require.ErrorIs(t, err, careplan.ErrCareExitPreviewChanged)
	assert.Empty(t, staffingAnnouncements(t, broadcaster, testpkg.Tenant(t)))

	endCare(t, ctx, svc, actorID, input)
	require.Len(t, staffingAnnouncements(t, broadcaster, testpkg.Tenant(t)), 1, "Betreuung beenden wakes the staffing views once")

	_, err = testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.NoError(t, err)
	require.Len(t, staffingAnnouncements(t, broadcaster, testpkg.Tenant(t)), 2, "the withdrawal wakes them again")

	// A refused second withdrawal changes nothing and announces nothing.
	_, err = testpkg.TenantQuery2(t, ctx, db, svc.Cancel, []int64{student.ID}, actorID)
	require.ErrorIs(t, err, careplan.ErrCareExitNotPlanned)
	assert.Len(t, staffingAnnouncements(t, broadcaster, testpkg.Tenant(t)), 2)
}

func TestCareLifecycle_DueEffectsAnnounceToStaffingViews(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	today := calendar.NewDate(2026, 8, 24)
	svc, broadcaster := newAnnouncingCareLifecycleAt(t, db, today, false)
	actorID := careActor(t, db)
	student := testpkg.CreateTestStudent(t, db, "Jonas", "Brandt", "2b")

	// Nobody is due: the effect pass wakes nobody.
	_, err := testpkg.TenantQuery1(t, ctx, db, svc.ApplyDueEffects, today)
	require.NoError(t, err)
	assert.Empty(t, staffingAnnouncements(t, broadcaster, testpkg.Tenant(t)))

	endCare(t, ctx, svc, actorID, careplan.CareExitInput{StudentIDs: []int64{student.ID}, LastCareDay: today, Reason: careplan.CareExitReasonMovedAway})
	require.Len(t, staffingAnnouncements(t, broadcaster, testpkg.Tenant(t)), 1)

	makeExitEffective(t, db, student.ID, today.AddDays(-1))
	applied, err := testpkg.TenantQuery1(t, ctx, db, svc.ApplyDueEffects, today)
	require.NoError(t, err)
	require.Equal(t, 1, applied)
	assert.Len(t, staffingAnnouncements(t, broadcaster, testpkg.Tenant(t)), 2, "the effect day removes late plans and wakes the views")
}

func TestCareLifecycle_BookingExpiryAnnouncesToStaffingViews(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	today := calendar.NewDate(2026, 8, 24)
	svc, broadcaster := newAnnouncingCareLifecycleAt(t, db, today, true)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Mila", "Ablauf", "2b")
	expiresOn := today
	createCareBooking(t, db, scope, student.ID, "staffing-expiry", nil, &expiresOn)

	applied, err := testpkg.TenantQuery1(t, scope.Context(), db, svc.ApplyDueEffects, today)
	require.NoError(t, err)
	assert.Zero(t, applied, "the booking expiry does not deactivate the child")
	require.Len(t, staffingAnnouncements(t, broadcaster, scope.TenantID), 1, "the new withdrawal boundary wakes the staffing views")

	_, err = testpkg.TenantQuery1(t, scope.Context(), db, svc.ApplyDueEffects, today)
	require.NoError(t, err)
	assert.Len(t, staffingAnnouncements(t, broadcaster, scope.TenantID), 1, "an unchanged booking expiry wakes nobody")
}

// Switching booking-led care on or off plans or drops the withdrawal tasks
// whose first bookingless day bounds who still counts as cared for, so the
// planned child counts of the staffing views move with it.
func TestCareLifecycle_BookingAuthoritySwitchAnnouncesToStaffingViews(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	today := calendar.NewDate(2026, 8, 24)
	svc, broadcaster := newAnnouncingCareLifecycleAt(t, db, today, false)
	student := testpkg.CreateTestStudentForTenant(t, db, scope.TenantID, "Nora", "Umstellung", "2b")
	expiresOn := today.AddDays(5)
	createCareBooking(t, db, scope, student.ID, "staffing-switch", nil, &expiresOn)

	_, err := testpkg.TenantQuery2(t, scope.Context(), db, svc.ApplyBookingAuthoritySetting, today, true)
	require.NoError(t, err)
	require.Len(t, staffingAnnouncements(t, broadcaster, scope.TenantID), 1, "switching on wakes the staffing views")

	_, err = testpkg.TenantQuery2(t, scope.Context(), db, svc.ApplyBookingAuthoritySetting, today, true)
	require.NoError(t, err)
	assert.Len(t, staffingAnnouncements(t, broadcaster, scope.TenantID), 1, "an unchanged enabled setting wakes nobody")

	_, err = testpkg.TenantQuery2(t, scope.Context(), db, svc.ApplyBookingAuthoritySetting, today, false)
	require.NoError(t, err)
	require.Len(t, staffingAnnouncements(t, broadcaster, scope.TenantID), 2, "switching off wakes the staffing views")

	_, err = testpkg.TenantQuery2(t, scope.Context(), db, svc.ApplyBookingAuthoritySetting, today, false)
	require.NoError(t, err)
	assert.Len(t, staffingAnnouncements(t, broadcaster, scope.TenantID), 2, "an unchanged disabled setting wakes nobody")
}
