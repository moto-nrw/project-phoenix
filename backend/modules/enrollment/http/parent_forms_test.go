package enrollmenthttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// parentFormsRequestStub records what the parents portal's form flow hands
// the intake. Every other RequestService method panics through the embedded
// nil interface, which would flag an unintended dependency.
type parentFormsRequestStub struct {
	RequestService
	submitCalled   bool
	got            SubmitRequest
	submitTenantID int64
	submitErr      error
	access         EnrolleeAudienceAccess
	loadCalled     bool
}

func (s *parentFormsRequestStub) Submit(ctx context.Context, req SubmitRequest) (*SubmitResult, error) {
	s.submitCalled = true
	s.got = req
	s.submitTenantID = tenant.FromContext(ctx)
	if s.submitErr != nil {
		return nil, s.submitErr
	}
	return &SubmitResult{Request: &Request{ID: 99001, StatusToken: "status-token"}, StatusURL: "/status/status-token"}, nil
}

func (s *parentFormsRequestStub) LoadEnrolleeFormBootstrap(_ context.Context, _ int64, _ time.Time, _ string, access EnrolleeAudienceAccess) (*PublicFormBootstrapData, error) {
	s.loadCalled = true
	s.access = access
	return &PublicFormBootstrapData{Phase: &capability.Phase{}}, nil
}

const parentFormsSubmitBody = `{"phase_id":4242,"guardian_first_name":"Anna","guardian_last_name":"Beispiel","guardian_email":"anna@example.test","late_invite_token":"parent-late-token","children":[{"first_name":"Lara","last_name":"Beispiel","date_of_birth":"2018-03-04"}]}`

// The portal hands over the resolved school, the caller's account and the
// school-wide submit fact; the owner side stamps them onto the public
// submission, under the school's tenant, and keeps the body's late-invite
// token (#1663, #2734).
func TestParentFormsSubmitStampsGuardianAccountTenantAndEligibility(t *testing.T) {
	t.Parallel()

	stub := &parentFormsRequestStub{}
	forms := ParentForms{Requests: stub}
	r := httptest.NewRequest(http.MethodPost, "/parent/enrollments/testschule/submit?late_invite=query-token", strings.NewReader(parentFormsSubmitBody))
	submit, err := forms.DecodeSubmission(r)
	require.NoError(t, err)

	respond, err := submit(context.Background(), 4711, 7777, true, "203.0.113.7")
	require.NoError(t, err)
	require.True(t, stub.submitCalled)
	require.NotNil(t, stub.got.GuardianAccountID)
	assert.Equal(t, int64(7777), *stub.got.GuardianAccountID)
	assert.True(t, stub.got.GuardianSubmitEligible)
	assert.Equal(t, int64(4711), stub.got.TenantID)
	assert.Equal(t, int64(4711), stub.submitTenantID, "the submission runs under the resolved school's tenant")
	assert.Equal(t, "parent-late-token", stub.got.LateInviteToken, "the body's token wins over the query parameter")
	assert.Equal(t, "203.0.113.7", stub.got.RemoteIP)
	assert.Equal(t, int64(4242), stub.got.PhaseID)

	w := httptest.NewRecorder()
	respond(w, httptest.NewRequest(http.MethodPost, "/", nil))
	require.Equal(t, http.StatusCreated, w.Code)
	var body struct {
		Data SubmitEnrollmentResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "99001", body.Data.RequestID)
	assert.Equal(t, "/status/status-token", body.Data.StatusURL)
}

// Without the school-wide submit fact the submission still reaches the
// owner, as a new-child application the phase audience decides (#1663).
func TestParentFormsSubmitForwardsMissingEligibility(t *testing.T) {
	t.Parallel()

	stub := &parentFormsRequestStub{}
	submit, err := ParentForms{Requests: stub}.DecodeSubmission(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(parentFormsSubmitBody)))
	require.NoError(t, err)

	_, err = submit(context.Background(), 4711, 7778, false, "")
	require.NoError(t, err)
	require.True(t, stub.submitCalled)
	assert.False(t, stub.got.GuardianSubmitEligible)
}

// A body without a token takes the late_invite query parameter.
func TestParentFormsDecodeFillsLateInviteTokenFromQuery(t *testing.T) {
	t.Parallel()

	stub := &parentFormsRequestStub{}
	body := strings.Replace(parentFormsSubmitBody, `"late_invite_token":"parent-late-token",`, "", 1)
	submit, err := ParentForms{Requests: stub}.DecodeSubmission(httptest.NewRequest(http.MethodPost, "/?late_invite=+query-token+", strings.NewReader(body)))
	require.NoError(t, err)

	_, err = submit(context.Background(), 4711, 7777, true, "")
	require.NoError(t, err)
	assert.Equal(t, "query-token", stub.got.LateInviteToken)
}

// A body that is not JSON fails the decode before anything is submitted.
func TestParentFormsDecodeRejectsMalformedBody(t *testing.T) {
	t.Parallel()

	stub := &parentFormsRequestStub{}
	_, err := ParentForms{Requests: stub}.DecodeSubmission(httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{")))
	require.Error(t, err)
	assert.False(t, stub.submitCalled)
}

// A refused submission comes back to the portal, which renders it through
// the owner's error mapping like the public route.
func TestParentFormsSubmitReturnsTheOwnersRefusal(t *testing.T) {
	t.Parallel()

	stub := &parentFormsRequestStub{submitErr: capability.ErrChildEnrollmentNotPermitted}
	forms := ParentForms{Requests: stub}
	submit, err := forms.DecodeSubmission(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(parentFormsSubmitBody)))
	require.NoError(t, err)

	_, err = submit(context.Background(), 4711, 7777, true, "")
	require.ErrorIs(t, err, capability.ErrChildEnrollmentNotPermitted)

	w := httptest.NewRecorder()
	forms.RenderSubmitError(w, httptest.NewRequest(http.MethodPost, "/", nil), err)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// The form load forwards the guardian's per-audience access to the enrollee
// gate and renders the bootstrap without a captcha: the parent JWT is the
// anti-bot signal (#1663).
func TestParentFormsLoadForwardsAudienceAccessWithoutCaptcha(t *testing.T) {
	t.Parallel()

	stub := &parentFormsRequestStub{}
	respond, err := ParentForms{Requests: stub}.LoadFormBootstrap(context.Background(), 4242, time.Now(), "", true, false)
	require.NoError(t, err)
	require.True(t, stub.loadCalled)
	assert.Equal(t, EnrolleeAudienceAccess{LinkedParents: true}, stub.access)

	w := httptest.NewRecorder()
	respond(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data PublicEnrollmentFormBootstrapResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, PublicCaptchaConfigResponse{}, body.Data.CaptchaConfig)
}

// A failed form load renders like the public route: a failed legal stage is
// a server error, every other refusal the public gate's 404.
func TestParentFormsRenderFormBootstrapErrorMapsThePublicGate(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	ParentForms{}.RenderFormBootstrapError(w, httptest.NewRequest(http.MethodGet, "/", nil),
		&BootstrapStageError{Stage: BootstrapStageLegal, Err: errors.New("settings unavailable")})
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	w = httptest.NewRecorder()
	ParentForms{}.RenderFormBootstrapError(w, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("phase not open"))
	assert.Equal(t, http.StatusNotFound, w.Code)
}
