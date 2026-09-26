package common

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionRecordingTransport keeps every event the Sentry client would send.
// The client sends synchronously, so the buffer only has to hold one
// request's events.
type sessionRecordingTransport chan *sentry.Event

func (t sessionRecordingTransport) Configure(sentry.ClientOptions)        {}
func (t sessionRecordingTransport) Flush(time.Duration) bool              { return true }
func (t sessionRecordingTransport) FlushWithContext(context.Context) bool { return true }
func (t sessionRecordingTransport) Close()                                {}
func (t sessionRecordingTransport) SendEvent(event *sentry.Event)         { t <- event }

type sessionEvents struct {
	requestID string
	events    []*sentry.Event
}

// serveSentrySession sends a request with the given session through the
// middleware order of api.New: request ID, Recoverer, Sentry reporting, the
// root verifier and the session tags. A nil session sends no token. The
// request comes from a client IP, directly and forwarded, so the tests can
// prove it stays out. The Sentry client records instead of sending and keeps
// the SDK defaults the serve command uses (sendDefaultPii off).
func serveSentrySession(t *testing.T, session *jwt.AppClaims, handler http.HandlerFunc) sessionEvents {
	t.Helper()

	tokenAuth, err := jwt.NewTokenAuthWithDurations("sentry-session-test-secret-32-chars", 15*time.Minute, time.Hour)
	require.NoError(t, err)

	transport := make(sessionRecordingTransport, 8)
	client, err := sentry.NewClient(sentry.ClientOptions{Transport: transport})
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(ServerErrorReporting)
	router.Use(tokenAuth.Verifier())
	router.Use(SentrySessionContext)
	var requestID string
	router.Get("/api/students/{id}", func(w http.ResponseWriter, r *http.Request) {
		requestID = middleware.GetReqID(r.Context())
		handler(w, r)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/students/42", nil)
	req.RemoteAddr = "203.0.113.7:4711"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")
	if session != nil {
		token, err := tokenAuth.CreateJWT(*session)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
	}

	hub := sentry.NewHub(client, sentry.NewScope())
	router.ServeHTTP(httptest.NewRecorder(), req.WithContext(sentry.SetHubOnContext(req.Context(), hub)))
	close(transport)
	got := sessionEvents{requestID: requestID}
	for event := range transport {
		got.events = append(got.events, event)
	}
	return got
}

func failWithCode(w http.ResponseWriter, r *http.Request) {
	errResp := ErrorInternalServer(errors.New("load student: connection reset")).(*ErrResponse)
	errResp.Code = "STUDENT_LOAD_FAILED"
	RenderError(w, r, errResp)
}

func panicking(http.ResponseWriter, *http.Request) { panic("nil student record") }

// staffSession is an OGS staff session with every personal field a token
// can carry, so the tests can prove none of it leaves for Sentry.
func staffSession() *jwt.AppClaims {
	return &jwt.AppClaims{
		ID:        7,
		Sub:       "erika.mustermann@ogs-wissingen.de",
		Username:  "erika.mustermann@ogs-wissingen.de",
		FirstName: "Erika",
		LastName:  "Mustermann",
		Roles:     []string{"Betreuung Nord"},
		Scope:     "tenant",
		TenantID:  12,
	}
}

// Issue #3643: a server error of a tenant-scoped session names the school,
// the account, the portal, the role, the Vorgangskennung and the error code.
func TestServerErrorEventOfTenantSessionCarriesSessionContext(t *testing.T) {
	t.Parallel()

	got := serveSentrySession(t, staffSession(), failWithCode)

	require.Len(t, got.events, 1)
	require.NotEmpty(t, got.requestID)
	event := got.events[0]
	assert.Equal(t, map[string]string{
		"school_id":        "12",
		"portal":           "tenant",
		"role":             "staff",
		"request_id":       got.requestID,
		"error_code":       "STUDENT_LOAD_FAILED",
		"http.status_code": "500",
	}, event.Tags)
	assert.Equal(t, sentry.User{ID: "7"}, event.User)
}

// sentryhttp reports a panic, not the 5xx check; the event names the
// session all the same.
func TestPanicEventCarriesSessionContext(t *testing.T) {
	t.Parallel()

	got := serveSentrySession(t, staffSession(), panicking)

	require.Len(t, got.events, 1)
	event := got.events[0]
	assert.Equal(t, map[string]string{
		"school_id":  "12",
		"portal":     "tenant",
		"role":       "staff",
		"request_id": got.requestID,
	}, event.Tags)
	assert.Equal(t, sentry.User{ID: "7"}, event.User)
}

// Operator and cross-school parent sessions are not bound to one school: the
// tag is absent, not a placeholder.
func TestServerErrorEventOmitsSchoolWithoutSingleSchoolSession(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		session jwt.AppClaims
		portal  string
		role    string
	}{
		{
			name:    "operator",
			session: jwt.AppClaims{ID: 3, Sub: "3", Roles: []string{"operator"}, Scope: "platform"},
			portal:  "operator", role: "operator",
		},
		{
			name:    "operator with a tenant claim",
			session: jwt.AppClaims{ID: 3, Sub: "3", Roles: []string{"operator"}, Scope: "platform", TenantID: 12},
			portal:  "operator", role: "operator",
		},
		{
			name:    "parent across schools",
			session: jwt.AppClaims{ID: 21, Sub: "21", Roles: []string{"guardian"}, Scope: "parent"},
			portal:  "parent", role: "guardian",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := serveSentrySession(t, &tc.session, failWithCode)

			require.Len(t, got.events, 1)
			event := got.events[0]
			assert.NotContains(t, event.Tags, "school_id")
			assert.Equal(t, tc.portal, event.Tags["portal"])
			assert.Equal(t, tc.role, event.Tags["role"])
			assert.Equal(t, got.requestID, event.Tags["request_id"])
			assert.Equal(t, sentry.User{ID: tc.session.Sub}, event.User)
		})
	}
}

// Portal and role follow the token scope with the frontend's values; role
// names of the school never leave.
func TestSentrySessionRoleFollowsTheTokenScope(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		session  jwt.AppClaims
		portal   string
		role     string
		schoolID string
	}{
		{
			name:    "OGS admin",
			session: jwt.AppClaims{ID: 7, Sub: "7", Roles: []string{"Leitung"}, Scope: "tenant", TenantID: 12, IsAdmin: true},
			portal:  "tenant", role: "admin", schoolID: "12",
		},
		{
			name:    "staff preview is an admin at work",
			session: jwt.AppClaims{ID: 8, Sub: "8", Roles: []string{"Betreuung"}, TenantID: 12, ReadOnly: true, ActingAdminID: 7},
			portal:  "tenant", role: "admin", schoolID: "12",
		},
		{
			name:    "carrier in one school",
			session: jwt.AppClaims{ID: 9, Sub: "9", Roles: []string{"Träger"}, Scope: "org", OrgID: 4, TenantID: 12},
			portal:  "tenant", role: "carrier", schoolID: "12",
		},
		{
			name:    "carrier without a school",
			session: jwt.AppClaims{ID: 9, Sub: "9", Roles: []string{"Träger"}, Scope: "org", OrgID: 4},
			portal:  "tenant", role: "carrier",
		},
		{
			name:    "Lehrkraft",
			session: jwt.AppClaims{ID: 5, Sub: "5", Roles: []string{"Lehrkraft"}, Scope: "school", TenantID: 12},
			portal:  "school", role: "lehrkraft", schoolID: "12",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := serveSentrySession(t, &tc.session, failWithCode)

			require.Len(t, got.events, 1)
			event := got.events[0]
			assert.Equal(t, tc.portal, event.Tags["portal"])
			assert.Equal(t, tc.role, event.Tags["role"])
			assert.Equal(t, tc.schoolID, event.Tags["school_id"])
			assert.Equal(t, sentry.User{ID: tc.session.Sub}, event.User)
		})
	}
}

// A request without a session names no one; the Vorgangskennung still finds
// it.
func TestServerErrorEventWithoutSessionCarriesNoUser(t *testing.T) {
	t.Parallel()

	got := serveSentrySession(t, nil, failWithCode)

	require.Len(t, got.events, 1)
	event := got.events[0]
	assert.Empty(t, event.User)
	assert.NotContains(t, event.Tags, "portal")
	assert.NotContains(t, event.Tags, "role")
	assert.NotContains(t, event.Tags, "school_id")
	assert.Equal(t, got.requestID, event.Tags["request_id"])
}

// No event carries a name, an e-mail address or the client IP (#3590), even
// where the token and the connection hold them.
func TestSentryEventsCarryNoNameEmailOrIP(t *testing.T) {
	t.Parallel()

	handlers := map[string]http.HandlerFunc{"server error": failWithCode, "panic": panicking}
	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := serveSentrySession(t, staffSession(), handler)

			require.Len(t, got.events, 1)
			payload, err := json.Marshal(got.events[0])
			require.NoError(t, err)
			for _, personal := range []string{"Erika", "Mustermann", "ogs-wissingen", "203.0.113.7"} {
				assert.NotContains(t, string(payload), personal)
			}
		})
	}
}
