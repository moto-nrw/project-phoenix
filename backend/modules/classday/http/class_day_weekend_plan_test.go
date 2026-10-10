package classdayhttp_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// classesWithWeekendPlan reads /classes with the weekend source the HTTP root
// binds over operations.weekend_follows_friday (#3921).
func classesWithWeekendPlan(t *testing.T, f *arrivalExceptionFixture, source calendar.WeekendPlanSource) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/classes", nil)
	req = req.WithContext(calendar.WithWeekendPlan(req.Context(), source))
	return testutil.ExecuteWithAuthPermissions(t, f.router, req, f.claims, []string{classDayReadPermission})
}

func TestSchoolClassesCarryTheWeekendPlan(t *testing.T) {
	t.Parallel()
	f := setupArrivalExceptionFixture(t)

	rec := f.do(t, http.MethodGet, "/classes", "", classDayReadPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"weekend_follows_friday":false`, "without a bound source the weekend stays closed")

	rec = classesWithWeekendPlan(t, f, func(context.Context) (bool, error) { return true, nil })
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"weekend_follows_friday":true`)

	rec = classesWithWeekendPlan(t, f, func(context.Context) (bool, error) { return false, errors.New("settings down") })
	assert.Equal(t, http.StatusInternalServerError, rec.Code, "a failed read is not answered as a closed weekend")
}
