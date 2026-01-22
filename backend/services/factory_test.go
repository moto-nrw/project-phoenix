package services_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFactory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)
	require.NotNil(t, repos)

	// Clear viper for clean test
	viper.Reset()

	factory, err := services.NewFactory(repos, db)
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
		assert.NotNil(t, factory.Feedback)
		assert.NotNil(t, factory.IoT)
		assert.NotNil(t, factory.Config)
		assert.NotNil(t, factory.Schedule)
		assert.NotNil(t, factory.Users)
		assert.NotNil(t, factory.Guardian)
		assert.NotNil(t, factory.UserContext)
		assert.NotNil(t, factory.Database)
		assert.NotNil(t, factory.Import)
	})

	t.Run("realtime hub", func(t *testing.T) {
		assert.NotNil(t, factory.RealtimeHub)
	})

	t.Run("email configuration", func(t *testing.T) {
		assert.NotNil(t, factory.Mailer)
		assert.NotNil(t, factory.DefaultFrom)
	})

	t.Run("default values", func(t *testing.T) {
		// Default frontend URL
		assert.Equal(t, "http://localhost:3000", factory.FrontendURL)

		// Default expiry values (when not configured)
		assert.Equal(t, 30*time.Minute, factory.PasswordResetTokenExpiry)
	})
}

func TestNewFactory_PasswordResetExpiry_ZeroDefaults(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set password reset expiry to zero (should default to 30m)
	viper.Reset()
	viper.Set("password_reset_token_expiry_minutes", 0)

	factory, err := services.NewFactory(repos, db)
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
	viper.Set("password_reset_token_expiry_minutes", 2000)

	factory, err := services.NewFactory(repos, db)
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
	viper.Set("password_reset_token_expiry_minutes", 60)

	factory, err := services.NewFactory(repos, db)
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
	viper.Set("frontend_url", "http://example.com/")

	factory, err := services.NewFactory(repos, db)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, "http://example.com", factory.FrontendURL)
}

func TestNewFactory_FrontendURL_DefaultWhenEmpty(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Clear frontend URL
	viper.Reset()
	viper.Set("frontend_url", "")

	factory, err := services.NewFactory(repos, db)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, "http://localhost:3000", factory.FrontendURL)
}

func TestNewFactory_DefaultEmailFrom_WhenNotConfigured(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Clear email config
	viper.Reset()

	factory, err := services.NewFactory(repos, db)
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
	viper.Set("email_from_name", "Test App")
	viper.Set("email_from_address", "test@example.com")

	factory, err := services.NewFactory(repos, db)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, "Test App", factory.DefaultFrom.Name)
	assert.Equal(t, "test@example.com", factory.DefaultFrom.Address)
}

func TestNewFactory_NegativePasswordResetExpiry(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set negative value (should default to 30m)
	viper.Reset()
	viper.Set("password_reset_token_expiry_minutes", -10)

	factory, err := services.NewFactory(repos, db)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 30*time.Minute, factory.PasswordResetTokenExpiry)
}
