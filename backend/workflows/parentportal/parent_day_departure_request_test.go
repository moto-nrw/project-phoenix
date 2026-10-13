package parentportal_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Day Abholart (#3945): the one-day request may ask for a different Abholart,
// with or without a different Gehzeit.

func withDepartureModeRequests(enabled bool) func(*parentportalcompose.Dependencies) {
	return func(cfg *parentportalcompose.Dependencies) {
		cfg.Settings = parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentPickupChangeEnabled:    true,
				configModels.KeyParentCareModeRequestEnabled: enabled,
			},
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		}
	}
}

func TestDayDepartureRequestStoresTheModeAndTheDayBefore(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildPickupChangeServiceWithRequests(t, withDepartureModeRequests(true))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)

	created, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID,
		timezone.TodayDate().AddDays(3), time.Time{}, "pickup", "Die Oma holt ab", nil)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(created.Payload, &payload))
	assert.Equal(t, "pickup", payload["departure_mode"])
	assert.NotContains(t, payload, "pickup_time", "an Abholart-only request leaves the Gehzeit untouched")
	assert.Equal(t, []any{"alone"}, payload["previous_departure_modes"],
		"a child without a weekly plan goes home alone, and the request records that")
}

func TestDayDepartureRequestNeedsTheSchoolsAbholartSetting(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildPickupChangeServiceWithRequests(t, withDepartureModeRequests(false))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)
	date := timezone.TodayDate().AddDays(3)

	_, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, time.Time{}, "bus", "Ausflug", nil)
	require.ErrorIs(t, err, parentService.ErrCareRequestFieldDisabled)

	// The Gehzeit alone stays open: the school only closed the Abholart.
	pickup := timezone.NormalizeWallClock(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC))
	_, err = svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, pickup, "", "Arzttermin", nil)
	require.NoError(t, err)
}

func TestDayDepartureRequestRefusesInBegleitung(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildPickupChangeServiceWithRequests(t, withDepartureModeRequests(true))
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.SubmitPickupChangeRequest(testpkg.Ctx(t), chain.AccountID, chain.StudentID,
		timezone.TodayDate().AddDays(3), time.Time{}, "accompanied", "Mit dem Nachbarskind", nil)
	require.ErrorIs(t, err, parentService.ErrInvalidCareRequestPayload)
}

// A day Abholart the OGS already set blocks a parent's Abholart request for
// that day, but not a request for the Gehzeit alone.
func TestDayDepartureRequestRespectsTheOGSDayValue(t *testing.T) {
	t.Parallel()

	svc, db, repos := buildPickupChangeServiceWithRequests(t, withDepartureModeRequests(true))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Day", "Departure")
	ctx := testpkg.Ctx(t)
	date := timezone.TodayDate().AddDays(4)

	staffID := staff.ID
	_, err := repos.CarePlan().SaveDepartureException(ctx, careplan.DepartureException{
		StudentID: chain.StudentID, ExceptionDate: careplan.Date(date), DepartureMode: careplan.DepartureModeBus, CreatedBy: &staffID,
	})
	require.NoError(t, err)

	_, err = svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, time.Time{}, "pickup", "Oma holt ab", nil)
	require.ErrorIs(t, err, parentService.ErrCareExceptionConflict)

	pickup := timezone.NormalizeWallClock(time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC))
	_, err = svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, pickup, "", "Arzttermin", nil)
	require.NoError(t, err)
}
