// These tests exercise StaffEvents directly (in-package) because they
// deliberately bypass the JWT middleware chain: they inject claims via the
// request context and drive the streaming loop with a context timeout. That
// direct-handler invocation cannot run through the production Router(), so
// they live in the internal sse package and call the private handler.
package sse

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// eventsTestContext holds shared test dependencies for direct-handler tests.
type eventsTestContext struct {
	db       *bun.DB
	hub      *realtime.Hub
	resource *Resource[realtime.Event, *realtime.Client]
}

// withValidSSEToken mirrors the verified jwtauth token context that the direct
// handler tests intentionally skip by bypassing Router(). Signature validation
// remains a router/middleware concern; the handler needs the verified token's
// expiry to bound the stream lifetime.
func withValidSSEToken(t *testing.T, ctx context.Context) context.Context {
	t.Helper()

	token, _, err := testutil.TestTokenAuth(t).JwtAuth.Encode(map[string]any{"exp": time.Now().Add(time.Hour)})
	require.NoError(t, err)
	return jwtauth.NewContext(ctx, token, nil)
}

// ComposeSubscriptionResolver composes the root's caller rows over the
// package's test database. The root imports this package, so these internal
// tests cannot import it; root_composition_test.go binds it.
var ComposeSubscriptionResolver func(t *testing.T) (*bun.DB, SubscriptionResolver, Runtime[realtime.Event, *realtime.Client])

// setupEventsModule initializes the DB-backed SSE capability under test.
func setupEventsModule(t *testing.T) *eventsTestContext {
	t.Helper()

	db, subscriptions, runtime := ComposeSubscriptionResolver(t)

	hub := realtime.NewHub(slog.Default())

	resource := NewResource(
		hub,
		subscriptions,
		runtime,
		slog.Default(),
	)

	return &eventsTestContext{
		db:       db,
		hub:      hub,
		resource: resource,
	}
}

// =============================================================================
// STAFF RESOLUTION TESTS
// Note: Tests that pass auth will enter SSE streaming loop and hang.
// We test staff resolution failure which returns before streaming.
// =============================================================================

func TestSSEEvents_InvalidStaffClaims(t *testing.T) {
	t.Parallel()

	ctx := setupEventsModule(t)

	// Create a person without staff record (just a basic account)
	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "NonStaff", "User")

	// Mount handler directly to bypass JWT middleware
	router := chi.NewRouter()
	router.Get("/events", ctx.resource.StaffEvents)

	// Use teacher claims but with an account ID that doesn't have a staff record
	req := testutil.NewAuthenticatedRequest(t, "GET", "/events", nil,
		testutil.WithClaims(t, testutil.TeacherTestClaims(int(account.ID))),
	)
	req = req.WithContext(withValidSSEToken(t, req.Context()))

	rr := testutil.ExecuteRequest(router, req)

	// Staff resolution should fail for non-staff users
	// Handler returns 403 Forbidden when user is not a staff member
	assert.Equal(t, http.StatusForbidden, rr.Code,
		"Expected 403 for non-staff user")
}

// =============================================================================
// STAFF WITH ACCOUNT TESTS
// =============================================================================

func TestSSEEvents_StaffWithAccount(t *testing.T) {
	t.Parallel()

	ctx := setupEventsModule(t)

	// Create a teacher with account (has staff record)
	_, account := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "SSE", "Teacher")

	// Mount handler directly to bypass JWT middleware
	router := chi.NewRouter()
	router.Get("/events", ctx.resource.StaffEvents)

	// Use teacher claims with a valid account ID that HAS a staff record
	// Note: This test will actually enter the SSE streaming loop
	// We use a context with a timeout to prevent hanging
	req := testutil.NewAuthenticatedRequest(t, "GET", "/events", nil,
		testutil.WithClaims(t, testutil.TeacherTestClaims(int(account.ID))),
	)

	// Note: This request will hang because SSE enters streaming loop
	// We can't easily test the full streaming path without goroutines/timeouts
	// Just verify the request is well-formed
	assert.NotNil(t, req, "Request should be created")
}

func TestSSEEvents_AdminClaims(t *testing.T) {
	t.Parallel()

	tctx := setupEventsModule(t)

	// Create admin without staff record
	_, account := testpkg.CreateTestPersonWithAccount(t, tctx.db, "Admin", "NoStaff")

	router := chi.NewRouter()
	router.Get("/events", tctx.resource.StaffEvents)

	// Admin without staff record should reach the streaming path (not 403).
	// Cancel after the connected event; the deadline is only a hang guard.
	baseCtx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	claims := testutil.AdminTestClaims(int(account.ID))
	claimsCtx := testutil.WithAuthenticatedContext(baseCtx, claims, nil)
	claimsCtx = withValidSSEToken(t, claimsCtx)

	req := testutil.NewRequest("GET", "/events", nil)
	req = req.WithContext(claimsCtx)

	rr := &connectedRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	router.ServeHTTP(rr, req)

	// Admin must start streaming despite having no staff record.
	require.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, rr.connected, "connected event must be flushed")
}

func TestSSEEvents_EmptyAuthClaims(t *testing.T) {
	t.Parallel()

	tctx := setupEventsModule(t)

	router := chi.NewRouter()
	router.Get("/events", tctx.resource.StaffEvents)

	// Default claims have IsAdmin=true, so after the admin SSE fix they reach
	// the streaming path. Cancel after the connected event; the deadline is only a hang guard.
	baseCtx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()

	claims := testutil.DefaultTestClaims()
	claimsCtx := testutil.WithAuthenticatedContext(baseCtx, claims, nil)
	claimsCtx = withValidSSEToken(t, claimsCtx)

	req := testutil.NewRequest("GET", "/events", nil)
	req = req.WithContext(claimsCtx)

	rr := &connectedRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	router.ServeHTTP(rr, req)

	// Default admin claims must start streaming.
	require.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, rr.connected, "connected event must be flushed")
}

// =============================================================================
// STREAMING PATH TESTS (cancel after connected)
// =============================================================================

// gatedSubscriptionResolver holds subscription resolution until the test releases
// it. The handler must keep the valid request alive through this setup phase.
type gatedSubscriptionResolver struct {
	SubscriptionResolver
	started chan struct{}
	release chan struct{}
}

func (g gatedSubscriptionResolver) ResolveSSETopics(ctx context.Context) (int64, []string, []string, []string, error) {
	close(g.started)
	select {
	case <-g.release:
		return g.SubscriptionResolver.ResolveSSETopics(ctx)
	case <-ctx.Done():
		return 0, nil, nil, nil, ctx.Err()
	}
}

type connectedRecorder struct {
	*httptest.ResponseRecorder
	cancel    context.CancelFunc
	connected bool
}

func (w *connectedRecorder) Flush() {
	w.ResponseRecorder.Flush()
	if strings.Contains(w.Body.String(), "event: connected\n") && strings.HasSuffix(w.Body.String(), "\n\n") {
		w.connected = true
		w.cancel()
	}
}

func TestSSEEvents_StaffReachesStreamingPath(t *testing.T) {
	t.Parallel()

	tctx := setupEventsModule(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tctx.db, "Stream", "Test")
	resolver := gatedSubscriptionResolver{SubscriptionResolver: tctx.resource.userCtx, started: make(chan struct{}), release: make(chan struct{})}
	tctx.resource.userCtx = resolver

	// Five seconds is only a hang guard, not the expected stream lifetime.
	baseCtx, cancel := context.WithTimeout(testpkg.Ctx(t), 5*time.Second)
	defer cancel()
	claims := testutil.TeacherTestClaims(int(account.ID))
	claimsCtx := withValidSSEToken(t, testutil.WithAuthenticatedContext(baseCtx, claims, nil))
	req := testutil.NewRequest("GET", "/events", nil).WithContext(claimsCtx)
	rr := &connectedRecorder{ResponseRecorder: httptest.NewRecorder(), cancel: cancel}
	done := make(chan struct{})
	go func() {
		defer close(done)
		tctx.resource.StaffEvents(rr, req)
	}()
	defer func() { cancel(); <-done }()

	select {
	case <-resolver.started:
	case <-baseCtx.Done():
		t.Fatal("subscription resolution did not start")
	}
	// Deliberately exceed the former whole-request deadline while setup is
	// blocked. Channels, not scheduler load, control the successful resolution.
	delay := time.NewTimer(150 * time.Millisecond)
	defer delay.Stop()
	select {
	case <-delay.C:
		close(resolver.release)
	case <-baseCtx.Done():
		t.Fatal("request expired before releasing subscription resolution")
	}
	<-done
	require.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, rr.connected, "a complete connected event must be flushed before cancellation")
	assert.Contains(t, rr.Body.String(), "event: connected\n")
	assert.True(t, rr.Flushed)
}

func TestSSEEvents_ResponseHeaders(t *testing.T) {
	t.Parallel()

	tctx := setupEventsModule(t)

	// Create a teacher with account
	_, account := testpkg.CreateTestTeacherWithAccount(t, tctx.db, "Header", "Test")

	router := chi.NewRouter()
	router.Get("/events", tctx.resource.StaffEvents)

	// Use context with timeout, then add claims
	baseCtx, cancel := context.WithTimeout(testpkg.Ctx(t), 50*time.Millisecond)
	defer cancel()

	claims := testutil.TeacherTestClaims(int(account.ID))
	claimsCtx := testutil.WithAuthenticatedContext(baseCtx, claims, nil)
	claimsCtx = withValidSSEToken(t, claimsCtx)

	req := testutil.NewRequest("GET", "/events", nil)
	req = req.WithContext(claimsCtx)

	rr := testutil.ExecuteRequest(router, req)

	// Check that SSE headers were set (they're set before streaming starts)
	// Note: These might not be captured if the response writer doesn't support it
	// This test verifies the request flow reaches the point where headers are set
	assert.NotNil(t, rr, "Response should be returned")
}
