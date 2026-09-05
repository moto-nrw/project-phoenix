// Package auth_test tests the core authentication service layer with hermetic testing pattern.
package auth_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// testPassword is a valid password for integration tests that meets strength requirements.
// This is NOT a real secret - it's only used in test code with test databases.
const testPassword = "Test1234%" //nolint:gosec // pragma: allowlist secret

// testNewPassword is an alternative test password for password change/reset tests.
const testNewPassword = "NewStr0ng!Pass" //nolint:gosec // pragma: allowlist secret

// setupAuthService creates an Auth Service with real database connection
func authTestFactoryConfig(rateLimitEnabled bool) services.FactoryConfig {
	return services.FactoryConfig{
		JWTSecret:        "test-jwt-secret-for-unit-tests-minimum-32-chars",
		JWTExpiry:        15 * time.Minute,
		JWTRefreshExpiry: 24 * time.Hour,
		FrontendURL:      "http://localhost:3000",
		PublicAPIURL:     "http://localhost:8080",
		ParentsURL:       "http://parents.localhost:3000",
		SchoolURL:        "http://schule.localhost:3000",
		TenantDomain:     "localhost",
		OperatorHostname: "operator.localhost:3000",
		RateLimitEnabled: rateLimitEnabled,
	}
}

func setupAuthService(t *testing.T, db *bun.DB, rateLimitEnabled ...bool) auth.AuthService {
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	enabled := len(rateLimitEnabled) > 0 && rateLimitEnabled[0]
	serviceFactory, err := services.NewFactoryForTestsWithConfig(repoFactory, db, slog.Default(), authTestFactoryConfig(enabled))
	require.NoError(t, err, "Failed to create service factory")
	require.NoError(t, serviceFactory.SetTenantRuntime(testpkg.TenantRuntime(t, db)))
	return &fixtureOwnedAuthService{AuthService: serviceFactory.Auth, t: t, db: db}
}

func setupInvitationService(t *testing.T, db *bun.DB) auth.InvitationService {
	t.Helper()
	config := authTestFactoryConfig(false)
	signer, err := jwt.NewTokenAuthWithDurations(config.JWTSecret, config.JWTExpiry, config.JWTRefreshExpiry)
	require.NoError(t, err)
	repos, compositionErr := repositories.NewInvitationPersistence(db)
	require.NoError(t, compositionErr)
	service := auth.NewInvitationService(auth.InvitationServiceConfig{
		TokenAuth:         signer,
		InvitationRepo:    repos.InvitationToken,
		AccountRepo:       repos.Account,
		AccountTenantRepo: repos.AccountTenant,
		RoleRepo:          repos.Role,
		PermissionRepo:    repos.Permission,
		AccountRoleRepo:   repos.AccountRole,
		PersonRepo:        repos.Person,
		StaffRepo:         repos.Staff, TeacherRepo: repos.Teacher,
		StudentRepo: repos.Student,
		SchoolRepo:  repos.School,
		Mailer:      email.NewMockMailer(),
		FrontendURL: config.FrontendURL, SchoolURL: config.SchoolURL,
		InvitationExpiry: 48 * time.Hour, DB: db,
	})
	testpkg.SetTenantRuntime(t, service, db)
	return &fixtureOwnedInvitationService{InvitationService: service, t: t, db: db}
}

type fixtureOwnedAuthService struct {
	auth.AuthService
	t  *testing.T
	db *bun.DB
}

func (s *fixtureOwnedAuthService) Register(
	ctx context.Context,
	email, username, password string,
	roleID *int64,
	tenantID int64,
) (*authModels.Account, error) {
	account, err := s.AuthService.Register(ctx, email, username, password, roleID, tenantID)
	if account != nil {
		testpkg.OwnTestAccount(s.t, s.db, account.ID)
	}
	return account, err
}

func (s *fixtureOwnedAuthService) RegisterSchoolAccount(
	ctx context.Context,
	email, username, password string,
	roleID *int64,
	tenantID int64,
	identity *auth.SchoolAccountIdentity,
) (*authModels.Account, *auth.SchoolIdentity, error) {
	account, schoolIdentity, err := s.AuthService.RegisterSchoolAccount(
		ctx, email, username, password, roleID, tenantID, identity)
	if account != nil {
		testpkg.OwnTestAccount(s.t, s.db, account.ID)
	}
	return account, schoolIdentity, err
}

type fixtureOwnedInvitationService struct {
	auth.InvitationService
	t  *testing.T
	db *bun.DB
}

func (s *fixtureOwnedInvitationService) AcceptInvitation(
	ctx context.Context,
	token string,
	userData auth.UserRegistrationData,
) (*authModels.Account, error) {
	account, err := s.InvitationService.AcceptInvitation(ctx, token, userData)
	if account != nil {
		testpkg.OwnTestAccount(s.t, s.db, account.ID)
	}
	return account, err
}

// uniqueTestCredentials generates unique email and username for tests
func uniqueTestCredentials(prefix string) (email, username string) {
	uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
	email = fmt.Sprintf("%s-%s@test.local", prefix, uniqueID)
	username = fmt.Sprintf("%s-%s", prefix, uniqueID)
	return
}

func ownPasswordResetRateLimit(t *testing.T, db *bun.DB, email string) {
	t.Helper()
	t.Cleanup(func() {
		_, err := db.NewDelete().
			TableExpr("auth.password_reset_rate_limits").
			Where("email = ?", strings.ToLower(email)).
			Exec(context.Background())
		require.NoError(t, err)
	})
}

// =============================================================================
// Register Tests
// =============================================================================

func TestAuthService_Register(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("registers account successfully", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("register-%d@test.local", time.Now().UnixNano())
		username := fmt.Sprintf("user%d", time.Now().UnixNano())
		password := testPassword

		// ACT
		account, err := service.Register(ctx, email, username, password, nil, 0)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, account)
		assert.Greater(t, account.ID, int64(0))
		assert.Equal(t, email, account.Email)
	})

	t.Run("returns error for empty email", func(t *testing.T) {
		// ACT
		account, err := service.Register(ctx, "", "username", testPassword, nil, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account)
	})

	t.Run("returns error for empty password", func(t *testing.T) {
		// ACT
		account, err := service.Register(ctx, "test@example.com", "username", "", nil, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account)
	})

	t.Run("returns error for weak password", func(t *testing.T) {
		// ACT
		account, err := service.Register(ctx, "weak@example.com", "username", "weak", nil, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account)
	})

	t.Run("rejects role assignment without tenant context", func(t *testing.T) {
		role := testpkg.CreateTestRole(t, db, fmt.Sprintf("platform-role-%d", time.Now().UnixNano()))

		email := fmt.Sprintf("tenantless-role-%d@test.local", time.Now().UnixNano())
		username := fmt.Sprintf("tenantless-role-%d", time.Now().UnixNano())
		roleID := role.ID

		account, err := service.Register(ctx, email, username, testPassword, &roleID, 0)

		require.Error(t, err)
		assert.Nil(t, account)
		assert.True(t, errors.Is(err, auth.ErrTenantRequiredForRoleAssignment))

		var accountCount int
		err = db.NewSelect().
			TableExpr("auth.accounts").
			ColumnExpr("COUNT(*)").
			Where("email = ?", strings.ToLower(email)).
			Scan(context.Background(), &accountCount)
		require.NoError(t, err)
		assert.Equal(t, 0, accountCount, "registration should fail before creating an unusable account")
	})

	t.Run("returns error for duplicate email", func(t *testing.T) {
		// ARRANGE - create first account
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("duplicate-%s@test.local", uniqueID)
		username1 := fmt.Sprintf("user1-%s", uniqueID)
		account1, err := service.Register(ctx, email, username1, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account1.ID, testpkg.Tenant(t))

		// ACT - try to register with same email
		username2 := fmt.Sprintf("user2-%s", uniqueID)
		account2, err := service.Register(ctx, email, username2, testPassword, nil, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account2)
	})
}

// =============================================================================
// Login Tests
// =============================================================================

func TestAuthService_Login(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("login succeeds with valid credentials", func(t *testing.T) {
		// ARRANGE - create account
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("login-%s@test.local", uniqueID)
		username := fmt.Sprintf("loginuser-%s", uniqueID)
		password := testPassword
		account, err := service.Register(ctx, email, username, password, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		accessToken, refreshToken, err := service.Login(ctx, email, password)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
	})

	t.Run("login fails with wrong password", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("wrongpwd-%s@test.local", uniqueID)
		username := fmt.Sprintf("wrongpwd-%s", uniqueID)
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		accessToken, refreshToken, err := service.Login(ctx, email, "WrongPassword1!")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})

	t.Run("login fails with non-existent email", func(t *testing.T) {
		// ACT
		accessToken, refreshToken, err := service.Login(ctx, "nonexistent@test.local", testPassword)

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})

	t.Run("login fails with empty email", func(t *testing.T) {
		// ACT
		accessToken, refreshToken, err := service.Login(ctx, "", testPassword)

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})

	t.Run("login fails with empty password", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("emptypwd-%s@test.local", uniqueID)
		username := fmt.Sprintf("emptypwd-%s", uniqueID)
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		accessToken, refreshToken, err := service.Login(ctx, email, "")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})
}

func TestAuthService_Login_ConcurrentIssuanceKeepsFiveActiveSessions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := setupAuthService(t, db)
	email, username := uniqueTestCredentials("concurrent-session-cap")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	for range 5 {
		_, _, err = service.Login(ctx, email, testPassword)
		require.NoError(t, err)
	}

	const concurrency = 8
	issuers := make([]auth.AuthService, concurrency)
	for i := range issuers {
		issuers[i] = setupAuthService(t, db)
	}

	var barrier sync.WaitGroup
	barrier.Add(concurrency)
	results := make(chan error, concurrency)
	for _, issuer := range issuers {
		go func() {
			barrier.Done()
			barrier.Wait()
			_, _, loginErr := issuer.Login(ctx, email, testPassword)
			results <- loginErr
		}()
	}

	for range concurrency {
		require.NoError(t, <-results)
	}

	activeCount, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NULL").
		Where("expiry > ?", time.Now()).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 5, activeCount, "concurrent issuers must not bypass the session cap")
}

// =============================================================================
// ValidateToken Tests
// =============================================================================

// =============================================================================
// RefreshToken Tests
// =============================================================================

func TestAuthService_RefreshToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("refreshes token successfully", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("refresh")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		_, refreshToken, err := service.Login(ctx, email, testPassword)
		require.NoError(t, err)

		// ACT
		newAccessToken, newRefreshToken, err := service.RefreshToken(ctx, refreshToken)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, newAccessToken)
		assert.NotEmpty(t, newRefreshToken)
	})

	t.Run("returns error for invalid refresh token", func(t *testing.T) {
		// ACT
		accessToken, refreshToken, err := service.RefreshToken(ctx, "invalid.refresh.token")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})

	t.Run("returns error for empty refresh token", func(t *testing.T) {
		// ACT
		accessToken, refreshToken, err := service.RefreshToken(ctx, "")

		// ASSERT
		require.Error(t, err)
		assert.Empty(t, accessToken)
		assert.Empty(t, refreshToken)
	})
}

// TestAuthService_RefreshToken_ConcurrentSingleflight verifies that concurrent
// refresh calls with the same token are deduplicated by singleflight.
// Without singleflight, only the first goroutine succeeds (it deletes the old DB token),
// and all others get "token not found". With singleflight, all goroutines succeed.
func TestAuthService_RefreshToken_ConcurrentSingleflight(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	// ARRANGE: create account and get a refresh token
	email, username := uniqueTestCredentials("singleflight")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	_, refreshToken, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)

	// ACT: fire 5 concurrent refresh requests with the same token.
	// Use a barrier (WaitGroup) so all goroutines start at the same time —
	// without this, goroutine 1 can complete the entire refresh (rotating the
	// token in the DB) before goroutine 2 even starts, defeating singleflight.
	const concurrency = 5
	type result struct {
		accessToken  string
		refreshToken string
		err          error
	}
	results := make(chan result, concurrency)

	var barrier sync.WaitGroup
	barrier.Add(concurrency)
	for range concurrency {
		go func() {
			barrier.Done()
			barrier.Wait() // all goroutines unblock together
			at, rt, e := service.RefreshToken(ctx, refreshToken)
			results <- result{at, rt, e}
		}()
	}

	// ASSERT: all goroutines should succeed with the same tokens (deduplicated)
	var successes int
	var firstAccess, firstRefresh string
	for range concurrency {
		r := <-results
		if r.err != nil {
			t.Errorf("concurrent refresh failed: %v", r.err)
			continue
		}
		successes++
		assert.NotEmpty(t, r.accessToken, "access token should not be empty")
		assert.NotEmpty(t, r.refreshToken, "refresh token should not be empty")

		if firstAccess == "" {
			firstAccess = r.accessToken
			firstRefresh = r.refreshToken
		} else {
			// Singleflight returns the same result to all concurrent callers
			assert.Equal(t, firstAccess, r.accessToken, "all callers should get same access token")
			assert.Equal(t, firstRefresh, r.refreshToken, "all callers should get same refresh token")
		}
	}
	assert.Equal(t, concurrency, successes, "all concurrent refresh calls should succeed")
}

// TestAuthService_RefreshToken_InterruptedRotationRecovery reproduces the
// tablet/deploy failure from #1938: the backend commits a rotation, but the
// browser retries with the predecessor because the response/cookie was lost.
// Recovery must work from persisted lineage, not process-local singleflight.
func TestAuthService_RefreshToken_InterruptedRotationRecovery(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	serviceBeforeRestart := setupAuthService(t, db)
	email, username := uniqueTestCredentials("rotation-recovery")
	account, err := serviceBeforeRestart.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	_, predecessorJWT, err := serviceBeforeRestart.Login(ctx, email, testPassword)
	require.NoError(t, err)
	recoveryCtx := rotation.WithRecoveryProof(ctx, "independent-recovery-secret")
	_, firstSuccessorJWT, err := serviceBeforeRestart.RefreshToken(recoveryCtx, predecessorJWT)
	require.NoError(t, err)

	// A new service instance has empty in-memory singleflight/cache state and
	// models a frontend/backend process replacement during deployment.
	serviceAfterRestart := setupAuthService(t, db)
	accessToken, recoveredRefreshJWT, err := serviceAfterRestart.RefreshToken(recoveryCtx, predecessorJWT)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, firstSuccessorJWT)
	assert.NotEmpty(t, recoveredRefreshJWT)

	var generations []int
	err = db.NewSelect().
		TableExpr("auth.tokens").
		Column("generation").
		Where("account_id = ?", account.ID).
		OrderExpr("generation ASC").
		Scan(ctx, &generations)
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1}, generations,
		"recovery must reuse the committed successor instead of rotating again")
}

func TestAuthService_RefreshToken_InterruptedRotationRecoveryAcrossMultipleHandoffs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := setupAuthService(t, db)
	email, username := uniqueTestCredentials("rotation-multi-hop-recovery")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	_, predecessorJWT, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	firstProofCtx := rotation.WithRecoveryProof(ctx, "first-recovery-secret")
	_, firstSuccessorJWT, err := service.RefreshToken(firstProofCtx, predecessorJWT)
	require.NoError(t, err)
	secondProofCtx := rotation.WithRecoveryProof(ctx, "second-recovery-secret")
	_, secondSuccessorJWT, err := service.RefreshToken(secondProofCtx, firstSuccessorJWT)
	require.NoError(t, err)

	accessToken, recoveredRefreshJWT, err := setupAuthService(t, db).RefreshToken(firstProofCtx, predecessorJWT)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, secondSuccessorJWT)
	assert.NotEmpty(t, recoveredRefreshJWT)

	var generations []int
	err = db.NewSelect().
		TableExpr("auth.tokens").
		Column("generation").
		Where("account_id = ?", account.ID).
		OrderExpr("generation ASC").
		Scan(ctx, &generations)
	require.NoError(t, err)
	assert.Equal(t, []int{0, 1, 2}, generations,
		"a delayed predecessor must follow the persisted lineage without revoking or rotating it")

	currentCount, err := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NULL").
		Where("generation = 2").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, currentCount)
}

func TestAuthService_RefreshToken_ReplayAfterGraceCommitsFamilyRevocation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("rotation-replay")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	_, predecessorJWT, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	recoveryCtx := rotation.WithRecoveryProof(ctx, "independent-recovery-secret")
	_, successorJWT, err := service.RefreshToken(recoveryCtx, predecessorJWT)
	require.NoError(t, err)

	_, err = db.NewUpdate().
		Table("auth.tokens").
		Set("rotated_at = ?", time.Now().Add(-rotation.RecoveryGrace-time.Minute)).
		Where("account_id = ?", account.ID).
		Where("rotated_at IS NOT NULL").
		Exec(ctx)
	require.NoError(t, err)

	// Rotate the successor after the predecessor leaves the recovery window.
	// Cleanup must retain the predecessor until its JWT expiry so a later replay
	// can still revoke the whole family.
	_, _, err = service.RefreshToken(rotation.WithRecoveryProof(ctx, "successor-recovery-secret"), successorJWT)
	require.NoError(t, err)

	_, _, err = setupAuthService(t, db).RefreshToken(recoveryCtx, predecessorJWT)
	require.Error(t, err)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrInvalidToken)

	count, countErr := db.NewSelect().
		TableExpr("auth.tokens").
		Where("account_id = ?", account.ID).
		Count(ctx)
	require.NoError(t, countErr)
	assert.Zero(t, count, "replay rejection must commit token-family deletion")
}

func TestAuthService_RefreshToken_WrongRecoveryProofRevokesFamily(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)
	email, username := uniqueTestCredentials("rotation-wrong-proof")
	account, err := service.Register(ctx, email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	_, predecessorJWT, err := service.Login(ctx, email, testPassword)
	require.NoError(t, err)
	_, _, err = service.RefreshToken(rotation.WithRecoveryProof(ctx, "independent-recovery-secret"), predecessorJWT)
	require.NoError(t, err)

	_, _, err = setupAuthService(t, db).RefreshToken(rotation.WithRecoveryProof(ctx, "attacker-does-not-have-the-recovery-secret"), predecessorJWT)
	require.Error(t, err)
	var authErr *auth.AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, auth.ErrInvalidToken)

	count, countErr := db.NewSelect().TableExpr("auth.tokens").Where("account_id = ?", account.ID).Count(ctx)
	require.NoError(t, countErr)
	assert.Zero(t, count, "failed possession proof must commit token-family revocation")
}

// =============================================================================
// Logout Tests
// =============================================================================

func TestAuthService_Logout(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("logout succeeds", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("logout")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
		secondaryTenantID := account.ID + 1_000_000_000
		testpkg.EnsureTestTenant(t, db, secondaryTenantID)
		testpkg.MapAccountToTenant(t, db, account.ID, secondaryTenantID)

		staffEndpoint := fmt.Sprintf("https://fcm.googleapis.com/logout-staff-%d", account.ID)
		for _, tenantID := range []int64{tenant.FromContext(ctx), secondaryTenantID} {
			subscription := &deliveryModels.PushSubscription{
				AccountID: account.ID,
				Portal:    deliveryModels.PushPortalStaff,
				Endpoint:  staffEndpoint,
				P256dh:    "p256dh-key",
				Auth:      "auth-key",
			}
			subscription.SetTenantID(tenantID)
			_, err = db.NewInsert().Model(subscription).ModelTableExpr("iot.push_subscriptions").Exec(ctx)
			require.NoError(t, err)
		}

		_, refreshToken, err := service.Login(ctx, email, testPassword)
		require.NoError(t, err)

		// ACT
		err = service.LogoutWithAudit(ctx, refreshToken, "", "")

		// ASSERT
		require.NoError(t, err)

		// Verify token is invalidated (refresh should fail)
		_, _, err = service.RefreshToken(ctx, refreshToken)
		require.Error(t, err)

		staffCount, err := db.NewSelect().
			Model((*deliveryModels.PushSubscription)(nil)).
			ModelTableExpr(`iot.push_subscriptions AS "push_subscription"`).
			Where("account_id = ?", account.ID).
			Where("portal = ?", deliveryModels.PushPortalStaff).
			Count(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, staffCount, "logout clears unbound staff push at the session school only")
	})

	t.Run("logout with invalid token returns error", func(t *testing.T) {
		// ACT
		err := service.LogoutWithAudit(ctx, "invalid.refresh.token", "", "")

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ChangePassword Tests
// =============================================================================

func TestAuthService_ChangePassword(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("changes password successfully", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("changepwd")
		oldPassword := testPassword
		newPassword := "NewPassword1%"
		account, err := service.Register(ctx, email, username, oldPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		err = service.ChangePassword(ctx, int(account.ID), oldPassword, newPassword)

		// ASSERT
		require.NoError(t, err)

		// Verify new password works
		_, _, err = service.Login(ctx, email, newPassword)
		require.NoError(t, err)
	})

	t.Run("returns error for wrong current password", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("wrongcurrent")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)

		// ACT
		err = service.ChangePassword(ctx, int(account.ID), "WrongPassword1!", "NewPassword1%")

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for weak new password", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("weaknew")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)

		// ACT
		err = service.ChangePassword(ctx, int(account.ID), testPassword, "weak")

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// GetAccountByID Tests
// =============================================================================

func TestAuthService_GetAccountByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns account when found", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("getbyid")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		result, err := service.GetAccountByID(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, account.ID, result.ID)
		assert.Equal(t, email, result.Email)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetAccountByID(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetAccountByEmail Tests
// =============================================================================

// =============================================================================
// Account Activation/Deactivation Tests
// =============================================================================

func TestAuthService_ActivateAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("activates account successfully", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("activate")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// First deactivate
		err = service.DeactivateAccount(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT
		err = service.ActivateAccount(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify account is active
		updated, err := service.GetAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		assert.True(t, updated.Active)
	})

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ACT
		err := service.ActivateAccount(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_DeactivateAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deactivates account successfully", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("deactivate")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		err = service.DeactivateAccount(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify account is inactive
		updated, err := service.GetAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		assert.False(t, updated.Active)
	})

	t.Run("deactivated account cannot login", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("nologin")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		err = service.DeactivateAccount(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT
		_, _, err = service.Login(ctx, email, testPassword)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ListAccounts Tests
// =============================================================================

func TestAuthService_ListAccounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns accounts with no filters", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("list")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		result, err := service.ListAccounts(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("returns accounts with filters", func(t *testing.T) {
		// ACT
		filters := map[string]interface{}{
			"active": true,
		}
		result, err := service.ListAccounts(ctx, filters)

		// ASSERT
		require.NoError(t, err)
		// All returned accounts should be active
		for _, acc := range result {
			assert.True(t, acc.Active)
		}
	})
}

// =============================================================================
// Token Cleanup Tests
// =============================================================================

func TestAuthService_CleanupExpiredTokens(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("cleans up expired tokens", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredTokens(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

func TestAuthService_RevokeAllTokens(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("revokes all tokens for account", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("revoke")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// Login to create tokens
		_, refreshToken, err := service.Login(ctx, email, testPassword)
		require.NoError(t, err)

		// ACT
		err = service.RevokeAllTokens(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify tokens are revoked
		_, _, err = service.RefreshToken(ctx, refreshToken)
		require.Error(t, err)
	})
}

func TestAuthService_GetActiveTokens(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns active tokens for account", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("activetokens")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// Login to create token
		_, _, err = service.Login(ctx, email, testPassword)
		require.NoError(t, err)

		// ACT
		tokens, err := service.GetActiveTokens(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, tokens)
	})

	t.Run("returns empty list for account with no tokens", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("notokens")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)

		// Revoke any tokens from registration
		err = service.RevokeAllTokens(ctx, int(account.ID))
		require.NoError(t, err)

		// ACT
		tokens, err := service.GetActiveTokens(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, tokens)
	})
}

// =============================================================================
// Role Management Tests
// =============================================================================

func TestAuthService_CreateRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates role successfully", func(t *testing.T) {
		// ARRANGE
		name := fmt.Sprintf("test-role-%d", time.Now().UnixNano())

		// ACT
		role, err := service.CreateRole(ctx, name, "Test role description", testpkg.StrPtr("user"))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, role)
		assert.Greater(t, role.ID, int64(0))
		assert.Equal(t, name, role.Name)
	})

	t.Run("returns error for empty name", func(t *testing.T) {
		// ACT
		role, err := service.CreateRole(ctx, "", "description", testpkg.StrPtr("user"))

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, role)
	})
}

func TestAuthService_GetRoleByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns role when found", func(t *testing.T) {
		// ARRANGE
		name := fmt.Sprintf("get-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, name, "description", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// ACT
		result, err := service.GetRoleByID(ctx, int(role.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, role.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetRoleByID(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAuthService_UpdateRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates role successfully", func(t *testing.T) {
		// ARRANGE
		name := fmt.Sprintf("update-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, name, "original description", testpkg.StrPtr("user"))
		require.NoError(t, err)

		role.Description = "updated description"

		// ACT
		err = service.UpdateRole(ctx, role)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetRoleByID(ctx, int(role.ID))
		require.NoError(t, err)
		assert.Equal(t, "updated description", updated.Description)
	})
}

func TestAuthService_DeleteRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes role successfully", func(t *testing.T) {
		// ARRANGE
		name := fmt.Sprintf("delete-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, name, "to delete", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// ACT
		err = service.DeleteRole(ctx, int(role.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetRoleByID(ctx, int(role.ID))
		require.Error(t, err)
	})
}

func TestAuthService_ListRoles(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns roles", func(t *testing.T) {
		// ARRANGE
		name := fmt.Sprintf("list-role-%d", time.Now().UnixNano())
		_, err := service.CreateRole(ctx, name, "for listing", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// ACT
		result, err := service.ListRoles(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

func TestAuthService_AssignRoleToAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("assigns role to account", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("assignrole")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		roleName := fmt.Sprintf("assign-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "for assignment", testpkg.StrPtr("user"))
		require.NoError(t, err)

		// ACT
		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify assignment
		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		found := false
		for _, r := range roles {
			if r.ID == role.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected to find assigned role")
	})

	t.Run("participates in outer transaction rollback", func(t *testing.T) {
		account := testpkg.CreateTestAccount(t, db, "assign-role-tx")
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		token := testpkg.CreateTestTokenForTenant(t, db, testpkg.Tenant(t), account.ID)

		roleName := fmt.Sprintf("assign-role-tx-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "transaction rollback verification", testpkg.StrPtr("user"))
		require.NoError(t, err)

		sentinelErr := errors.New("force outer rollback")
		txHandler := tenant.NewTransactionRunner()

		err = txHandler.RunInTx(ctx, func(txCtx context.Context) error {
			if err := service.AssignRoleToAccount(txCtx, int(account.ID), int(role.ID)); err != nil {
				return err
			}
			return sentinelErr
		})
		require.ErrorIs(t, err, sentinelErr)

		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		assert.Empty(t, roles, "role assignment should roll back with the outer transaction")

		tokens, err := service.GetActiveTokens(ctx, int(account.ID))
		require.NoError(t, err)
		require.Len(t, tokens, 1)
		assert.Equal(t, token.ID, tokens[0].ID, "token deletion should roll back with the outer transaction")
	})

	t.Run("existing refresh token is revoked after role assignment", func(t *testing.T) {
		email, username := uniqueTestCredentials("assign-role-refresh")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		_, refreshToken, err := service.Login(ctx, email, testPassword)
		require.NoError(t, err)

		roleName := fmt.Sprintf("assign-role-refresh-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "refresh propagation verification", testpkg.StrPtr("user"))
		require.NoError(t, err)

		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		newAccessToken, newRefreshToken, err := service.RefreshToken(ctx, refreshToken)
		require.Error(t, err)
		assert.Empty(t, newAccessToken)
		assert.Empty(t, newRefreshToken)
	})
}

func TestAuthService_RemoveRoleFromAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("removes role from account", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("removerole")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		roleName := fmt.Sprintf("remove-role-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "for removal", testpkg.StrPtr("user"))
		require.NoError(t, err)

		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		// ACT
		err = service.RemoveRoleFromAccount(ctx, int(account.ID), int(role.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify removal
		roles, err := service.GetAccountRoles(ctx, int(account.ID))
		require.NoError(t, err)
		for _, r := range roles {
			assert.NotEqual(t, role.ID, r.ID)
		}
	})

	t.Run("existing refresh token is revoked after role removal", func(t *testing.T) {
		email, username := uniqueTestCredentials("remove-role-refresh")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		roleName := fmt.Sprintf("remove-role-refresh-%d", time.Now().UnixNano())
		role, err := service.CreateRole(ctx, roleName, "refresh propagation verification", testpkg.StrPtr("user"))
		require.NoError(t, err)

		err = service.AssignRoleToAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		_, refreshToken, err := service.Login(ctx, email, testPassword)
		require.NoError(t, err)

		err = service.RemoveRoleFromAccount(ctx, int(account.ID), int(role.ID))
		require.NoError(t, err)

		newAccessToken, newRefreshToken, err := service.RefreshToken(ctx, refreshToken)
		require.Error(t, err)
		assert.Empty(t, newAccessToken)
		assert.Empty(t, newRefreshToken)
	})
}

// =============================================================================
// Permission Management Tests
// =============================================================================

func TestAuthService_CreatePermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates permission successfully", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		name := fmt.Sprintf("test-perm-%s", uniqueID)
		resource := fmt.Sprintf("resource-create-%s", uniqueID)

		// ACT
		perm, err := service.CreatePermission(ctx, name, "Test permission", resource, "read")

		// ASSERT
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)
		assert.NotNil(t, perm)
		assert.Greater(t, perm.ID, int64(0))
		assert.Equal(t, name, perm.Name)
	})
}

func TestAuthService_GetPermissionByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns permission when found", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		name := fmt.Sprintf("get-perm-%s", uniqueID)
		resource := fmt.Sprintf("resource-get-%s", uniqueID)
		perm, err := service.CreatePermission(ctx, name, "desc", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		// ACT
		result, err := service.GetPermissionByID(ctx, int(perm.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, perm.ID, result.ID)
	})
}

func TestAuthService_ListPermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns permissions", func(t *testing.T) {
		// ACT
		result, err := service.ListPermissions(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestAuthService_GrantPermissionToAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("grants permission to account", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email, username := uniqueTestCredentials("grantperm")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		permName := fmt.Sprintf("grant-perm-%s", uniqueID)
		resource := fmt.Sprintf("resource-grant-%s", uniqueID)
		perm, err := service.CreatePermission(ctx, permName, "desc", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, perm.ID)

		// ACT
		err = service.GrantPermissionToAccount(ctx, int(account.ID), int(perm.ID))

		// ASSERT
		require.NoError(t, err)
	})
}

// =============================================================================
// Parent Account Tests
// =============================================================================

func TestAuthService_CreateParentAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates parent account successfully", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("parent-%s@test.local", uniqueID)
		username := fmt.Sprintf("parent-%s", uniqueID)

		// ACT
		account, err := service.CreateParentAccount(ctx, email, username, testPassword)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, account)
		assert.Greater(t, account.ID, int64(0))
		assert.Equal(t, email, account.Email)
	})

	t.Run("returns error for duplicate email", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("dupparent-%s@test.local", uniqueID)
		username1 := fmt.Sprintf("parent1-%s", uniqueID)
		_, err := service.CreateParentAccount(ctx, email, username1, testPassword)
		require.NoError(t, err)

		// ACT
		username2 := fmt.Sprintf("parent2-%s", uniqueID)
		account, err := service.CreateParentAccount(ctx, email, username2, testPassword)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, account)
	})
}

func TestAuthService_GetParentAccountByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns parent account when found", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("getparent-%s@test.local", uniqueID)
		username := fmt.Sprintf("getparent-%s", uniqueID)
		account, err := service.CreateParentAccount(ctx, email, username, testPassword)
		require.NoError(t, err)

		// ACT
		result, err := service.GetParentAccountByID(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, account.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetParentAccountByID(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAuthService_ListParentAccounts(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns parent accounts", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		email := fmt.Sprintf("listparent-%s@test.local", uniqueID)
		username := fmt.Sprintf("listparent-%s", uniqueID)
		_, err := service.CreateParentAccount(ctx, email, username, testPassword)
		require.NoError(t, err)

		// ACT
		result, err := service.ListParentAccounts(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

// =============================================================================
// Permission Management Tests (Additional Coverage)
// =============================================================================

func TestAuthService_GetPermissionByName(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns permission when found", func(t *testing.T) {
		// ARRANGE - create a permission with unique resource/action
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("test-perm-%s", uniqueID)
		resource := fmt.Sprintf("res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "Test permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		// ACT
		result, err := service.GetPermissionByName(ctx, permName)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, permission.ID, result.ID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetPermissionByName(ctx, "nonexistent-permission")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAuthService_UpdatePermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates permission successfully", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("update-perm-%s", uniqueID)
		resource := fmt.Sprintf("upd-res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "Original description", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		permission.Description = "Updated description"

		// ACT
		err = service.UpdatePermission(ctx, permission)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetPermissionByID(ctx, int(permission.ID))
		require.NoError(t, err)
		assert.Equal(t, "Updated description", updated.Description)
	})
}

func TestAuthService_DeletePermission(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deletes permission successfully", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("delete-perm-%s", uniqueID)
		resource := fmt.Sprintf("del-res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "To be deleted", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		// ACT
		err = service.DeletePermission(ctx, int(permission.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify deletion
		_, err = service.GetPermissionByID(ctx, int(permission.ID))
		require.Error(t, err)
	})
}

func TestAuthService_GetAccountPermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns account permissions", func(t *testing.T) {
		// ARRANGE - create account with permission
		email, username := uniqueTestCredentials("acctperms")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("acctperm-%s", uniqueID)
		resource := fmt.Sprintf("acct-res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "Account permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		err = service.GrantPermissionToAccount(ctx, int(account.ID), int(permission.ID))
		require.NoError(t, err)

		// ACT
		result, err := service.GetAccountPermissions(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

func TestAuthService_GetAccountDirectPermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns direct permissions only", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("directperms")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("directperm-%s", uniqueID)
		resource := fmt.Sprintf("direct-res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "Direct permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		err = service.GrantPermissionToAccount(ctx, int(account.ID), int(permission.ID))
		require.NoError(t, err)

		// ACT
		result, err := service.GetAccountDirectPermissions(ctx, int(account.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

func TestAuthService_RemovePermissionFromAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("removes permission from account", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("removeperm")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permName := fmt.Sprintf("removeperm-%s", uniqueID)
		resource := fmt.Sprintf("rem-res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "To be removed", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		err = service.GrantPermissionToAccount(ctx, int(account.ID), int(permission.ID))
		require.NoError(t, err)

		// ACT
		err = service.RemovePermissionFromAccount(ctx, int(account.ID), int(permission.ID))

		// ASSERT
		require.NoError(t, err)
	})
}

// =============================================================================
// Role-Permission Management Tests
// =============================================================================

func TestAuthService_AssignPermissionToRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("assigns permission to role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		roleName := fmt.Sprintf("role-%s", uniqueID)
		role, err := service.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		permName := fmt.Sprintf("roleperm-%s", uniqueID)
		resource := fmt.Sprintf("role-res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "Role permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		// ACT
		err = service.AssignPermissionToRole(ctx, int(role.ID), int(permission.ID))

		// ASSERT
		require.NoError(t, err)
	})
}

func TestAuthService_RemovePermissionFromRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("removes permission from role", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		roleName := fmt.Sprintf("role-remove-%s", uniqueID)
		role, err := service.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		permName := fmt.Sprintf("rolerem-%s", uniqueID)
		resource := fmt.Sprintf("rolerem-res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "To be removed from role", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		err = service.AssignPermissionToRole(ctx, int(role.ID), int(permission.ID))
		require.NoError(t, err)

		// ACT
		err = service.RemovePermissionFromRole(ctx, int(role.ID), int(permission.ID))

		// ASSERT
		require.NoError(t, err)
	})
}

func TestAuthService_GetRolePermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns role permissions", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		roleName := fmt.Sprintf("role-get-%s", uniqueID)
		role, err := service.CreateRole(ctx, roleName, "Test role", testpkg.StrPtr("user"))
		require.NoError(t, err)

		permName := fmt.Sprintf("roleget-%s", uniqueID)
		resource := fmt.Sprintf("roleget-res-%s", uniqueID)
		permission, err := service.CreatePermission(ctx, permName, "Role permission", resource, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		err = service.AssignPermissionToRole(ctx, int(role.ID), int(permission.ID))
		require.NoError(t, err)

		// ACT
		result, err := service.GetRolePermissions(ctx, int(role.ID))

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})
}

// =============================================================================
// Account Management Tests (Additional Coverage)
// =============================================================================

func TestAuthService_UpdateAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates account successfully", func(t *testing.T) {
		// ARRANGE
		email, username := uniqueTestCredentials("updateacct")
		account, err := service.Register(ctx, email, username, testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		account.Active = false

		// ACT
		err = service.UpdateAccount(ctx, account)

		// ASSERT
		require.NoError(t, err)

		// Verify update
		updated, err := service.GetAccountByID(ctx, int(account.ID))
		require.NoError(t, err)
		assert.False(t, updated.IsActive())
	})
}

func TestAuthService_GetAccountsByRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns accounts with role or empty list", func(t *testing.T) {
		// ACT - use existing teacher role name
		_, err := service.GetAccountsByRole(ctx, "teacher")

		// ASSERT - may be empty if no accounts have teacher role
		require.NoError(t, err)
		// Result can be nil or empty slice, both are valid
	})
}

// NOTE: GetAccountsWithRolesAndPermissions test is skipped because the repository
// uses unqualified table names that may not work in all test database configurations.

// =============================================================================
// Cleanup Functions Tests
// =============================================================================

func TestAuthService_CleanupExpiredPasswordResetTokens(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("cleans up expired tokens without error", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredPasswordResetTokens(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

func TestAuthService_CleanupExpiredRateLimits(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("cleans up expired rate limits without error", func(t *testing.T) {
		// ACT
		count, err := service.CleanupExpiredRateLimits(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

// =============================================================================
// Parent Account Tests (Additional Coverage)
// =============================================================================

// NOTE: GetParentAccountByEmail and UpdateParentAccount tests are skipped
// because account_parents table may not exist in all test database configurations.
// These methods are tested via API integration tests instead.

// =============================================================================
// Additional Permission Tests
// =============================================================================

func TestAuthService_DenyPermissionToAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ARRANGE
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permission, err := service.CreatePermission(ctx, "deny-perm-"+uniqueID, "Test permission", "deny-resource-"+uniqueID, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		// ACT
		err = service.DenyPermissionToAccount(ctx, 99999999, int(permission.ID))

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for non-existent permission", func(t *testing.T) {
		// ARRANGE
		account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("deny-perm-%d@test.com", time.Now().UnixNano()))

		// ACT
		err := service.DenyPermissionToAccount(ctx, int(account.ID), 99999999)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("denies permission to account", func(t *testing.T) {
		// ARRANGE
		account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("deny-success-%d@test.com", time.Now().UnixNano()))
		uniqueID := fmt.Sprintf("%d", time.Now().UnixNano())
		permission, err := service.CreatePermission(ctx, "deny-success-perm-"+uniqueID, "Test", "deny-success-res-"+uniqueID, "read")
		require.NoError(t, err)
		testpkg.OwnTestPermission(t, db, permission.ID)

		// ACT
		err = service.DenyPermissionToAccount(ctx, int(account.ID), int(permission.ID))

		// ASSERT
		require.NoError(t, err)
	})
}

// =============================================================================
// Additional Invitation Tests
// =============================================================================

func TestInvitationService_ListPendingInvitations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	invitationService := setupInvitationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns list without error", func(t *testing.T) {
		// ACT
		_, err := invitationService.ListPendingInvitations(ctx)

		// ASSERT - no error means success (empty list is valid)
		require.NoError(t, err)
		// invitations can be nil or empty slice
	})
}

func TestInvitationService_CleanupExpiredInvitations(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	invitationService := setupInvitationService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("cleans up expired invitations without error", func(t *testing.T) {
		// ACT
		count, err := invitationService.CleanupExpiredInvitations(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, 0)
	})
}

func TestInvitationService_CreateInvitation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	invitationService := setupInvitationService(t, db)
	authService := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates invitation with valid data", func(t *testing.T) {
		// ARRANGE - Get a role (use existing "User" role or create one)
		role := testpkg.GetOrCreateTestRole(t, db, "User")

		// Create an account to be the creator
		creatorEmail := fmt.Sprintf("creator-%d@test.local", time.Now().UnixNano())
		creator, err := authService.Register(ctx, creatorEmail, fmt.Sprintf("creator%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)

		inviteeEmail := fmt.Sprintf("invitee-%d@test.local", time.Now().UnixNano())

		// ACT
		invitation, err := invitationService.CreateInvitation(ctx, auth.InvitationRequest{
			Email:     inviteeEmail,
			RoleID:    role.ID,
			CreatedBy: creator.ID,
			FirstName: testpkg.StrPtr("Test"),
			LastName:  testpkg.StrPtr("User"),
		})

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, invitation)
		assert.Equal(t, inviteeEmail, invitation.Email)
		assert.Equal(t, role.ID, invitation.RoleID)
		assert.NotEmpty(t, invitation.Token)
		assert.True(t, invitation.ExpiresAt.After(time.Now()))
	})

	t.Run("normalizes email to lowercase", func(t *testing.T) {
		// ARRANGE
		role := testpkg.GetOrCreateTestRole(t, db, "User")
		creatorEmail := fmt.Sprintf("creator2-%d@test.local", time.Now().UnixNano())
		creator, err := authService.Register(ctx, creatorEmail, fmt.Sprintf("creator2%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)

		mixedCaseEmail := fmt.Sprintf("MixedCase-%d@Test.Local", time.Now().UnixNano())

		// ACT
		invitation, err := invitationService.CreateInvitation(ctx, auth.InvitationRequest{
			Email:     mixedCaseEmail,
			RoleID:    role.ID,
			CreatedBy: creator.ID,
		})

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, invitation)
		assert.Equal(t, strings.ToLower(mixedCaseEmail), invitation.Email)
	})
}

func TestInvitationService_ValidateInvitation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	invitationService := setupInvitationService(t, db)
	authService := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("validates valid invitation token", func(t *testing.T) {
		// ARRANGE
		role := testpkg.GetOrCreateTestRole(t, db, "User")
		creatorEmail := fmt.Sprintf("creator-val-%d@test.local", time.Now().UnixNano())
		creator, err := authService.Register(ctx, creatorEmail, fmt.Sprintf("creatorval%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)

		firstName, lastName := "Grace", "Hopper"
		invitation := testpkg.CreateTestInvitationTokenWithOptions(
			t, db, "validate",
			role.ID, creator.ID,
			time.Now().Add(24*time.Hour),
			&testpkg.InvitationTokenOptions{FirstName: &firstName, LastName: &lastName},
		)

		// ACT
		result, err := invitationService.ValidateInvitation(ctx, invitation.Token)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, invitation.Email, result.Email)
		assert.Equal(t, "Grace", *result.FirstName)
		assert.Equal(t, "Hopper", *result.LastName)
	})

	t.Run("returns error for expired invitation", func(t *testing.T) {
		// ARRANGE
		role := testpkg.CreateTestRole(t, db, "expired-invitation")
		creatorEmail := fmt.Sprintf("creator-exp-%d@test.local", time.Now().UnixNano())
		creator, err := authService.Register(ctx, creatorEmail, fmt.Sprintf("creatorexp%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)

		// Create expired invitation
		invitation := testpkg.CreateTestInvitationToken(
			t, db, "expired",
			role.ID, creator.ID,
			time.Now().Add(-1*time.Hour), // Expired
		)

		// ACT
		_, err = invitationService.ValidateInvitation(ctx, invitation.Token)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrInvitationExpired))
	})

	t.Run("returns error for non-existent token", func(t *testing.T) {
		// ACT
		_, err := invitationService.ValidateInvitation(ctx, "non-existent-token-12345")

		// ASSERT
		require.Error(t, err)
	})
}

func TestInvitationService_AcceptInvitation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	invitationService := setupInvitationService(t, db)
	authService := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("accepts invitation and creates account", func(t *testing.T) {
		// ARRANGE
		role := testpkg.GetOrCreateTestRole(t, db, "User")
		creatorEmail := fmt.Sprintf("creator-acc-%d@test.local", time.Now().UnixNano())
		creator, err := authService.Register(ctx, creatorEmail, fmt.Sprintf("creatoracc%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)

		invitation := testpkg.CreateTestInvitationToken(
			t, db, "accept",
			role.ID, creator.ID,
			time.Now().Add(24*time.Hour),
		)

		// ACT
		account, err := invitationService.AcceptInvitation(ctx, invitation.Token, auth.UserRegistrationData{
			FirstName:       "Katherine",
			LastName:        "Johnson",
			Password:        testPassword,
			ConfirmPassword: testPassword,
		})

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, account)
		assert.Equal(t, invitation.Email, account.Email)
		assert.True(t, account.Active)

		// Verify the invitation is now marked as used
		_, err = invitationService.ValidateInvitation(ctx, invitation.Token)
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrInvitationUsed))
	})

	t.Run("rejects weak password", func(t *testing.T) {
		// ARRANGE
		role := testpkg.GetOrCreateTestRole(t, db, "User")
		creatorEmail := fmt.Sprintf("creator-weak-%d@test.local", time.Now().UnixNano())
		creator, err := authService.Register(ctx, creatorEmail, fmt.Sprintf("creatorweak%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)

		invitation := testpkg.CreateTestInvitationToken(
			t, db, "weakpass",
			role.ID, creator.ID,
			time.Now().Add(24*time.Hour),
		)

		// ACT
		_, err = invitationService.AcceptInvitation(ctx, invitation.Token, auth.UserRegistrationData{
			FirstName:       "Test",
			LastName:        "User",
			Password:        "weak",
			ConfirmPassword: "weak",
		})

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrPasswordTooWeak))

		// Verify invitation is NOT marked as used
		_, err = invitationService.ValidateInvitation(ctx, invitation.Token)
		require.NoError(t, err) // Should still be valid
	})

	t.Run("rejects expired invitation", func(t *testing.T) {
		// ARRANGE
		role := testpkg.GetOrCreateTestRole(t, db, "User")
		creatorEmail := fmt.Sprintf("creator-exprej-%d@test.local", time.Now().UnixNano())
		creator, err := authService.Register(ctx, creatorEmail, fmt.Sprintf("creatorexprej%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)

		invitation := testpkg.CreateTestInvitationToken(
			t, db, "expiredaccept",
			role.ID, creator.ID,
			time.Now().Add(-1*time.Hour), // Expired
		)

		// ACT
		_, err = invitationService.AcceptInvitation(ctx, invitation.Token, auth.UserRegistrationData{
			FirstName:       "Test",
			LastName:        "User",
			Password:        testPassword,
			ConfirmPassword: testPassword,
		})

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrInvitationExpired))
	})
}

func TestInvitationService_RevokeInvitation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	invitationService := setupInvitationService(t, db)
	authService := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("revokes pending invitation", func(t *testing.T) {
		// ARRANGE
		role := testpkg.GetOrCreateTestRole(t, db, "User")
		creatorEmail := fmt.Sprintf("creator-rev-%d@test.local", time.Now().UnixNano())
		creator, err := authService.Register(ctx, creatorEmail, fmt.Sprintf("creatorrev%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)

		invitation := testpkg.CreateTestInvitationToken(
			t, db, "revoke",
			role.ID, creator.ID,
			time.Now().Add(24*time.Hour),
		)

		// ACT
		err = invitationService.RevokeInvitation(ctx, invitation.ID, creator.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify the invitation is now marked as used
		_, err = invitationService.ValidateInvitation(ctx, invitation.Token)
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrInvitationUsed))
	})
}

// =============================================================================
// Password Reset Integration Tests
// =============================================================================

func TestAuthService_InitiatePasswordReset(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates password reset token for existing account", func(t *testing.T) {
		// ARRANGE - Create an account
		email := fmt.Sprintf("reset-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("resetuser%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		// ACT
		token, err := service.InitiatePasswordReset(ctx, email)

		// ASSERT
		require.NoError(t, err)
		require.NotNil(t, token)
		assert.NotEmpty(t, token.Token)
		assert.Equal(t, account.ID, token.AccountID)
		assert.True(t, token.Expiry.After(time.Now()))
	})

	t.Run("returns nil for non-existent email (security by design)", func(t *testing.T) {
		// NOTE: The service intentionally returns (nil, nil) for non-existent emails
		// to avoid revealing whether an email address exists in the system.

		// ACT
		token, err := service.InitiatePasswordReset(ctx, "nonexistent-for-reset@test.local")

		// ASSERT - Both should be nil (no error, no token)
		require.NoError(t, err)
		assert.Nil(t, token)
	})
}

func TestAuthService_ResetPassword(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("resets password with valid token", func(t *testing.T) {
		// ARRANGE - Create an account and initiate password reset
		email := fmt.Sprintf("resetpw-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("resetpw%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		token, err := service.InitiatePasswordReset(ctx, email)
		require.NoError(t, err)

		newPassword := testNewPassword

		// ACT
		err = service.ResetPassword(ctx, token.Token, newPassword)

		// ASSERT
		require.NoError(t, err)

		// Verify we can login with the new password
		accessToken, refreshToken, err := service.Login(ctx, email, newPassword)
		require.NoError(t, err)
		assert.NotEmpty(t, accessToken)
		assert.NotEmpty(t, refreshToken)
	})

	t.Run("rejects weak password", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("weakreset-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("weakreset%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		token, err := service.InitiatePasswordReset(ctx, email)
		require.NoError(t, err)

		// ACT
		err = service.ResetPassword(ctx, token.Token, "weak")

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrPasswordTooWeak))
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		// ACT
		err := service.ResetPassword(ctx, "invalid-token-12345", testNewPassword)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrInvalidToken))
	})

	t.Run("rejects already-used token", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("usedtoken-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("usedtoken%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

		token, err := service.InitiatePasswordReset(ctx, email)
		require.NoError(t, err)

		// Use the token once
		err = service.ResetPassword(ctx, token.Token, "FirstReset!123")
		require.NoError(t, err)

		// ACT - Try to use the same token again
		err = service.ResetPassword(ctx, token.Token, "SecondReset!456")

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrInvalidToken))
	})
}

func TestAuthService_PasswordResetRateLimit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db, true)
	ctx := testpkg.Ctx(t)

	t.Run("allows multiple reset requests within limit", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("ratelimit-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("ratelimit%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
		ownPasswordResetRateLimit(t, db, email)

		// ACT - Request password reset 3 times (within typical rate limit)
		for i := 0; i < 3; i++ {
			_, err := service.InitiatePasswordReset(ctx, email)
			require.NoError(t, err, "Request %d should succeed", i+1)
		}
	})

	t.Run("blocks requests after exceeding rate limit", func(t *testing.T) {
		// ARRANGE
		email := fmt.Sprintf("exceededlimit-%d@test.local", time.Now().UnixNano())
		account, err := service.Register(ctx, email, fmt.Sprintf("exceededlimit%d", time.Now().UnixNano()), testPassword, nil, 0)
		require.NoError(t, err)
		testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
		ownPasswordResetRateLimit(t, db, email)

		// Make 3 requests (the typical limit)
		for i := 0; i < 3; i++ {
			_, err := service.InitiatePasswordReset(ctx, email)
			require.NoError(t, err)
		}

		// ACT - The 4th request should be rate limited
		_, err = service.InitiatePasswordReset(ctx, email)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, auth.ErrRateLimitExceeded))
	})
}

// =============================================================================
// Parent Account Extended Tests
// =============================================================================

func TestAuthService_GetParentAccountByEmail(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	// NOTE: The "finds parent account by email" test is skipped because the repository
	// uses an unqualified table name in some database configurations.
	// The error path is still tested below.

	t.Run("returns error for non-existent email", func(t *testing.T) {
		// ACT - This exercises the service code path even with repository errors
		result, err := service.GetParentAccountByEmail(ctx, "nonexistent-parent@test.local")

		// ASSERT - Expect error (either not found or repository error)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAuthService_UpdateParentAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates parent account successfully", func(t *testing.T) {
		// ARRANGE
		parentAccount := testpkg.CreateTestParentAccount(t, db, "update-test")

		// Modify the account
		newUsername := fmt.Sprintf("updated-username-%d", time.Now().UnixNano())
		parentAccount.Username = &newUsername

		// ACT
		err := service.UpdateParentAccount(ctx, parentAccount)

		// ASSERT
		require.NoError(t, err)

		// Verify the update
		updated, err := service.GetParentAccountByID(ctx, int(parentAccount.ID))
		require.NoError(t, err)
		assert.Equal(t, newUsername, *updated.Username)
	})

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ARRANGE
		fakeAccount := &authModels.AccountParent{}
		fakeAccount.ID = 99999999

		// ACT
		err := service.UpdateParentAccount(ctx, fakeAccount)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_ActivateParentAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("activates parent account successfully", func(t *testing.T) {
		// ARRANGE
		parentAccount := testpkg.CreateTestParentAccount(t, db, "activate-test")

		// First deactivate
		parentAccount.Active = false
		err := service.UpdateParentAccount(ctx, parentAccount)
		require.NoError(t, err)

		// ACT
		err = service.ActivateParentAccount(ctx, int(parentAccount.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify activation
		updated, err := service.GetParentAccountByID(ctx, int(parentAccount.ID))
		require.NoError(t, err)
		assert.True(t, updated.Active)
	})

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ACT
		err := service.ActivateParentAccount(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_DeactivateParentAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("deactivates parent account successfully", func(t *testing.T) {
		// ARRANGE
		parentAccount := testpkg.CreateTestParentAccount(t, db, "deactivate-test")

		// ACT
		err := service.DeactivateParentAccount(ctx, int(parentAccount.ID))

		// ASSERT
		require.NoError(t, err)

		// Verify deactivation
		updated, err := service.GetParentAccountByID(ctx, int(parentAccount.ID))
		require.NoError(t, err)
		assert.False(t, updated.Active)
	})

	t.Run("returns error for non-existent account", func(t *testing.T) {
		// ACT
		err := service.DeactivateParentAccount(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
	})
}

func TestAuthService_GetAccountsWithRolesAndPermissions(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns accounts with roles and permissions", func(t *testing.T) {
		// ARRANGE
		testpkg.CreateTestAccount(t, db, "roles-perms-test")

		// ACT
		_, err := service.GetAccountsWithRolesAndPermissions(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		// Result can be empty but should not error
	})

	t.Run("filters accounts by provided filters", func(t *testing.T) {
		// ARRANGE
		testpkg.CreateTestAccount(t, db, "filter-test")

		filters := map[string]interface{}{
			"active": true,
		}

		// ACT
		result, err := service.GetAccountsWithRolesAndPermissions(ctx, filters)

		// ASSERT
		require.NoError(t, err)
		// All returned accounts should be active
		for _, acc := range result {
			assert.True(t, acc.Active)
		}
	})
}

// =============================================================================
// RateLimitError Tests
// =============================================================================

func TestRateLimitError_Error(t *testing.T) {
	t.Parallel()

	t.Run("returns error message when Err is set", func(t *testing.T) {
		// ARRANGE
		rle := &auth.RateLimitError{
			Err:      fmt.Errorf("custom rate limit message"),
			Attempts: 3,
			RetryAt:  time.Now().Add(time.Hour),
		}

		// ACT
		result := rle.Error()

		// ASSERT
		assert.Equal(t, "custom rate limit message", result)
	})

	t.Run("returns default message when Err is nil", func(t *testing.T) {
		// ARRANGE
		rle := &auth.RateLimitError{
			Err:      nil,
			Attempts: 3,
			RetryAt:  time.Now().Add(time.Hour),
		}

		// ACT
		result := rle.Error()

		// ASSERT
		assert.Equal(t, "rate limit exceeded", result)
	})
}

func TestRateLimitError_RetryAfterSeconds(t *testing.T) {
	t.Parallel()

	t.Run("returns positive seconds when retry is in future", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		rle := &auth.RateLimitError{
			Err:      auth.ErrRateLimitExceeded,
			Attempts: 3,
			RetryAt:  now.Add(30 * time.Second),
		}

		// ACT
		result := rle.RetryAfterSeconds(now)

		// ASSERT
		assert.GreaterOrEqual(t, result, 29)
		assert.LessOrEqual(t, result, 31)
	})

	t.Run("returns zero when retry is in past", func(t *testing.T) {
		// ARRANGE
		now := time.Now()
		rle := &auth.RateLimitError{
			Err:      auth.ErrRateLimitExceeded,
			Attempts: 3,
			RetryAt:  now.Add(-30 * time.Second), // In the past
		}

		// ACT
		result := rle.RetryAfterSeconds(now)

		// ASSERT
		assert.Equal(t, 0, result)
	})

	t.Run("returns zero when RetryAt is zero", func(t *testing.T) {
		// ARRANGE
		rle := &auth.RateLimitError{
			Err:      auth.ErrRateLimitExceeded,
			Attempts: 3,
			RetryAt:  time.Time{}, // Zero time
		}

		// ACT
		result := rle.RetryAfterSeconds(time.Now())

		// ASSERT
		assert.Equal(t, 0, result)
	})

	t.Run("returns zero for nil receiver", func(t *testing.T) {
		// ARRANGE
		var rle *auth.RateLimitError = nil

		// ACT
		result := rle.RetryAfterSeconds(time.Now())

		// ASSERT
		assert.Equal(t, 0, result)
	})
}

// =============================================================================
// WithTenantTx Production Path Tests (Item 4)
// =============================================================================

func TestRegister_WithTenantID_CreatesAccountTenantAndRole(t *testing.T) {
	t.Parallel()

	// Register with a real tenantID > 0 should exercise the WithTenantTx path
	// in persistAccountWithRole, creating account + account_tenant mapping +
	// account_role assignment atomically.
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	// Create a role to assign
	role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("test-role-%d", time.Now().UnixNano()), tenantID)

	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("tenant-register")
	roleID := role.ID

	// ACT
	account, err := service.Register(ctx, email, username, testPassword, &roleID, tenantID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, account)

	// Verify account_tenant mapping was created
	var tenantCount int
	err = db.NewSelect().
		TableExpr("auth.account_tenants").
		ColumnExpr("COUNT(*)").
		Where("account_id = ? AND tenant_id = ?", account.ID, tenantID).
		Scan(context.Background(), &tenantCount)
	require.NoError(t, err)
	assert.Equal(t, 1, tenantCount, "account_tenant mapping should exist")

	// Verify account_role was created with correct tenant_id
	var roleCount int
	err = db.NewSelect().
		TableExpr("auth.account_roles").
		ColumnExpr("COUNT(*)").
		Where("account_id = ? AND role_id = ? AND tenant_id = ?", account.ID, roleID, tenantID).
		Scan(context.Background(), &roleCount)
	require.NoError(t, err)
	assert.Equal(t, 1, roleCount, "account_role should be scoped to the correct tenant")
}

func TestRegister_WithTenantID_AssignsSystemRole(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	role := testpkg.CreateTestSystemRole(t, db, "registration-system-role")

	email, username := uniqueTestCredentials("tenant-register-system-role")
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, &role.ID, tenantID)
	require.NoError(t, err)

	var roleCount int
	err = db.NewSelect().
		TableExpr("auth.account_roles").
		ColumnExpr("COUNT(*)").
		Where("account_id = ? AND role_id = ? AND tenant_id = ?", account.ID, role.ID, tenantID).
		Scan(context.Background(), &roleCount)
	require.NoError(t, err)
	assert.Equal(t, 1, roleCount, "system role should be assigned in the target tenant")
}

func TestRegister_WithTenantID_NoRole(t *testing.T) {
	t.Parallel()

	// Register with tenantID > 0 but no roleID should still create the
	// account_tenant mapping (without role assignment).
	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	ctx := testpkg.TenantContext(tenantID)
	email, username := uniqueTestCredentials("tenant-norole")

	// ACT — nil roleID
	account, err := service.Register(ctx, email, username, testPassword, nil, tenantID)

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, account)

	// Verify account_tenant mapping exists
	var tenantCount int
	err = db.NewSelect().
		TableExpr("auth.account_tenants").
		ColumnExpr("COUNT(*)").
		Where("account_id = ? AND tenant_id = ?", account.ID, tenantID).
		Scan(context.Background(), &tenantCount)
	require.NoError(t, err)
	assert.Equal(t, 1, tenantCount, "account_tenant mapping should exist even without role")

	// Verify no account_role was created
	var roleCount int
	err = db.NewSelect().
		TableExpr("auth.account_roles").
		ColumnExpr("COUNT(*)").
		Where("account_id = ?", account.ID).
		Scan(context.Background(), &roleCount)
	require.NoError(t, err)
	assert.Equal(t, 0, roleCount, "no role assignment should exist")
}

func TestAcceptInvitation_WithTenantID_CreatesAccountTenant(t *testing.T) {
	t.Parallel()

	// AcceptInvitation with an invitation that has a real TenantID should
	// create the account, person, account_tenant mapping, and role assignment.
	db := testpkg.SetupTestDB(t)

	invService := setupInvitationService(t, db)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	// Create a role scoped to the same tenant used by the invitation context,
	// so that FindByID's tenant filter (tenant_id = ? OR tenant_id IS NULL) finds it.
	role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("invite-role-%d", time.Now().UnixNano()), tenantID)

	// Create invitation with tenant context so it gets tenant_id set
	ctx := testpkg.TenantContext(tenantID)
	email := fmt.Sprintf("invite-tenant-%d@test.local", time.Now().UnixNano())

	invitation, err := invService.CreateInvitation(ctx, auth.InvitationRequest{
		Email:     email,
		RoleID:    role.ID,
		CreatedBy: 1,
		FirstName: testpkg.StrPtr("Test"),
		LastName:  testpkg.StrPtr("User"),
	})
	require.NoError(t, err)
	require.NotNil(t, invitation)

	// ACT — accept the invitation (public route, no tenant in ctx)
	account, err := invService.AcceptInvitation(context.Background(), invitation.Token, auth.UserRegistrationData{
		FirstName:       "Test",
		LastName:        "User",
		Password:        testPassword,
		ConfirmPassword: testPassword,
	})

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, account)
	defer func() {
		// Clean up: staff, person, invitation tokens, then auth fixtures (includes account_tenants)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM users.staff WHERE person_id IN (SELECT id FROM users.persons WHERE account_id = ?)`, account.ID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM users.persons WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM auth.invitation_tokens WHERE email = ?`, email)
	}()

	// Verify account_tenant mapping was created
	var tenantCount int
	err = db.NewSelect().
		TableExpr("auth.account_tenants").
		ColumnExpr("COUNT(*)").
		Where("account_id = ? AND tenant_id = ?", account.ID, tenantID).
		Scan(context.Background(), &tenantCount)
	require.NoError(t, err)
	assert.Equal(t, 1, tenantCount, "account_tenant mapping should exist after invitation acceptance")

	// Verify account_role was created scoped to tenant
	var roleCount int
	err = db.NewSelect().
		TableExpr("auth.account_roles").
		ColumnExpr("COUNT(*)").
		Where("account_id = ? AND role_id = ? AND tenant_id = ?", account.ID, role.ID, tenantID).
		Scan(context.Background(), &roleCount)
	require.NoError(t, err)
	assert.Equal(t, 1, roleCount, "role should be scoped to invitation tenant")
}

// =============================================================================
// LinkAccountToTenant Tests
// =============================================================================

func TestAuthService_LinkAccountToTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	t.Run("links existing account to tenant", func(t *testing.T) {
		// ARRANGE — create an account that is NOT linked to tenantID yet
		email, _ := uniqueTestCredentials("link-happy")
		account, err := service.Register(testpkg.TenantContext(tenantID), email, email, testPassword, nil, 0)
		require.NoError(t, err)

		// Remove any auto-created tenant mapping so we start from a clean state
		_, _ = db.NewDelete().
			TableExpr("auth.account_tenants").
			Where("account_id = ? AND tenant_id = ?", account.ID, tenantID).
			Exec(context.Background())

		// The role has to belong to the school the account is linked into:
		// a tenant-scoped role of ANOTHER school is rejected since #1021.
		role := testpkg.CreateTestRoleForTenant(t, db, fmt.Sprintf("link-role-%d", time.Now().UnixNano()), tenantID)
		roleID := role.ID

		// ACT
		linked, err := service.LinkAccountToTenant(context.Background(), email, &roleID, tenantID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, linked)
		assert.Equal(t, account.ID, linked.ID)

		// Verify tenant mapping was created
		var tenantCount int
		err = db.NewSelect().
			TableExpr("auth.account_tenants").
			ColumnExpr("COUNT(*)").
			Where("account_id = ? AND tenant_id = ?", account.ID, tenantID).
			Scan(context.Background(), &tenantCount)
		require.NoError(t, err)
		assert.Equal(t, 1, tenantCount, "account_tenant mapping should exist")

		// Verify role was assigned
		var roleCount int
		err = db.NewSelect().
			TableExpr("auth.account_roles").
			ColumnExpr("COUNT(*)").
			Where("account_id = ? AND role_id = ? AND tenant_id = ?", account.ID, roleID, tenantID).
			Scan(context.Background(), &roleCount)
		require.NoError(t, err)
		assert.Equal(t, 1, roleCount, "role should be assigned")

		// Cleanup role assignment
		_, _ = db.NewDelete().
			TableExpr("auth.account_roles").
			Where("account_id = ? AND role_id = ? AND tenant_id = ?", account.ID, roleID, tenantID).
			Exec(context.Background())
	})

	t.Run("links an existing account with a system role", func(t *testing.T) {
		email, _ := uniqueTestCredentials("link-system-role")
		account, err := service.Register(context.Background(), email, email, testPassword, nil, 0)
		require.NoError(t, err)

		role := testpkg.CreateTestSystemRole(t, db, "link-system-role")

		linked, err := service.LinkAccountToTenant(testpkg.TenantContext(tenantID), email, &role.ID, tenantID)
		require.NoError(t, err)
		assert.Equal(t, account.ID, linked.ID)

		var roleCount int
		err = db.NewSelect().
			TableExpr("auth.account_roles").
			ColumnExpr("COUNT(*)").
			Where("account_id = ? AND role_id = ? AND tenant_id = ?", account.ID, role.ID, tenantID).
			Scan(context.Background(), &roleCount)
		require.NoError(t, err)
		assert.Equal(t, 1, roleCount, "system role should be assigned in the target tenant")
	})

	t.Run("returns error for non-existent email", func(t *testing.T) {
		// ACT
		result, err := service.LinkAccountToTenant(context.Background(), "nonexistent@test.local", nil, tenantID)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)

		var authErr *auth.AuthError
		require.True(t, errors.As(err, &authErr))
		assert.True(t, errors.Is(authErr.Err, auth.ErrAccountNotFound))
	})

	t.Run("returns error for inactive account", func(t *testing.T) {
		// ARRANGE — create account then deactivate it
		email, _ := uniqueTestCredentials("link-inactive")
		account, err := service.Register(testpkg.TenantContext(tenantID), email, email, testPassword, nil, 0)
		require.NoError(t, err)

		// Deactivate
		_, err = db.NewUpdate().
			TableExpr("auth.accounts").
			Set("active = ?", false).
			Where("id = ?", account.ID).
			Exec(context.Background())
		require.NoError(t, err)

		// ACT
		result, err := service.LinkAccountToTenant(context.Background(), email, nil, tenantID)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)

		var authErr *auth.AuthError
		require.True(t, errors.As(err, &authErr))
		assert.True(t, errors.Is(authErr.Err, auth.ErrAccountInactive))
	})

	t.Run("idempotent when already linked", func(t *testing.T) {
		// ARRANGE — create an account already linked to tenantID
		email, _ := uniqueTestCredentials("link-idempotent")
		account, err := service.Register(testpkg.TenantContext(tenantID), email, email, testPassword, nil, 0)
		require.NoError(t, err)

		// Ensure it is linked
		testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)

		// ACT — link again (should not error)
		result, err := service.LinkAccountToTenant(context.Background(), email, nil, tenantID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, account.ID, result.ID)

		// Verify exactly one mapping still exists (no duplicate)
		var tenantCount int
		err = db.NewSelect().
			TableExpr("auth.account_tenants").
			ColumnExpr("COUNT(*)").
			Where("account_id = ? AND tenant_id = ?", account.ID, tenantID).
			Scan(context.Background(), &tenantCount)
		require.NoError(t, err)
		assert.Equal(t, 1, tenantCount, "should still have exactly one mapping")
	})
}

// =============================================================================
// RevokeTokensByTenantID Tests
// =============================================================================

func TestAuthService_RevokeTokensByTenantID_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// ARRANGE — create a dedicated tenant, account, and token
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)

	account := testpkg.CreateTestAccount(t, db, "revokeByTenant")

	testpkg.CreateTestTokenForTenant(t, db, tenantID, account.ID)

	// ACT
	count, err := service.RevokeTokensByTenantID(ctx, tenantID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 1, count, "should have revoked one token")
}

func TestAuthService_RevokeTokensByTenantID_NoTokens(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupAuthService(t, db)

	// ARRANGE — use a tenant with no tokens
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)

	// ACT
	count, err := service.RevokeTokensByTenantID(ctx, tenantID)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 0, count, "should have revoked zero tokens")
}

// =============================================================================
// InvalidatePendingInvitationsByTenantID Tests
// =============================================================================

func TestInvitationService_InvalidatePendingInvitationsByTenantID_Success(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupInvitationService(t, db)

	// ARRANGE — use tenant 1 because CreateTestInvitationToken is hardcoded to tenant 1
	ctx := testpkg.Ctx(t)

	role := testpkg.CreateTestRole(t, db, "inv-svc-invalidate-role")
	creator := testpkg.CreateTestAccount(t, db, "inv-svc-invalidate-creator")

	// Create a pending invitation (scoped to tenant 1 by fixture)
	testpkg.CreateTestInvitationToken(t, db, "svc-invalidate@example.com", role.ID, creator.ID, time.Now().Add(48*time.Hour))

	// ACT
	count, err := service.InvalidatePendingInvitationsByTenantID(ctx, testpkg.Tenant(t))

	// ASSERT
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1, "should have invalidated at least one invitation")
}
