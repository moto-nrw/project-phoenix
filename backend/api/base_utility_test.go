package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	customMiddleware "github.com/moto-nrw/project-phoenix/middleware"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMealPlanErrorRendererContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, targetCode, internalMessage, errorText, code, anchor string
		err                                                        error
		status                                                     int
	}{
		{name: "disabled", targetCode: "meal_plan_disabled", status: http.StatusForbidden, errorText: "feature_disabled", code: "meals.plan_disabled", anchor: "anleitung-zugriff-pruefen"},
		{name: "invalid date", targetCode: "invalid_meal_date", status: http.StatusBadRequest, errorText: "meal plan covers weekdays only (Monday-Friday)", code: "general.input", anchor: "anleitung-eingabe-pruefen"},
		{name: "invalid allergens", targetCode: "invalid_allergens", status: http.StatusBadRequest, errorText: "allergens must be codes of the 14 main allergens", code: "meals.invalid_allergens", anchor: "anleitung-eingabe-pruefen"},
		{name: "invalid dishes", targetCode: "invalid_dishes", status: http.StatusBadRequest, errorText: "invalid_dishes", code: "general.input", anchor: "anleitung-eingabe-pruefen"},
		{name: "internal", err: errors.New("database unavailable"), status: http.StatusInternalServerError, internalMessage: "failed to load meal plan", errorText: "failed to load meal plan", code: "general.server", anchor: "anleitung-unerwarteter-fehler"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			const requestID = "meal-plan-contract-request"
			request := httptest.NewRequest(http.MethodGet, "/meal-plan", nil)
			request = request.WithContext(context.WithValue(request.Context(), chimiddleware.RequestIDKey, requestID))
			response := httptest.NewRecorder()
			err := test.err
			if test.targetCode != "" {
				for _, rule := range mealPlanErrorRules {
					if rule.Target.Error() == test.targetCode {
						err = rule.Target
						break
					}
				}
				require.Error(t, err)
			}
			renderMealPlanFailure(response, request, err, test.internalMessage)
			assert.Equal(t, test.status, response.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, map[string]any{
				"status":   float64(test.status),
				"code":     test.code,
				"type":     "https://moto-app.de/help/fehlermeldungen#" + test.anchor,
				"title":    http.StatusText(test.status),
				"detail":   test.errorText,
				"instance": requestID,
			}, body)
		})
	}
}

func TestRequireHomeLayoutOperationsPanicsWhenTheCapabilityIsMissing(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t,
		"settings platform does not provide home layout operations",
		func() { requireHomeLayoutOperations(struct{}{}) },
	)
}

// TestParseAllowedOrigins tests the parseAllowedOrigins function
func TestParseAllowedOrigins(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		envValue          string
		expectedExact     []string
		expectedWildcards []string
	}{
		{
			name:              "empty env var returns wildcard",
			envValue:          "",
			expectedExact:     []string{"*"},
			expectedWildcards: nil,
		},
		{
			name:              "single origin",
			envValue:          "http://localhost:3000",
			expectedExact:     []string{"http://localhost:3000"},
			expectedWildcards: nil,
		},
		{
			name:              "multiple origins with spaces",
			envValue:          "http://localhost:3000, https://example.com, https://app.example.com",
			expectedExact:     []string{"http://localhost:3000", "https://example.com", "https://app.example.com"},
			expectedWildcards: nil,
		},
		{
			name:              "multiple origins without spaces",
			envValue:          "http://localhost:3000,https://example.com,https://app.example.com",
			expectedExact:     []string{"http://localhost:3000", "https://example.com", "https://app.example.com"},
			expectedWildcards: nil,
		},
		{
			name:              "origins with excessive whitespace",
			envValue:          "  http://localhost:3000  ,  https://example.com  ",
			expectedExact:     []string{"http://localhost:3000", "https://example.com"},
			expectedWildcards: nil,
		},
		{
			name:              "wildcard explicitly set",
			envValue:          "*",
			expectedExact:     []string{"*"},
			expectedWildcards: nil,
		},
		{
			name:              "wildcard subdomain pattern",
			envValue:          "http://localhost:3000, *.example.com",
			expectedExact:     []string{"http://localhost:3000"},
			expectedWildcards: []string{".example.com"},
		},
		{
			name:              "only blanks fall back to wildcard",
			envValue:          " ,  , ",
			expectedExact:     []string{"*"},
			expectedWildcards: nil,
		},
		{
			name:              "only wildcard patterns",
			envValue:          "*.example.com, *.test.local",
			expectedExact:     nil,
			expectedWildcards: []string{".example.com", ".test.local"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call function under test
			exact, wildcards := parseAllowedOrigins(tt.envValue)

			// Assert results
			assert.Equal(t, tt.expectedExact, exact)
			assert.Equal(t, tt.expectedWildcards, wildcards)
		})
	}
}

// TestParsePositiveInt tests the parsePositiveInt function
func TestParsePositiveInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		envVar       string
		envValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "empty env var returns default",
			envVar:       "TEST_INT_EMPTY",
			envValue:     "",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "valid positive int",
			envVar:       "TEST_INT_VALID",
			envValue:     "100",
			defaultValue: 60,
			expected:     100,
		},
		{
			name:         "zero returns default",
			envVar:       "TEST_INT_ZERO",
			envValue:     "0",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "negative returns default",
			envVar:       "TEST_INT_NEGATIVE",
			envValue:     "-5",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "non-numeric string returns default",
			envVar:       "TEST_INT_INVALID",
			envValue:     "invalid",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "float string returns default",
			envVar:       "TEST_INT_FLOAT",
			envValue:     "12.5",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "very large positive int",
			envVar:       "TEST_INT_LARGE",
			envValue:     "999999",
			defaultValue: 60,
			expected:     999999,
		},
		{
			name:         "whitespace returns default",
			envVar:       "TEST_INT_WHITESPACE",
			envValue:     "  ",
			defaultValue: 60,
			expected:     60,
		},
		{
			name:         "number with leading/trailing spaces returns default",
			envVar:       "TEST_INT_SPACES",
			envValue:     " 100 ",
			defaultValue: 60,
			expected:     60, // strconv.Atoi doesn't trim, so this fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call function under test
			result := parsePositiveInt(tt.envValue, tt.defaultValue)

			// Assert result
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParsePositiveInt_DifferentDefaults tests parsePositiveInt with various default values
func TestParsePositiveInt_DifferentDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		defaultValue int
	}{
		{"default 1", 1},
		{"default 10", 10},
		{"default 60", 60},
		{"default 100", 100},
		{"default 1000", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't set any env var, so it uses default
			result := parsePositiveInt("", tt.defaultValue)
			assert.Equal(t, tt.defaultValue, result)
		})
	}
}

func checkCaregiverWiring(t *testing.T, graph *goldenRuntime) {
	t.Parallel()

	composition := setupCaregiverCompositionModule(t, graph)

	require.True(t, composition.authWired)
	require.True(t, composition.operatorWired)
	assert.True(t, composition.sharedCapability)
}

type caregiverComposition struct {
	authWired        bool
	operatorWired    bool
	sharedCapability bool
}

// setupCaregiverCompositionModule composes the account and operator routes
// over the golden graph with the builders the Serve root mounts them with.
func setupCaregiverCompositionModule(t *testing.T, graph *goldenRuntime) *caregiverComposition {
	t.Helper()
	sessionAuth, err := newSessionTokenAuth()
	require.NoError(t, err)
	_, _, _, _, _, hub, _, _, settings, _, services, schedules, _, active, _, _, _, _, provisioning, _, _, _, deviceFleet, _, settingsWrites, _, _, err := composeTestRoot(graph.db, slog.Default(), currentTestServiceConfig())
	require.NoError(t, err)
	auth := newAccountResource(graph.db, nil, nil, services.CaregiverCapability, settings.Service)
	operator, err := newOperatorRouter(graph.db, slog.Default(), sessionAuth, authRateLimiters{}, devicefleetCompose.NewIoTService(deviceFleet), nil,
		settings.Service, hub, active, settingsWrites, nil, provisioning, services.CaregiverCapability, schedules.Lifecycle, nil, nil)
	require.NoError(t, err)
	return &caregiverComposition{
		authWired:        auth != nil,
		operatorWired:    operator != nil,
		sharedCapability: auth.CaregiverCapabilityService == any(caregiverCapabilityViews(graph.db, services.CaregiverCapability)),
	}
}

func setupSettingsCallbackRoute(t *testing.T) *settingsCallbackRoute {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, hub, err := NewSettingsCallbacksTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), newStudentPhotoTestUnlinker())
	require.NoError(t, err)
	repos, err := NewEnrollmentTestRepositoriesWithMembers(testpkg.NewDatabase(t, db), owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	homeLayouts, ok := module.Settings.(interface {
		HomeLayout(context.Context, int64, int64, []string) (any, error)
		SetHomeLayout(context.Context, int64, int64, map[string]bool, []string, map[string]int, map[string]int, map[string]int, bool) error
		ResetHomeLayout(context.Context, int64, int64) error
		SetHomeBlockPolicies(context.Context, int64, int64, []string, map[string]string) error
	})
	require.True(t, ok)
	return &settingsCallbackRoute{router: newSettingsResource(testpkg.NewDatabase(t, db), module.TenantSettings, homeLayouts, repos.Enrollment().SchemaReferencesLegalDocument).SettingsRouter(), hub: hub}
}

func setupOperatorInvitationRoute(t *testing.T, graph *goldenRuntime) chi.Router {
	t.Helper()
	// Mount the operator routes of the golden graph again, with rate limiting
	// enabled. The golden graph's router remains unchanged.
	sessionAuth, err := newSessionTokenAuth()
	require.NoError(t, err)
	_, _, _, _, _, hub, _, _, settings, _, services, schedules, _, active, _, _, _, _, provisioning, _, _, _, deviceFleet, _, settingsWrites, _, _, err := composeTestRoot(graph.db, slog.Default(), currentTestServiceConfig())
	require.NoError(t, err)
	operator, err := newOperatorRouter(graph.db, slog.Default(), sessionAuth, buildAuthRateLimiters(nil, "5", demoLoopbackExempt()),
		devicefleetCompose.NewIoTService(deviceFleet), nil, settings.Service, hub, active, settingsWrites, nil, provisioning,
		services.CaregiverCapability, schedules.Lifecycle, nil, nil)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Mount("/operator", operator)
	return router
}

func TestSyncClientIPToRemoteAddrUsesChiClientIP(t *testing.T) {
	t.Parallel()

	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFFTrustedProxies(1))
	router.Use(syncClientIPToRemoteAddr)
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "203.0.113.10", r.RemoteAddr)
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.RemoteAddr = "172.18.0.4:54321"
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func checkOperatorInvitationMount(t *testing.T, graph *goldenRuntime) {
	t.Parallel()
	router := setupOperatorInvitationRoute(t, graph)

	req := httptest.NewRequest(http.MethodPost, "/operator/auth/invitations/validate", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code)
}

// The tests below pin the quota-identity contract from #2064: authenticated
// requests are bucketed by account ID + scope + tenant ID from verified
// claims, not by a per-token hash. Re-logins and token refreshes therefore
// share the account's budget instead of minting a fresh one.

func rateLimitTestAuth(t *testing.T) *sessiontoken.TokenAuth {
	t.Helper()
	tokenAuth, err := sessiontoken.NewTokenAuthWithDurations("rate-limit-test-secret-32-chars!!", 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)
	return tokenAuth
}

func mintRateLimitJWT(t *testing.T, tokenAuth *sessiontoken.TokenAuth, claims sessiontoken.AppClaims) string {
	t.Helper()
	if len(claims.Roles) == 0 {
		claims.Roles = []string{"user"}
	}
	token, err := tokenAuth.CreateJWT(claims)
	require.NoError(t, err)
	return token
}

func rateLimitRequest(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/students", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// TestRateLimiting_SameIdentitySharesBudget wires the identity key function
// into the real limiter middleware and asserts the #2064 acceptance
// criteria end to end: two valid sessions of one identity drain ONE budget,
// while a different account and unauthenticated IP traffic stay unaffected.
func TestRateLimiting_SameIdentitySharesBudget(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	// 1 request/minute refill with burst 3: the refill contributes no tokens
	// within the test's runtime, so exactly 3 requests per key pass.
	limiter := customMiddleware.NewRateLimiter(1, 3)
	limiter.SetKeyFunc(tokenAuth.RateLimitKey)

	router := chi.NewRouter()
	router.Use(limiter.Middleware())
	router.Get("/api/students", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	sessionA1 := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
		ID: 42, Sub: "user@example.com", FirstName: "SessionA1", TenantID: 7,
	})
	sessionA2 := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
		ID: 42, Sub: "user@example.com", FirstName: "SessionA2", TenantID: 7,
	})
	require.NotEqual(t, sessionA1, sessionA2)
	otherAccount := mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
		ID: 43, Sub: "other@example.com", TenantID: 7,
	})

	do := func(token string) int {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, rateLimitRequest(token))
		return rr.Code
	}

	// Alternating sessions of the same identity drain one shared budget.
	assert.Equal(t, http.StatusOK, do(sessionA1))
	assert.Equal(t, http.StatusOK, do(sessionA2))
	assert.Equal(t, http.StatusOK, do(sessionA1))
	assert.Equal(t, http.StatusTooManyRequests, do(sessionA2),
		"the second session must NOT have its own budget")
	assert.Equal(t, http.StatusTooManyRequests, do(sessionA1))

	// A different account is unaffected by the exhausted budget.
	assert.Equal(t, http.StatusOK, do(otherAccount))

	// Unauthenticated traffic uses the IP bucket, which is also untouched.
	assert.Equal(t, http.StatusOK, do(""))
}

// TestRateLimiting_ConcurrentSessionsShareBudget hammers one identity from
// two sessions in parallel (run with -race in CI) and asserts the combined
// allowance equals exactly one burst — no per-session multiplication.
func TestRateLimiting_ConcurrentSessionsShareBudget(t *testing.T) {
	t.Parallel()
	tokenAuth := rateLimitTestAuth(t)

	const burst = 10
	limiter := customMiddleware.NewRateLimiter(1, burst)
	limiter.SetKeyFunc(tokenAuth.RateLimitKey)

	router := chi.NewRouter()
	router.Use(limiter.Middleware())
	router.Get("/api/students", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	sessions := []string{
		mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
			ID: 42, Sub: "user@example.com", FirstName: "SessionA", TenantID: 7,
		}),
		mintRateLimitJWT(t, tokenAuth, sessiontoken.AppClaims{
			ID: 42, Sub: "user@example.com", FirstName: "SessionB", TenantID: 7,
		}),
	}
	require.NotEqual(t, sessions[0], sessions[1])

	var allowed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, rateLimitRequest(sessions[i%2]))
			if rr.Code == http.StatusOK {
				allowed.Add(1)
			}
		}(i)
	}
	wg.Wait()

	assert.EqualValues(t, burst, allowed.Load(),
		"50 parallel requests across two sessions of one identity must pass exactly one burst")
}

// TestOnValueSetCallback_WCEnabled tests that the OnValueSet callback
// the Serve root registers triggers WC infrastructure creation
// when checkout.wc_enabled is set to true.
func TestOnValueSetCallback_WCEnabled(t *testing.T) {
	t.Parallel()

	route := setupSettingsCallbackRoute(t)
	require.False(t, tenantHasSystemActivity(t, facilities.WCActivityName))

	// Set checkout.wc_enabled = true → triggers callback → creates WC infrastructure
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.wc_enabled",
		map[string]any{"value": true},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)

	rr := testutil.ExecuteRequest(route.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.True(t, tenantHasSystemActivity(t, facilities.WCActivityName),
		"enabling the WC checkout must provision the WC system activity")
}

// TestOnValueSetCallback_SchulhofEnabled tests that the callback triggers
// Schulhof infrastructure creation when checkout.schulhof_enabled is set to true.
func TestOnValueSetCallback_SchulhofEnabled(t *testing.T) {
	t.Parallel()

	route := setupSettingsCallbackRoute(t)
	require.False(t, tenantHasSystemActivity(t, facilities.SchulhofActivityName))

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled",
		map[string]any{"value": true},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)

	rr := testutil.ExecuteRequest(route.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.True(t, tenantHasSystemActivity(t, facilities.SchulhofActivityName),
		"enabling the Schulhof checkout must provision the Schulhof system activity")
}

// TestOnValueSetCallback_FalseValueSkipsInfrastructure tests that setting
// a checkout setting to false does not trigger infrastructure creation.
func TestOnValueSetCallback_FalseValueSkipsInfrastructure(t *testing.T) {
	t.Parallel()

	route := setupSettingsCallbackRoute(t)

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.wc_enabled",
		map[string]any{"value": false},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)

	rr := testutil.ExecuteRequest(route.router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	assert.False(t, tenantHasSystemActivity(t, facilities.WCActivityName),
		"disabling the WC checkout must not provision the WC system activity")
}

// tenantHasSystemActivity reports whether the test's tenant holds the system
// activity a checkout switch provisions.
func tenantHasSystemActivity(t *testing.T, name string) bool {
	t.Helper()
	exists, err := testpkg.SetupTestDB(t).NewSelect().
		TableExpr("activities.groups").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("name = ?", name).
		Where("is_system = TRUE").
		Exists(testpkg.Ctx(t))
	require.NoError(t, err)
	return exists
}

// adminClaimsForCallback returns admin claims with config permissions
// for testing the OnValueSet callback.
func adminClaimsForCallback() sessiontoken.AppClaims {
	claims := testutil.DefaultTestClaims()
	claims.Permissions = append(claims.Permissions,
		permissions.ConfigRead,
		permissions.ConfigUpdate,
		permissions.ConfigManage,
	)
	return claims
}

// TestOnValueSetCallback_StudentPhotosDisableBroadcastsUpdate locks in the
// invariant that disabling operations.student_photos_enabled emits a single
// student_updated event after the post-commit purge runs. Other open tabs
// rely on this broadcast to invalidate cached photo_url values that point
// at files the purge just deleted — without it, lists / detail slide-overs
// / room views keep rendering the stale URLs and 404 on every avatar until
// a manual reload.
//
// We register a fake SSE client on the real Hub (the type is concrete and
// can't be mocked through an interface) and read events off its channel.
// Empty subscribed-groups slice still receives BroadcastToAll events, which
// is the path the post-commit closure uses.
func TestOnValueSetCallback_StudentPhotosDisableBroadcastsUpdate(t *testing.T) {
	t.Parallel()

	route := setupSettingsCallbackRoute(t)

	// Subscribe a fake SSE client to the real Hub so we can observe
	// BroadcastToAll without mocking. UserID/TenantID are arbitrary —
	// BroadcastToAll fan-outs ignore both.
	client := &realtime.Client{
		Channel:          make(chan realtime.Event, 4),
		UserID:           1,
		SubscribedGroups: map[string]bool{},
	}
	route.hub.Register(client, testpkg.Tenant(t), nil)
	defer route.hub.Unregister(client)

	router := route.router

	// First, enable so PUT-false is a real disable transition (default is
	// already false; set true→false to exercise the purge branch). The
	// enable path now ALSO broadcasts student_updated (tenant_photos_enabled)
	// so tabs that fetched students while photos were off invalidate their
	// photo-stripped responses; verify that here before draining the
	// channel for the disable assertion below.
	enableReq := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled",
		map[string]any{"value": true},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)
	enableRR := testutil.ExecuteRequest(router, enableReq)
	require.Equal(t, http.StatusOK, enableRR.Code, "enable PUT must succeed")

	enableDeadline := time.After(500 * time.Millisecond)
	enableSawStudentUpdated := false
	for !enableSawStudentUpdated {
		select {
		case ev := <-client.Channel:
			if ev.Type != realtime.EventStudentUpdated {
				continue
			}
			require.NotNil(t, ev.Data.Source,
				"student_updated on enable must carry a source label")
			assert.Equal(t, "tenant_photos_enabled", *ev.Data.Source,
				"the enable-side broadcast must label its source so log review distinguishes it from the disable-side purge broadcast")
			enableSawStudentUpdated = true
		case <-enableDeadline:
			t.Fatal("expected student_updated event after photo enable, none received within 500ms")
		}
	}
	drainEvents(client.Channel)

	disableReq := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled",
		map[string]any{"value": false},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)
	disableRR := testutil.ExecuteRequest(router, disableReq)
	require.Equal(t, http.StatusOK, disableRR.Code, "disable PUT must succeed")

	// The post-commit closure broadcasts synchronously after the tx
	// commits, but the channel send is non-blocking (buffered) — give it a
	// short window to surface.
	deadline := time.After(500 * time.Millisecond)
	for {
		select {
		case ev := <-client.Channel:
			if ev.Type == realtime.EventStudentUpdated {
				assert.NotNil(t, ev.Data.Source, "student_updated must carry a source label")
				if ev.Data.Source != nil {
					assert.Equal(t, "tenant_photos_disabled", *ev.Data.Source,
						"the post-purge broadcast must label its source so log review can spot it")
				}
				return
			}
		case <-deadline:
			t.Fatal("expected student_updated event after photo purge, none received within 500ms")
		}
	}
}

// drainEvents non-blockingly empties any pending events on the channel.
// Used between PUTs in the photo-disable broadcast test so we don't read
// pre-existing events from the enable PUT and confuse them with the
// purge-driven broadcast we're actually testing.
func drainEvents(ch chan realtime.Event) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// TestOnValueSetCallback_TenantSettingsChangedBroadcasts pins the cross-
// origin sync contract for settings whose value travels through
// /auth/tenant/resolve. Operator-side writes happen on a different origin
// from the tenant tabs that need to react (operator.<domain> vs
// <slug>.<domain>); the in-browser BroadcastChannel does not cross that
// boundary, so we rely on this SSE event reaching every authenticated tab.
//
// Asserts the photo toggle emits a tenant_settings_changed event whose
// Source labels which key changed, on both enable and disable PUT paths.
func TestOnValueSetCallback_TenantSettingsChangedBroadcasts(t *testing.T) {
	t.Parallel()

	route := setupSettingsCallbackRoute(t)

	client := &realtime.Client{
		Channel:          make(chan realtime.Event, 8),
		UserID:           1,
		SubscribedGroups: map[string]bool{},
	}
	route.hub.Register(client, testpkg.Tenant(t), nil)
	defer route.hub.Unregister(client)

	router := route.router

	// awaitTenantSettings drains events on the buffered channel until it
	// finds a tenant_settings_changed for the expected key, or fails the
	// test after a short window. The 500ms cap matches the existing photo-
	// purge test; the post-commit closure runs synchronously after tx commit
	// so the event is on the channel before this returns under any
	// reasonable scheduler.
	awaitTenantSettings := func(t *testing.T, expectedKey string) {
		t.Helper()
		deadline := time.After(500 * time.Millisecond)
		for {
			select {
			case ev := <-client.Channel:
				if ev.Type != realtime.EventTenantSettingsChanged {
					continue
				}
				require.NotNil(t, ev.Data.Source,
					"tenant_settings_changed must carry a Source label so log review can identify the key")
				assert.Equal(t, expectedKey, *ev.Data.Source,
					"Source must be the setting key the writer changed")
				return
			case <-deadline:
				t.Fatalf("expected tenant_settings_changed event for %q within 500ms", expectedKey)
			}
		}
	}

	// 1. Enable photo feature → tenant_settings_changed with the photo key.
	enableReq := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled",
		map[string]any{"value": true},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)
	require.Equal(t, http.StatusOK, testutil.ExecuteRequest(router, enableReq).Code)
	awaitTenantSettings(t, settings.KeyStudentPhotosEnabled)
	drainEvents(client.Channel)

	// 2. Disable photo feature → tenant_settings_changed (alongside the
	//    student_updated post-purge broadcast asserted in the sibling test).
	disableReq := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled",
		map[string]any{"value": false},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)
	require.Equal(t, http.StatusOK, testutil.ExecuteRequest(router, disableReq).Code)
	awaitTenantSettings(t, settings.KeyStudentPhotosEnabled)
	drainEvents(client.Channel)

}

// The People Directory adapter is wired through the shared protected group:
// the real JWT verification, permission check and tenant transaction sit in
// front of every /api/users route (#2661).
func TestRegisterRoutes_UsersRoutesRunThroughProtectedGroup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	people, err := owners.NewPeopleDirectory(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	identity, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	resource := newUsersResource(people, identity.ListAccountEmails, func(ctx context.Context, tagID string) (bool, error) {
		_, _, found, err := identity.LookupRFIDCard(ctx, tagID)
		return found, err
	})
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/api/users", resource.Router())
	person := testpkg.CreateTestPerson(t, db, "Wired", "Route")

	anonymous := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	anonymousRecorder := httptest.NewRecorder()
	router.ServeHTTP(anonymousRecorder, anonymous)
	assert.Equal(t, http.StatusUnauthorized, anonymousRecorder.Code)

	forbiddenClaims := testutil.DefaultTestClaims()
	forbiddenClaims.Roles = []string{"user"}
	forbiddenClaims.IsAdmin = false
	forbiddenClaims.Permissions = nil
	forbidden := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/users", nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, forbiddenClaims)))
	forbiddenRecorder := httptest.NewRecorder()
	router.ServeHTTP(forbiddenRecorder, forbidden)
	assert.Equal(t, http.StatusForbidden, forbiddenRecorder.Code, forbiddenRecorder.Body.String())

	allowedClaims := testutil.DefaultTestClaims()
	allowedClaims.Roles = []string{"user"}
	allowedClaims.IsAdmin = false
	allowedClaims.Permissions = []string{"users:read"}
	allowed := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/api/users/"+strconv.FormatInt(person.ID, 10), nil,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, allowedClaims)))
	allowedRecorder := httptest.NewRecorder()
	router.ServeHTTP(allowedRecorder, allowed)
	require.Equal(t, http.StatusOK, allowedRecorder.Code, allowedRecorder.Body.String())
	assert.Contains(t, allowedRecorder.Body.String(), `"first_name":"Wired"`)
}

type settingsCallbackRoute struct {
	router chi.Router
	hub    *realtime.Hub
}

// checkTimetableConflictWiring pins that the timetable routes reach the
// Timetable owner's conflict detection and staffing capability (#3550): the
// conflict probes, the staff pool and the shift-coverage probe answer 500
// when it is unwired, and the calendar list silently drops its warnings.
func checkTimetableConflictWiring(t *testing.T, graph *goldenRuntime) {
	t.Parallel()

	_, _, _, _, _, _, _, _, settings, _, services, schedules, requests, _, _, lifecycle, planning, materialization, _, _, _, _, _, _, _, _, _, err := composeTestRoot(graph.db, slog.Default(), currentTestServiceConfig())
	require.NoError(t, err)
	timetable := newTimetableResource(nil, nil, materialization, lifecycle, planning, nil, services.Users, schedules, nil, settings.Service,
		requests.CareOfferings, ClassDayViews{}, nil, nil, nil, slog.Default())
	require.NotNil(t, timetable)
	require.NotNil(t, timetable.ConflictDetection, "api/timetable must hold the owner's conflict detection")
	assert.Same(t, planning.Planning.ConflictDetection, timetable.ConflictDetection,
		"the routes and the instance lifecycle share one composed capability")

	// The mounted routes reach it too: an unwired conflict probe answers 500.
	db := testpkg.SetupTestDB(t)
	_, staff := testpkg.CreateTestTeacherWithAccount(t, db, "Conflict", "Probe")
	token := testpkg.MintLiveSessionJWT(t, db, testutil.AdminTestClaimsForTenant(int(staff.ID), testpkg.Tenant(t)))
	probe := checkpointRequest(graph, checkpointScenario{
		Method: http.MethodGet, Path: "/api/timetable/exception-conflicts?date=2099-01-05", Authenticated: true,
	}, token)
	require.Equal(t, http.StatusOK, probe.Code, probe.Body.String())
}

// The auth limiters' burst follows the per-minute budget (minimum 10), so a
// school whose staff share one public IP can sign in together.
func TestBuildAuthRateLimitersBurstFollowsLimit(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		limit string
		burst int
	}{{"5", 10}, {"30", 30}} {
		limiters := buildAuthRateLimiters(nil, tc.limit, false)
		handler := limiters.auth.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		codes := make([]int, 0, tc.burst+1)
		for range tc.burst + 1 {
			req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
			req.RemoteAddr = "203.0.113.7:54321"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			codes = append(codes, rr.Code)
		}
		assert.Equal(t, http.StatusNoContent, codes[tc.burst-1], "limit %s: request %d is within the burst", tc.limit, tc.burst)
		assert.Equal(t, http.StatusTooManyRequests, codes[tc.burst], "limit %s: request %d exceeds the burst", tc.limit, tc.burst+1)
	}
}

// The demo exempts its demo process (a loopback peer) from the auth limiters;
// every other environment keeps limiting loopback like any other address.
func TestBuildAuthRateLimitersExemptLoopbackOnlyWhenAsked(t *testing.T) {
	t.Parallel()

	for _, exempt := range []bool{true, false} {
		limiters := buildAuthRateLimiters(nil, "1", exempt)
		for name, limiter := range map[string]*customMiddleware.RateLimiter{
			"auth": limiters.auth, "email confirm": limiters.emailConfirm, "invitation": limiters.invitation,
		} {
			handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			last := 0
			for range 11 { // one more than the burst of 10
				req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
				req.RemoteAddr = "127.0.0.1:54321"
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				last = rr.Code
			}
			want := http.StatusTooManyRequests
			if exempt {
				want = http.StatusNoContent
			}
			assert.Equal(t, want, last, "%s limiter, exempt=%v", name, exempt)
		}
	}
}
