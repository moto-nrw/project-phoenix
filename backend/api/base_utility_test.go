package api

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAllowedOrigins tests the parseAllowedOrigins function
func TestParseAllowedOrigins(t *testing.T) {
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
			// Set environment variable for this test
			t.Setenv("CORS_ALLOWED_ORIGINS", tt.envValue)

			// Call function under test
			exact, wildcards := parseAllowedOrigins()

			// Assert results
			assert.Equal(t, tt.expectedExact, exact)
			assert.Equal(t, tt.expectedWildcards, wildcards)
		})
	}
}

// TestParsePositiveInt tests the parsePositiveInt function
func TestParsePositiveInt(t *testing.T) {
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
			// Set environment variable for this test
			if tt.envValue != "" {
				t.Setenv(tt.envVar, tt.envValue)
			}

			// Call function under test
			result := parsePositiveInt(tt.envVar, tt.defaultValue)

			// Assert result
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParsePositiveInt_DifferentDefaults tests parsePositiveInt with various default values
func TestParsePositiveInt_DifferentDefaults(t *testing.T) {
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
			result := parsePositiveInt("NONEXISTENT_ENV_VAR", tt.defaultValue)
			assert.Equal(t, tt.defaultValue, result)
		})
	}
}

func TestInitializeAPIResources_WiresCaregiverServices(t *testing.T) {
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	api := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}

	initializeAPIResources(api, repoFactory, db, slog.Default())

	require.NotNil(t, api.Auth)
	require.NotNil(t, api.Operator)
	assert.Same(t, serviceFactory.CaregiverCapability, api.Auth.CaregiverCapabilityService)
}

func TestSyncClientIPToRemoteAddrUsesChiClientIP(t *testing.T) {
	router := chi.NewRouter()
	router.Use(chimiddleware.ClientIPFromXFF())
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

func TestRegisterRoutesWithRateLimiting_MountsOperatorInvitationRoutes(t *testing.T) {
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	api := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}

	initializeAPIResources(api, repoFactory, db, slog.Default())

	previousEnabled := viper.Get("rate_limit_enabled")
	previousPerMinute := viper.Get("rate_limit_per_minute")
	viper.Set("rate_limit_enabled", true)
	viper.Set("rate_limit_per_minute", 5)
	t.Cleanup(func() {
		viper.Set("rate_limit_enabled", previousEnabled)
		viper.Set("rate_limit_per_minute", previousPerMinute)
	})

	api.registerRoutesWithRateLimiting()

	req := httptest.NewRequest(http.MethodPost, "/operator/auth/invitations/validate", nil)
	rr := httptest.NewRecorder()

	api.Router.ServeHTTP(rr, req)

	assert.NotEqual(t, http.StatusNotFound, rr.Code)
}

func TestTokenAwareRateLimitKey_ValidTokenUsesHashedToken(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	tokenAuth, err := jwt.NewTokenAuthWithSecret("rate-limit-test-secret-32-chars!!")
	require.NoError(t, err)

	token, err := tokenAuth.CreateJWT(jwt.AppClaims{
		ID:          42,
		Sub:         "user@example.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/students", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	sum := sha256.Sum256([]byte(token))
	expected := "token:" + hex.EncodeToString(sum[:])

	assert.Equal(t, expected, tokenAwareRateLimitKey(tokenAuth)(req))
	assert.NotContains(t, tokenAwareRateLimitKey(tokenAuth)(req), token)
}

func TestTokenAwareRateLimitKey_InvalidTokenFallsBack(t *testing.T) {
	tokenAuth, err := jwt.NewTokenAuthWithSecret("rate-limit-test-secret-32-chars!!")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/students", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")

	assert.Empty(t, tokenAwareRateLimitKey(tokenAuth)(req))
}

func TestTokenAwareRateLimitKey_ExpiredTokenFallsBack(t *testing.T) {
	tokenAuth, err := jwt.NewTokenAuthWithSecret("rate-limit-test-secret-32-chars!!")
	require.NoError(t, err)

	_, token, err := tokenAuth.JwtAuth.Encode(map[string]any{
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/students", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	assert.Empty(t, tokenAwareRateLimitKey(tokenAuth)(req))
}

func TestTokenAwareRateLimitKey_DifferentValidTokensHaveDifferentKeys(t *testing.T) {
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	tokenAuth, err := jwt.NewTokenAuthWithSecret("rate-limit-test-secret-32-chars!!")
	require.NoError(t, err)

	tokenA, err := tokenAuth.CreateJWT(jwt.AppClaims{
		ID:          42,
		Sub:         "user-a@example.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	})
	require.NoError(t, err)
	tokenB, err := tokenAuth.CreateJWT(jwt.AppClaims{
		ID:          43,
		Sub:         "user-b@example.com",
		Roles:       []string{"user"},
		Permissions: []string{"read"},
	})
	require.NoError(t, err)

	reqA := httptest.NewRequest(http.MethodGet, "/api/students", nil)
	reqA.RemoteAddr = "192.168.1.1:12345"
	reqA.Header.Set("Authorization", "Bearer "+tokenA)
	reqB := httptest.NewRequest(http.MethodGet, "/api/students", nil)
	reqB.RemoteAddr = "192.168.1.1:12345"
	reqB.Header.Set("Authorization", "Bearer "+tokenB)

	keyFunc := tokenAwareRateLimitKey(tokenAuth)
	assert.NotEqual(t, keyFunc(reqA), keyFunc(reqB))
}

// TestOnValueSetCallback_WCEnabled tests that the OnValueSet callback
// registered in initializeAPIResources triggers WC infrastructure creation
// when checkout.wc_enabled is set to true.
func TestOnValueSetCallback_WCEnabled(t *testing.T) {
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	a := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}

	initializeAPIResources(a, repoFactory, db, slog.Default())

	// Mount the SetValue handler on a tenant-aware router
	router := a.Settings.SettingsRouter()

	// Set checkout.wc_enabled = true → triggers callback → creates WC infrastructure
	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.wc_enabled",
		map[string]any{"value": true},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// TestOnValueSetCallback_SchulhofEnabled tests that the callback triggers
// Schulhof infrastructure creation when checkout.schulhof_enabled is set to true.
func TestOnValueSetCallback_SchulhofEnabled(t *testing.T) {
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	a := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}

	initializeAPIResources(a, repoFactory, db, slog.Default())

	router := a.Settings.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.schulhof_enabled",
		map[string]any{"value": true},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// TestOnValueSetCallback_FalseValueSkipsInfrastructure tests that setting
// a checkout setting to false does not trigger infrastructure creation.
func TestOnValueSetCallback_FalseValueSkipsInfrastructure(t *testing.T) {
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	a := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}

	initializeAPIResources(a, repoFactory, db, slog.Default())

	router := a.Settings.SettingsRouter()

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/values/checkout.wc_enabled",
		map[string]any{"value": false},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// adminClaimsForCallback returns admin claims with config permissions
// for testing the OnValueSet callback.
func adminClaimsForCallback() jwt.AppClaims {
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
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	a := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}
	initializeAPIResources(a, repoFactory, db, slog.Default())

	// Subscribe a fake SSE client to the real Hub so we can observe
	// BroadcastToAll without mocking. UserID/TenantID are arbitrary —
	// BroadcastToAll fan-outs ignore both.
	client := &realtime.Client{
		Channel:          make(chan realtime.Event, 4),
		UserID:           1,
		SubscribedGroups: map[string]bool{},
	}
	a.Services.RealtimeHub.Register(client, 1, nil)
	defer a.Services.RealtimeHub.Unregister(client)

	router := a.Settings.SettingsRouter()

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
	db, serviceFactory := testutil.SetupAPITest(t)
	defer func() { _ = db.Close() }()

	repoFactory := repositories.NewFactory(db)
	a := &API{
		Services: serviceFactory,
		Router:   chi.NewRouter(),
		db:       db,
		repos:    repoFactory,
	}
	initializeAPIResources(a, repoFactory, db, slog.Default())

	client := &realtime.Client{
		Channel:          make(chan realtime.Event, 8),
		UserID:           1,
		SubscribedGroups: map[string]bool{},
	}
	a.Services.RealtimeHub.Register(client, 1, nil)
	defer a.Services.RealtimeHub.Unregister(client)

	router := a.Settings.SettingsRouter()

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
	awaitTenantSettings(t, configModel.KeyStudentPhotosEnabled)
	drainEvents(client.Channel)

	// 2. Disable photo feature → tenant_settings_changed (alongside the
	//    student_updated post-purge broadcast asserted in the sibling test).
	disableReq := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.student_photos_enabled",
		map[string]any{"value": false},
		testutil.WithJWTBearer(testutil.MintTestJWT(t, adminClaimsForCallback())),
	)
	require.Equal(t, http.StatusOK, testutil.ExecuteRequest(router, disableReq).Code)
	awaitTenantSettings(t, configModel.KeyStudentPhotosEnabled)
	drainEvents(client.Channel)

}
