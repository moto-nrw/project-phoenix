package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type stubAuthLoginSchoolRepo struct {
	findByIDFn        func(context.Context, int64) (*platformModels.School, error)
	findBySubdomainFn func(context.Context, string) (*platformModels.School, error)
}

func (s stubAuthLoginSchoolRepo) Create(context.Context, *platformModels.School) error {
	panic("unexpected Create")
}
func (s stubAuthLoginSchoolRepo) FindByID(ctx context.Context, id int64) (*platformModels.School, error) {
	return s.findByIDFn(ctx, id)
}
func (s stubAuthLoginSchoolRepo) FindBySlug(context.Context, string) (*platformModels.School, error) {
	panic("unexpected FindBySlug")
}
func (s stubAuthLoginSchoolRepo) FindByOrganizationAndSlug(context.Context, int64, string) (*platformModels.School, error) {
	panic("unexpected FindByOrganizationAndSlug")
}
func (s stubAuthLoginSchoolRepo) FindBySubdomain(ctx context.Context, subdomain string) (*platformModels.School, error) {
	return s.findBySubdomainFn(ctx, subdomain)
}
func (s stubAuthLoginSchoolRepo) List(context.Context) ([]*platformModels.School, error) {
	panic("unexpected List")
}
func (s stubAuthLoginSchoolRepo) ListActive(context.Context) ([]platformModels.School, error) {
	panic("unexpected ListActive")
}
func (s stubAuthLoginSchoolRepo) ListPublic(context.Context) ([]platformModels.School, error) {
	return nil, nil
}
func (s stubAuthLoginSchoolRepo) FindActiveByAccountID(context.Context, int64) ([]platformModels.School, error) {
	panic("unexpected FindActiveByAccountID")
}
func (s stubAuthLoginSchoolRepo) Update(context.Context, *platformModels.School) error {
	panic("unexpected Update")
}
func (s stubAuthLoginSchoolRepo) CountByIDs(context.Context, []int64) (int, error) {
	panic("unexpected CountByIDs")
}
func (s stubAuthLoginSchoolRepo) FindByIDForShare(ctx context.Context, id int64) (*platformModels.School, error) {
	return s.FindByID(ctx, id)
}
func (s stubAuthLoginSchoolRepo) FindByIDForUpdate(ctx context.Context, id int64) (*platformModels.School, error) {
	return s.FindByID(ctx, id)
}
func (r stubAuthLoginSchoolRepo) SoftDelete(context.Context, int64) error { return nil }
func (r stubAuthLoginSchoolRepo) Restore(context.Context, int64) error    { return nil }
func (r stubAuthLoginSchoolRepo) CountNonDeletedByOrganizationID(context.Context, int64) (int, error) {
	return 0, nil
}

type stubAuthLoginAccountTenantRepo struct {
	findActiveFn func(context.Context, int64) ([]authModels.AccountTenant, error)
	existsFn     func(context.Context, int64, int64) (bool, error)
}

func (s stubAuthLoginAccountTenantRepo) Create(context.Context, *authModels.AccountTenant) error {
	panic("unexpected Create")
}
func (s stubAuthLoginAccountTenantRepo) EnsureActive(context.Context, *authModels.AccountTenant) error {
	panic("unexpected EnsureActive")
}
func (s stubAuthLoginAccountTenantRepo) FindActiveByAccountID(ctx context.Context, accountID int64) ([]authModels.AccountTenant, error) {
	return s.findActiveFn(ctx, accountID)
}
func (s stubAuthLoginAccountTenantRepo) ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	return s.existsFn(ctx, accountID, tenantID)
}
func (s stubAuthLoginAccountTenantRepo) ListAccountsByTenantID(context.Context, int64) ([]authModels.TenantAccountInfo, error) {
	panic("unexpected ListAccountsByTenantID")
}
func (s stubAuthLoginAccountTenantRepo) ListAccountsByOrganizationID(context.Context, int64) ([]authModels.OrgAccountInfo, error) {
	panic("unexpected ListAccountsByOrganizationID")
}
func (s stubAuthLoginAccountTenantRepo) ListAllAccounts(context.Context) ([]authModels.OrgAccountInfo, error) {
	panic("unexpected ListAllAccounts")
}

func setupInternalAuthService(t *testing.T, db *bun.DB) *Service {
	t.Helper()
	repoFactory := repositories.NewFactory(db)
	cfg, err := NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	require.NoError(t, err)
	service, err := NewService(repoFactory, cfg, db, slog.Default())
	require.NoError(t, err)
	return service
}

func TestResolveAccountTenantBySlug_ReturnsTenantNotFoundWhenSchoolLookupReturnsNil(t *testing.T) {
	service := &Service{
		repos: &repositories.Factory{
			School: stubAuthLoginSchoolRepo{
				findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) { return nil, nil },
			},
			AccountTenant: stubAuthLoginAccountTenantRepo{
				existsFn: func(context.Context, int64, int64) (bool, error) { return false, nil },
			},
		},
		logger: slog.Default(),
	}

	tenantID, orgID, err := service.resolveAccountTenantBySlug(context.Background(), 5, "missing-school")
	require.Error(t, err)
	assert.Zero(t, tenantID)
	assert.Zero(t, orgID)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// DB errors from FindBySubdomain must propagate as-is, not collapse into ErrTenantNotFound.
// A connection timeout is not "tenant not found", hiding it makes debugging impossible.
func TestResolveAccountTenantBySlug_PropagatesDBErrors(t *testing.T) {
	dbErr := errors.New("connection timed out")
	service := &Service{
		repos: &repositories.Factory{
			School: stubAuthLoginSchoolRepo{
				findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) { return nil, dbErr },
			},
		},
		logger: slog.Default(),
	}

	tenantID, orgID, err := service.resolveAccountTenantBySlug(context.Background(), 5, "some-school")
	require.Error(t, err)
	assert.Zero(t, tenantID)
	assert.Zero(t, orgID)

	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	// The wrapped error must be the original DB error, NOT ErrTenantNotFound.
	assert.ErrorIs(t, authErr.Err, dbErr)
	assert.NotErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// Updated: resolveAccountTenantDefault now iterates all mappings and skips schools
// that can't be looked up (nil). When no valid school is found, returns ErrTenantNotFound
// so the caller gets a clean error instead of proceeding with tenant_id=0 (which would
// hit an FK constraint on auth.tokens).
func TestResolveAccountTenantDefault_ReturnsErrWhenSchoolLookupReturnsNil(t *testing.T) {
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				findActiveFn: func(context.Context, int64) ([]authModels.AccountTenant, error) {
					return []authModels.AccountTenant{{TenantID: 99, Status: authModels.AccountTenantStatusActive}}, nil
				},
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(context.Context, int64) (*platformModels.School, error) { return nil, nil },
			},
		},
		logger: slog.Default(),
	}

	_, _, err := service.resolveAccountTenantDefault(context.Background(), 5)
	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

func TestResolveAccountTenantDefault_SkipsDeletedSchoolAndFallsThrough(t *testing.T) {
	now := time.Now()
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				findActiveFn: func(context.Context, int64) ([]authModels.AccountTenant, error) {
					return []authModels.AccountTenant{
						{TenantID: 900, Status: authModels.AccountTenantStatusActive},
						{TenantID: 901, Status: authModels.AccountTenantStatusActive},
					}, nil
				},
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
					if id == 900 {
						return &platformModels.School{DeletedAt: &now, Active: true, OrganizationID: 10}, nil
					}
					return &platformModels.School{Active: true, OrganizationID: 20}, nil
				},
			},
		},
		logger: slog.Default(),
	}

	tenantID, orgID, err := service.resolveAccountTenantDefault(context.Background(), 5)
	require.NoError(t, err)
	assert.Equal(t, int64(901), tenantID, "should skip deleted school 900 and resolve to school 901")
	assert.Equal(t, int64(20), orgID)
}

func TestResolveAccountTenantDefault_ReturnsErrWhenAllSchoolsDeleted(t *testing.T) {
	now := time.Now()
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				findActiveFn: func(context.Context, int64) ([]authModels.AccountTenant, error) {
					return []authModels.AccountTenant{
						{TenantID: 900, Status: authModels.AccountTenantStatusActive},
					}, nil
				},
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
					return &platformModels.School{DeletedAt: &now, Active: true, OrganizationID: 10}, nil
				},
			},
		},
		logger: slog.Default(),
	}

	_, _, err := service.resolveAccountTenantDefault(context.Background(), 5)
	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

func TestPersistAccountWithRole_CreatesTenantMappingWhenTenantIDProvided(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := setupInternalAuthService(t, db)
	ctx := testpkg.TenantContext(1)
	email := "persist-role-" + time.Now().Format("150405.000000000") + "@test.local"
	passwordHash := "$argon2id$v=19$m=65536,t=3,p=4$persist"
	account := &authModels.Account{
		Model:        modelBase.Model{},
		Email:        email,
		PasswordHash: &passwordHash,
		Active:       true,
	}
	role := testpkg.CreateTestRole(t, db, "PersistRole")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_roles WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.roles WHERE id = ?`, role.ID)
	})

	err := service.persistAccountWithRole(ctx, account, &role.ID, 1)
	require.NoError(t, err)
	assert.NotZero(t, account.ID)

	exists, err := service.repos.AccountTenant.ExistsByAccountAndTenant(ctx, account.ID, 1)
	require.NoError(t, err)
	assert.True(t, exists)
}

// resolveAccountTenantBySlug must treat a soft-deleted school the same as "not found".
// A school with a non-nil DeletedAt must never be resolved as a valid tenant, even if
// the account has an active mapping to it. This prevents login into decommissioned tenants.
func TestResolveAccountTenantBySlug_DeletedSchool_ReturnsTenantNotFound(t *testing.T) {
	now := time.Now()
	service := &Service{
		repos: &repositories.Factory{
			School: stubAuthLoginSchoolRepo{
				findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
					return &platformModels.School{
						DeletedAt:      &now,
						Active:         true,
						OrganizationID: 10,
					}, nil
				},
			},
			AccountTenant: stubAuthLoginAccountTenantRepo{
				existsFn: func(context.Context, int64, int64) (bool, error) { return true, nil },
			},
		},
		logger: slog.Default(),
	}

	tenantID, orgID, err := service.resolveAccountTenantBySlug(context.Background(), 500, "deleted-school")
	require.Error(t, err)
	assert.Zero(t, tenantID)
	assert.Zero(t, orgID)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// resolveAccountTenantBySlug must reject inactive schools (Active == false) even when
// the school is not soft-deleted and the account has a valid tenant mapping.
func TestResolveAccountTenantBySlug_InactiveSchool_ReturnsTenantNotFound(t *testing.T) {
	service := &Service{
		repos: &repositories.Factory{
			School: stubAuthLoginSchoolRepo{
				findBySubdomainFn: func(context.Context, string) (*platformModels.School, error) {
					return &platformModels.School{
						Active:         false,
						OrganizationID: 10,
					}, nil
				},
			},
			AccountTenant: stubAuthLoginAccountTenantRepo{
				existsFn: func(context.Context, int64, int64) (bool, error) { return true, nil },
			},
		},
		logger: slog.Default(),
	}

	tenantID, orgID, err := service.resolveAccountTenantBySlug(context.Background(), 501, "inactive-school")
	require.Error(t, err)
	assert.Zero(t, tenantID)
	assert.Zero(t, orgID)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// ---------------------------------------------------------------------------
// loadAccountMetadataForTenant coverage (DB-based tests)
// ---------------------------------------------------------------------------

// TestLoadAccountMetadataForTenant_WithValidTenant verifies that loading metadata
// for a valid tenant populates the organization ID and returns no error.
func TestLoadAccountMetadataForTenant_WithValidTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 77701
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "loadmeta-valid")
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	service := setupInternalAuthService(t, db)
	ctx := context.Background()

	meta, err := service.loadAccountMetadataForTenant(ctx, account, tenantID)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, tenantID, meta.tenantID)
	// EnsureTestTenant creates org with ID == tenantID
	assert.Equal(t, tenantID, meta.orgID, "orgID should match the organization created by EnsureTestTenant")
}

// TestLoadAccountMetadataForTenant_WithDeletedTenant verifies that a soft-deleted
// school is treated as not found, the refresh flow must reject it with ErrTenantNotFound.
func TestLoadAccountMetadataForTenant_WithDeletedTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	const tenantID int64 = 77702
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "loadmeta-deleted")
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.account_tenants WHERE account_id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.schools WHERE id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM platform.organizations WHERE id = ?`, tenantID)
	})

	// Soft-delete the school.
	_, err := db.ExecContext(context.Background(),
		`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, tenantID)
	require.NoError(t, err)

	service := setupInternalAuthService(t, db)
	ctx := context.Background()

	meta, err := service.loadAccountMetadataForTenant(ctx, account, tenantID)
	require.Error(t, err)
	assert.Nil(t, meta)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// TestLoadAccountMetadataForTenant_WithNonExistentTenant verifies that a tenantID
// pointing to a school that was never created (or hard-deleted) returns ErrTenantNotFound.
func TestLoadAccountMetadataForTenant_WithNonExistentTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "loadmeta-noschool")
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	})

	service := setupInternalAuthService(t, db)
	ctx := context.Background()

	meta, err := service.loadAccountMetadataForTenant(ctx, account, 999999999)
	require.Error(t, err)
	assert.Nil(t, meta)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// TestLoadAccountMetadataForTenant_WithZeroTenant verifies that tenantID=0 is a valid
// edge case (e.g. platform-scoped accounts), the function should succeed and return
// orgID=0 without attempting a school lookup.
func TestLoadAccountMetadataForTenant_WithZeroTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	account := testpkg.CreateTestAccount(t, db, "loadmeta-zero")
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	})

	service := setupInternalAuthService(t, db)
	ctx := context.Background()

	meta, err := service.loadAccountMetadataForTenant(ctx, account, 0)
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, int64(0), meta.tenantID)
	assert.Equal(t, int64(0), meta.orgID, "orgID should be 0 when tenantID is 0")
}

// ---------------------------------------------------------------------------
// resolveAccountTenantDefault additional coverage (stub-based)
// ---------------------------------------------------------------------------

// TestResolveAccountTenantDefault_FindActiveByAccountID_Error verifies that when
// FindActiveByAccountID returns a DB error, it propagates as-is and is NOT masked
// as ErrTenantNotFound.
func TestResolveAccountTenantDefault_FindActiveByAccountID_Error(t *testing.T) {
	dbErr := fmt.Errorf("connection refused")
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				findActiveFn: func(context.Context, int64) ([]authModels.AccountTenant, error) {
					return nil, dbErr
				},
			},
		},
		logger: slog.Default(),
	}

	_, _, err := service.resolveAccountTenantDefault(context.Background(), 42)
	require.Error(t, err)
	assert.ErrorIs(t, err, dbErr, "original DB error must be in the chain")
	// Must NOT be wrapped as ErrTenantNotFound.
	var authErr *AuthError
	if errors.As(err, &authErr) {
		assert.NotErrorIs(t, authErr.Err, ErrTenantNotFound)
	}
}

// TestResolveAccountTenantDefault_SkipsInactiveSchool verifies that a school with
// Active=false is skipped, and the next valid mapping is used.
func TestResolveAccountTenantDefault_SkipsInactiveSchool(t *testing.T) {
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				findActiveFn: func(context.Context, int64) ([]authModels.AccountTenant, error) {
					return []authModels.AccountTenant{
						{TenantID: 800, Status: authModels.AccountTenantStatusActive},
						{TenantID: 801, Status: authModels.AccountTenantStatusActive},
					}, nil
				},
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(_ context.Context, id int64) (*platformModels.School, error) {
					if id == 800 {
						return &platformModels.School{Active: false, OrganizationID: 10}, nil
					}
					return &platformModels.School{Active: true, OrganizationID: 20}, nil
				},
			},
		},
		logger: slog.Default(),
	}

	tenantID, orgID, err := service.resolveAccountTenantDefault(context.Background(), 42)
	require.NoError(t, err)
	assert.Equal(t, int64(801), tenantID, "should skip inactive school 800 and resolve to 801")
	assert.Equal(t, int64(20), orgID)
}

// TestResolveAccountTenantDefault_DBErrorPropagatedWhenAllLookupsFail verifies that
// when FindByID returns an error for every mapping, the last DB error is propagated
// instead of returning ErrTenantNotFound.
func TestResolveAccountTenantDefault_DBErrorPropagatedWhenAllLookupsFail(t *testing.T) {
	dbErr := fmt.Errorf("disk I/O error")
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				findActiveFn: func(context.Context, int64) ([]authModels.AccountTenant, error) {
					return []authModels.AccountTenant{
						{TenantID: 700, Status: authModels.AccountTenantStatusActive},
						{TenantID: 701, Status: authModels.AccountTenantStatusActive},
					}, nil
				},
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
					return nil, dbErr
				},
			},
		},
		logger: slog.Default(),
	}

	_, _, err := service.resolveAccountTenantDefault(context.Background(), 42)
	require.Error(t, err)
	assert.ErrorIs(t, err, dbErr, "DB error must be propagated, not masked")
	// Must NOT be ErrTenantNotFound.
	var authErr *AuthError
	if errors.As(err, &authErr) {
		assert.NotErrorIs(t, authErr.Err, ErrTenantNotFound)
	}
}

// ---------------------------------------------------------------------------
// validateTenantAccess coverage (stub-based)
// ---------------------------------------------------------------------------

// TestValidateTenantAccess_SchoolDeletedReturnsErrTenantNotFound verifies that a
// soft-deleted school (IsDeleted() true) causes validateTenantAccess to return
// ErrTenantNotFound even though the account_tenant mapping exists.
func TestValidateTenantAccess_SchoolDeletedReturnsErrTenantNotFound(t *testing.T) {
	now := time.Now()
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				existsFn: func(context.Context, int64, int64) (bool, error) { return true, nil },
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
					return &platformModels.School{
						DeletedAt:      &now,
						Active:         true,
						OrganizationID: 10,
					}, nil
				},
			},
		},
		logger: slog.Default(),
	}

	claims := &jwt.RefreshClaims{ID: 100, TenantID: 50, Token: "test-token"}
	err := service.validateTenantAccess(context.Background(), claims)
	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// TestValidateTenantAccess_SchoolNotFoundReturnsErrTenantNotFound verifies that when
// FindByID returns sql.ErrNoRows (school hard-deleted or never existed), the function
// returns ErrTenantNotFound for a clean 401 response.
func TestValidateTenantAccess_SchoolNotFoundReturnsErrTenantNotFound(t *testing.T) {
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				existsFn: func(context.Context, int64, int64) (bool, error) { return true, nil },
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
					return nil, sql.ErrNoRows
				},
			},
		},
		logger: slog.Default(),
	}

	claims := &jwt.RefreshClaims{ID: 101, TenantID: 51, Token: "test-token"}
	err := service.validateTenantAccess(context.Background(), claims)
	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// TestValidateTenantAccess_SchoolLookupDBError verifies that a generic DB error
// (not sql.ErrNoRows) from FindByID is propagated so the frontend can retry,
// rather than force-logging out the user.
func TestValidateTenantAccess_SchoolLookupDBError(t *testing.T) {
	dbErr := fmt.Errorf("connection reset by peer")
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				existsFn: func(context.Context, int64, int64) (bool, error) { return true, nil },
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
					return nil, dbErr
				},
			},
		},
		logger: slog.Default(),
	}

	claims := &jwt.RefreshClaims{ID: 102, TenantID: 52, Token: "test-token"}
	err := service.validateTenantAccess(context.Background(), claims)
	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	// The wrapped error must contain the original DB error, not ErrTenantNotFound.
	assert.ErrorIs(t, authErr.Err, dbErr)
	assert.NotErrorIs(t, authErr.Err, ErrTenantNotFound)
}

// TestValidateTenantAccess_SchoolNilReturnsErrTenantNotFound verifies that when
// FindByID returns (nil, nil), a defensive edge case, the function treats it
// the same as a deleted school and returns ErrTenantNotFound.
func TestValidateTenantAccess_SchoolNilReturnsErrTenantNotFound(t *testing.T) {
	service := &Service{
		repos: &repositories.Factory{
			AccountTenant: stubAuthLoginAccountTenantRepo{
				existsFn: func(context.Context, int64, int64) (bool, error) { return true, nil },
			},
			School: stubAuthLoginSchoolRepo{
				findByIDFn: func(context.Context, int64) (*platformModels.School, error) {
					return nil, nil
				},
			},
		},
		logger: slog.Default(),
	}

	claims := &jwt.RefreshClaims{ID: 103, TenantID: 53, Token: "test-token"}
	err := service.validateTenantAccess(context.Background(), claims)
	require.Error(t, err)
	var authErr *AuthError
	require.ErrorAs(t, err, &authErr)
	assert.ErrorIs(t, authErr.Err, ErrTenantNotFound)
}
