package enrollmenthttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A stale offering id (deleted in another tab, #4599) is a refusal the admin
// can act on: 404 with the offering's own code, never a server error.
func TestCareOfferingWritesOnAStaleIDAnswerNotFound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	catalog := enrollmentAPI.NewCareOfferingCatalog(api.NewEnrollmentCareOfferingValues(newCareOfferingCatalog(t, db).Catalog))
	resource := enrollmentAPI.NewResource(
		nil, catalog, nil, nil, nil, nil, nil, nil, nil,
		nil, enrollmentAPI.GuardianInvitationRuntime{}, nil, nil,
		nativeUploads.UploadFiles{}, enrollmentAPI.DatabaseFailures{Transient: transactionFailures.IsTransientDatabaseError, Constraint: transactionFailures.IsConstraintViolation})
	router := testpkg.TenantRuntimeMiddleware(t, db)(resource.Router(api.TenantSessionChain))
	_, admin := testpkg.CreateTestStaffWithAccount(t, db, "Olga", "Angebot")
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{
		ID: int(admin.ID), Sub: admin.Email, Username: admin.Email, FirstName: "Olga", LastName: "Angebot",
		Roles: []string{"admin"}, Permissions: []string{"config:manage"}, TenantID: testpkg.Tenant(t),
	})
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	var staleID int64
	require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence('enrollment.care_offerings', 'id'))").Scan(context.Background(), &staleID))
	path := fmt.Sprintf("/care-offerings/%d", staleID)
	body := func(translations bool) map[string]any {
		value := map[string]any{
			"phase_id": phase.ID, "name": "OGS", "days_of_week_mode": "fixed",
			"available_days": []string{"mon"}, "pickup_times": map[string]string{"mon": "15:00"}, "is_active": true,
		}
		if translations {
			value["translations"] = map[string]any{}
		}
		return value
	}

	tests := []struct {
		name, method string
		body         map[string]any
	}{
		{"update with translations", http.MethodPut, body(true)},
		{"update without translations", http.MethodPut, body(false)},
		{"delete", http.MethodDelete, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var raw []byte
			if tc.body != nil {
				var err error
				raw, err = json.Marshal(tc.body)
				require.NoError(t, err)
			}
			req := httptest.NewRequest(tc.method, path, bytes.NewReader(raw))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			assert.Equal(t, "enrollment.care_offering_not_found", response["code"])
		})
	}
}
