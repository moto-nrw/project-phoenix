package api

import (
	"log/slog"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testServiceJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"

func testServiceConfig() ServiceConfig {
	return ServiceConfig{
		JWTSecret:        testServiceJWTSecret,
		JWTExpiry:        15 * time.Minute,
		JWTRefreshExpiry: 24 * time.Hour,
		FrontendURL:      "http://localhost:3000",
		PublicAPIURL:     "http://localhost:8080",
		ParentsURL:       "http://parents.localhost:3000",
		SchoolURL:        "http://schule.localhost:3000",
		TenantDomain:     "localhost",
		OperatorHostname: "operator.localhost:3000",
	}
}

func TestServiceGraphComposesItsServices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	require.NotNil(t, repos)

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), testServiceConfig())
	require.NoError(t, err)
	require.NotNil(t, graph)

	// Verify all services are initialized
	t.Run("core services", func(t *testing.T) {
		assert.NotNil(t, graph.Auth)
		assert.NotNil(t, graph.Active)
		assert.NotNil(t, graph.ActiveCleanup)
		assert.NotNil(t, graph.Activities)
		assert.NotNil(t, graph.Education)
		assert.NotNil(t, graph.Facilities)
		assert.NotNil(t, graph.IoT)
		assert.NotNil(t, graph.Settings)
		assert.NotNil(t, GuardedCalendarAdministration(graph.Settings, graph.TimetableData.RecurrenceLock, graph.EnrollmentCareOffering).RecurrenceGate)
		assert.NotNil(t, graph.Users)
		assert.NotNil(t, graph.Guardian)
		assert.NotNil(t, graph.UserContext)
		assert.NotNil(t, graph.Invitation)
	})

	t.Run("realtime hub", func(t *testing.T) {
		assert.NotNil(t, graph.RealtimeHub)
	})

	t.Run("email configuration", func(t *testing.T) {
		assert.NotNil(t, graph.Mailer)
		assert.NotNil(t, graph.DefaultFrom)
	})

	t.Run("configured values", func(t *testing.T) {
		assert.Equal(t, "http://localhost:3000", graph.FrontendURL)

		// Default expiry values (when not configured)
		assert.Equal(t, 48*time.Hour, graph.InvitationTokenExpiry)
		assert.Equal(t, 30*time.Minute, graph.PasswordResetTokenExpiry)
	})
}

func TestServiceGraph_EnrollmentDeletionKeepsGuardianDirectoryAfterAuditSetup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	graph, err := NewServiceGraphForTestsWithConfig(NewRepositoryTestFactory(testpkg.NewDatabase(t, db)), testpkg.NewDatabase(t, db), slog.Default(), testServiceConfig())
	require.NoError(t, err)

	_, err = graph.EnrollmentDeletion.PreviewRequest(testpkg.Ctx(t), testpkg.UniqueTestTenantID(t))
	require.EqualError(t, err, "enrollment deletion target not found")
}

func TestServiceGraph_RejectsPartialVAPIDConfig(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	cfg := testServiceConfig()
	cfg.VAPIDPublicKey = "configured-without-the-other-required-values"

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.Error(t, err)
	require.Nil(t, graph)
	assert.ErrorContains(t, err, "invalid VAPID configuration")
	assert.ErrorContains(t, err, "VAPID_PRIVATE_KEY")
	assert.ErrorContains(t, err, "VAPID_SUBSCRIBER")
}

func TestServiceGraph_InvitationTokenExpiry_ZeroDefaults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, 48*time.Hour, graph.InvitationTokenExpiry)
}

func TestServiceGraph_InvitationTokenExpiry_ClampedToMax(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.InvitationTokenExpiryHours = 500

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, 168*time.Hour, graph.InvitationTokenExpiry)
}

func TestServiceGraph_InvitationTokenExpiry_ValidValue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.InvitationTokenExpiryHours = 72

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, 72*time.Hour, graph.InvitationTokenExpiry)
}

func TestServiceGraph_PasswordResetExpiry_ZeroDefaults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, 30*time.Minute, graph.PasswordResetTokenExpiry)
}

func TestServiceGraph_PasswordResetExpiry_ClampedToMax(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.PasswordResetExpiryMinutes = 2000

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, 1440*time.Minute, graph.PasswordResetTokenExpiry)
}

func TestServiceGraph_PasswordResetExpiry_ValidValue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.PasswordResetExpiryMinutes = 60

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, 60*time.Minute, graph.PasswordResetTokenExpiry)
}

func TestServiceGraph_FrontendURL_TrailingSlashRemoved(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.FrontendURL = "http://example.com/"

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, "http://example.com", graph.FrontendURL)
}

func TestServiceGraph_FrontendURL_Required(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.FrontendURL = ""

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.Error(t, err)
	require.Nil(t, graph)
	assert.Contains(t, err.Error(), "FRONTEND_URL")
}

func TestServiceGraph_PublicAPIURL_Required(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	cfg := testServiceConfig()
	cfg.PublicAPIURL = ""

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.Error(t, err)
	require.Nil(t, graph)
	assert.ErrorContains(t, err, "NEXT_PUBLIC_API_URL")
}

func TestServiceGraph_PortalURLs_Required(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	for _, tc := range []struct {
		name, want string
		clear      func(*ServiceConfig)
	}{
		{"parents", "PARENTS_URL", func(cfg *ServiceConfig) { cfg.ParentsURL = "" }},
		{"school", "SCHOOL_URL", func(cfg *ServiceConfig) { cfg.SchoolURL = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testServiceConfig()
			tc.clear(&cfg)

			graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
			require.Error(t, err)
			require.Nil(t, graph)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestServiceGraph_DefaultEmailFrom_WhenNotConfigured(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), testServiceConfig())
	require.NoError(t, err)
	require.NotNil(t, graph)

	// Default values when not configured
	assert.Equal(t, "moto", graph.DefaultFrom.Name)
	assert.Equal(t, "no-reply@moto.local", graph.DefaultFrom.Address)
}

func TestServiceGraph_EmailFrom_WhenConfigured(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.EmailFromName = "Test App"
	cfg.EmailFromAddress = "test@example.com"

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, "Test App", graph.DefaultFrom.Name)
	assert.Equal(t, "test@example.com", graph.DefaultFrom.Address)
}

func TestServiceGraph_NegativeInvitationExpiry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.InvitationTokenExpiryHours = -10

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, 48*time.Hour, graph.InvitationTokenExpiry)
}

func TestServiceGraph_NegativePasswordResetExpiry(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	repos := NewRepositoryTestFactory(testpkg.NewDatabase(t, db))

	cfg := testServiceConfig()
	cfg.PasswordResetExpiryMinutes = -10

	graph, err := NewServiceGraphForTestsWithConfig(repos, testpkg.NewDatabase(t, db), slog.Default(), cfg)
	require.NoError(t, err)
	require.NotNil(t, graph)

	assert.Equal(t, 30*time.Minute, graph.PasswordResetTokenExpiry)
}
