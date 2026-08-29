package auth

import (
	"context"
	"log/slog"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type staffPINAccountRepository struct {
	*stubAccountRepository
	failedAttempts int
	resets         int
}

func (r *staffPINAccountRepository) FindByIDForUpdate(
	ctx context.Context,
	id int64,
) (*authModel.Account, error) {
	return r.FindByID(ctx, id)
}

func (r *staffPINAccountRepository) IncrementPINAttempts(
	_ context.Context,
	_ int64,
	_ int,
	_ time.Duration,
) (authModel.PINAttemptResult, error) {
	r.failedAttempts++
	return authModel.PINAttemptResult{}, nil
}

func (r *staffPINAccountRepository) ResetPINAttempts(context.Context, int64) error {
	r.resets++
	return nil
}

func newStaffPINAuthService(
	t *testing.T,
	account *authModel.Account,
	mappingStatus string,
) (*Service, *staffPINAccountRepository, sqlmock.Sqlmock, func()) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	accountRepo := &staffPINAccountRepository{
		stubAccountRepository: newStubAccountRepository(account),
	}
	accountTenantRepo := newStubAccountTenantRepository()
	require.NoError(t, accountTenantRepo.Create(context.Background(), &authModel.AccountTenant{
		AccountID: account.ID,
		TenantID:  7,
		Status:    mappingStatus,
	}))
	personRepo := newStubPersonRepository()
	accountID := account.ID
	require.NoError(t, personRepo.Create(context.Background(), &userModel.Person{
		Model:       base.Model{ID: 20},
		TenantModel: base.TenantModel{TenantID: 7},
		AccountID:   &accountID,
	}))

	staff := &userModel.Staff{
		Model:       base.Model{ID: 42},
		TenantModel: base.TenantModel{TenantID: 7},
		PersonID:    20,
	}
	staffRepo := &testpkg.StaffRepoMock{
		FindByIDFn: func(_ context.Context, id any) (*userModel.Staff, error) {
			if id == int64(42) {
				return staff, nil
			}
			return nil, nil
		},
	}

	service := &Service{
		repos: &repositories.Factory{
			Account:       accountRepo,
			AccountTenant: accountTenantRepo,
			Person:        personRepo,
			Staff:         staffRepo,
		},
		db:            bunDB,
		logger:        slog.New(slog.DiscardHandler),
		tenantRuntime: newMockTenantRuntime(t, bunDB),
	}
	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	return service, accountRepo, mock, cleanup
}

func expectStaffPINAuthTenantTx(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_tenant").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SELECT set_config").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func TestAuthenticateStaffPINAcceptsMatchingAccountPIN(t *testing.T) {
	t.Parallel()

	account := &authModel.Account{
		Model:  base.Model{ID: 10},
		Active: true,
	}
	require.NoError(t, account.HashPIN("2468"))
	service, accountRepo, mock, cleanup := newStaffPINAuthService(t, account, authModel.AccountTenantStatusActive)
	defer cleanup()
	expectStaffPINAuthTenantTx(mock)

	staff, err := service.AuthenticateStaffPIN(context.Background(), 7, 42, "2468")

	require.NoError(t, err)
	require.NotNil(t, staff)
	assert.Equal(t, int64(42), staff.ID)
	assert.Zero(t, accountRepo.failedAttempts)
	assert.Equal(t, 1, accountRepo.resets)
}

func TestAuthenticateStaffPINCommitsFailedAttemptForWrongPIN(t *testing.T) {
	t.Parallel()

	account := &authModel.Account{
		Model:  base.Model{ID: 10},
		Active: true,
	}
	require.NoError(t, account.HashPIN("2468"))
	service, accountRepo, mock, cleanup := newStaffPINAuthService(t, account, authModel.AccountTenantStatusActive)
	defer cleanup()
	expectStaffPINAuthTenantTx(mock)

	staff, err := service.AuthenticateStaffPIN(context.Background(), 7, 42, "9999")

	assert.Nil(t, staff)
	assert.ErrorIs(t, err, ErrInvalidStaffPINCredentials)
	assert.Equal(t, 1, accountRepo.failedAttempts)
	assert.Zero(t, accountRepo.resets)
}

func TestAuthenticateStaffPINRejectsLockedAccount(t *testing.T) {
	t.Parallel()

	lockedUntil := time.Now().Add(time.Hour)
	account := &authModel.Account{
		Model:          base.Model{ID: 10},
		Active:         true,
		PINLockedUntil: &lockedUntil,
	}
	require.NoError(t, account.HashPIN("2468"))
	service, accountRepo, mock, cleanup := newStaffPINAuthService(t, account, authModel.AccountTenantStatusActive)
	defer cleanup()
	expectStaffPINAuthTenantTx(mock)

	staff, err := service.AuthenticateStaffPIN(context.Background(), 7, 42, "2468")

	assert.Nil(t, staff)
	assert.ErrorIs(t, err, ErrStaffPINLocked)
	assert.Zero(t, accountRepo.failedAttempts)
	assert.Zero(t, accountRepo.resets)
}

func TestAuthenticateStaffPINRejectsNonActiveTenantMapping(t *testing.T) {
	t.Parallel()

	for _, status := range []string{
		authModel.AccountTenantStatusPending,
		authModel.AccountTenantStatusInactive,
	} {
		t.Run(status, func(t *testing.T) {
			account := &authModel.Account{
				Model:  base.Model{ID: 10},
				Active: true,
			}
			require.NoError(t, account.HashPIN("2468"))
			service, accountRepo, mock, cleanup := newStaffPINAuthService(t, account, status)
			defer cleanup()
			expectStaffPINAuthTenantTx(mock)

			staff, err := service.AuthenticateStaffPIN(context.Background(), 7, 42, "2468")

			assert.Nil(t, staff)
			assert.ErrorIs(t, err, ErrInvalidStaffPINCredentials)
			assert.Zero(t, accountRepo.failedAttempts)
			assert.Zero(t, accountRepo.resets)
		})
	}
}
