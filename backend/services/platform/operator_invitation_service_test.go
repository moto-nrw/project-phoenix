package platform_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// testStrongInvitationPassword meets the password strength requirements.
// This is NOT a real secret — it's only used with mocked services in tests.
const testStrongInvitationPassword = "Str0ngP@ssword!" //nolint:gosec // Test-only constant, not a real credential

// --- Mock invitation token repository ---

type mockInvitationTokenRepo struct {
	createFn               func(ctx context.Context, token *platform.OperatorInvitationToken) error
	findByIDFn             func(ctx context.Context, id int64) (*platform.OperatorInvitationToken, error)
	findValidByTokenFn     func(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error)
	consumeByTokenFn       func(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error)
	invalidateByEmailFn    func(ctx context.Context, email string) error
	updateDeliveryResultFn func(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	listPendingFn          func(ctx context.Context) ([]*platform.OperatorInvitationToken, error)
	invalidateExpiredFn    func(ctx context.Context) (int, error)
	deleteStaleFn          func(ctx context.Context) (int, error)
}

func (m *mockInvitationTokenRepo) Create(ctx context.Context, token *platform.OperatorInvitationToken) error {
	if m.createFn != nil {
		return m.createFn(ctx, token)
	}
	token.ID = 100
	token.CreatedAt = time.Now()
	token.UpdatedAt = time.Now()
	return nil
}

func (m *mockInvitationTokenRepo) FindByID(ctx context.Context, id int64) (*platform.OperatorInvitationToken, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockInvitationTokenRepo) FindValidByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	if m.findValidByTokenFn != nil {
		return m.findValidByTokenFn(ctx, tokenStr)
	}
	return nil, nil
}

func (m *mockInvitationTokenRepo) ConsumeByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	if m.consumeByTokenFn != nil {
		return m.consumeByTokenFn(ctx, tokenStr)
	}
	return nil, nil
}

func (m *mockInvitationTokenRepo) InvalidateByEmail(ctx context.Context, emailAddr string) error {
	if m.invalidateByEmailFn != nil {
		return m.invalidateByEmailFn(ctx, emailAddr)
	}
	return nil
}

func (m *mockInvitationTokenRepo) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	if m.updateDeliveryResultFn != nil {
		return m.updateDeliveryResultFn(ctx, tokenID, sentAt, emailError, retryCount)
	}
	return nil
}

func (m *mockInvitationTokenRepo) ListPending(ctx context.Context) ([]*platform.OperatorInvitationToken, error) {
	if m.listPendingFn != nil {
		return m.listPendingFn(ctx)
	}
	return nil, nil
}

func (m *mockInvitationTokenRepo) InvalidateExpiredTokens(ctx context.Context) (int, error) {
	if m.invalidateExpiredFn != nil {
		return m.invalidateExpiredFn(ctx)
	}
	return 0, nil
}

func (m *mockInvitationTokenRepo) DeleteStaleTokens(ctx context.Context) (int, error) {
	if m.deleteStaleFn != nil {
		return m.deleteStaleFn(ctx)
	}
	return 0, nil
}

// --- Test helper ---

func newInvitationServiceTestEnv(t *testing.T) (
	platformSvc.OperatorInvitationService,
	*mockInvitationTokenRepo,
	*mockOperatorRepo,
	*mockAuditLogRepoShared,
	sqlmock.Sqlmock,
	func(),
) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := &mockInvitationTokenRepo{}
	operatorRepo := &mockOperatorRepo{}
	auditLogRepo := &mockAuditLogRepoShared{}

	dispatcher := email.NewDispatcher(nil, slog.Default())

	service := platformSvc.NewOperatorInvitationService(platformSvc.OperatorInvitationServiceConfig{
		InvitationRepo:      invitationRepo,
		OperatorRepo:        operatorRepo,
		AuditLogRepo:        auditLogRepo,
		DB:                  bunDB,
		Logger:              slog.Default(),
		Dispatcher:          dispatcher,
		DefaultFrom:         email.NewEmail("moto", "test@example.com"),
		FrontendURL:         "http://localhost:3000",
		OperatorFrontendURL: "http://operator.localhost:3000",
		InvitationExpiry:    48 * time.Hour,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return service, invitationRepo, operatorRepo, auditLogRepo, mock, cleanup
}

// --- CreateInvitation tests ---

func TestCreateInvitation_Success(t *testing.T) {
	service, invitationRepo, operatorRepo, _, mock, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	inviterID := int64(42)
	operatorRepo.findByEmailFn = func(_ context.Context, emailAddr string) (*platform.Operator, error) {
		return nil, nil // no existing operator with that email
	}
	operatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
		if id == inviterID {
			return &platform.Operator{
				Model:       base.Model{ID: inviterID},
				Email:       "inviter@example.com",
				DisplayName: "Inviter",
				Active:      true,
			}, nil
		}
		return nil, nil
	}
	invitationRepo.invalidateByEmailFn = func(_ context.Context, _ string) error {
		return nil
	}
	invitationRepo.createFn = func(_ context.Context, token *platform.OperatorInvitationToken) error {
		token.ID = 100
		token.CreatedAt = time.Now()
		token.UpdatedAt = time.Now()
		return nil
	}

	// WithAdminTx opens a real transaction — mock expects BEGIN + SET LOCAL ROLE + COMMIT
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	token, err := service.CreateInvitation(context.Background(), "new@example.com", "New Operator", inviterID)
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, "new@example.com", token.Email)
	assert.NotEmpty(t, token.Token)
}

func TestCreateInvitation_InvalidEmail(t *testing.T) {
	service, _, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	_, err := service.CreateInvitation(context.Background(), "not-an-email", "", int64(42))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestCreateInvitation_EmailAlreadyRegistered(t *testing.T) {
	service, _, operatorRepo, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	operatorRepo.findByEmailFn = func(_ context.Context, _ string) (*platform.Operator, error) {
		return &platform.Operator{
			Model: base.Model{ID: int64(99)},
			Email: "existing@example.com",
		}, nil
	}

	_, err := service.CreateInvitation(context.Background(), "existing@example.com", "", int64(42))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.EmailAlreadyInUseError{}, err)
}

func TestCreateInvitation_InviterNotFound(t *testing.T) {
	service, _, operatorRepo, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	operatorRepo.findByEmailFn = func(_ context.Context, _ string) (*platform.Operator, error) {
		return nil, nil
	}
	operatorRepo.findByIDFn = func(_ context.Context, _ int64) (*platform.Operator, error) {
		return nil, nil
	}

	_, err := service.CreateInvitation(context.Background(), "new@example.com", "", int64(999))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorNotFoundError{}, err)
}

// --- ValidateInvitation tests ---

func TestValidateInvitation_Success(t *testing.T) {
	service, invitationRepo, _, _, mock, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findValidByTokenFn = func(_ context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
		if tokenStr == "valid-token" {
			return &platform.OperatorInvitationToken{
				Model:     base.Model{ID: int64(10)},
				Email:     "invited@example.com",
				Token:     "valid-token",
				Expiry:    time.Now().Add(24 * time.Hour),
				InvitedBy: int64(42),
				Inviter: &platform.Operator{
					Model:       base.Model{ID: int64(42)},
					DisplayName: "Admin",
				},
			}, nil
		}
		return nil, nil
	}

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result, err := service.ValidateInvitation(context.Background(), "valid-token")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "invited@example.com", result.Email)
	assert.Equal(t, "Admin", result.InviterName)
}

func TestValidateInvitation_EmptyToken(t *testing.T) {
	service, _, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	_, err := service.ValidateInvitation(context.Background(), "")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationExpiredOrUsedError{}, err)
}

func TestValidateInvitation_TokenNotFound(t *testing.T) {
	service, invitationRepo, _, _, mock, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findValidByTokenFn = func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
		return nil, nil
	}

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := service.ValidateInvitation(context.Background(), "nonexistent-token")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationExpiredOrUsedError{}, err)
}

// --- AcceptInvitation tests ---

func TestAcceptInvitation_Success(t *testing.T) {
	service, invitationRepo, operatorRepo, _, mock, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findValidByTokenFn = func(_ context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
		if tokenStr == "valid-token" {
			return &platform.OperatorInvitationToken{
				Model:     base.Model{ID: int64(10)},
				Email:     "new@example.com",
				Token:     "valid-token",
				Expiry:    time.Now().Add(24 * time.Hour),
				InvitedBy: int64(42),
			}, nil
		}
		return nil, nil
	}
	invitationRepo.consumeByTokenFn = func(_ context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
		if tokenStr == "valid-token" {
			return &platform.OperatorInvitationToken{
				Model:     base.Model{ID: int64(10)},
				Email:     "new@example.com",
				Token:     "valid-token",
				Expiry:    time.Now().Add(24 * time.Hour),
				InvitedBy: int64(42),
			}, nil
		}
		return nil, nil
	}
	operatorRepo.findByEmailFn = func(_ context.Context, _ string) (*platform.Operator, error) {
		return nil, nil // no existing operator
	}

	// The mockOperatorRepo.Create always returns nil, which is fine — we just check the result

	// Preflight admin tx (FindValidByToken)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	// Consume admin tx (ConsumeByToken + CreateOperator)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	result, err := service.AcceptInvitation(context.Background(), "valid-token", "New Operator", testStrongInvitationPassword, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "new@example.com", result.Email)
	assert.Equal(t, "New Operator", result.DisplayName)
	assert.True(t, result.Active)
}

func TestAcceptInvitation_EmptyToken(t *testing.T) {
	service, _, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	_, err := service.AcceptInvitation(context.Background(), "", "Name", testStrongInvitationPassword, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationExpiredOrUsedError{}, err)
}

func TestAcceptInvitation_WeakPassword(t *testing.T) {
	service, _, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	_, err := service.AcceptInvitation(context.Background(), "valid-token", "Name", "weak", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestAcceptInvitation_EmptyDisplayName(t *testing.T) {
	service, _, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	_, err := service.AcceptInvitation(context.Background(), "valid-token", "", testStrongInvitationPassword, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestAcceptInvitation_DisplayNameTooLong(t *testing.T) {
	service, _, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	longName := ""
	for range 101 {
		longName += "a"
	}

	_, err := service.AcceptInvitation(context.Background(), "valid-token", longName, testStrongInvitationPassword, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestAcceptInvitation_TokenExpiredOrUsed(t *testing.T) {
	service, invitationRepo, _, _, mock, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findValidByTokenFn = func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
		return nil, nil // token not found / expired / used
	}

	// Preflight admin tx — FindValidByToken returns nil, so accept aborts before consume tx
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	_, err := service.AcceptInvitation(context.Background(), "expired-token", "Name", testStrongInvitationPassword, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationExpiredOrUsedError{}, err)
}

func TestAcceptInvitation_EmailAlreadyRegistered(t *testing.T) {
	service, invitationRepo, operatorRepo, _, mock, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findValidByTokenFn = func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
		return &platform.OperatorInvitationToken{
			Model:     base.Model{ID: int64(10)},
			Email:     "existing@example.com",
			Token:     "valid-token",
			Expiry:    time.Now().Add(24 * time.Hour),
			InvitedBy: int64(42),
		}, nil
	}
	invitationRepo.consumeByTokenFn = func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
		return &platform.OperatorInvitationToken{
			Model:     base.Model{ID: int64(10)},
			Email:     "existing@example.com",
			Token:     "valid-token",
			Expiry:    time.Now().Add(24 * time.Hour),
			InvitedBy: int64(42),
		}, nil
	}
	operatorRepo.findByEmailFn = func(_ context.Context, _ string) (*platform.Operator, error) {
		return &platform.Operator{
			Model: base.Model{ID: int64(99)},
			Email: "existing@example.com",
		}, nil
	}

	// Preflight admin tx (FindValidByToken)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	// Consume admin tx (ConsumeByToken + FindByEmail → EmailAlreadyInUseError → rollback)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := service.AcceptInvitation(context.Background(), "valid-token", "Name", testStrongInvitationPassword, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.EmailAlreadyInUseError{}, err)
}

// --- ResendInvitation tests ---

func TestResendInvitation_Success(t *testing.T) {
	service, invitationRepo, operatorRepo, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findByIDFn = func(_ context.Context, id int64) (*platform.OperatorInvitationToken, error) {
		return &platform.OperatorInvitationToken{
			Model:     base.Model{ID: id},
			Email:     "invited@example.com",
			Token:     "some-token",
			Expiry:    time.Now().Add(24 * time.Hour),
			Used:      false,
			InvitedBy: int64(42),
		}, nil
	}
	operatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
		return &platform.Operator{
			Model:       base.Model{ID: id},
			DisplayName: "Inviter",
		}, nil
	}

	err := service.ResendInvitation(context.Background(), int64(10), int64(42))
	require.NoError(t, err)
}

func TestResendInvitation_NotFound(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findByIDFn = func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
		return nil, nil
	}

	err := service.ResendInvitation(context.Background(), int64(999), int64(42))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationNotFoundError{}, err)
}

func TestResendInvitation_AlreadyAccepted(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findByIDFn = func(_ context.Context, id int64) (*platform.OperatorInvitationToken, error) {
		return &platform.OperatorInvitationToken{
			Model:     base.Model{ID: id},
			Email:     "invited@example.com",
			Token:     "some-token",
			Expiry:    time.Now().Add(24 * time.Hour),
			Used:      true,
			InvitedBy: int64(42),
		}, nil
	}

	err := service.ResendInvitation(context.Background(), int64(10), int64(42))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationAlreadyAcceptedError{}, err)
}

func TestResendInvitation_Expired(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findByIDFn = func(_ context.Context, id int64) (*platform.OperatorInvitationToken, error) {
		return &platform.OperatorInvitationToken{
			Model:     base.Model{ID: id},
			Email:     "invited@example.com",
			Token:     "some-token",
			Expiry:    time.Now().Add(-1 * time.Hour), // expired
			Used:      false,
			InvitedBy: int64(42),
		}, nil
	}

	err := service.ResendInvitation(context.Background(), int64(10), int64(42))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationExpiredOrUsedError{}, err)
}

// --- RevokeInvitation tests ---

func TestRevokeInvitation_Success(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findByIDFn = func(_ context.Context, id int64) (*platform.OperatorInvitationToken, error) {
		return &platform.OperatorInvitationToken{
			Model:     base.Model{ID: id},
			Email:     "invited@example.com",
			Token:     "some-token",
			Expiry:    time.Now().Add(24 * time.Hour),
			Used:      false,
			InvitedBy: int64(42),
		}, nil
	}

	err := service.RevokeInvitation(context.Background(), int64(10), int64(42))
	require.NoError(t, err)
}

func TestRevokeInvitation_NotFound(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findByIDFn = func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
		return nil, nil
	}

	err := service.RevokeInvitation(context.Background(), int64(999), int64(42))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationNotFoundError{}, err)
}

func TestRevokeInvitation_AlreadyAccepted(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.findByIDFn = func(_ context.Context, id int64) (*platform.OperatorInvitationToken, error) {
		return &platform.OperatorInvitationToken{
			Model:     base.Model{ID: id},
			Email:     "invited@example.com",
			Token:     "some-token",
			Expiry:    time.Now().Add(24 * time.Hour),
			Used:      true,
			InvitedBy: int64(42),
		}, nil
	}

	err := service.RevokeInvitation(context.Background(), int64(10), int64(42))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInvitationAlreadyAcceptedError{}, err)
}

// --- CleanupExpiredOperatorInvitations tests ---

func TestCleanupExpiredOperatorInvitations_Success(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.invalidateExpiredFn = func(_ context.Context) (int, error) {
		return 3, nil
	}
	invitationRepo.deleteStaleFn = func(_ context.Context) (int, error) {
		return 2, nil
	}

	total, err := service.CleanupExpiredOperatorInvitations(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, total)
}

func TestCleanupExpiredOperatorInvitations_InvalidateError(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.invalidateExpiredFn = func(_ context.Context) (int, error) {
		return 0, fmt.Errorf("database error")
	}

	_, err := service.CleanupExpiredOperatorInvitations(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalidate expired")
}

func TestCleanupExpiredOperatorInvitations_DeleteStaleError(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.invalidateExpiredFn = func(_ context.Context) (int, error) {
		return 2, nil
	}
	invitationRepo.deleteStaleFn = func(_ context.Context) (int, error) {
		return 0, fmt.Errorf("database error")
	}

	count, err := service.CleanupExpiredOperatorInvitations(context.Background())
	require.Error(t, err)
	assert.Equal(t, 2, count) // invalidated count is still returned
	assert.Contains(t, err.Error(), "delete stale")
}

// --- ListPendingInvitations tests ---

func TestListPendingInvitations_Success(t *testing.T) {
	service, invitationRepo, _, _, _, cleanup := newInvitationServiceTestEnv(t)
	defer cleanup()

	invitationRepo.listPendingFn = func(_ context.Context) ([]*platform.OperatorInvitationToken, error) {
		return []*platform.OperatorInvitationToken{
			{
				Model: base.Model{ID: int64(10)},
				Email: "a@example.com",
			},
			{
				Model: base.Model{ID: int64(11)},
				Email: "b@example.com",
			},
		}, nil
	}

	result, err := service.ListPendingInvitations(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 2)
}
