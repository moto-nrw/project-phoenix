package api

import (
	"log/slog"
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testServiceJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"

func testServiceConfig() owners.ServiceConfig {
	return owners.ServiceConfig{
		JWTSecret:        testServiceJWTSecret,
		JWTExpiry:        15 * time.Minute,
		JWTRefreshExpiry: 24 * time.Hour,
		FrontendURL:      "http://localhost:3000",
		PublicAPIURL:     "http://localhost:8080",
		ParentsURL:       "http://parents.localhost:3000",
		SchoolURL:        "http://schule.localhost:3000",
		CatererURL:       "http://caterer.localhost:3000",
		TenantDomain:     "localhost",
		OperatorHostname: "operator.localhost:3000",
	}
}

func TestServiceGraphComposesItsServices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	env, mailSender, _, mailFrom, _, hub, identity, caller, settings, _, services, _, _, active, reports, _, planning, _, _, activities, structureViews, facilitiesRooms, deviceFleet, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), testServiceConfig())
	require.NoError(t, err)
	require.NotNil(t, identity)

	// Verify all services are initialized
	t.Run("core services", func(t *testing.T) {
		assert.NotNil(t, identity)
		assert.NotNil(t, active)
		assert.NotNil(t, reports.ActiveCleanup)
		assert.NotNil(t, activities)
		assert.NotNil(t, structureViews.Education)
		assert.NotNil(t, facilitiesRooms.RoomService)
		assert.NotNil(t, deviceFleet)
		assert.NotNil(t, settings.Service)
		assert.NotNil(t, planning.Planning.RecurrenceLock)
		assert.NotNil(t, services.Users)
		assert.NotNil(t, services.Guardian)
		assert.NotNil(t, caller)
		assert.NotNil(t, identity)
	})

	t.Run("realtime hub", func(t *testing.T) {
		assert.NotNil(t, hub)
	})

	t.Run("email configuration", func(t *testing.T) {
		assert.NotNil(t, mailSender)
		assert.NotNil(t, mailFrom)
	})

	t.Run("configured values", func(t *testing.T) {
		assert.Equal(t, "http://localhost:3000", env.Links.FrontendURL)

		// Default expiry values (when not configured)
		assert.Equal(t, 48*time.Hour, env.Links.InvitationTokenExpiry)
		assert.Equal(t, 30*time.Minute, env.Links.PasswordResetTokenExpiry)
	})
}

func TestServiceGraph_EnrollmentDeletionKeepsGuardianDirectoryAfterAuditSetup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	_, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, notices, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), testServiceConfig())
	require.NoError(t, err)

	_, err = testpkg.TenantQuery1(t, testpkg.Ctx(t), db, notices.Deletions.PreviewRequest, testpkg.UniqueTestTenantID(t))
	require.EqualError(t, err, "enrollment deletion target not found")
}

func TestServiceGraph_RejectsPartialVAPIDConfig(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.VAPIDPublicKey = "configured-without-the-other-required-values"

	_, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.Error(t, err)
	require.Nil(t, identity)
	assert.ErrorContains(t, err, "invalid VAPID configuration")
	assert.ErrorContains(t, err, "VAPID_PRIVATE_KEY")
	assert.ErrorContains(t, err, "VAPID_SUBSCRIBER")
}

func TestServiceGraph_InvitationTokenExpiry_ZeroDefaults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, 48*time.Hour, env.Links.InvitationTokenExpiry)
}

func TestServiceGraph_InvitationTokenExpiry_ClampedToMax(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.InvitationTokenExpiryHours = 500

	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, 168*time.Hour, env.Links.InvitationTokenExpiry)
}

func TestServiceGraph_InvitationTokenExpiry_ValidValue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.InvitationTokenExpiryHours = 72

	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, 72*time.Hour, env.Links.InvitationTokenExpiry)
}

func TestServiceGraph_PasswordResetExpiry_ZeroDefaults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, 30*time.Minute, env.Links.PasswordResetTokenExpiry)
}

func TestServiceGraph_PasswordResetExpiry_ClampedToMax(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.PasswordResetExpiryMinutes = 2000

	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, 1440*time.Minute, env.Links.PasswordResetTokenExpiry)
}

func TestServiceGraph_PasswordResetExpiry_ValidValue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.PasswordResetExpiryMinutes = 60

	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, 60*time.Minute, env.Links.PasswordResetTokenExpiry)
}

func TestServiceGraph_FrontendURL_TrailingSlashRemoved(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.FrontendURL = "http://example.com/"

	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, "http://example.com", env.Links.FrontendURL)
}

func TestServiceGraph_FrontendURL_Required(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.FrontendURL = ""

	_, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.Error(t, err)
	require.Nil(t, identity)
	assert.Contains(t, err.Error(), "FRONTEND_URL")
}

func TestServiceGraph_PublicAPIURL_Required(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.PublicAPIURL = ""

	_, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.Error(t, err)
	require.Nil(t, identity)
	assert.ErrorContains(t, err, "NEXT_PUBLIC_API_URL")
}

func TestServiceGraph_PortalURLs_Required(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	for _, tc := range []struct {
		name, want string
		clear      func(*owners.ServiceConfig)
	}{
		{"parents", "PARENTS_URL", func(cfg *owners.ServiceConfig) { cfg.ParentsURL = "" }},
		{"school", "SCHOOL_URL", func(cfg *owners.ServiceConfig) { cfg.SchoolURL = "" }},
		{"caterer", "CATERER_URL", func(cfg *owners.ServiceConfig) { cfg.CatererURL = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testServiceConfig()
			tc.clear(&cfg)

			_, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
			require.Error(t, err)
			require.Nil(t, identity)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestServiceGraph_DefaultEmailFrom_WhenNotConfigured(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	_, _, _, mailFrom, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), testServiceConfig())
	require.NoError(t, err)
	require.NotNil(t, identity)

	// Default values when not configured
	assert.Equal(t, "moto", mailFrom.Name)
	assert.Equal(t, "no-reply@moto.local", mailFrom.Address)
}

func TestServiceGraph_EmailFrom_WhenConfigured(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.EmailFromName = "Test App"
	cfg.EmailFromAddress = "test@example.com"

	_, _, _, mailFrom, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, "Test App", mailFrom.Name)
	assert.Equal(t, "test@example.com", mailFrom.Address)
}

func TestServiceGraph_NegativeInvitationExpiry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.InvitationTokenExpiryHours = -10

	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, 48*time.Hour, env.Links.InvitationTokenExpiry)
}

func TestServiceGraph_NegativePasswordResetExpiry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.PasswordResetExpiryMinutes = -10

	env, _, _, _, _, _, identity, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _, err := composeTestRoot(testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, identity)

	assert.Equal(t, 30*time.Minute, env.Links.PasswordResetTokenExpiry)
}
