//go:build testhooks

package data_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	dataAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/data"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// This is the real Membership -> Workforce read failure, not a failed teacher
// query. Even though the kiosk DTO has no employment fields, it must remain 500.
func TestKioskTeachersPreserveEmploymentReadFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, db, "Employment", "Failure")
	database := testpkg.NewDatabase(t, db)
	directory, employmentCalls, err := api.NewIoTTeacherDirectoryWithEmploymentFailure(database, errors.New("employment owner unavailable"))
	require.NoError(t, err)
	router := dataAPI.NewResource(directory, nil, nil, testRuntime()).TeachersRouter()
	response := httptest.NewRecorder()
	require.NoError(t, testpkg.WithinCurrentTenant(testpkg.Ctx(t), func(ctx context.Context) error {
		request := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
		testutil.WithDeviceIdentity(teacher.StaffID, "employment-failure-device")(request)
		router.ServeHTTP(response, request)
		return nil
	}))
	t.Logf("teacher roster employment failure: status=%d calls=%d body=%s", response.Code, *employmentCalls, response.Body.String())
	require.Equal(t, http.StatusInternalServerError, response.Code)
	var problem struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
	require.Equal(t, "general.server", problem.Code)
	require.Equal(t, 1, *employmentCalls)
}
