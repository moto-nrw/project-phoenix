package students_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestGetArrivalSettings(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("weekly plan supplies care days by default", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/arrival-settings", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"care_days_source":"weekly_plan"`)
	})

	t.Run("bookings supply care days in booking mode", func(t *testing.T) {
		ctx := testpkg.Ctx(t)
		require.NoError(t, tc.settings().SetValue(
			ctx,
			settings.KeyEnrollmentBookingsAuthoritative,
			true,
			nil,
			nil,
		))
		t.Cleanup(func() {
			require.NoError(t, tc.settings().ResetValue(
				testpkg.Ctx(t),
				settings.KeyEnrollmentBookingsAuthoritative,
				nil,
				nil,
			))
		})

		req := testutil.NewRequest("GET", "/arrival-settings", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"care_days_source":"bookings"`)
	})
}

// The lesson end times a school maintains feed the "nach der 5. Stunde" choice
// of the arrival forms (#3372).
func TestGetArrivalSettingsSchoolPeriods(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("no lesson is offered while the school maintains none", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/arrival-settings", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"school_periods":[]`)
	})

	t.Run("maintained lessons come back in lesson order, gaps left out", func(t *testing.T) {
		ctx := testpkg.Ctx(t)
		for period, endTime := range map[int]string{6: "13:20", 5: "12:35", 4: ""} {
			key := settings.SchoolPeriodEndKey(period)
			require.NoError(t, tc.settings().SetValue(ctx, key, endTime, nil, nil))
			t.Cleanup(func() {
				require.NoError(t, tc.settings().ResetValue(testpkg.Ctx(t), key, nil, nil))
			})
		}

		req := testutil.NewRequest("GET", "/arrival-settings", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(),
			`"school_periods":[{"period":5,"end_time":"12:35"},{"period":6,"end_time":"13:20"}]`)
	})

	t.Run("a value that is no clock time is refused", func(t *testing.T) {
		err := tc.settings().SetValue(
			testpkg.Ctx(t), settings.SchoolPeriodEndKey(1), "nach der Pause", nil, nil)
		require.Error(t, err)
	})
}

// The usual arrival and pickup time a school maintains feed the one-click
// adoption of the weekly plan (#3371).
func TestGetArrivalSettingsCareTimePresets(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("nothing is offered while the school maintains no preset", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/arrival-settings", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"default_arrival_time":""`)
		assert.Contains(t, rr.Body.String(), `"default_pickup_time":""`)
	})

	t.Run("maintained presets come back as clock times", func(t *testing.T) {
		ctx := testpkg.Ctx(t)
		for key, value := range map[string]string{
			settings.KeyCareDefaultArrivalTime: "12:30",
			settings.KeyCareDefaultPickupTime:  "16:00",
		} {
			require.NoError(t, tc.settings().SetValue(ctx, key, value, nil, nil))
			t.Cleanup(func() {
				require.NoError(t, tc.settings().ResetValue(testpkg.Ctx(t), key, nil, nil))
			})
		}

		req := testutil.NewRequest("GET", "/arrival-settings", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"default_arrival_time":"12:30"`)
		assert.Contains(t, rr.Body.String(), `"default_pickup_time":"16:00"`)
	})

	t.Run("a value that is no clock time is refused", func(t *testing.T) {
		err := tc.settings().SetValue(
			testpkg.Ctx(t), settings.KeyCareDefaultPickupTime, "nachmittags", nil, nil)
		require.Error(t, err)
	})
}

func TestGetClassArrivalTimesUsesStandardEnvelope(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	req := testutil.NewRequest("GET", "/class-arrival-times/Klasse%201b", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"status":"success"`)
	assert.Contains(t, rr.Body.String(), `"data":{"school_class":"Klasse 1b","times":{}}`)
}
