package enrollment

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// mockDecisionService is a hand-rolled fake that records inputs and
// replays canned outputs. Lives in this file because every admin-
// decision test uses it; sharing it with other suites is a refactor
// for when the second consumer shows up.
type mockDecisionService struct {
	// List
	listFilters       enrollmentService.RequestFilters
	listResult        []*enrollmentService.RequestSummary
	listErr           error
	listCalls         int
	listStudentID     int64
	listStudentResult []*enrollmentService.RequestSummary
	listStudentErr    error
	getRequestID      int64
	getResult         *enrollmentService.RequestSummary
	getErr            error

	listChildOffResult map[int64][]enrollmentService.ChildOfferingRow
	listChildOffErr    error

	updateChildOffResult *enrollmentModels.RequestChild
	updateChildOffErr    error

	decideInput  enrollmentService.DecideInput
	decideCalls  int
	decideResult *enrollmentService.DecideOutcome
	decideErrs   []error
	decideErr    error

	restoreRequestID  int64
	restoreRestoredBy int64
	restoreCalls      int
	restoreResult     *enrollmentService.RestoreOutcome
	restoreErr        error

	// ExportPhase: records the args the handler forwards (so a handler
	// test can assert the format + actor were threaded through) and
	// replays a canned payload/error.
	exportPhaseID       int64
	exportActorID       int64
	exportActorRole     string
	exportFormat        string
	exportChildStatus   string
	exportCalls         int
	exportResult        *enrollmentService.PhaseExport
	exportErr           error
	exportStudentID     int64
	exportStudentResult *enrollmentService.StudentEnrollmentExport
	exportStudentErr    error
}

func (m *mockDecisionService) List(_ context.Context, f enrollmentService.RequestFilters) ([]*enrollmentService.RequestSummary, error) {
	m.listFilters = f
	m.listCalls++
	return m.listResult, m.listErr
}

func (m *mockDecisionService) ListByStudent(_ context.Context, studentID int64) ([]*enrollmentService.RequestSummary, error) {
	m.listStudentID = studentID
	return m.listStudentResult, m.listStudentErr
}

func (m *mockDecisionService) Get(_ context.Context, id int64) (*enrollmentService.RequestSummary, error) {
	m.getRequestID = id
	return m.getResult, m.getErr
}

func (m *mockDecisionService) Decide(_ context.Context, input enrollmentService.DecideInput) (*enrollmentService.DecideOutcome, error) {
	m.decideInput = input
	m.decideCalls++
	if len(m.decideErrs) > 0 {
		err := m.decideErrs[0]
		m.decideErrs = m.decideErrs[1:]
		return m.decideResult, err
	}
	return m.decideResult, m.decideErr
}

func (m *mockDecisionService) RestoreWithdrawn(_ context.Context, requestID, restoredBy int64) (*enrollmentService.RestoreOutcome, error) {
	m.restoreRequestID = requestID
	m.restoreRestoredBy = restoredBy
	m.restoreCalls++
	return m.restoreResult, m.restoreErr
}

func (m *mockDecisionService) UpdateChildOfferings(_ context.Context, _ enrollmentService.UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error) {
	return m.updateChildOffResult, m.updateChildOffErr
}

func (m *mockDecisionService) ListChildOfferings(_ context.Context, _ int64) (map[int64][]enrollmentService.ChildOfferingRow, error) {
	return m.listChildOffResult, m.listChildOffErr
}

func (m *mockDecisionService) ListOfferingAdjustments(_ context.Context, _, _ int64) ([]*auditModels.EnrollmentOfferingAdjustment, error) {
	return nil, nil
}

func (m *mockDecisionService) ExportPhase(_ context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*enrollmentService.PhaseExport, error) {
	m.exportCalls++
	m.exportPhaseID = phaseID
	m.exportActorID = actorAccountID
	m.exportActorRole = actorRole
	m.exportFormat = format
	m.exportChildStatus = childStatusFilter
	return m.exportResult, m.exportErr
}

func (m *mockDecisionService) ExportStudent(_ context.Context, studentID, actorAccountID int64, actorRole, format string) (*enrollmentService.StudentEnrollmentExport, error) {
	m.exportStudentID = studentID
	m.exportActorID = actorAccountID
	m.exportActorRole = actorRole
	m.exportFormat = format
	return m.exportStudentResult, m.exportStudentErr
}

func (m *mockDecisionService) RecordPhaseExportAudit(_ context.Context, _ int64, _ string, _ *enrollmentModels.Phase, _, _ string, _, _ int) error {
	return nil
}

func buildAdminDecisionRouter(svc enrollmentService.DecisionService) chi.Router {
	rs := &Resource{
		DecisionService: svc,
		// db nil → runInTenantTx short-circuits straight to the closure.
	}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/enrollment/admin/requests", rs.listAdminRequests)
	r.Get("/enrollment/admin/requests/{id}", rs.getAdminRequest)
	r.Get("/enrollment/admin/students/{studentId}/requests", rs.listAdminRequestsByStudent)
	r.Post("/enrollment/admin/students/{studentId}/requests/export", rs.exportStudentEnrollmentRequests)
	r.Post("/enrollment/admin/requests/{id}/children/{childId}/decide", rs.decideAdminChild)
	r.Put("/enrollment/admin/requests/{id}/children/{childId}/offerings", rs.updateAdminChildOfferings)
	r.Post("/enrollment/admin/requests/{id}/restore", rs.restoreAdminRequest)
	return r
}

func buildProtectedAdminDecisionRouter(svc enrollmentService.DecisionService) chi.Router {
	rs := &Resource{
		DecisionService: svc,
		// db nil → runInTenantTx short-circuits straight to the closure.
	}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.With(authorize.RequiresPermission("config:read")).Get("/enrollment/admin/requests", rs.listAdminRequests)
	r.With(authorize.RequiresPermission("config:manage")).Get("/enrollment/admin/requests/{id}", rs.getAdminRequest)
	r.With(authorize.RequiresPermission("config:manage")).Get("/enrollment/admin/students/{studentId}/requests", rs.listAdminRequestsByStudent)
	return r
}

func executeAdminJSON(t *testing.T, router chi.Router, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		req = httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func executeAdminJSONWithPermissions(t *testing.T, router chi.Router, method, path string, permissions []string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxPermissions, permissions))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// makeReqSummary builds a RequestSummary via base.Model so the
// embedded ID field is settable. Mirrors the mkRequest/mkChild
// approach in api/enrollment/admin_handlers_helpers_test.go.
func makeReqSummary(id, phaseID int64, children ...*enrollmentModels.RequestChild) *enrollmentService.RequestSummary {
	return &enrollmentService.RequestSummary{
		Request: &enrollmentModels.Request{
			Model:             baseModel.Model{ID: id},
			PhaseID:           phaseID,
			GuardianFirstName: "Anna",
			GuardianLastName:  "Beispiel",
			GuardianEmail:     "anna@example.test",
			SubmittedAt:       time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		},
		Phase:    &enrollmentModels.Phase{Name: "Schuljahr 2026"},
		Children: children,
	}
}

func makeChildSummary(id int64, firstName, lastName, status string) *enrollmentModels.RequestChild {
	return &enrollmentModels.RequestChild{
		Model:          baseModel.Model{ID: id},
		FirstName:      firstName,
		LastName:       lastName,
		DateOfBirth:    timezone.NewDate(2018, 4, 15),
		Status:         status,
		ActivationMode: enrollmentModels.ChildActivationScheduled,
	}
}

// --- listAdminRequests ---------------------------------------------------

func TestListAdminRequestsHandler_NilServiceReturns500(t *testing.T) {
	router := buildAdminDecisionRouter(nil)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestListAdminRequestsHandler_HappyPathReturns200(t *testing.T) {
	mock := &mockDecisionService{
		listResult: []*enrollmentService.RequestSummary{
			makeReqSummary(1234, 5678, makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted)),
		},
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":"1234"`,
		"int64 ID must be stringified per CLAUDE rule 4")
	assert.Contains(t, w.Body.String(), `"phase_name":"Schuljahr 2026"`)
}

func TestListAdminRequestsHandler_ConfigReadDoesNotReturnStatusToken(t *testing.T) {
	summary := makeReqSummary(1234, 5678,
		makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted),
	)
	summary.Request.StatusToken = "leaked-token"
	mock := &mockDecisionService{
		listResult: []*enrollmentService.RequestSummary{summary},
	}
	router := buildProtectedAdminDecisionRouter(mock)
	w := executeAdminJSONWithPermissions(t, router, http.MethodGet, "/enrollment/admin/requests", []string{"config:read"})
	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.NotContains(t, body, `"status_token"`)
	assert.NotContains(t, body, "leaked-token")
}

func TestAdminRequestRoutes_DetailRequiresConfigManage(t *testing.T) {
	mock := &mockDecisionService{
		listResult: []*enrollmentService.RequestSummary{
			makeReqSummary(1234, 5678, makeChildSummary(99, "Lina", "Kind", enrollmentModels.ChildStatusSubmitted)),
		},
		getResult: makeReqSummary(1234, 5678, makeChildSummary(99, "Lina", "Kind", enrollmentModels.ChildStatusSubmitted)),
		listStudentResult: []*enrollmentService.RequestSummary{
			makeReqSummary(1234, 5678, makeChildSummary(99, "Lina", "Kind", enrollmentModels.ChildStatusSubmitted)),
		},
	}
	router := buildProtectedAdminDecisionRouter(mock)

	listReadOnly := executeAdminJSONWithPermissions(t, router, http.MethodGet, "/enrollment/admin/requests", []string{"config:read"})
	assert.Equal(t, http.StatusOK, listReadOnly.Code)

	requestDetailReadOnly := executeAdminJSONWithPermissions(t, router, http.MethodGet, "/enrollment/admin/requests/1234", []string{"config:read"})
	assert.Equal(t, http.StatusForbidden, requestDetailReadOnly.Code)

	studentDetailReadOnly := executeAdminJSONWithPermissions(t, router, http.MethodGet, "/enrollment/admin/students/777/requests", []string{"config:read"})
	assert.Equal(t, http.StatusForbidden, studentDetailReadOnly.Code)

	requestDetailManage := executeAdminJSONWithPermissions(t, router, http.MethodGet, "/enrollment/admin/requests/1234", []string{"config:manage"})
	assert.Equal(t, http.StatusOK, requestDetailManage.Code)

	studentDetailManage := executeAdminJSONWithPermissions(t, router, http.MethodGet, "/enrollment/admin/students/777/requests", []string{"config:manage"})
	assert.Equal(t, http.StatusOK, studentDetailManage.Code)
}

func TestListAdminRequestsHandler_PhaseFilterParsed(t *testing.T) {
	mock := &mockDecisionService{listResult: []*enrollmentService.RequestSummary{}}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests?phase_id=42", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(42), mock.listFilters.PhaseID,
		"phase_id query param must be parsed to filter")
}

func TestListAdminRequestsHandler_ChildStatusFilterParsed(t *testing.T) {
	mock := &mockDecisionService{listResult: []*enrollmentService.RequestSummary{}}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests?child_status=waitlisted", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "waitlisted", mock.listFilters.ChildStatus)
}

func TestListAdminRequestsHandler_InvalidPhaseIDRejected(t *testing.T) {
	mock := &mockDecisionService{}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests?phase_id=notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, 0, mock.listCalls, "service must NOT be called when phase_id is invalid")
}

func TestListAdminRequestsHandler_ZeroPhaseIDRejected(t *testing.T) {
	// phase_id=0 is logically invalid (postgres ids are positive) —
	// the handler rejects it as 400.
	mock := &mockDecisionService{}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests?phase_id=0", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListAdminRequestsHandler_ServiceErrorReturns500(t *testing.T) {
	mock := &mockDecisionService{listErr: errors.New("synthetic boom")}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- getAdminRequest -----------------------------------------------------

func TestGetAdminRequestHandler_NilServiceReturns500(t *testing.T) {
	router := buildAdminDecisionRouter(nil)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetAdminRequestHandler_InvalidIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/notanumber", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAdminRequestHandler_NegativeIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/-1", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAdminRequestHandler_NotFoundMapped(t *testing.T) {
	mock := &mockDecisionService{getErr: enrollmentService.ErrDecisionRequestNotFound}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAdminRequestHandler_GenericErrorMappedAs500(t *testing.T) {
	mock := &mockDecisionService{getErr: errors.New("synthetic boom")}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetAdminRequestHandler_HappyPathReturnsDetail(t *testing.T) {
	summary := makeReqSummary(1234, 5678,
		makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted),
	)
	summary.Request.StatusToken = "detail-token"
	mock := &mockDecisionService{
		getResult: summary,
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"id":"1234"`)
	assert.Contains(t, body, `"id":"99"`)
	assert.Contains(t, body, `"date_of_birth":"2018-04-15"`)
	assert.Contains(t, body, `"status_token":"detail-token"`)
}

func TestGetAdminRequestHandler_ReportsLateInviteEmailMismatch(t *testing.T) {
	summary := makeReqSummary(1234, 5678,
		makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted),
	)
	summary.Request.StatusToken = "detail-token"
	summary.Request.GuardianEmail = "submitted@example.test"
	summary.LateInvite = &enrollmentModels.LateInvite{GuardianEmail: "invited@example.test"}
	router := buildAdminDecisionRouter(&mockDecisionService{getResult: summary})

	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"late_invite_guardian_email":"invited@example.test"`)
	assert.Contains(t, w.Body.String(), `"late_invite_email_mismatch":true`)
}

func TestGetAdminRequestHandler_StitchesChildOfferings(t *testing.T) {
	mock := &mockDecisionService{
		getResult: makeReqSummary(1234, 5678,
			makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted),
		),
		listChildOffResult: map[int64][]enrollmentService.ChildOfferingRow{
			99: {{
				OfferingID:     7777,
				OfferingName:   "OGS-Nachmittag",
				DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
				SelectedDays:   nil,
				AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
			}},
		},
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"offering_id":"7777"`,
		"per-child offerings must be stitched onto the detail response")
	assert.Contains(t, body, `"offering_name":"OGS-Nachmittag"`)
}

// #2185: valid_until goes over the wire as the INCLUSIVE last covered day,
// the shape api/parent/care_offerings_handlers.go has always used. One JSON
// name must not mean two different days depending on which endpoint answered.
func TestGetAdminRequestHandler_ReportsInclusiveOfferingEndDate(t *testing.T) {
	exclusiveEnd := timezone.NewDate(2027, 8, 1)
	mock := &mockDecisionService{
		getResult: makeReqSummary(1234, 5678,
			makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusApproved),
		),
		listChildOffResult: map[int64][]enrollmentService.ChildOfferingRow{
			99: {{
				OfferingID:         7777,
				OfferingName:       "OGS-Nachmittag",
				DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
				AvailableDays:      []string{"mon"},
				ValidUntil:         &exclusiveEnd,
				IsCurrentSelection: true,
			}},
		},
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"valid_until":"2027-07-31"`,
		"stored end 2027-08-01 is exclusive; the last covered day is 2027-07-31")
	assert.NotContains(t, body, `"valid_until":"2027-08-01"`)
	assert.Contains(t, body, `"offerings":[`,
		"the current selection ships under offerings, which is what a correction replaces")
}

// #2185: a booking that only starts later ships in its own array. A client
// built before that field keeps seeing exactly the selection it always saw,
// so a stale browser tab cannot pre-check a future booking and apply it
// months early on an untouched save.
func TestGetAdminRequestHandler_SeparatesUpcomingOfferings(t *testing.T) {
	mock := &mockDecisionService{
		getResult: makeReqSummary(1234, 5678,
			makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusApproved),
		),
		listChildOffResult: map[int64][]enrollmentService.ChildOfferingRow{
			99: {
				{
					OfferingID:         7777,
					OfferingName:       "Jetzt gebucht",
					DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
					IsCurrentSelection: true,
				},
				{
					OfferingID:         8888,
					OfferingName:       "Ab September",
					DaysOfWeekMode:     enrollmentModels.DaysOfWeekModeFixed,
					StartsLater:        true,
					IsCurrentSelection: false,
				},
			},
		},
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)

	var payload struct {
		Data struct {
			Children []struct {
				Offerings         []map[string]any `json:"offerings"`
				UpcomingOfferings []map[string]any `json:"upcoming_offerings"`
			} `json:"children"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Len(t, payload.Data.Children, 1)
	child := payload.Data.Children[0]
	require.Len(t, child.Offerings, 1, "only the current selection belongs in offerings")
	assert.Equal(t, "7777", child.Offerings[0]["offering_id"])
	require.Len(t, child.UpcomingOfferings, 1)
	assert.Equal(t, "8888", child.UpcomingOfferings[0]["offering_id"])
}

// A failed offerings lookup must be distinguishable from "this child booked
// nothing": the client refuses to save a correction on top of the empty
// state, which would otherwise delete the family's real bookings.
func TestGetAdminRequestHandler_FlagsUnavailableOfferings(t *testing.T) {
	mock := &mockDecisionService{
		getResult: makeReqSummary(1234, 5678,
			makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusApproved),
		),
		listChildOffErr: errors.New("synthetic offerings error"),
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"offerings_unavailable":true`,
		"an empty offerings list after a failed lookup must not read as 'booked nothing'")
}

// #2185: the correction is committed before this response is built, so a
// failed re-read must not report the save as failed. A 500 here made the
// admin retry and apply the replacement twice, with a second audit entry
// blaming them for it.
func TestUpdateAdminChildOfferingsHandler_SucceedsWhenRereadFails(t *testing.T) {
	mock := &mockDecisionService{
		updateChildOffResult: makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusApproved),
		listChildOffErr:      errors.New("synthetic re-read error"),
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPut,
		"/enrollment/admin/requests/1234/children/99/offerings",
		map[string]any{
			"reason":    "Randstunde nachgetragen",
			"offerings": []map[string]any{{"offering_id": "7777"}},
		})

	require.Equal(t, http.StatusOK, w.Code,
		"the correction is already committed; a failed re-read must not undo that verdict")
	assert.Contains(t, w.Body.String(), `"offerings_unavailable":true`,
		"the client must learn the returned selection is unknown")
}

func TestGetAdminRequestHandler_TolerantOfMissingChildOfferings(t *testing.T) {
	// ListChildOfferings is best-effort — failure must NOT kill the
	// detail response, the admin just sees no Betreuungsangebote
	// section.
	mock := &mockDecisionService{
		getResult: makeReqSummary(1234, 5678,
			makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted),
		),
		listChildOffErr: errors.New("synthetic offerings error"),
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodGet, "/enrollment/admin/requests/1234", nil)
	assert.Equal(t, http.StatusOK, w.Code,
		"detail must succeed even when ListChildOfferings fails")
	assert.NotContains(t, w.Body.String(), `"offering_id"`,
		"failed offerings lookup → no offerings in response")
}

// --- decideAdminChild ----------------------------------------------------

func TestDecideAdminChildHandler_NilServiceReturns500(t *testing.T) {
	router := buildAdminDecisionRouter(nil)
	body := map[string]any{"status": "approved"}
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide", body)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDecideAdminChildHandler_InvalidRequestIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/notanumber/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_InvalidChildIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/notanumber/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_HappyPathReturns200(t *testing.T) {
	mock := &mockDecisionService{
		decideResult: &enrollmentService.DecideOutcome{
			Child: makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusApproved),
		},
	}
	router := buildAdminDecisionRouter(mock)
	body := map[string]any{"status": "approved", "reason": "OK"}
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide", body)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), mock.decideInput.RequestID)
	assert.Equal(t, int64(99), mock.decideInput.ChildID)
	assert.Equal(t, enrollmentService.DecisionApproved, mock.decideInput.Status)
	assert.Equal(t, "OK", mock.decideInput.Reason)
	assert.Contains(t, w.Body.String(), `"status":"approved"`)
}

func TestDecideAdminChildHandler_RetriesTransientDatabaseError(t *testing.T) {
	mock := &mockDecisionService{
		decideResult: &enrollmentService.DecideOutcome{
			Child: makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusApproved),
		},
		decideErrs: []error{
			fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn),
			nil,
		},
	}
	router := buildAdminDecisionRouter(mock)

	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 2, mock.decideCalls)
	assert.Contains(t, w.Body.String(), `"status":"approved"`)
}

func TestDecideAdminChildHandler_DoesNotRetryAfterDecisionBodySucceeded(t *testing.T) {
	mock := &mockDecisionService{
		decideResult: &enrollmentService.DecideOutcome{
			Child: makeChildSummary(99, "Lara", "Beispiel", enrollmentModels.ChildStatusApproved),
		},
	}
	rs := &Resource{
		DecisionService: mock,
		runInTenantTxForTest: func(r *http.Request, fn func(ctx context.Context) error) error {
			if err := fn(r.Context()); err != nil {
				return err
			}
			return fmt.Errorf("commit failed: %w", driver.ErrBadConn)
		},
	}
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Post("/enrollment/admin/requests/{id}/children/{childId}/decide", rs.decideAdminChild)

	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, 1, mock.decideCalls,
		"commit-boundary errors after a successful decision body must not replay the approval")
}

func TestDecideAdminChildHandler_DoesNotRetryCanceledRequestContext(t *testing.T) {
	tests := []struct {
		name       string
		setupCtx   func(context.Context) (context.Context, context.CancelFunc)
		wantStatus int
	}{
		{
			name: "canceled",
			setupCtx: func(parent context.Context) (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(parent)
				cancel()
				return ctx, func() {}
			},
			wantStatus: 499,
		},
		{
			name: "deadline exceeded",
			setupCtx: func(parent context.Context) (context.Context, context.CancelFunc) {
				return context.WithDeadline(parent, time.Now().Add(-time.Second))
			},
			wantStatus: http.StatusRequestTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockDecisionService{
				decideErrs: []error{fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn)},
			}
			router := buildAdminDecisionRouter(mock)

			req := httptest.NewRequest(http.MethodPost,
				"/enrollment/admin/requests/1234/children/99/decide",
				strings.NewReader(`{"status":"approved"}`))
			req.Header.Set("Content-Type", "application/json")
			ctx, cancel := tt.setupCtx(req.Context())
			defer cancel()
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, 1, mock.decideCalls)
		})
	}
}

func TestDecideAdminChildHandler_TransientDatabaseErrorMapsTo503AfterRetry(t *testing.T) {
	mock := &mockDecisionService{
		decideErrs: []error{
			fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn),
			fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn),
		},
	}
	router := buildAdminDecisionRouter(mock)

	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, 2, mock.decideCalls)
}

func TestDecideAdminChildHandler_ChildNotFoundMapsTo404(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionChildNotFound}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDecideAdminChildHandler_RequestNotFoundMapsTo404(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionRequestNotFound}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDecideAdminChildHandler_InvalidStatusMapsTo400(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionInvalidStatus}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "bogus"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_AlreadyTerminalMapsTo400(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionAlreadyTerminal}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_InvalidDataMapsTo400(t *testing.T) {
	mock := &mockDecisionService{decideErr: enrollmentService.ErrDecisionInvalidData}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDecideAdminChildHandler_GenericErrorMapsTo500(t *testing.T) {
	mock := &mockDecisionService{decideErr: errors.New("synthetic boom")}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		map[string]any{"status": "approved"})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestDecideAdminChildHandler_MalformedJSONReturns400(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	req := httptest.NewRequest(http.MethodPost,
		"/enrollment/admin/requests/1234/children/99/decide",
		strings.NewReader("{not valid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- restoreAdminRequest -------------------------------------------------

func TestRestoreAdminRequestHandler_NilServiceReturns500(t *testing.T) {
	router := buildAdminDecisionRouter(nil)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/restore", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRestoreAdminRequestHandler_InvalidRequestIDRejected(t *testing.T) {
	router := buildAdminDecisionRouter(&mockDecisionService{})
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/notanumber/restore", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRestoreAdminRequestHandler_HappyPathReturns200(t *testing.T) {
	mock := &mockDecisionService{
		restoreResult: &enrollmentService.RestoreOutcome{RestoredChildIDs: []int64{7, 8}},
	}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/restore", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mock.restoreCalls)
	assert.Equal(t, int64(1234), mock.restoreRequestID)
	assert.Contains(t, w.Body.String(), `"restored_children":2`)
}

func TestRestoreAdminRequestHandler_RequestNotFoundMapsTo404(t *testing.T) {
	mock := &mockDecisionService{restoreErr: enrollmentService.ErrDecisionRequestNotFound}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/restore", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRestoreAdminRequestHandler_NothingWithdrawnMapsTo400(t *testing.T) {
	mock := &mockDecisionService{restoreErr: enrollmentService.ErrRestoreNothingWithdrawn}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/restore", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRestoreAdminRequestHandler_PhaseInactiveMapsTo409(t *testing.T) {
	mock := &mockDecisionService{restoreErr: enrollmentService.ErrRestorePhaseInactive}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/restore", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "enrollment.restore_phase_inactive")
}

func TestRestoreAdminRequestHandler_DuplicateMapsTo409(t *testing.T) {
	mock := &mockDecisionService{restoreErr: enrollmentService.ErrRestoreDuplicateActive}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/restore", nil)
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "enrollment.restore_duplicate")
}

func TestRestoreAdminRequestHandler_GenericErrorMapsTo500(t *testing.T) {
	mock := &mockDecisionService{restoreErr: errors.New("synthetic boom")}
	router := buildAdminDecisionRouter(mock)
	w := executeAdminJSON(t, router, http.MethodPost,
		"/enrollment/admin/requests/1234/restore", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
