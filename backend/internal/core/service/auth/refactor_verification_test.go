package auth

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/internal/adapter/mailer"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	authModel "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// TestRefactoringPreservesRepositoryAccess verifies that after refactoring,
// the service can still access all repositories through the factory pattern
func TestRefactoringPreservesRepositoryAccess(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()

	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	// Create stub repositories
	accountRepo := newStubAccountRepository(&authModel.Account{
		Model:        modelBase.Model{ID: 1},
		Email:        "test@example.com",
		Active:       true,
		PasswordHash: stringPtr("$argon2id$v=19$m=65536,t=3,p=2$somesalt$somehash"),
	})
	tokenRepo := newStubTokenRepository()
	roleRepo := newStubRoleRepository()

	repos := Repositories{
		Account: accountRepo,
		Token:   tokenRepo,
		Role:    roleRepo,
	}

	// Create service config with validation
	config, err := NewServiceConfig(
		mailer.NewDispatcher(newCapturingMailer()),
		newDefaultFromEmail(),
		"http://localhost:3000",
		30*time.Minute,
		false, // rateLimitEnabled
		3,     // rateLimitMaxRequests (explicit test value)
	)
	require.NoError(t, err, "NewServiceConfig should succeed with valid config")

	tokenProvider, err := jwt.NewTokenAuthWithSecret("test-jwt-secret-for-unit-tests-minimum-32-chars", 15*time.Minute, 24*time.Hour)
	require.NoError(t, err, "NewTokenAuthWithSecret should succeed")

	service, err := NewService(repos, config, bunDB, tokenProvider)
	require.NoError(t, err, "NewService should succeed with factory pattern")
	require.NotNil(t, service, "Service should not be nil")

	// Verify service can access repositories through factory
	require.NotNil(t, service.repos.Account, "Should access Account repo through factory")
	require.NotNil(t, service.repos.Token, "Should access Token repo through factory")
	require.NotNil(t, service.repos.Role, "Should access Role repo through factory")

	// Verify GetAccountByEmail uses factory (calls s.repos.Account.FindByEmail)
	ctx := context.Background()
	mock.ExpectBegin()
	mock.ExpectCommit()

	account, err := service.GetAccountByEmail(ctx, "test@example.com")
	require.NoError(t, err, "GetAccountByEmail should work with factory pattern")
	require.NotNil(t, account, "Should return account")
	require.Equal(t, "test@example.com", account.Email)

	t.Log("✅ Service successfully accesses repositories through factory")
	t.Log("✅ GetAccountByEmail verified to work after refactoring")
}

func stringPtr(s string) *string {
	return &s
}
