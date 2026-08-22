package students_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestGetArrivalSettings(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)

	t.Run("weekly plan supplies care days by default", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/arrival-settings", nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), `"care_days_source":"weekly_plan"`)
	})

	t.Run("bookings supply care days in booking mode", func(t *testing.T) {
		ctx := testpkg.Ctx(t)
		require.NoError(t, tc.services.Settings.SetValue(
			ctx,
			configModel.KeyEnrollmentBookingsAuthoritative,
			true,
			nil,
			nil,
		))
		t.Cleanup(func() {
			require.NoError(t, tc.services.Settings.ResetValue(
				testpkg.Ctx(t),
				configModel.KeyEnrollmentBookingsAuthoritative,
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

func TestGetClassArrivalTimesUsesStandardEnvelope(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	req := testutil.NewRequest("GET", "/class-arrival-times/Klasse%201b", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"status":"success"`)
	assert.Contains(t, rr.Body.String(), `"data":{"school_class":"Klasse 1b","times":{}}`)
}
