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

var runtimeCheckpointEnrollmentAcceptance = flag.Bool("runtime-checkpoint-enrollment-acceptance", false, "measure the #2699 Enrollment acceptance workload instead of the checkpoint baseline")

const acceptanceWorkloadVersion = "enrollment-2699-acceptance-v1"

// acceptanceChild is one submitted request child a decision scenario consumes.
type acceptanceChild struct {
	requestID int64
	childID   int64
}

// acceptanceWorkload drives the #2699 acceptance workflow through the
// production router: every decision scenario consumes a freshly submitted
// child that untimed preparation created, so each measured request performs a
// real state transition. The parent-account scenario submits through the
// authenticated parent route so the approval attaches the existing platform
// account through the Identity & Access capability instead of queueing an
// invitation.
type acceptanceWorkload struct {
	handler         http.Handler
	db              *testpkg.DB
	tenantID        int64
	token           string
	parentToken     string
	parentAccountID int64
	phaseID         int64
	slug            string
	subdomain       string
	// prepared counts untimed preparation requests so their unique client
	// addresses and e-mails never collide with the measured sequence.
	prepared int
	// pending holds the child the next measured request of a scenario decides.
	pending map[string]acceptanceChild
	// anchor is a submitted request whose children the not-found scenario
	// addresses with an unused child ID.
	anchor         acceptanceChild
	missingChildID int64

	submitted, parentSubmitted, approved, parentApproved, rejected, waitlisted int
}

func newAcceptanceWorkload(t *testing.T, handler http.Handler, token, parentToken string, parentAccountID, phaseID int64, slug, subdomain string) *acceptanceWorkload {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	w := &acceptanceWorkload{
		handler: handler, db: db, tenantID: testpkg.Tenant(t), token: token, parentToken: parentToken,
		parentAccountID: parentAccountID, phaseID: phaseID, slug: slug, subdomain: subdomain,
		pending: map[string]acceptanceChild{},
	}
	w.anchor = w.submitPublic(t)
	require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence('enrollment.request_children', 'id'))").Scan(context.Background(), &w.missingChildID))
	return w
}

func (w *acceptanceWorkload) nextSequence() int {
	w.prepared++
	return 300000 + w.prepared
}

// submitPublic files one anonymous enrollment with a unique e-mail and client
// address, outside timing, and returns the child to decide.
func (w *acceptanceWorkload) submitPublic(t *testing.T) acceptanceChild {
	t.Helper()
	sequence := w.nextSequence()
	body := fmt.Sprintf(`{"phase_id":%d,"guardian_first_name":"Checkpoint","guardian_last_name":"Applicant","guardian_email":"checkpoint-acceptance-{{attempt}}@example.test","consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},"children":[{"first_name":"Checkpoint","last_name":"Child{{attempt}}","date_of_birth":"2018-04-15","target_grade_level":2}]}`, w.phaseID)
	response := checkpointSequencedRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: "/api/enrollment/" + w.slug + "/submit", Body: body}, "", sequence)
	require.Equal(t, http.StatusCreated, response.Code, "prepare public submission: %s", response.Body.String())
	w.submitted++
	return w.childOf(t, response)
}

// submitParent files one enrollment through the authenticated parent route,
// so the request carries the guardian's account and the approval reaches the
// Identity & Access capability.
func (w *acceptanceWorkload) submitParent(t *testing.T) acceptanceChild {
	t.Helper()
	sequence := w.nextSequence()
	body := fmt.Sprintf(`{"phase_id":%d,"guardian_first_name":"Checkpoint","guardian_last_name":"Parent","guardian_email":"checkpoint-parent-acceptance-{{attempt}}@example.test","consent_flags":{"agb":true,"data_processing":true,"email_contact":true,"photo":true},"children":[{"first_name":"Checkpoint","last_name":"ParentChild{{attempt}}","date_of_birth":"2018-04-15","target_grade_level":2}]}`, w.phaseID)
	response := checkpointSequencedRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: "/parent/enrollments/" + w.subdomain + "/submit", Authenticated: true, Body: body}, w.parentToken, sequence)
	require.Equal(t, http.StatusCreated, response.Code, "prepare parent submission: %s", response.Body.String())
	w.submitted++
	w.parentSubmitted++
	return w.childOf(t, response)
}

func (w *acceptanceWorkload) childOf(t *testing.T, response *httptest.ResponseRecorder) acceptanceChild {
	t.Helper()
	var envelope struct {
		Data struct {
			RequestID string `json:"request_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	requestID, err := strconv.ParseInt(envelope.Data.RequestID, 10, 64)
	require.NoError(t, err)
	var childID int64
	require.NoError(t, w.db.NewRaw("SELECT id FROM enrollment.request_children WHERE request_id = ? AND tenant_id = ?", requestID, w.tenantID).Scan(context.Background(), &childID))
	return acceptanceChild{requestID: requestID, childID: childID}
}

func (w *acceptanceWorkload) decidePath(child acceptanceChild) string {
	return fmt.Sprintf("/api/enrollment/admin/requests/%d/children/%d/decide", child.requestID, child.childID)
}

// rejectUntimed moves a fresh child into a terminal status outside timing so
// the terminal-conflict scenario measures the stable 400 contract.
func (w *acceptanceWorkload) rejectUntimed(t *testing.T, child acceptanceChild) {
	t.Helper()
	response := checkpointRequest(w.handler, checkpointScenario{Method: http.MethodPost, Path: w.decidePath(child), Authenticated: true, Body: `{"status":"rejected","reason":"Checkpoint terminal"}`}, w.token)
	require.Equal(t, http.StatusOK, response.Code, "prepare terminal child: %s", response.Body.String())
	w.rejected++
}

// Placeholders resolved per request. Scenario definitions keep them so the
// recorded operation stays identical across runs.
const acceptanceChildPlaceholder = "{{pending_child}}"

func (w *acceptanceWorkload) scenarios() []checkpointScenario {
	return []checkpointScenario{
		{"enrollment.acceptance.approve", "POST", acceptanceChildPlaceholder, 200, true, `{"status":"approved","reason":"Aufgenommen."}`},
		{"enrollment.acceptance.approve-parent-account", "POST", acceptanceChildPlaceholder, 200, true, `{"status":"approved","reason":"Aufgenommen."}`},
		{"enrollment.acceptance.reject", "POST", acceptanceChildPlaceholder, 200, true, `{"status":"rejected","reason":"Kein Platz."}`},
		{"enrollment.acceptance.waitlist", "POST", acceptanceChildPlaceholder, 200, true, `{"status":"waitlisted","reason":"Warteliste."}`},
		{"enrollment.acceptance.approve-terminal", "POST", acceptanceChildPlaceholder, 400, true, `{"status":"approved","reason":"Zu spät."}`},
		{"enrollment.acceptance.invalid-status", "POST", acceptanceChildPlaceholder, 400, true, `{"status":"bogus"}`},
		{"enrollment.acceptance.child-not-found", "POST", w.decidePath(acceptanceChild{requestID: w.anchor.requestID, childID: w.missingChildID}), 404, true, `{"status":"approved"}`},
	}
}

// prepare creates the child a scenario decides, outside timing, and resolves
// the request path. Every measured request gets its own child; the terminal
// scenario additionally rejects it first.
func (w *acceptanceWorkload) prepare(t *testing.T, scenario checkpointScenario) (checkpointScenario, string) {
	t.Helper()
	var child acceptanceChild
	switch scenario.Name {
	case "enrollment.acceptance.approve", "enrollment.acceptance.reject", "enrollment.acceptance.waitlist", "enrollment.acceptance.invalid-status":
		child = w.submitPublic(t)
	case "enrollment.acceptance.approve-parent-account":
		child = w.submitParent(t)
	case "enrollment.acceptance.approve-terminal":
		child = w.submitPublic(t)
		w.rejectUntimed(t, child)
	}
	if child.childID != 0 {
		w.pending[scenario.Name] = child
		scenario.Path = strings.ReplaceAll(scenario.Path, acceptanceChildPlaceholder, w.decidePath(child))
	}
	return scenario, w.token
}

// observe records the state change a successful measured request made.
func (w *acceptanceWorkload) observe(scenario checkpointScenario, response *httptest.ResponseRecorder) {
	if response.Code != scenario.ExpectedStatus {
		return
	}
	switch scenario.Name {
	case "enrollment.acceptance.approve":
		w.approved++
	case "enrollment.acceptance.approve-parent-account":
		w.approved++
		w.parentApproved++
	case "enrollment.acceptance.reject":
		w.rejected++
	case "enrollment.acceptance.waitlist":
		w.waitlisted++
	}
}

// finalState proves every successful decision is persisted and nothing else:
// child statuses, one student per approval, the parent account's school
// access, and the absence of invitations for the linked account.
func (w *acceptanceWorkload) finalState(t *testing.T) map[string]int {
	t.Helper()
	ctx := context.Background()
	var children, approved, rejected, waitlisted, submitted int
	require.NoError(t, w.db.NewRaw(`SELECT count(*), count(*) FILTER (WHERE child.status = 'approved'), count(*) FILTER (WHERE child.status = 'rejected'),
		count(*) FILTER (WHERE child.status = 'waitlisted'), count(*) FILTER (WHERE child.status = 'submitted')
		FROM enrollment.request_children child JOIN enrollment.requests request ON request.id = child.request_id AND request.tenant_id = child.tenant_id
		WHERE request.phase_id = ? AND child.tenant_id = ?`, w.phaseID, w.tenantID).Scan(ctx, &children, &approved, &rejected, &waitlisted, &submitted))
	require.Equal(t, w.submitted, children, "every prepared submission must be persisted")
	require.Equal(t, w.approved, approved, "approved children must match the successful approvals")
	require.Equal(t, w.rejected, rejected, "rejected children must match the successful rejections")
	require.Equal(t, w.waitlisted, waitlisted)
	require.Equal(t, w.submitted-w.approved-w.rejected-w.waitlisted, submitted, "invalid and not-found decisions must not change a child")

	var students, linkedStudents int
	require.NoError(t, w.db.NewRaw(`SELECT count(*) FILTER (WHERE child.created_student_id IS NOT NULL), count(student.id)
		FROM enrollment.request_children child JOIN enrollment.requests request ON request.id = child.request_id AND request.tenant_id = child.tenant_id
		LEFT JOIN users.students student ON student.id = child.created_student_id AND student.tenant_id = child.tenant_id
		WHERE request.phase_id = ? AND child.tenant_id = ?`, w.phaseID, w.tenantID).Scan(ctx, &students, &linkedStudents))
	require.Equal(t, w.approved, students, "every approval must link exactly one created student")
	require.Equal(t, students, linkedStudents, "every linked student must exist")

	var parentApproved, parentLinked int
	require.NoError(t, w.db.NewRaw(`SELECT count(*) FILTER (WHERE child.status = 'approved'),
		count(*) FILTER (WHERE child.status = 'approved' AND link.guardian_profile_id = profile.id)
		FROM enrollment.request_children child
		JOIN enrollment.requests request ON request.id = child.request_id AND request.tenant_id = child.tenant_id
		JOIN users.guardian_profiles profile ON profile.account_id = request.guardian_account_id AND profile.tenant_id = child.tenant_id
		LEFT JOIN users.students_guardians link ON link.student_id = child.created_student_id AND link.tenant_id = child.tenant_id AND link.is_primary
		WHERE request.phase_id = ? AND child.tenant_id = ? AND request.guardian_account_id = ?`, w.phaseID, w.tenantID, w.parentAccountID).Scan(ctx, &parentApproved, &parentLinked))
	require.Equal(t, w.parentApproved, parentApproved, "parent-account approvals must be persisted")
	require.Equal(t, parentApproved, parentLinked, "every parent-account approval must link the student to the parent's guardian profile")

	var mappingStatus string
	require.NoError(t, w.db.NewRaw("SELECT status FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?", w.parentAccountID, w.tenantID).Scan(ctx, &mappingStatus))
	require.Equal(t, "active", mappingStatus, "the parent account keeps active school access")
	var guardianRoles int
	require.NoError(t, w.db.NewRaw("SELECT count(*) FROM auth.account_roles ar JOIN auth.roles r ON r.id = ar.role_id WHERE ar.account_id = ? AND ar.tenant_id = ? AND LOWER(r.name) = 'guardian'", w.parentAccountID, w.tenantID).Scan(ctx, &guardianRoles))
	require.Equal(t, 1, guardianRoles, "repeated approvals must not duplicate the guardian role assignment")
	var parentInvitations int
	require.NoError(t, w.db.NewRaw("SELECT count(*) FROM auth.guardian_invitations invitation JOIN users.guardian_profiles profile ON profile.id = invitation.guardian_profile_id WHERE profile.account_id = ? AND invitation.tenant_id = ?", w.parentAccountID, w.tenantID).Scan(ctx, &parentInvitations))
	require.Zero(t, parentInvitations, "a linked account must never receive an invitation")

	return map[string]int{
		"submitted_children": children, "approved": approved, "rejected": rejected, "waitlisted": waitlisted, "undecided": submitted,
		"created_students": students, "parent_submissions": w.parentSubmitted, "parent_approved": parentApproved,
	}
}
