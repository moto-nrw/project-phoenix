package careplanhttp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-ogs/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

// lifecycleExcusedFake records the arguments the route forwards and returns a
// canned error, so the tests pin the wire contract rather than the domain.
type lifecycleExcusedFake struct {
	excusedrequests.Service
	markDoneErr     error
	correctErr      error
	gotRequestID    int64
	gotVersion      string
	gotReason       string
	gotApprove      bool
	gotReviewedBy   int64
	markDoneCalls   int
	correctCalls    int
	lastMethodOrder []string
}

func (f *lifecycleExcusedFake) MarkDone(_ context.Context, requestID int64, expectedVersion, reason string, reviewedBy int64) error {
	f.markDoneCalls++
	f.gotRequestID, f.gotVersion, f.gotReason, f.gotReviewedBy = requestID, expectedVersion, reason, reviewedBy
	f.lastMethodOrder = append(f.lastMethodOrder, "mark_done")
	return f.markDoneErr
}

func (f *lifecycleExcusedFake) Correct(_ context.Context, requestID int64, approve bool, expectedVersion, reason string, reviewedBy int64) error {
	f.correctCalls++
	f.gotRequestID, f.gotApprove, f.gotVersion, f.gotReason, f.gotReviewedBy =
		requestID, approve, expectedVersion, reason, reviewedBy
	f.lastMethodOrder = append(f.lastMethodOrder, "correct")
	return f.correctErr
}

// The two request ids these tests drive. Named rather than inlined so the
// assertions and the path cannot drift apart — and because a bare int64
// literal in a test reads like a database id, which these are not.
const (
	markDoneTestRequestID int64 = 42
	correctTestRequestID  int64 = 7
)

// lifecycleRequest builds a POST to the {kind}/{requestId} route of action.
func lifecycleRequest(t *testing.T, action, kind, requestID, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest("POST", "/students/change-requests/"+kind+"/"+requestID+"/"+action, strings.NewReader(body))
	ctx := context.WithValue(req.Context(), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: 55})
	ctx = context.WithValue(ctx, sessiontoken.CtxPermissions, aggUpdatePerms)
	return req.WithContext(ctx)
}

func decodeErrorCode(t *testing.T, rr *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	return body.Code
}

func TestMarkParentRequestDone_ForwardsVersionAndReviewer(t *testing.T) {
	t.Parallel()

	fake := &lifecycleExcusedFake{}
	rs := testResource(careplanhttp.ResourceConfig{
		ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	serveRoute(t, rs, rr, lifecycleRequest(t, "mark-done", "excused", "42",
		`{"expected_version":"v1","reason":"Tage sind vorbei"}`))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, 1, fake.markDoneCalls)
	assert.Equal(t, markDoneTestRequestID, fake.gotRequestID)
	assert.Equal(t, "v1", fake.gotVersion)
	assert.Equal(t, "Tage sind vorbei", fake.gotReason)
	assert.Equal(t, int64(55), fake.gotReviewedBy, "the reviewer comes from the JWT, never the body")
}

// A Stammdaten change applies from the decision onwards: it has no effective
// scope, so it can never be past and the route says so without calling any
// service. That is the same answer a future-dated request of any type gets,
// which keeps the client's branch single.
func TestMarkParentRequestDone_MasterDataIsNeverPast(t *testing.T) {
	t.Parallel()

	fake := &lifecycleExcusedFake{}
	rs := testResource(careplanhttp.ResourceConfig{
		ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	serveRoute(t, rs, rr, lifecycleRequest(t, "mark-done", "master_data", "42", `{}`))

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "students.request_not_past", decodeErrorCode(t, rr))
	assert.Zero(t, fake.markDoneCalls)
}

func TestMarkParentRequestDone_RejectsUnknownKind(t *testing.T) {
	t.Parallel()

	rs := testResource(careplanhttp.ResourceConfig{
		ExcusedRequestService: &lifecycleExcusedFake{}})
	rr := httptest.NewRecorder()
	serveRoute(t, rs, rr, lifecycleRequest(t, "mark-done", "invented", "42", `{}`))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMarkParentRequestDone_MapsLifecycleSentinels(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  error
		code string
		want int
	}{
		{"stale version", parentrequests.ErrStale, "students.change_request_stale", http.StatusConflict},
		{"still in the future", parentrequests.ErrNotPast, "students.request_not_past", http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &lifecycleExcusedFake{markDoneErr: tc.err}
			rs := testResource(careplanhttp.ResourceConfig{
				ExcusedRequestService: fake})
			rr := httptest.NewRecorder()
			serveRoute(t, rs, rr, lifecycleRequest(t, "mark-done", "excused", "42", `{}`))
			assert.Equal(t, tc.want, rr.Code)
			assert.Equal(t, tc.code, decodeErrorCode(t, rr))
		})
	}
}

func TestCorrectParentRequestDecision_ForwardsTheVerdict(t *testing.T) {
	t.Parallel()

	fake := &lifecycleExcusedFake{}
	rs := testResource(careplanhttp.ResourceConfig{
		ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	serveRoute(t, rs, rr, lifecycleRequest(t, "correct", "excused", "7",
		`{"approve":false,"reason":"Falsch entschieden","expected_version":"v2"}`))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, 1, fake.correctCalls)
	assert.Equal(t, correctTestRequestID, fake.gotRequestID)
	assert.False(t, fake.gotApprove)
	assert.Equal(t, "v2", fake.gotVersion)
	assert.Equal(t, "Falsch entschieden", fake.gotReason)
}

func TestCorrectParentRequestDecision_RequiresAVerdict(t *testing.T) {
	t.Parallel()

	fake := &lifecycleExcusedFake{}
	rs := testResource(careplanhttp.ResourceConfig{
		ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	serveRoute(t, rs, rr, lifecycleRequest(t, "correct", "excused", "7", `{"reason":"x"}`))

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Zero(t, fake.correctCalls)
}

// Betreuungszeiten and Angebote keep no pre-decision state, so their decisions
// cannot be reverted. The route says why instead of pretending it worked.
func TestCorrectParentRequestDecision_UnsupportedKindsNameTheReason(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"offering"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			rs := testResource(careplanhttp.ResourceConfig{
				ExcusedRequestService: &lifecycleExcusedFake{}})
			rr := httptest.NewRecorder()
			serveRoute(t, rs, rr, lifecycleRequest(t, "correct", kind, "7", `{"approve":true}`))

			assert.Equal(t, http.StatusConflict, rr.Code)
			assert.Equal(t, "students.correction_unsupported", decodeErrorCode(t, rr))
			var body struct {
				Detail string `json:"detail"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Contains(t, body.Detail, "direkt", "the message must say what to do instead")
		})
	}
}

func TestCorrectParentRequestDecision_MapsNotDecided(t *testing.T) {
	t.Parallel()

	fake := &lifecycleExcusedFake{correctErr: parentrequests.ErrNotDecided}
	rs := testResource(careplanhttp.ResourceConfig{
		ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	serveRoute(t, rs, rr, lifecycleRequest(t, "correct", "excused", "7", `{"approve":true}`))

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "students.request_not_decided", decodeErrorCode(t, rr))
}
