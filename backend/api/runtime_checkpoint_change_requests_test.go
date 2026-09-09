package api

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

var runtimeCheckpointEnrollmentChangeRequests = flag.Bool("runtime-checkpoint-enrollment-change-requests", false, "measure the #2696 Enrollment change-request and parent/OGS dialogue workload instead of the checkpoint baseline")

const changeRequestWorkloadVersion = "enrollment-2696-change-requests-v1"

// changeRequestCase is one enrollment request the workload drives over its
// status token: the family side of the dialogue.
type changeRequestCase struct {
	requestID   int64
	childID     int64
	statusToken string
	email       string
	// openID is the change request currently open on the request, 0 when the
	// last one was decided. status mirrors its dialogue state.
	openID int64
	status string
	// proposedName is the guardian last name the open change request proposes.
	proposedName string
}

// changeRequestWorkload drives the #2696 capability through the production
// router. The dialogue case keeps one change request open for questions and
// replies; the decision case is filed and decided over and over. Scenario
// definitions stay identical across runs; only the resolved path and body of
// each request change. Preparation requests run outside the timed window.
type changeRequestWorkload struct {
	handler   http.Handler
	db        *testpkg.DB
	tenantID  int64
	token     string
	phaseID   int64
	slug      string
	dialogue  changeRequestCase
	decision  changeRequestCase
	missingID int64
	// prepared counts untimed preparation requests so their unique client
	// addresses never collide with the measured sequence.
	prepared int

	created, rejected, approved, questions, replies int
	approvedName                                    string
}

func newChangeRequestWorkload(t *testing.T, handler http.Handler, token string, phaseID int64, slug string) *changeRequestWorkload {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	w := &changeRequestWorkload{handler: handler, db: db, tenantID: testpkg.Tenant(t), token: token, phaseID: phaseID, slug: slug}
	w.dialogue = w.submit(t, "dialogue")
	w.decision = w.submit(t, "decision")
	require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence('enrollment.change_requests', 'id'))").Scan(context.Background(), &w.missingID))
	w.dialogue.openID, w.dialogue.proposedName = w.create(t, &w.dialogue, "Dialogue-Korrigiert")
	w.dialogue.status = "pending_review"
	return w
}

// submit files one public enrollment and moves its child onto the waitlist:
// a submitted child is edited directly, a decided one via change request.
func (w *changeRequestWorkload) submit(t *testing.T, name string) changeRequestCase {
	t.Helper()
	c := changeRequestCase{email: "checkpoint-" + name + "@example.test"}
	body := fmt.Sprintf(`{"phase_id":%d,"guardian_first_name":"Checkpoint","guardian_last_name":%q,"guardian_email":%q,"consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},"children":[{"first_name":"Checkpoint","last_name":%q,"date_of_birth":"2018-04-15","target_grade_level":2}]}`, w.phaseID, "Applicant", c.email, name)
	submitted := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: "/api/enrollment/" + w.slug + "/submit", Body: body}, "")
	require.Equal(t, http.StatusCreated, submitted.Code, submitted.Body.String())
	var envelope struct {
		Data struct {
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(submitted.Body.Bytes(), &envelope))
	var err error
	c.requestID, err = strconv.ParseInt(envelope.Data.RequestID, 10, 64)
	require.NoError(t, err)
	require.NoError(t, w.db.NewRaw("SELECT status_token FROM enrollment.requests WHERE id = ? AND tenant_id = ?", c.requestID, w.tenantID).Scan(context.Background(), &c.statusToken))
	require.NoError(t, w.db.NewRaw("SELECT id FROM enrollment.request_children WHERE request_id = ? AND tenant_id = ?", c.requestID, w.tenantID).Scan(context.Background(), &c.childID))
	decided := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: fmt.Sprintf("/api/enrollment/admin/requests/%d/children/%d/decide", c.requestID, c.childID), Authenticated: true, Body: `{"status":"waitlisted","reason":"Checkpoint waitlist"}`}, w.token)
	require.Equal(t, http.StatusOK, decided.Code, decided.Body.String())
	return c
}

func (w *changeRequestWorkload) publicPath(c *changeRequestCase) string {
	return "/api/enrollment/requests/" + c.statusToken + "/change-requests"
}

func (w *changeRequestWorkload) proposal(c *changeRequestCase, name string) string {
	return fmt.Sprintf(`{"phase_id":%d,"guardian_first_name":"Checkpoint","guardian_last_name":%q,"guardian_email":%q,"consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},"children":[{"id":"%d","first_name":"Checkpoint","last_name":"Child","date_of_birth":"2018-04-15","target_grade_level":2}]}`, w.phaseID, name, c.email, c.childID)
}

// create files a change request outside timing and returns its ID and the
// guardian last name it proposes. Preparation requests use their own client
// sequence so their addresses never collide with measured ones.
func (w *changeRequestWorkload) create(t *testing.T, c *changeRequestCase, name string) (int64, string) {
	t.Helper()
	w.prepared++
	sequence := 200000 + w.prepared
	name = strings.ReplaceAll(name, "{{attempt}}", fmt.Sprint(sequence))
	body := w.proposal(c, name)
	response := checkpointSequencedRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: w.publicPath(c), Body: body}, "", sequence)
	require.Equal(t, http.StatusCreated, response.Code, "prepare change request: %s", response.Body.String())
	w.created++
	return changeRequestResponseID(t, response), name
}

func changeRequestResponseID(t *testing.T, response *httptest.ResponseRecorder) int64 {
	t.Helper()
	var envelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	id, err := strconv.ParseInt(envelope.Data.ID, 10, 64)
	require.NoError(t, err)
	require.Positive(t, id)
	return id
}

func (w *changeRequestWorkload) adminPath(id int64, suffix string) string {
	return fmt.Sprintf("/api/enrollment/admin/change-requests/%d%s", id, suffix)
}

func (w *changeRequestWorkload) untimed(t *testing.T, scenario checkpointScenario, token string, expected int, what string) {
	t.Helper()
	response := checkpointRequest(w.handler, scenario, token)
	require.Equal(t, expected, response.Code, "prepare %s: %s", what, response.Body.String())
}

func (w *changeRequestWorkload) askUntimed(t *testing.T) {
	w.untimed(t, checkpointScenario{Method: http.MethodPost, Path: w.adminPath(w.dialogue.openID, "/question"), Authenticated: true, Body: `{"body":"Welche Klasse besucht das Kind?"}`}, w.token, http.StatusOK, "question")
	w.questions++
	w.dialogue.status = "needs_parent_response"
}

func (w *changeRequestWorkload) replyUntimed(t *testing.T) {
	w.untimed(t, checkpointScenario{Method: http.MethodPost, Path: fmt.Sprintf("%s/%d/messages", w.publicPath(&w.dialogue), w.dialogue.openID), Body: `{"body":"Die 2a."}`}, "", http.StatusOK, "reply")
	w.replies++
	w.dialogue.status = "pending_review"
}

func (w *changeRequestWorkload) rejectUntimed(t *testing.T) {
	w.untimed(t, checkpointScenario{Method: http.MethodPost, Path: w.adminPath(w.decision.openID, "/reject"), Authenticated: true, Body: `{"note":"Bitte mit Nachweis neu einreichen."}`}, w.token, http.StatusOK, "reject")
	w.rejected++
	w.decision.openID = 0
}

// Placeholders resolved per request. Scenario definitions keep them so the
// recorded operation stays identical across runs.
const (
	changeRequestOpenPlaceholder     = "{{open_change_request}}"
	changeRequestDecisionPlaceholder = "{{decision_change_request}}"
)

func (w *changeRequestWorkload) scenarios() []checkpointScenario {
	dialogue := w.publicPath(&w.dialogue)
	return []checkpointScenario{
		{"enrollment.change-request.public-list", "GET", dialogue, 200, false, ""},
		{"enrollment.change-request.admin-list", "GET", fmt.Sprintf("/api/enrollment/admin/change-requests/?request_id=%d", w.dialogue.requestID), 200, true, ""},
		{"enrollment.change-request.admin-detail", "GET", "/api/enrollment/admin/change-requests/" + changeRequestOpenPlaceholder, 200, true, ""},
		{"enrollment.change-request.review-open", "GET", "/api/enrollment/admin/change-requests/list?view=open&limit=20", 200, true, ""},
		{"enrollment.change-request.review-history", "GET", "/api/enrollment/admin/change-requests/list?view=history&limit=20", 200, true, ""},
		{"enrollment.change-request.pending-count", "GET", "/api/enrollment/admin/change-requests/pending-count", 200, true, ""},
		{"enrollment.change-request.question", "POST", "/api/enrollment/admin/change-requests/" + changeRequestOpenPlaceholder + "/question", 200, true, `{"body":"Welche Klasse besucht das Kind?"}`},
		{"enrollment.change-request.parent-reply", "POST", dialogue + "/" + changeRequestOpenPlaceholder + "/messages", 200, false, `{"body":"Die 2a."}`},
		{"enrollment.change-request.parent-reply-wrong-status", "POST", dialogue + "/" + changeRequestOpenPlaceholder + "/messages", 400, false, `{"body":"Zu früh."}`},
		{"enrollment.change-request.create", "POST", w.publicPath(&w.decision), 201, false, w.proposal(&w.decision, "Applicant-{{attempt}}")},
		{"enrollment.change-request.reject", "POST", "/api/enrollment/admin/change-requests/" + changeRequestDecisionPlaceholder + "/reject", 200, true, `{"note":"Bitte mit Nachweis neu einreichen."}`},
		{"enrollment.change-request.approve", "POST", "/api/enrollment/admin/change-requests/" + changeRequestDecisionPlaceholder + "/approve", 200, true, `{"note":"Freigegeben."}`},
		{"enrollment.change-request.admin-not-found", "GET", fmt.Sprintf("/api/enrollment/admin/change-requests/%d", w.missingID), 404, true, ""},
		{"enrollment.change-request.public-list-unknown-token", "GET", "/api/enrollment/requests/unknown-token/change-requests", 404, false, ""},
	}
}

// prepare puts the dialogue or decision case into the state the scenario
// needs, outside timing, and resolves the request path. The token is the
// staff token for authenticated scenarios and empty for the family side.
func (w *changeRequestWorkload) prepare(t *testing.T, scenario checkpointScenario, sequence int) (checkpointScenario, string) {
	t.Helper()
	switch scenario.Name {
	case "enrollment.change-request.question", "enrollment.change-request.parent-reply-wrong-status":
		if w.dialogue.status == "needs_parent_response" {
			w.replyUntimed(t)
		}
	case "enrollment.change-request.parent-reply":
		if w.dialogue.status == "pending_review" {
			w.askUntimed(t)
		}
	case "enrollment.change-request.create":
		if w.decision.openID != 0 {
			w.rejectUntimed(t)
		}
		w.decision.proposedName = strings.ReplaceAll("Applicant-{{attempt}}", "{{attempt}}", fmt.Sprint(sequence))
	case "enrollment.change-request.reject", "enrollment.change-request.approve":
		if w.decision.openID == 0 {
			w.decision.openID, w.decision.proposedName = w.create(t, &w.decision, "Applicant-{{attempt}}")
		}
	}
	scenario.Path = strings.ReplaceAll(scenario.Path, changeRequestOpenPlaceholder, strconv.FormatInt(w.dialogue.openID, 10))
	scenario.Path = strings.ReplaceAll(scenario.Path, changeRequestDecisionPlaceholder, strconv.FormatInt(w.decision.openID, 10))
	token := ""
	if scenario.Authenticated {
		token = w.token
	}
	return scenario, token
}

// observe records the state change a successful measured request made.
func (w *changeRequestWorkload) observe(t *testing.T, scenario checkpointScenario, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Code != scenario.ExpectedStatus {
		return
	}
	switch scenario.Name {
	case "enrollment.change-request.question":
		w.questions++
		w.dialogue.status = "needs_parent_response"
	case "enrollment.change-request.parent-reply":
		w.replies++
		w.dialogue.status = "pending_review"
	case "enrollment.change-request.create":
		w.created++
		w.decision.openID = changeRequestResponseID(t, response)
	case "enrollment.change-request.reject":
		w.rejected++
		w.decision.openID = 0
	case "enrollment.change-request.approve":
		w.approved++
		w.approvedName = w.decision.proposedName
		w.decision.openID = 0
	}
}

// finalState proves every successful request is persisted and nothing else:
// counts, terminal statuses, the dialogue state, and the applied proposal.
func (w *changeRequestWorkload) finalState(t *testing.T) map[string]int {
	t.Helper()
	ctx := context.Background()
	var total, pending, needsParent, approved, rejected, messages, dialogueOpen int
	require.NoError(t, w.db.NewRaw("SELECT count(*) FROM enrollment.change_requests WHERE tenant_id = ?", w.tenantID).Scan(ctx, &total))
	require.NoError(t, w.db.NewRaw("SELECT count(*) FILTER (WHERE status = 'pending_review'), count(*) FILTER (WHERE status = 'needs_parent_response'), count(*) FILTER (WHERE status = 'approved'), count(*) FILTER (WHERE status = 'rejected') FROM enrollment.change_requests WHERE tenant_id = ?", w.tenantID).Scan(ctx, &pending, &needsParent, &approved, &rejected))
	require.NoError(t, w.db.NewRaw("SELECT count(*) FROM enrollment.change_request_messages WHERE tenant_id = ?", w.tenantID).Scan(ctx, &messages))
	require.NoError(t, w.db.NewRaw("SELECT count(*) FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ? AND status IN ('pending_review', 'needs_parent_response')", w.tenantID, w.dialogue.requestID).Scan(ctx, &dialogueOpen))
	require.Equal(t, w.created, total, "every accepted change request must be persisted")
	require.Equal(t, w.approved, approved)
	require.Equal(t, w.rejected, rejected)
	require.Equal(t, 1, dialogueOpen, "the dialogue keeps exactly one open change request")
	openDecision := 0
	if w.decision.openID != 0 {
		openDecision = 1
	}
	require.Equal(t, w.created-w.approved-w.rejected, pending+needsParent)
	require.Equal(t, 1+openDecision, pending+needsParent)
	require.Equal(t, w.questions+w.replies+w.approved+w.rejected, messages, "questions, replies and decision notes are the dialogue")
	var guardianLastName string
	require.NoError(t, w.db.NewRaw("SELECT guardian_last_name FROM enrollment.requests WHERE id = ? AND tenant_id = ?", w.decision.requestID, w.tenantID).Scan(ctx, &guardianLastName))
	require.Equal(t, w.approvedName, guardianLastName, "the last approved proposal must be applied")
	return map[string]int{
		"change_requests": total, "approved": approved, "rejected": rejected, "open": pending + needsParent,
		"messages": messages, "questions": w.questions, "replies": w.replies,
	}
}
