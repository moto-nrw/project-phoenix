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
			if scenario.name == "public" {
				checkEnrollmentChangeRequestDialogueGolden(t, api, db, staffToken, phase.ID, requestID, storedToken)
			}
			if scenario.name == "parent" {
				checkEnrollmentAcceptanceGolden(t, api, db, staffToken, requestID, parent)
			}
		})
	}
}

// checkEnrollmentAcceptanceGolden pins the acceptance decision (#2699) at the
// production router for a child a logged-in parent submitted: the approval
// creates the student, links it to the parent's guardian profile, and grants
// the parent's existing platform account guardian access to this school
// through the Identity & Access capability instead of an invitation. The
// response contract and the resulting rows must not change.
func checkEnrollmentAcceptanceGolden(t *testing.T, api *API, db *testpkg.DB, staffToken string, requestID int64, parent testpkg.ParentChain) {
	t.Helper()
	tenantID := testpkg.Tenant(t)
	var childID int64
	require.NoError(t, db.NewRaw("SELECT id FROM enrollment.request_children WHERE request_id = ? AND tenant_id = ?", requestID, tenantID).Scan(context.Background(), &childID))
	var roleAssignmentsBefore int
	require.NoError(t, db.NewRaw("SELECT count(*) FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id WHERE ar.account_id = ? AND ar.tenant_id = ? AND LOWER(r.name) = 'guardian'", parent.AccountID, tenantID).Scan(context.Background(), &roleAssignmentsBefore))
	require.Equal(t, 1, roleAssignmentsBefore, "the parent fixture carries the guardian role once")

	decided := checkpointRequest(api, checkpointScenario{Method: http.MethodPost, Path: fmt.Sprintf("/api/enrollment/admin/requests/%d/children/%d/decide", requestID, childID), Authenticated: true, Body: `{"status":"approved","reason":"Contract approval"}`}, staffToken)
	require.Equal(t, http.StatusOK, decided.Code, decided.Body.String())
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(decided.Body.Bytes(), &envelope))
	data, ok := envelope["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, strconv.FormatInt(childID, 10), data["id"])
	studentID, ok := data["created_student_id"].(string)
	require.True(t, ok, "an approval must report the created student as a JSON string")
	require.NotEmpty(t, studentID)
	require.NotEmpty(t, data["reviewed_at"])
	data["id"] = "CHILD_ID"
	data["created_student_id"] = "STUDENT_ID"
	data["reviewed_at"] = "REVIEWED_AT"
	data["reviewed_by"] = "REVIEWER_ACCOUNT_ID"
	normalized, err := json.MarshalIndent(envelope, "", "  ")
	require.NoError(t, err)
	compareGolden(t, "testdata/enrollment_decision_approved.golden", string(normalized)+"\n", "Enrollment acceptance response contract changed")

	// The student exists and is linked back to the request child.
	var linkedStudentID int64
	require.NoError(t, db.NewRaw("SELECT created_student_id FROM enrollment.request_children WHERE id = ? AND tenant_id = ?", childID, tenantID).Scan(context.Background(), &linkedStudentID))
	require.Equal(t, studentID, strconv.FormatInt(linkedStudentID, 10))
	var firstName, lastName string
	require.NoError(t, db.NewRaw("SELECT p.first_name, p.last_name FROM users.students s JOIN users.persons p ON p.id = s.person_id WHERE s.id = ? AND s.tenant_id = ?", linkedStudentID, tenantID).Scan(context.Background(), &firstName, &lastName))
	require.Equal(t, "Contract", firstName)
	require.Equal(t, "Child-parent", lastName)
	// The submitting parent's guardian profile holds the primary link.
	var primaryProfileID int64
	require.NoError(t, db.NewRaw("SELECT guardian_profile_id FROM users.students_guardians WHERE student_id = ? AND tenant_id = ? AND is_primary", linkedStudentID, tenantID).Scan(context.Background(), &primaryProfileID))
	require.Equal(t, parent.GuardianProfileID, primaryProfileID)
	// Identity & Access: the existing account keeps an active school mapping
	// and exactly one guardian role assignment for this tenant.
	var mappingStatus string
	var deactivatedAt *time.Time
	require.NoError(t, db.NewRaw("SELECT status, deactivated_at FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", parent.AccountID, tenantID).Scan(context.Background(), &mappingStatus, &deactivatedAt))
	require.Equal(t, "active", mappingStatus)
	require.Nil(t, deactivatedAt)
	var roleAssignments int
	require.NoError(t, db.NewRaw("SELECT count(*) FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id WHERE ar.account_id = ? AND ar.tenant_id = ? AND LOWER(r.name) = 'guardian'", parent.AccountID, tenantID).Scan(context.Background(), &roleAssignments))
	require.Equal(t, 1, roleAssignments, "granting guardian access must not duplicate the role assignment")
	var linkedAccountID *int64
	require.NoError(t, db.NewRaw("SELECT account_id FROM users.guardian_profiles WHERE id = ? AND tenant_id = ?", parent.GuardianProfileID, tenantID).Scan(context.Background(), &linkedAccountID))
	require.NotNil(t, linkedAccountID)
	require.Equal(t, parent.AccountID, *linkedAccountID)
	// A linked account gets no invitation.
	var invitations int
	require.NoError(t, db.NewRaw("SELECT count(*) FROM auth.guardian_invitations WHERE guardian_profile_id = ? AND tenant_id = ?", parent.GuardianProfileID, tenantID).Scan(context.Background(), &invitations))
	require.Zero(t, invitations, "an approval for a parent with a platform account must not queue an invitation")
}

// checkEnrollmentChangeRequestDialogueGolden pins the parent/OGS dialogue on
// an enrollment change request (#2696) at the production router: the family
// files a correction over its status token, staff asks back, the family
// answers, staff decides. The public list is the family's view of that
// dialogue and must not leak reviewer identities or internal notes.
func checkEnrollmentChangeRequestDialogueGolden(t *testing.T, api *API, db *testpkg.DB, staffToken string, phaseID, requestID int64, statusToken string) {
	t.Helper()
	var childID int64
	require.NoError(t, db.NewRaw("SELECT id FROM enrollment.request_children WHERE request_id = ? AND tenant_id = ?", requestID, testpkg.Tenant(t)).Scan(context.Background(), &childID))
	// A submitted child is edited directly; only a decided child moves the
	// request into change-request mode.
	decided := checkpointRequest(api, checkpointScenario{Method: http.MethodPost, Path: fmt.Sprintf("/api/enrollment/admin/requests/%d/children/%d/decide", requestID, childID), Authenticated: true, Body: `{"status":"waitlisted","reason":"Contract waitlist"}`}, staffToken)
	require.Equal(t, http.StatusOK, decided.Code, decided.Body.String())

	publicBase := "/api/enrollment/requests/" + statusToken + "/change-requests"
	proposal := fmt.Sprintf(`{"phase_id":%d,"guardian_first_name":"Contract","guardian_last_name":"Guardian-Korrigiert","guardian_email":"contract-public@example.test","consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},"children":[{"id":"%d","first_name":"Contract","last_name":"Child-public","date_of_birth":"2018-04-15","target_grade_level":2}],"parent_note":"Nachname korrigiert."}`, phaseID, childID)
	created := checkpointRequest(api, checkpointScenario{Method: http.MethodPost, Path: publicBase, Body: proposal}, "")
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var createdEnvelope struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &createdEnvelope))
	changeRequestID, err := strconv.ParseInt(createdEnvelope.Data.ID, 10, 64)
	require.NoError(t, err, "change request IDs must remain JSON strings")
	require.Positive(t, changeRequestID)
	require.Equal(t, "pending_review", createdEnvelope.Data.Status)
	adminBase := "/api/enrollment/admin/change-requests/" + createdEnvelope.Data.ID

	// A reply is only possible after staff asked back: the stable 400 keeps
	// the family from answering a question nobody asked.
	early := checkpointRequest(api, checkpointScenario{Method: http.MethodPost, Path: publicBase + "/" + createdEnvelope.Data.ID + "/messages", Body: `{"body":"Zu früh."}`}, "")
	require.Equal(t, http.StatusBadRequest, early.Code, early.Body.String())
	require.JSONEq(t, `{"status":"error","error":"enrollment change request has invalid status"}`, early.Body.String())

	for _, step := range []struct {
		name, method, path, body, token string
	}{
		{"question", http.MethodPost, adminBase + "/question", `{"body":"Welche Klasse besucht das Kind?"}`, staffToken},
		{"reply", http.MethodPost, publicBase + "/" + createdEnvelope.Data.ID + "/messages", `{"body":"Die 2a."}`, ""},
		{"reject", http.MethodPost, adminBase + "/reject", `{"note":"Bitte mit Nachweis neu einreichen."}`, staffToken},
	} {
		response := checkpointRequest(api, checkpointScenario{Method: step.method, Path: step.path, Authenticated: step.token != "", Body: step.body}, step.token)
		require.Equal(t, http.StatusOK, response.Code, "%s: %s", step.name, response.Body.String())
	}
	foreign := checkpointRequest(api, checkpointScenario{Method: http.MethodGet, Path: "/api/enrollment/requests/not-a-token/change-requests"}, "")
	require.Equal(t, http.StatusNotFound, foreign.Code, foreign.Body.String())

	list := checkpointRequest(api, checkpointScenario{Method: http.MethodGet, Path: publicBase}, "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &envelope))
	items, ok := envelope["data"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item, ok := items[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, createdEnvelope.Data.ID, item["id"])
	require.Equal(t, strconv.FormatInt(requestID, 10), item["request_id"])
	messages, ok := item["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 4, "parent note, staff question, parent reply, decision note")
	normalized, err := json.MarshalIndent(normalizeContractIdentifiers(envelope), "", "  ")
	require.NoError(t, err)
	compareGolden(t, "testdata/enrollment_change_request_dialogue.golden", string(normalized)+"\n", "Enrollment change request dialogue contract changed")

	// The staff side: the decided request in the shared request-module history
	// format, and the badge count that no longer includes it.
	history := checkpointRequest(api, checkpointScenario{Method: http.MethodGet, Path: "/api/enrollment/admin/change-requests/list?view=history&limit=5", Authenticated: true}, staffToken)
	require.Equal(t, http.StatusOK, history.Code, history.Body.String())
	var historyEnvelope map[string]any
	require.NoError(t, json.Unmarshal(history.Body.Bytes(), &historyEnvelope))
	historyData, ok := historyEnvelope["data"].(map[string]any)
	require.True(t, ok)
	historyItems, ok := historyData["items"].([]any)
	require.True(t, ok)
	require.Len(t, historyItems, 1)
	historyItem, ok := historyItems[0].(map[string]any)
	require.True(t, ok)
	entry, ok := historyItem["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, createdEnvelope.Data.ID, entry["id"])
	require.Equal(t, "Contract Staff", entry["decided_by_name"])
	normalized, err = json.MarshalIndent(normalizeContractIdentifiers(historyEnvelope), "", "  ")
	require.NoError(t, err)
	compareGolden(t, "testdata/enrollment_change_request_review_history.golden", string(normalized)+"\n", "Enrollment change request review history contract changed")
	pending := checkpointRequest(api, checkpointScenario{Method: http.MethodGet, Path: "/api/enrollment/admin/change-requests/pending-count", Authenticated: true}, staffToken)
	require.Equal(t, http.StatusOK, pending.Code, pending.Body.String())
	require.JSONEq(t, `{"status":"success","message":"Pending enrollment change request count retrieved","data":{"pending_count":0}}`, pending.Body.String())
}

// normalizeContractIdentifiers replaces generated identifiers and instants so
// the golden pins field names, types and ordering, not fixture values.
func normalizeContractIdentifiers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, entry := range typed {
			switch {
			case key == "case_id":
				// "<request id>:<request child id>", a stable case identity.
				if text, ok := entry.(string); ok {
					if requestID, childID, found := strings.Cut(text, ":"); found {
						if _, err := strconv.ParseInt(requestID, 10, 64); err == nil {
							if _, err := strconv.ParseInt(childID, 10, 64); err == nil {
								out[key] = "ID:ID"
								continue
							}
						}
					}
				}
			case key == "id" || strings.HasSuffix(key, "_id"):
				if text, ok := entry.(string); ok && text != "" {
					if _, err := strconv.ParseInt(text, 10, 64); err == nil {
						out[key] = "ID"
						continue
					}
				}
				if _, ok := entry.(float64); ok {
					out[key] = "ID"
					continue
				}
			case strings.HasSuffix(key, "_at"):
				if text, ok := entry.(string); ok {
					if _, err := time.Parse(time.RFC3339Nano, text); err == nil {
						out[key] = "TIMESTAMP"
						continue
					}
				}
			}
			out[key] = normalizeContractIdentifiers(entry)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, entry := range typed {
			out[i] = normalizeContractIdentifiers(entry)
		}
		return out
	default:
		return value
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
