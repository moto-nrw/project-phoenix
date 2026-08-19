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
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testFactoryJWTSecret is seeded into viper after every viper.Reset() so the
// MFA service init inside services.NewFactory has a stable HMAC key. The
// factory hard-fails without it (MFAServiceConfig.JWTSecret is required) —
// production / dev always provide AUTH_JWT_SECRET, but unit tests reset
// viper to verify default behaviour and would otherwise lose the secret.
const testFactoryJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"

func TestNewFactory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)
	require.NotNil(t, repos)

	// Clear viper for clean test
	viper.Reset()
	seedFactoryRequiredConfig()

	factory, err := services.NewFactory(repos, db, slog.Default())
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
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("vapid_public_key", "configured-without-the-other-required-values")

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.Error(t, err)
	require.Nil(t, factory)
	assert.ErrorContains(t, err, "invalid VAPID configuration")
	assert.ErrorContains(t, err, "VAPID_PRIVATE_KEY")
	assert.ErrorContains(t, err, "VAPID_SUBSCRIBER")
}

func TestNewFactory_InvitationTokenExpiry_ZeroDefaults(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set invitation expiry to zero (should default to 48h)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("invitation_token_expiry_hours", 0)

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 48*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_InvitationTokenExpiry_ClampedToMax(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set invitation expiry to > 168 hours (should clamp to 168h)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("invitation_token_expiry_hours", 500)

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 168*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_InvitationTokenExpiry_ValidValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set invitation expiry to valid value (72 hours)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("invitation_token_expiry_hours", 72)

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 72*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_PasswordResetExpiry_ZeroDefaults(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set password reset expiry to zero (should default to 30m)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("password_reset_token_expiry_minutes", 0)

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 30*time.Minute, factory.PasswordResetTokenExpiry)
}

func TestNewFactory_PasswordResetExpiry_ClampedToMax(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set password reset expiry to > 1440 minutes (should clamp to 1440m)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("password_reset_token_expiry_minutes", 2000)

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 1440*time.Minute, factory.PasswordResetTokenExpiry)
}

func TestNewFactory_PasswordResetExpiry_ValidValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set password reset expiry to valid value (60 minutes)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("password_reset_token_expiry_minutes", 60)

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 60*time.Minute, factory.PasswordResetTokenExpiry)
}

func TestNewFactory_FrontendURL_TrailingSlashRemoved(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set frontend URL with trailing slash
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("frontend_url", "http://example.com/")

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, "http://example.com", factory.FrontendURL)
}

func TestNewFactory_FrontendURL_Required(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Clear frontend URL
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("frontend_url", "")

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.Error(t, err)
	require.Nil(t, factory)
	assert.Contains(t, err.Error(), "FRONTEND_URL")
}

func TestNewFactory_ParentsURL_Required(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("parents_url", "")

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.Error(t, err)
	require.Nil(t, factory)
	assert.Contains(t, err.Error(), "PARENTS_URL")
}

func TestNewFactory_DefaultEmailFrom_WhenNotConfigured(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Clear email config
	viper.Reset()
	seedFactoryRequiredConfig()

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	// Default values when not configured
	assert.Equal(t, "moto", factory.DefaultFrom.Name)
	assert.Equal(t, "no-reply@moto.local", factory.DefaultFrom.Address)
}

func TestNewFactory_EmailFrom_WhenConfigured(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set email config
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("email_from_name", "Test App")
	viper.Set("email_from_address", "test@example.com")

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, "Test App", factory.DefaultFrom.Name)
	assert.Equal(t, "test@example.com", factory.DefaultFrom.Address)
}

func TestNewFactory_NegativeInvitationExpiry(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set negative value (should default to 48h)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("invitation_token_expiry_hours", -10)

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 48*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_NegativePasswordResetExpiry(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set negative value (should default to 30m)
	viper.Reset()
	seedFactoryRequiredConfig()
	viper.Set("password_reset_token_expiry_minutes", -10)

	factory, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 30*time.Minute, factory.PasswordResetTokenExpiry)
}

func seedFactoryRequiredConfig() {
	viper.Set("auth_jwt_secret", testFactoryJWTSecret)
	viper.Set("frontend_url", "http://localhost:3000")
	viper.Set("parents_url", "http://parents.localhost:3000")
	viper.Set("tenant_domain", "localhost")
	viper.Set("next_public_operator_hostname", "operator.localhost:3000")
}

func TestEnableStudentPhotos(t *testing.T) {
	f := &services.Factory{SettingsSideEffects: sideeffects.NewRegistry()}
	f.EnableStudentPhotos(services.StudentPhotoBootstrap{Logger: slog.Default()})
	require.NotNil(t, f.StudentPhotos)
	post, err := f.SettingsSideEffects.Dispatch(context.Background(), 1, "operations.student_photos_enabled", "x")
	require.NoError(t, err)
	require.NotNil(t, post)
}
