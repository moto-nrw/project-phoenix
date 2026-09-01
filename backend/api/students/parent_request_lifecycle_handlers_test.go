package students

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// lifecycleExcusedFake records the arguments the route forwards and returns a
// canned error, so the tests pin the wire contract rather than the domain.
type lifecycleExcusedFake struct {
	absenceService.ExcusedAbsenceRequestService
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

// lifecycleRequest builds a POST carrying the {kind}/{requestId} path params
// the chi router would have bound.
func lifecycleRequest(t *testing.T, kind, requestID, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest("POST", "/change-requests/"+kind+"/"+requestID, strings.NewReader(body))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("kind", kind)
	routeCtx.URLParams.Add("requestId", requestID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: 55})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, aggUpdatePerms)
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
	rs := NewResource(ResourceConfig{ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	rs.markParentRequestDone(rr, lifecycleRequest(t, "excused", "42",
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
	rs := NewResource(ResourceConfig{ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	rs.markParentRequestDone(rr, lifecycleRequest(t, "master_data", "42", `{}`))

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "request_not_past", decodeErrorCode(t, rr))
	assert.Zero(t, fake.markDoneCalls)
}

func TestMarkParentRequestDone_RejectsUnknownKind(t *testing.T) {
	t.Parallel()

	rs := NewResource(ResourceConfig{ExcusedRequestService: &lifecycleExcusedFake{}})
	rr := httptest.NewRecorder()
	rs.markParentRequestDone(rr, lifecycleRequest(t, "invented", "42", `{}`))

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
		{"stale version", userService.ErrParentRequestStale, "change_request_stale", http.StatusConflict},
		{"still in the future", userService.ErrParentRequestNotPast, "request_not_past", http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := &lifecycleExcusedFake{markDoneErr: tc.err}
			rs := NewResource(ResourceConfig{ExcusedRequestService: fake})
			rr := httptest.NewRecorder()
			rs.markParentRequestDone(rr, lifecycleRequest(t, "excused", "42", `{}`))
			assert.Equal(t, tc.want, rr.Code)
			assert.Equal(t, tc.code, decodeErrorCode(t, rr))
		})
	}
}

func TestCorrectParentRequestDecision_ForwardsTheVerdict(t *testing.T) {
	t.Parallel()

	fake := &lifecycleExcusedFake{}
	rs := NewResource(ResourceConfig{ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	rs.correctParentRequestDecision(rr, lifecycleRequest(t, "excused", "7",
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
	rs := NewResource(ResourceConfig{ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	rs.correctParentRequestDecision(rr, lifecycleRequest(t, "excused", "7", `{"reason":"x"}`))

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
			rs := NewResource(ResourceConfig{ExcusedRequestService: &lifecycleExcusedFake{}})
			rr := httptest.NewRecorder()
			rs.correctParentRequestDecision(rr, lifecycleRequest(t, kind, "7", `{"approve":true}`))

			assert.Equal(t, http.StatusConflict, rr.Code)
			assert.Equal(t, "correction_unsupported", decodeErrorCode(t, rr))
			var body struct {
				Error string `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Contains(t, body.Error, "direkt", "the message must say what to do instead")
		})
	}
}

func TestCorrectParentRequestDecision_MapsNotDecided(t *testing.T) {
	t.Parallel()

	fake := &lifecycleExcusedFake{correctErr: userService.ErrParentRequestNotDecided}
	rs := NewResource(ResourceConfig{ExcusedRequestService: fake})
	rr := httptest.NewRecorder()
	rs.correctParentRequestDecision(rr, lifecycleRequest(t, "excused", "7", `{"approve":true}`))

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Equal(t, "request_not_decided", decodeErrorCode(t, rr))
}
