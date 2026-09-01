package services_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFactoryJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"

func testFactoryConfig() services.FactoryConfig {
	return services.FactoryConfig{
		JWTSecret:        testFactoryJWTSecret,
		JWTExpiry:        15 * time.Minute,
		JWTRefreshExpiry: 24 * time.Hour,
		FrontendURL:      "http://localhost:3000",
		ParentsURL:       "http://parents.localhost:3000",
		SchoolURL:        "http://schule.localhost:3000",
		TenantDomain:     "localhost",
		OperatorHostname: "operator.localhost:3000",
	}
}

func TestNewFactory(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	require.NotNil(t, repos)

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), testFactoryConfig())
	require.NoError(t, err)
	require.NotNil(t, factory)

	// Verify all services are initialized
	t.Run("core services", func(t *testing.T) {
		assert.NotNil(t, factory.Auth)
		assert.NotNil(t, factory.Active)
		assert.NotNil(t, factory.ActiveCleanup)
		assert.NotNil(t, factory.Activities)
		assert.NotNil(t, factory.Education)
		assert.NotNil(t, factory.Facilities)
		assert.NotNil(t, factory.IoT)
		assert.NotNil(t, factory.Settings)
		assert.NotNil(t, factory.Schedule)
		assert.NotNil(t, factory.Users)
		assert.NotNil(t, factory.Guardian)
		assert.NotNil(t, factory.UserContext)
		assert.NotNil(t, factory.Database)
		assert.NotNil(t, factory.Import)
		assert.NotNil(t, factory.Invitation)
	})

	t.Run("realtime hub", func(t *testing.T) {
		assert.NotNil(t, factory.RealtimeHub)
	})

	t.Run("email configuration", func(t *testing.T) {
		assert.NotNil(t, factory.Mailer)
		assert.NotNil(t, factory.DefaultFrom)
	})

	t.Run("configured values", func(t *testing.T) {
		assert.Equal(t, "http://localhost:3000", factory.FrontendURL)

		// Default expiry values (when not configured)
		assert.Equal(t, 48*time.Hour, factory.InvitationTokenExpiry)
		assert.Equal(t, 30*time.Minute, factory.PasswordResetTokenExpiry)
	})
}

func TestNewFactory_RejectsPartialVAPIDConfig(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	cfg := testFactoryConfig()
	cfg.VAPIDPublicKey = "configured-without-the-other-required-values"

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.Error(t, err)
	require.Nil(t, factory)
	assert.ErrorContains(t, err, "invalid VAPID configuration")
	assert.ErrorContains(t, err, "VAPID_PRIVATE_KEY")
	assert.ErrorContains(t, err, "VAPID_SUBSCRIBER")
}

func TestNewFactory_InvitationTokenExpiry_ZeroDefaults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 48*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_InvitationTokenExpiry_ClampedToMax(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.InvitationTokenExpiryHours = 500

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 168*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_InvitationTokenExpiry_ValidValue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.InvitationTokenExpiryHours = 72

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 72*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_PasswordResetExpiry_ZeroDefaults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 30*time.Minute, factory.PasswordResetTokenExpiry)
}

func TestNewFactory_PasswordResetExpiry_ClampedToMax(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.PasswordResetExpiryMinutes = 2000

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 1440*time.Minute, factory.PasswordResetTokenExpiry)
}

func TestNewFactory_PasswordResetExpiry_ValidValue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.PasswordResetExpiryMinutes = 60

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 60*time.Minute, factory.PasswordResetTokenExpiry)
}

func TestNewFactory_FrontendURL_TrailingSlashRemoved(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.FrontendURL = "http://example.com/"

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, "http://example.com", factory.FrontendURL)
}

func TestNewFactory_FrontendURL_Required(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.FrontendURL = ""

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.Error(t, err)
	require.Nil(t, factory)
	assert.Contains(t, err.Error(), "FRONTEND_URL")
}

func TestNewFactory_PortalURLs_Required(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	for _, tc := range []struct {
		name, want string
		clear      func(*services.FactoryConfig)
	}{
		{"parents", "PARENTS_URL", func(cfg *services.FactoryConfig) { cfg.ParentsURL = "" }},
		{"school", "SCHOOL_URL", func(cfg *services.FactoryConfig) { cfg.SchoolURL = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testFactoryConfig()
			tc.clear(&cfg)

			factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
			require.Error(t, err)
			require.Nil(t, factory)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestNewFactory_DefaultEmailFrom_WhenNotConfigured(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), testFactoryConfig())
	require.NoError(t, err)
	require.NotNil(t, factory)

	// Default values when not configured
	assert.Equal(t, "moto", factory.DefaultFrom.Name)
	assert.Equal(t, "no-reply@moto.local", factory.DefaultFrom.Address)
}

func TestNewFactory_EmailFrom_WhenConfigured(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.EmailFromName = "Test App"
	cfg.EmailFromAddress = "test@example.com"

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, "Test App", factory.DefaultFrom.Name)
	assert.Equal(t, "test@example.com", factory.DefaultFrom.Address)
}

func TestNewFactory_NegativeInvitationExpiry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.InvitationTokenExpiryHours = -10

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 48*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_NegativePasswordResetExpiry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)

	cfg := testFactoryConfig()
	cfg.PasswordResetExpiryMinutes = -10

	factory, err := services.NewFactoryForTestsWithConfig(repos, db, slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 30*time.Minute, factory.PasswordResetTokenExpiry)
}

func TestEnableStudentPhotos(t *testing.T) {
	t.Parallel()

	f := &services.Factory{SettingsSideEffects: sideeffects.NewRegistry()}
	f.EnableStudentPhotos(services.StudentPhotoBootstrap{Logger: slog.Default()})
	require.NotNil(t, f.StudentPhotos)
	post, err := f.SettingsSideEffects.Dispatch(context.Background(), 1, "operations.student_photos_enabled", "x")
	require.NoError(t, err)
	require.NotNil(t, post)
}
