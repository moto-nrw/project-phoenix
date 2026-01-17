package services_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRequiredFactoryConfig(t *testing.T) {
	t.Helper()
	viper.Set("app_env", "test")
	viper.Set("email_from_name", "Test App")
	viper.Set("email_from_address", "test@example.com")
	viper.Set("frontend_url", "http://localhost:3000")
	viper.Set("invitation_token_expiry_hours", 48)
	viper.Set("password_reset_token_expiry_minutes", 30)
	viper.Set("auth_jwt_secret", "test-secret-32-chars-minimum!!!!")
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)
}

func TestNewFactory(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)
	require.NotNil(t, repos)

	viper.Reset()
	setRequiredFactoryConfig(t)

	factory, err := services.NewFactory(repos, db, nil, nil)
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
		assert.NotNil(t, factory.Invitation)
	})

	t.Run("email configuration", func(t *testing.T) {
		assert.NotNil(t, factory.Mailer)
		assert.NotNil(t, factory.DefaultFrom)
	})

	t.Run("configured values", func(t *testing.T) {
		assert.Equal(t, "http://localhost:3000", factory.FrontendURL)
		assert.Equal(t, 48*time.Hour, factory.InvitationTokenExpiry)
		assert.Equal(t, 30*time.Minute, factory.PasswordResetTokenExpiry)
	})
}

func TestNewFactory_InvitationTokenExpiry_ZeroDefaults(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("invitation_token_expiry_hours", 0)

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.Error(t, err)
	assert.Nil(t, factory)
}

func TestNewFactory_InvitationTokenExpiry_ClampedToMax(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("invitation_token_expiry_hours", 500)

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.Error(t, err)
	assert.Nil(t, factory)
}

func TestNewFactory_InvitationTokenExpiry_ValidValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set invitation expiry to valid value (72 hours)
	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("invitation_token_expiry_hours", 72)

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, 72*time.Hour, factory.InvitationTokenExpiry)
}

func TestNewFactory_PasswordResetExpiry_ZeroDefaults(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("password_reset_token_expiry_minutes", 0)

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.Error(t, err)
	assert.Nil(t, factory)
}

func TestNewFactory_PasswordResetExpiry_ClampedToMax(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("password_reset_token_expiry_minutes", 2000)

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.Error(t, err)
	assert.Nil(t, factory)
}

func TestNewFactory_PasswordResetExpiry_ValidValue(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set password reset expiry to valid value (60 minutes)
	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("password_reset_token_expiry_minutes", 60)

	factory, err := services.NewFactory(repos, db, nil, nil)
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
	setRequiredFactoryConfig(t)
	viper.Set("frontend_url", "http://example.com/")

	factory, err := services.NewFactory(repos, db, nil, nil)
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
	setRequiredFactoryConfig(t)
	viper.Set("frontend_url", "")

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.Error(t, err)
	assert.Nil(t, factory)
}

func TestNewFactory_DefaultEmailFrom_WhenNotConfigured(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Clear email config
	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("email_from_name", "")
	viper.Set("email_from_address", "")

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.Error(t, err)
	assert.Nil(t, factory)
}

func TestNewFactory_EmailFrom_WhenConfigured(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	// Set email config
	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("email_from_name", "Test App")
	viper.Set("email_from_address", "test@example.com")

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, factory)

	assert.Equal(t, "Test App", factory.DefaultFrom.Name)
	assert.Equal(t, "test@example.com", factory.DefaultFrom.Address)
}

func TestNewFactory_NegativeInvitationExpiry(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("invitation_token_expiry_hours", -10)

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.Error(t, err)
	assert.Nil(t, factory)
}

func TestNewFactory_NegativePasswordResetExpiry(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repos := repositories.NewFactory(db)

	viper.Reset()
	setRequiredFactoryConfig(t)
	viper.Set("password_reset_token_expiry_minutes", -10)

	factory, err := services.NewFactory(repos, db, nil, nil)
	require.Error(t, err)
	assert.Nil(t, factory)
}
