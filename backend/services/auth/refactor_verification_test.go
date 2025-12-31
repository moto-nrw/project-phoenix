package auth

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// TestRefactoringPreservesRepositoryAccess verifies that after refactoring,
// the service can still access all repositories through the factory pattern
func TestRefactoringPreservesRepositoryAccess(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

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

	// Create factory with repositories
	repos := &repositories.Factory{
		Account: accountRepo,
		Token:   tokenRepo,
		Role:    roleRepo,
	}

	// Create service config with validation
	config, err := NewServiceConfig(
		email.NewDispatcher(newCapturingMailer()),
		newDefaultFromEmail(),
		"http://localhost:3000",
		30*time.Minute,
	)
	require.NoError(t, err, "NewServiceConfig should succeed with valid config")

	// Create service with new factory-based signature
	service, err := NewService(repos, config, bunDB)
	require.NoError(t, err, "NewService should succeed with factory pattern")
	require.NotNil(t, service, "Service should not be nil")

	// Verify service can access repositories through factory
	require.NotNil(t, service.repos, "Service should store factory reference")
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
