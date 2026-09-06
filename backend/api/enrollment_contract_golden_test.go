package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func checkEnrollmentSubmissionGolden(t *testing.T, api *API) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	_, staff := testpkg.CreateTestTeacherWithAccount(t, db, "Contract", "Staff")
	parent := testpkg.CreateTestParentGuardianChain(t, db)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	staffToken := testutil.MintTestJWT(t, testutil.AdminTestClaimsForTenant(int(staff.ID), testpkg.Tenant(t)))
	parentToken := testutil.MintTestJWT(t, jwt.AppClaims{ID: int(parent.AccountID), Sub: parent.Email, Scope: "parent", Roles: []string{"guardian"}})
	enabled := checkpointRequest(api, checkpointScenario{
		Method: http.MethodPut, Path: "/api/settings/values/enrollment.enabled", Authenticated: true, Body: `{"value":true}`,
	}, staffToken)
	require.Equal(t, http.StatusOK, enabled.Code, enabled.Body.String())
	checkEnrollmentCatalogGoldens(t, api, staffToken, staff.ID)
	var slug, subdomain string
	require.NoError(t, db.NewRaw("SELECT slug, subdomain FROM platform.schools WHERE id = ?", testpkg.Tenant(t)).Scan(context.Background(), &slug, &subdomain))
	for _, scenario := range []struct {
		name, path, token string
		accountID         *int64
	}{
		{"public", "/api/enrollment/" + slug + "/submit", "", nil},
		{"parent", "/parent/enrollments/" + subdomain + "/submit", parentToken, &parent.AccountID},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"phase_id":%d,"guardian_first_name":"Contract","guardian_last_name":"Guardian","guardian_email":"contract-%s@example.test","consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},"children":[{"first_name":"Contract","last_name":"Child-%s","date_of_birth":"2018-04-15","target_grade_level":2}]}`, phase.ID, scenario.name, scenario.name)
			response := checkpointRequest(api, checkpointScenario{Method: http.MethodPost, Path: scenario.path, Authenticated: scenario.token != "", Body: body}, scenario.token)
			require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
			require.True(t, strings.HasPrefix(response.Header().Get("Content-Type"), "application/json"))
			var envelope map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok)
			id, ok := data["request_id"].(string)
			require.True(t, ok, "request IDs must remain JSON strings")
			requestID, err := strconv.ParseInt(id, 10, 64)
			require.NoError(t, err)
			require.Positive(t, requestID)
			statusURL, ok := data["status_url"].(string)
			require.True(t, ok)
			parsed, err := url.Parse(statusURL)
			require.NoError(t, err)
			require.Contains(t, []string{"http", "https"}, parsed.Scheme)
			require.NotEmpty(t, parsed.Host)
			require.Empty(t, parsed.RawQuery)
			require.Empty(t, parsed.Fragment)
			var storedToken string
			var storedAccountID *int64
			require.NoError(t, db.NewRaw("SELECT status_token, guardian_account_id FROM enrollment.requests WHERE id = ? AND tenant_id = ?", requestID, parent.TenantID).Scan(context.Background(), &storedToken, &storedAccountID))
			require.NotEmpty(t, storedToken)
			require.Equal(t, "/anmeldung/status/"+storedToken, parsed.Path)
			require.Equal(t, scenario.accountID, storedAccountID)
			data["request_id"] = "REQUEST_ID"
			data["status_url"] = "PARENTS_ORIGIN/anmeldung/status/STATUS_TOKEN"
			normalized, err := json.MarshalIndent(envelope, "", "  ")
			require.NoError(t, err)
			compareGolden(t, "testdata/enrollment_submission.golden", string(normalized)+"\n", "Enrollment submission response contract changed")
		})
	}
}

func checkEnrollmentCatalogGoldens(t *testing.T, api *API, token string, staffID int64) {
	t.Helper()
	for _, scenario := range []struct{ name, path, body string }{
		{"phase", "/api/enrollment/phases/", `{"name":"Contract Phase","kind":"school_year","service_start_date":"2030-08-01","service_end_date":"2031-07-31","care_overflow_mode":"waitlist","care_offering_selection_mode":"optional","is_active":true}`},
		{"schema", "/api/enrollment/schema/", `{"name":"Contract Schema","fields":[]}`},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			created := checkpointRequest(api, checkpointScenario{Method: http.MethodPost, Path: scenario.path, Authenticated: true, Body: scenario.body}, token)
			require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
			var createdEnvelope struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdEnvelope))
			id, err := strconv.ParseInt(createdEnvelope.Data.ID, 10, 64)
			require.NoError(t, err)
			require.Positive(t, id)
			response := checkpointRequest(api, checkpointScenario{Method: http.MethodGet, Path: scenario.path + createdEnvelope.Data.ID, Authenticated: true}, token)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var envelope map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
			data, ok := envelope["data"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, createdEnvelope.Data.ID, data["id"])
			data["id"] = "RECORD_ID"
			for _, key := range []string{"created_at", "updated_at"} {
				if scenario.name == "schema" && key == "updated_at" {
					continue
				}
				value, ok := data[key].(string)
				require.True(t, ok)
				parsed, err := time.Parse(time.RFC3339Nano, value)
				require.NoError(t, err)
				require.False(t, parsed.IsZero())
				data[key] = "TIMESTAMP"
			}
			if scenario.name == "schema" {
				require.Equal(t, strconv.FormatInt(staffID, 10), data["created_by"])
				data["created_by"] = "ACCOUNT_ID"
			}
			normalized, err := json.MarshalIndent(envelope, "", "  ")
			require.NoError(t, err)
			compareGolden(t, "testdata/enrollment_"+scenario.name+".golden", string(normalized)+"\n", "Enrollment catalog response contract changed")
		})
	}
}
