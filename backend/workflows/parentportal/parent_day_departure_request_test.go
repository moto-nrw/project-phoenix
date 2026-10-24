package parentportal_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Day Abholart (#3945): the one-day request may ask for a different Abholart,
// with or without a different Gehzeit.

const dayDepartureRequestToday = calendar.Date("2026-08-24")

// buildDayDepartureRequestService pins both clocks to dayDepartureRequestToday,
// so the requested days stay care days whatever the wall clock says.
func buildDayDepartureRequestService(t *testing.T, departureModeRequests bool) (*parentService.Portal, *bun.DB, *api.RepositoryTestFactory) {
	t.Helper()
	return buildPickupChangeServiceWithRequestOptions(t,
		[]owners.CareRequestOption{owners.WithCareRequestToday(func() calendar.Date { return dayDepartureRequestToday })},
		withDepartureModeRequests(departureModeRequests),
		func(cfg *parentportalcompose.Dependencies) {
			cfg.Now = func() time.Time { return dayDepartureRequestToday.BerlinMidnight().Add(12 * time.Hour) }
		},
	)
}

func withDepartureModeRequests(enabled bool) func(*parentportalcompose.Dependencies) {
	return func(cfg *parentportalcompose.Dependencies) {
		cfg.Settings = parentSettingsStub{
			boolValues: map[string]bool{
				parentService.KeyParentPickupChangeEnabled:    true,
				parentService.KeyParentCareModeRequestEnabled: enabled,
			},
			stringValues: map[string]string{
				parentService.KeyGuardianParentInviteMode: parentService.ParentInviteModeDisabled,
			},
		}
	}
}

func TestDayDepartureRequestStoresTheModeAndTheDayBefore(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildDayDepartureRequestService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)

	created, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID,
		dayDepartureRequestToday.AddDays(3), time.Time{}, "pickup", "Die Oma holt ab", nil)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(created.Payload, &payload))
	assert.Equal(t, "pickup", payload["departure_mode"])
	assert.NotContains(t, payload, "pickup_time", "an Abholart-only request leaves the Gehzeit untouched")
	assert.Equal(t, []any{"alone"}, payload["previous_departure_modes"],
		"a child without a weekly plan goes home alone, and the request records that")
}

func TestDayDepartureRequestStoresTheLegacyDepartureModeBefore(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildDayDepartureRequestService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)
	_, err := db.NewUpdate().TableExpr("users.student_care_profiles").
		Set(`allowed_departure_modes = '{}'::jsonb`).
		Set(`departure_days = '{"mon":"bus"}'::jsonb`).
		Set(`bus_days = '{}'::jsonb`).
		Set(`pickup_days = '{}'::jsonb`).
		Where("membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", chain.StudentID).
		Exec(ctx)
	require.NoError(t, err)

	created, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID,
		dayDepartureRequestToday.AddDays(7), time.Time{}, "pickup", "Die Oma holt ab", nil)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(created.Payload, &payload))
	assert.Equal(t, []any{"bus"}, payload["previous_departure_modes"],
		"the request history reads the legacy weekday departure plan before the change")
}

func TestDayDepartureRequestNeedsTheSchoolsAbholartSetting(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildDayDepartureRequestService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)
	date := dayDepartureRequestToday.AddDays(3)

	_, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, time.Time{}, "bus", "Ausflug", nil)
	require.ErrorIs(t, err, parentService.ErrCareRequestFieldDisabled)

	// The Gehzeit alone stays open: the school only closed the Abholart.
	pickup := calendar.NormalizeWallClock(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC))
	_, err = svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, pickup, "", "Arzttermin", nil)
	require.NoError(t, err)
}

func TestDayDepartureRequestRefusesInBegleitung(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildDayDepartureRequestService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.SubmitPickupChangeRequest(testpkg.Ctx(t), chain.AccountID, chain.StudentID,
		dayDepartureRequestToday.AddDays(3), time.Time{}, "accompanied", "Mit dem Nachbarskind", nil)
	require.ErrorIs(t, err, parentService.ErrInvalidCareRequestPayload)
}

func TestDayDepartureRequestRefusesNonCareWeekend(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildDayDepartureRequestService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	date := dayDepartureRequestToday.AddDays(5)

	_, err := svc.SubmitPickupChangeRequest(testpkg.Ctx(t), chain.AccountID, chain.StudentID,
		date, time.Time{}, "bus", "Ausflug", nil)
	require.ErrorIs(t, err, parentService.ErrCareExceptionNotCareDay)
}

// A day Abholart the OGS already set blocks a parent's Abholart request for
// that day, but not a request for the Gehzeit alone.
func TestDayDepartureRequestRespectsTheOGSDayValue(t *testing.T) {
	t.Parallel()

	svc, db, repos := buildDayDepartureRequestService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Day", "Departure")
	ctx := testpkg.Ctx(t)
	date := dayDepartureRequestToday.AddDays(3)

	staffID := staff.ID
	_, err := repos.CarePlan().SaveDepartureException(ctx, careplan.DepartureException{
		StudentID: chain.StudentID, ExceptionDate: careplan.Date(date), DepartureMode: careplan.DepartureModeBus, CreatedBy: &staffID,
	})
	require.NoError(t, err)

	_, err = svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, time.Time{}, "pickup", "Oma holt ab", nil)
	require.ErrorIs(t, err, parentService.ErrCareExceptionConflict)

	pickup := calendar.NormalizeWallClock(time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC))
	_, err = svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, pickup, "", "Arzttermin", nil)
	require.NoError(t, err)
}
