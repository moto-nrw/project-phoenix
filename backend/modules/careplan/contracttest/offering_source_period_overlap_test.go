package contracttest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A booking phase that follows the school year (2026-09-01 .. 2027-07-31)
// while the planning period ends earlier (2027-07-07) is the normal setup of
// a school. The Regeltermin checks, the connect preview and the roster
// writers count the offering for the overlap instead of dropping it.
var (
	overlapPeriodStart = calendar.NewDate(2026, 8, 1)
	overlapPeriodEnd   = calendar.NewDate(2027, 7, 7)
)

func TestOfferingSourcePeriodOverlap_ReadsCountTheOverlap(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), ctx, testpkg.Tenant(t),
		uniqueSchemaName(t.Name()), overlapPeriodStart, overlapPeriodEnd)
	offering := createSourceOffering(t, env, "UeberlappLesen", nil)
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "overlap-read@example.com", "Ida", 2)
	// A second child books from 2027-07-10, after the planning period ends:
	// nothing of that booking falls into the period, so it is not counted.
	_, lateChildID := submitAndApproveOfferingChild(t, env, offering.ID, "overlap-read-late@example.com", "Lea", 2)
	_, err := env.db.NewRaw(
		`UPDATE enrollment.care_offering_bookings SET valid_from = ? WHERE request_child_id = ? AND care_offering_id = ?`,
		calendar.NewDate(2027, 7, 10), lateChildID, offering.ID,
	).Exec(ctx)
	require.NoError(t, err)
	editor, ok := env.bookings.(careplan.OfferingSourceEditor)
	require.True(t, ok)

	options, err := testpkg.TenantQuery1(t, ctx, env.db, editor.ListOfferingSourceOptions, &period.ID)
	require.NoError(t, err)
	optionIDs := make([]int64, 0, len(options))
	for _, option := range options {
		optionIDs = append(optionIDs, option.ID)
	}
	assert.Contains(t, optionIDs, offering.ID, "the picker offers the offering")
	for _, option := range options {
		if option.ID == offering.ID {
			assert.Equal(t, 1, option.TotalCount, "a booking starting after the period end is not counted")
		}
	}

	counts, err := testpkg.TenantQuery2(t, ctx, env.db, editor.CombinedOfferingSourceCounts, []int64{offering.ID}, &period.ID)
	require.NoError(t, err, "the connect preview must not refuse an overlapping phase")
	assert.Equal(t, 1, counts.TotalCount, "a booking starting after the period end is not counted")

	bookings, err := testpkg.TenantQuery2(t, ctx, env.db, editor.OfferingWeekdayBookings, &period.ID, offeringResyncToday)
	require.NoError(t, err)
	var booking *careplan.OfferingWeekdayBooking
	for i := range bookings {
		if bookings[i].OfferingID == offering.ID && bookings[i].StudentID == studentID {
			booking = &bookings[i]
		}
	}
	require.NotNil(t, booking, "the gap and deviation checks must see the booking")
	require.Len(t, booking.Windows, 1)
	assert.Equal(t, calendar.Date(env.sourcePhase.ServiceStartDate), *booking.Windows[0].ValidFrom)
	assert.Equal(t, overlapPeriodEnd.AddDays(1), *booking.Windows[0].ValidUntil, "the window stops with the planning period")

	feeds, err := testpkg.TenantQuery1(t, ctx, env.db, editor.TemplateRosterMaintenanceFeeds, []careplan.TemplateRosterFeedQuery{{
		TemplateID: 987654321, CalendarPeriodID: &period.ID, SourceCareOfferingIDs: []int64{offering.ID},
	}})
	require.NoError(t, err)
	require.Len(t, feeds[987654321].Sources, 1)
	assert.False(t, feeds[987654321].Sources[0].IsInvalid, "an overlapping source keeps the roster automatic")
}

func TestOfferingSourcePeriodOverlap_RostersStopAtThePeriodEnd(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), ctx, testpkg.Tenant(t),
		uniqueSchemaName(t.Name()), overlapPeriodStart, overlapPeriodEnd)
	offering := createSourceOffering(t, env, "UeberlappSchreiben", nil)
	template := createSourcedTemplate(t, env, "UeberlappTermin", offering.ID, nil, period.ID)

	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "overlap-write@example.com", "Ole", 2)

	approved := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, approved, 1, "the approval plans the child on the sourced Regeltermin")
	assert.Equal(t, studentID, approved[0].StudentID)
	assert.Equal(t, string(calendar.Date(env.sourcePhase.ServiceStartDate)), approved[0].ValidFrom)
	require.NotNil(t, approved[0].ValidUntil)
	assert.Equal(t, string(overlapPeriodEnd.AddDays(1)), *approved[0].ValidUntil, "the approval row stops with the planning period")

	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx, timetable.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offering.ID},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    offeringResyncToday,
	}))
	resynced := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, resynced, 1, "the resync keeps the approval row")
	assert.Equal(t, approved[0].ID, resynced[0].ID)
	require.NotNil(t, resynced[0].ValidUntil)
	assert.Equal(t, string(overlapPeriodEnd.AddDays(1)), *resynced[0].ValidUntil)
}

func TestOfferingSourcePeriodOverlap_NoOverlapStaysExcluded(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	// The planning period ends the day before the phase's first service day.
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), ctx, testpkg.Tenant(t),
		uniqueSchemaName(t.Name()), overlapPeriodStart, calendar.Date(env.sourcePhase.ServiceStartDate).AddDays(-1))
	offering := createSourceOffering(t, env, "OhneUeberlapp", nil)
	submitAndApproveOfferingChild(t, env, offering.ID, "overlap-none@example.com", "Nia", 2)
	editor, ok := env.bookings.(careplan.OfferingSourceEditor)
	require.True(t, ok)

	options, err := testpkg.TenantQuery1(t, ctx, env.db, editor.ListOfferingSourceOptions, &period.ID)
	require.NoError(t, err)
	for _, option := range options {
		assert.NotEqual(t, offering.ID, option.ID, "the picker must not offer an offering it would refuse")
	}
	_, err = testpkg.TenantQuery2(t, ctx, env.db, editor.CombinedOfferingSourceCounts, []int64{offering.ID}, &period.ID)
	require.ErrorIs(t, err, timetable.ErrOfferingSourceInvalid)
	bookings, err := testpkg.TenantQuery2(t, ctx, env.db, editor.OfferingWeekdayBookings, &period.ID, offeringResyncToday)
	require.NoError(t, err)
	for _, booking := range bookings {
		assert.NotEqual(t, offering.ID, booking.OfferingID)
	}
}
