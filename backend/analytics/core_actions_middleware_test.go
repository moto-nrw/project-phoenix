package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturedEvent struct {
	sessionID  string
	distinctID string
	event      string
	props      map[string]any
}

// recordingTracker stands in for the PostHog tracker; the tracker's own
// wire format is covered in analytics_test.go.
type recordingTracker struct {
	mu     sync.Mutex
	events []capturedEvent
}

func (r *recordingTracker) Capture(distinctID, event string, props map[string]any) {
	r.CaptureContext(context.Background(), distinctID, event, props)
}

func (r *recordingTracker) CaptureContext(ctx context.Context, distinctID, event string, props map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, capturedEvent{sessionID: SessionIDFromContext(ctx), distinctID: distinctID, event: event, props: props})
}

func (r *recordingTracker) Close() error { return nil }

func (r *recordingTracker) captured() []capturedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]capturedEvent(nil), r.events...)
}

var (
	staffActor  = Actor{Surface: SurfaceOGS, Role: RoleStaff, SchoolID: 42}
	parentActor = Actor{Surface: SurfaceParents, Role: RoleGuardian}
)

const validSessionToken = "session-token"

// newCoreActionRouter mounts routes of the real table the way the production
// root does: the middleware router-wide, the resources as mounted
// subrouters. Every handler answers with the status from ?status=.
func newCoreActionRouter(tracker Tracker, requestActor *Actor) http.Handler {
	answer := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			status := http.StatusOK
			switch r.URL.Query().Get("status") {
			case "201":
				status = http.StatusCreated
			case "204":
				status = http.StatusNoContent
			case "400":
				status = http.StatusBadRequest
			case "403":
				status = http.StatusForbidden
			case "500":
				status = http.StatusInternalServerError
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}
	}

	root := chi.NewRouter()
	root.Use(CoreActionMiddleware(CoreActionConfig{
		Tracker: tracker,
		RequestActor: func(*http.Request) (Actor, bool) {
			if requestActor == nil {
				return Actor{}, false
			}
			return *requestActor, true
		},
		SessionActor: func(token string) (Actor, bool) {
			if token != validSessionToken {
				return Actor{}, false
			}
			return Actor{Surface: SurfaceOGS, Role: RoleAdmin, SchoolID: 9}, true
		},
	}))
	root.Route("/api", func(r chi.Router) {
		groups := chi.NewRouter()
		groups.Post("/", answer(`{"id":123}`))
		groups.Put("/{id}", answer(`{"id":123}`))
		groups.Delete("/{id}", answer(``))
		r.Mount("/groups", groups)
		r.Post("/rooms/export", answer(`%PDF`))
		r.Post("/enrollment/{tenantSlug}/submit", answer(`{}`))
	})
	root.Route("/auth", func(r chi.Router) {
		r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
			// The body picks the login outcome: a session, a second factor,
			// or the enveloped shape.
			var body struct{ Outcome string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			switch body.Outcome {
			case "mfa":
				_, _ = w.Write([]byte(`{"status":"mfa_required","challenge_token":"challenge"}`))
			case "enveloped":
				_, _ = w.Write([]byte(`{"status":"success","data":{"access_token":"` + validSessionToken + `"}}`))
			case "forged":
				_, _ = w.Write([]byte(`{"access_token":"forged"}`))
			default:
				_, _ = w.Write([]byte(`{"status":"authenticated","access_token":"` + validSessionToken + `","refresh_token":"r"}`))
			}
		})
		r.Post("/guardian-invitations/{token}/accept", answer(`{"accepted":true}`))
	})
	root.Get("/api/groups/{id}", answer(`{}`))
	return root
}

func serve(handler http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestCoreActionMiddlewareSendsOneEventOnSuccess(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"200", "201", "204"} {
		tracker := &recordingTracker{}
		router := newCoreActionRouter(tracker, &staffActor)

		rec := serve(router, http.MethodPost, "/api/groups/?status="+status, `{"name":"Bärengruppe"}`, map[string]string{SessionIDHeader: testSessionID})
		require.Less(t, rec.Code, 300)

		events := tracker.captured()
		require.Len(t, events, 1, "status %s", status)
		assert.Equal(t, "group_created", events[0].event)
		assert.Equal(t, "school:42", events[0].distinctID)
		assert.Equal(t, testSessionID, events[0].sessionID)
		assert.Equal(t, map[string]any{"surface": SurfaceOGS, "role": RoleStaff, "school_id": "42"}, events[0].props)
	}
}

func TestCoreActionMiddlewareSendsNothingOnFailure(t *testing.T) {
	t.Parallel()

	for _, status := range []string{"400", "403", "500"} {
		tracker := &recordingTracker{}
		router := newCoreActionRouter(tracker, &staffActor)

		serve(router, http.MethodPost, "/api/groups/?status="+status, `{}`, nil)
		serve(router, http.MethodPut, "/api/groups/123?status="+status, `{}`, nil)

		assert.Empty(t, tracker.captured(), "status %s", status)
	}
}

// chi reports a mounted collection root as /api/groups/ whether or not the
// client wrote the trailing slash; both reach the same entry.
func TestCoreActionMiddlewareMatchesCollectionRootsWithoutTrailingSlash(t *testing.T) {
	t.Parallel()

	tracker := &recordingTracker{}
	router := newCoreActionRouter(tracker, &staffActor)

	serve(router, http.MethodPost, "/api/groups", `{}`, nil)

	events := tracker.captured()
	require.Len(t, events, 1)
	assert.Equal(t, "group_created", events[0].event)
}

// No path parameter, no query and no body value may reach the event.
func TestCoreActionMiddlewareSendsNoRequestValues(t *testing.T) {
	t.Parallel()

	tracker := &recordingTracker{}
	router := newCoreActionRouter(tracker, &staffActor)

	serve(router, http.MethodPut, "/api/groups/98765?status=200&name=Mia", `{"name":"Mia Müller","id":98765}`, map[string]string{SessionIDHeader: "Mia Müller"})

	events := tracker.captured()
	require.Len(t, events, 1)
	assert.Equal(t, "group_updated", events[0].event)
	assert.Empty(t, events[0].sessionID, "a header that is not a session UUID is dropped")
	assert.Equal(t, "school:42", events[0].distinctID)
	assert.Equal(t, map[string]any{"surface": SurfaceOGS, "role": RoleStaff, "school_id": "42"}, events[0].props)
}

func TestCoreActionMiddlewareIgnoresRoutesThatAreNotCaptured(t *testing.T) {
	t.Parallel()

	tracker := &recordingTracker{}
	router := newCoreActionRouter(tracker, &staffActor)

	serve(router, http.MethodDelete, "/api/groups/123", ``, nil)
	serve(router, http.MethodGet, "/api/groups/123", ``, nil)
	serve(router, http.MethodPost, "/api/unknown", `{}`, nil)

	assert.Empty(t, tracker.captured())
}

func TestCoreActionMiddlewareNeedsAnActor(t *testing.T) {
	t.Parallel()

	tracker := &recordingTracker{}
	router := newCoreActionRouter(tracker, nil)

	serve(router, http.MethodPost, "/api/groups/", `{}`, nil)

	assert.Empty(t, tracker.captured(), "a writing route without a session names no surface")
}

func TestCoreActionMiddlewareAddsTheExportType(t *testing.T) {
	t.Parallel()

	tracker := &recordingTracker{}
	router := newCoreActionRouter(tracker, &staffActor)

	serve(router, http.MethodPost, "/api/rooms/export", `{"format":"pdf","room_ids":[1,2]}`, nil)

	events := tracker.captured()
	require.Len(t, events, 1)
	assert.Equal(t, "data_exported", events[0].event)
	assert.Equal(t, "rooms", events[0].props["export_type"])
	assert.NotContains(t, events[0].props, "format")
}

func TestCoreActionMiddlewareParentsHaveNoSchool(t *testing.T) {
	t.Parallel()

	tracker := &recordingTracker{}
	router := newCoreActionRouter(tracker, &parentActor)

	serve(router, http.MethodPost, "/api/groups/", `{}`, nil)

	events := tracker.captured()
	require.Len(t, events, 1)
	assert.Equal(t, "surface:parents", events[0].distinctID)
	assert.Equal(t, map[string]any{"surface": SurfaceParents, "role": RoleGuardian}, events[0].props)
}

// A public form has no session; the route names its surface.
func TestCoreActionMiddlewarePublicRouteUsesItsSurface(t *testing.T) {
	t.Parallel()

	tracker := &recordingTracker{}
	router := newCoreActionRouter(tracker, nil)

	serve(router, http.MethodPost, "/api/enrollment/grundschule-nord/submit", `{}`, nil)

	events := tracker.captured()
	require.Len(t, events, 1)
	assert.Equal(t, "enrollment_submitted", events[0].event)
	assert.Equal(t, map[string]any{"surface": SurfacePublic}, events[0].props)
	assert.NotContains(t, events[0].distinctID, "grundschule-nord")
}

// A login counts once the response mints a session, flat or enveloped. The
// second-factor step and an unverifiable token count nothing, whatever
// session the request itself carried.
func TestCoreActionMiddlewareLoginCountsTheMintedSession(t *testing.T) {
	t.Parallel()

	cases := []struct {
		outcome string
		want    int
	}{
		{outcome: "session", want: 1},
		{outcome: "enveloped", want: 1},
		{outcome: "mfa", want: 0},
		{outcome: "forged", want: 0},
	}
	for _, tc := range cases {
		tracker := &recordingTracker{}
		router := newCoreActionRouter(tracker, &staffActor)

		rec := serve(router, http.MethodPost, "/auth/login", `{"Outcome":"`+tc.outcome+`"}`, map[string]string{SessionIDHeader: testSessionID})
		require.Equal(t, http.StatusOK, rec.Code)
		assert.NotEmpty(t, rec.Body.String(), "the client still gets the whole response")

		events := tracker.captured()
		require.Len(t, events, tc.want, tc.outcome)
		if tc.want == 1 {
			assert.Equal(t, "login_success", events[0].event)
			assert.Equal(t, map[string]any{"surface": SurfaceOGS, "role": RoleAdmin, "school_id": "9"}, events[0].props)
			assert.Equal(t, testSessionID, events[0].sessionID)
		}
	}
}

// A session route that names its surface still counts when the response
// carries no session.
func TestCoreActionMiddlewareSessionRouteFallsBackToItsSurface(t *testing.T) {
	t.Parallel()

	tracker := &recordingTracker{}
	router := newCoreActionRouter(tracker, nil)

	serve(router, http.MethodPost, "/auth/guardian-invitations/secret-token/accept", `{"password":"x"}`, nil)

	events := tracker.captured()
	require.Len(t, events, 1)
	assert.Equal(t, "guardian_invite_accepted", events[0].event)
	assert.Equal(t, map[string]any{"surface": SurfaceParents}, events[0].props)
}

func TestCoreActionMiddlewareWithoutTrackerPassesThrough(t *testing.T) {
	t.Parallel()

	router := newCoreActionRouter(nil, &staffActor)

	rec := serve(router, http.MethodPost, "/api/groups/", `{}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// The session reaches the request context for every request, so a service
// capturing inside the request links its event too.
func TestCoreActionMiddlewareStoresTheSessionForHandlers(t *testing.T) {
	t.Parallel()

	var seen string
	root := chi.NewRouter()
	root.Use(CoreActionMiddleware(CoreActionConfig{Tracker: &recordingTracker{}}))
	root.Get("/anything", func(_ http.ResponseWriter, r *http.Request) { seen = SessionIDFromContext(r.Context()) })

	serve(root, http.MethodGet, "/anything", ``, map[string]string{SessionIDHeader: testSessionID})

	assert.Equal(t, testSessionID, seen)
}

// A response too large to hold is passed on untouched and never parsed.
func TestCappedBufferDropsOversizedBodies(t *testing.T) {
	t.Parallel()

	buffer := &cappedBuffer{limit: 8}
	n, err := buffer.Write([]byte("0123"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)
	n, err = buffer.Write([]byte("456789"))
	require.NoError(t, err)
	assert.Equal(t, 6, n)
	assert.True(t, buffer.overflow)
	assert.Zero(t, buffer.Len())
}

// Every captured entry names an event in snake_case; sessions and surfaces
// are only used where the table's contract allows them.
func TestCoreActionTableEntriesAreWellFormed(t *testing.T) {
	t.Parallel()

	for key, action := range CoreActions() {
		assert.Contains(t, []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}, key.Method, "%v", key)
		assert.True(t, strings.HasPrefix(key.Pattern, "/"), "%v", key)
		if action.Event == "" {
			assert.Equal(t, CoreAction{}, action, "%v: a route that is not captured carries nothing", key)
			continue
		}
		assert.Regexp(t, `^[a-z]+(_[a-z]+)*$`, action.Event, "%v", key)
		if action.ExportType != "" {
			assert.Equal(t, "data_exported", action.Event, "%v", key)
		}
		if action.Surface != "" {
			assert.Contains(t, []string{SurfaceOGS, SurfaceParents, SurfaceSchool, SurfacePublic}, action.Surface, "%v", key)
		}
	}
}
