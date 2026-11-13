//go:build testhooks

package data_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	dataAPI "github.com/moto-ogs/project-phoenix/modules/devicescan/http/data"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// This is the real Membership -> Workforce read failure, not a failed teacher
// query. E1 (#5860) removes this dependency because the kiosk DTO has no Employment fields.
func TestKioskTeachersIgnoreUnusedEmploymentReadFailure(t *testing.T) {
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
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		Status string `json:"status"`
		Data   []struct {
			StaffID     int64  `json:"staff_id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "success", body.Status)
	require.Len(t, body.Data, 1)
	require.Equal(t, teacher.StaffID, body.Data[0].StaffID)
	require.Equal(t, "Employment Failure", body.Data[0].DisplayName)
	require.Zero(t, *employmentCalls)
}
