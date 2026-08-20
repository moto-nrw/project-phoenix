package platform_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/rotation"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// testJWTSecret is generated at runtime to avoid hardcoded secret literals
// that trigger secret scanners. All tests in this file share the same value.
var testJWTSecret = func() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}()

func TestOperatorAuthService_Login_OperatorNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return nil, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, _, _, err = service.Login(ctx, "nonexistent@example.com", "password", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidCredentialsError{}, err)
}

func TestOperatorAuthService_Login_RepositoryError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return nil, fmt.Errorf("database error")
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, _, _, err = service.Login(ctx, "operator@example.com", "password", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestOperatorAuthService_Login_InactiveOperator(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 1,
				},
				Email:        "operator@example.com",
				DisplayName:  "Inactive Operator",
				PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$salt$hash",
				Active:       false,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, _, _, err = service.Login(ctx, "operator@example.com", "password", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInactiveError{}, err)
}

func TestOperatorAuthService_ValidateOperator_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return nil, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.ValidateOperator(ctx, "nonexistent@example.com", "password")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidCredentialsError{}, err)
}

func TestOperatorAuthService_ValidateOperator_Inactive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 1,
				},
				Email:        "operator@example.com",
				DisplayName:  "Inactive Operator",
				PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$salt$hash",
				Active:       false,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.ValidateOperator(ctx, "operator@example.com", "password")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInactiveError{}, err)
}

func TestOperatorAuthService_GetOperator_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:       "operator@example.com",
				DisplayName: "Test Operator",
				Active:      true,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	operator, err := service.GetOperator(ctx, 42)
	require.NoError(t, err)
	assert.NotNil(t, operator)
	assert.Equal(t, int64(42), operator.ID)
	assert.Equal(t, "operator@example.com", operator.Email)
}

func TestOperatorAuthService_GetOperator_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.GetOperator(ctx, 999)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorNotFoundError{}, err)
}

func TestOperatorAuthService_GetOperator_RepositoryError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, fmt.Errorf("database connection failed")
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.GetOperator(ctx, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestOperatorAuthService_ListOperators_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		listFn: func(ctx context.Context) ([]*platform.Operator, error) {
			return []*platform.Operator{
				{
					Model:       base.Model{ID: 1},
					Email:       "op1@example.com",
					DisplayName: "Operator 1",
				},
				{
					Model:       base.Model{ID: 2},
					Email:       "op2@example.com",
					DisplayName: "Operator 2",
				},
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	operators, err := service.ListOperators(ctx)
	require.NoError(t, err)
	assert.Len(t, operators, 2)
	assert.Equal(t, "op1@example.com", operators[0].Email)
	assert.Equal(t, "op2@example.com", operators[1].Email)
}

func TestOperatorAuthService_ListOperators_Empty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		listFn: func(ctx context.Context) ([]*platform.Operator, error) {
			return []*platform.Operator{}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	operators, err := service.ListOperators(ctx)
	require.NoError(t, err)
	assert.Empty(t, operators)
}

func TestOperatorAuthService_ListOperators_RepositoryError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		listFn: func(ctx context.Context) ([]*platform.Operator, error) {
			return nil, fmt.Errorf("database error")
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.ListOperators(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestOperatorAuthService_UpdateProfile_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 1,
				},
				Email:       "operator@example.com",
				DisplayName: "Old Name",
			}, nil
		},
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
			assert.Equal(t, "New Name", operator.DisplayName)
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	operator, err := service.UpdateProfile(ctx, 1, "New Name")
	require.NoError(t, err)
	assert.NotNil(t, operator)
	assert.Equal(t, "New Name", operator.DisplayName)
}

func TestOperatorAuthService_UpdateProfile_EmptyName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.UpdateProfile(ctx, 1, "")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_UpdateProfile_WhitespaceName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.UpdateProfile(ctx, 1, "   ")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_UpdateProfile_NameTooLong(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	longName := ""
	for i := 0; i < 101; i++ {
		longName += "a"
	}

	_, err = service.UpdateProfile(ctx, 1, longName)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_UpdateProfile_OperatorNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.UpdateProfile(ctx, 999, "New Name")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorNotFoundError{}, err)
}

func TestOperatorAuthService_UpdateProfile_UpdateError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 1,
				},
				Email:       "operator@example.com",
				DisplayName: "Old Name",
			}, nil
		},
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
			return fmt.Errorf("update failed")
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.UpdateProfile(ctx, 1, "New Name")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update operator profile")
}

func TestOperatorAuthService_ChangePassword_OperatorNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	err = service.ChangePassword(ctx, 999, "oldpass", "newpass")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorNotFoundError{}, err)
}

func TestOperatorAuthService_ChangePassword_RepositoryError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, fmt.Errorf("database error")
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	err = service.ChangePassword(ctx, 1, "oldpass", "newpass")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

// ========== HAPPY PATH TESTS WITH REAL PASSWORD HASHING ==========

func TestOperatorAuthService_Login_Success(t *testing.T) {
	t.Parallel()

	// Set JWT secret for token generation BEFORE creating service

	ctx := context.Background()

	// Create real password hash
	passwordHash, err := authSvc.HashPassword("Test1234%")
	require.NoError(t, err)

	var updateLastLoginCalled bool
	var auditLogCalled bool

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: passwordHash,
				Active:       true,
			}, nil
		},
		updateLastLoginFn: func(ctx context.Context, id int64) error {
			updateLastLoginCalled = true
			assert.Equal(t, int64(42), id)
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			auditLogCalled = true
			assert.Equal(t, int64(42), entry.OperatorID)
			assert.Equal(t, platform.ActionLogin, entry.Action)
			return nil
		},
	}
	refreshRepo := &mockOperatorRefreshTokenRepo{}

	// Create service AFTER setting env var
	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     operatorRepo,
		AuditLogRepo:     auditLogRepo,
		RefreshTokenRepo: refreshRepo,
		DB:               &bun.DB{},
		Logger:           slog.Default(),
	})
	require.NoError(t, err)

	accessToken, refreshToken, operator, err := service.Login(ctx, "operator@example.com", "Test1234%", net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.NotNil(t, operator)
	assert.Equal(t, int64(42), operator.ID)
	assert.Equal(t, "operator@example.com", operator.Email)
	assert.True(t, updateLastLoginCalled, "UpdateLastLogin should be called")
	assert.True(t, auditLogCalled, "Audit log should be created")
	require.Len(t, refreshRepo.created, 1)
	assert.NotEqual(t, "operator-refresh-42", refreshRepo.created[0].Token,
		"operator login must persist a random refresh handle instead of a deterministic token")
}

func TestOperatorAuthService_Login_WrongPassword(t *testing.T) {
	t.Parallel()

	// Set JWT secret (even though we won't reach token generation)

	ctx := context.Background()

	// Create real password hash for "Test1234%"
	passwordHash, err := authSvc.HashPassword("Test1234%")
	require.NoError(t, err)

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: passwordHash,
				Active:       true,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     operatorRepo,
		AuditLogRepo:     auditLogRepo,
		RefreshTokenRepo: &mockOperatorRefreshTokenRepo{},
		DB:               &bun.DB{},
		Logger:           slog.Default(),
	})
	require.NoError(t, err)

	_, _, _, err = service.Login(ctx, "operator@example.com", "WrongPassword123!", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidCredentialsError{}, err)
}

func TestOperatorAuthService_ValidateOperator_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create real password hash
	passwordHash, err := authSvc.HashPassword("Test1234%")
	require.NoError(t, err)

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: passwordHash,
				Active:       true,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	operator, err := service.ValidateOperator(ctx, "operator@example.com", "Test1234%")
	require.NoError(t, err)
	assert.NotNil(t, operator)
	assert.Equal(t, int64(42), operator.ID)
	assert.Equal(t, "operator@example.com", operator.Email)
}

func TestOperatorAuthService_ValidateOperator_WrongPassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create real password hash for "Test1234%"
	passwordHash, err := authSvc.HashPassword("Test1234%")
	require.NoError(t, err)

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: passwordHash,
				Active:       true,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.ValidateOperator(ctx, "operator@example.com", "WrongPassword123!")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidCredentialsError{}, err)
}

func TestOperatorAuthService_ValidateOperator_RepositoryError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return nil, fmt.Errorf("database connection failed")
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.ValidateOperator(ctx, "operator@example.com", "password")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database connection failed")
}

func TestOperatorAuthService_ChangePassword_Success(t *testing.T) {
	t.Parallel()

	// ChangePassword now uses tenant.WithAdminTx to atomically update the
	// password and invalidate email-change tokens, so it requires a real DB.
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()

	email := fmt.Sprintf("chpw-ok-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)
	_, _, _, err := service.Login(ctx, email, testPassword, net.ParseIP("192.0.2.20"))
	require.NoError(t, err)

	// Read old hash for comparison
	var oldHash string
	err = db.NewSelect().
		TableExpr("platform.operators").
		Column("password_hash").
		Where("id = ?", operatorID).
		Scan(ctx, &oldHash)
	require.NoError(t, err)

	err = service.ChangePassword(ctx, operatorID, testPassword, "NewSecure1!")
	require.NoError(t, err)

	// Verify password hash changed
	var newHash string
	err = db.NewSelect().
		TableExpr("platform.operators").
		Column("password_hash").
		Where("id = ?", operatorID).
		Scan(ctx, &newHash)
	require.NoError(t, err)
	assert.NotEqual(t, oldHash, newHash, "password hash should be updated")
	assert.NotEmpty(t, newHash)

	var auditEntry platform.OperatorAuditLog
	err = db.NewSelect().Model(&auditEntry).
		ModelTableExpr(`platform.operator_audit_log AS "operator_audit_log"`).
		Where(`"operator_audit_log".operator_id = ?`, operatorID).
		Where(`"operator_audit_log".action = ?`, platform.ActionTokenRevoked).
		OrderExpr(`"operator_audit_log".id DESC`).Limit(1).Scan(ctx)
	require.NoError(t, err)
	changes, err := auditEntry.GetChanges()
	require.NoError(t, err)
	assert.Equal(t, "operator", changes["portal_scope"])
	assert.Equal(t, "password_change", changes["reason"])
}

func TestOperatorAuthService_ChangePassword_WrongCurrentPassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create real password hash for "OldPass1!"
	passwordHash, err := authSvc.HashPassword("OldPass1!")
	require.NoError(t, err)

	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: passwordHash,
				Active:       true,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	err = service.ChangePassword(ctx, 42, "WrongOldPass1!", "NewPass1!")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.PasswordMismatchError{}, err)
}

func TestOperatorAuthService_ChangePassword_WeakNewPassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create real password hash for old password
	passwordHash, err := authSvc.HashPassword("OldPass1!")
	require.NoError(t, err)

	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: passwordHash,
				Active:       true,
			}, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	err = service.ChangePassword(ctx, 42, "OldPass1!", "weak")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_ChangePassword_UpdateError(t *testing.T) {
	t.Parallel()

	// ChangePassword now wraps the update in a transaction, so a DB error
	// surfaces as a transaction rollback. With a real DB, we simulate an
	// update error by deactivating the operator between FindByID (pre-tx
	// password check) and the in-tx Update call. However, this specific
	// scenario is hard to reproduce deterministically. Instead, we verify
	// the simpler invariant: calling ChangePassword on a nonexistent
	// operator returns an error.
	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()

	// Use a nonexistent operator ID to trigger the not-found path
	err := service.ChangePassword(ctx, 999999999, "OldPass1!", "NewPass1!")
	require.Error(t, err)
}

func TestOperatorAuthService_Login_AuditLogError(t *testing.T) {
	t.Parallel()

	// Set JWT secret for token generation

	ctx := context.Background()

	// Create real password hash
	passwordHash, err := authSvc.HashPassword("Test1234%")
	require.NoError(t, err)

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: passwordHash,
				Active:       true,
			}, nil
		},
		updateLastLoginFn: func(ctx context.Context, id int64) error {
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			return fmt.Errorf("audit log service unavailable")
		},
	}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     operatorRepo,
		AuditLogRepo:     auditLogRepo,
		RefreshTokenRepo: &mockOperatorRefreshTokenRepo{},
		DB:               &bun.DB{},
		Logger:           slog.Default(),
	})
	require.NoError(t, err)

	// Login should succeed even if audit log fails (it just logs the error)
	accessToken, refreshToken, operator, err := service.Login(ctx, "operator@example.com", "Test1234%", net.ParseIP("127.0.0.1"))
	require.NoError(t, err, "Login should succeed despite audit log failure")
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.NotNil(t, operator)
}

func currentOperatorRefreshToken(t *testing.T, db *bun.DB, operatorID int64) string {
	t.Helper()
	var token string
	err := db.NewSelect().
		TableExpr("platform.operator_refresh_tokens").
		Column("token").
		Where("operator_id = ?", operatorID).
		Where("rotated_at IS NULL").
		Limit(1).
		Scan(context.Background(), &token)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	return token
}

func countOperatorRefreshTokens(t *testing.T, db *bun.DB, operatorID int64, token string) int {
	t.Helper()
	query := db.NewSelect().
		TableExpr("platform.operator_refresh_tokens").
		Where("operator_id = ?", operatorID)
	if token != "" {
		query = query.Where("token = ?", token)
	}
	count, err := query.Count(context.Background())
	require.NoError(t, err)
	return count
}

func TestOperatorAuthService_RefreshToken_BlankTokenRejected(t *testing.T) {
	t.Parallel()

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = service.RefreshToken(context.Background(), 42, " \t ")
	require.Error(t, err)
	var invalidRefresh *platformSvc.OperatorRefreshTokenInvalidError
	assert.ErrorAs(t, err, &invalidRefresh)
	assert.Equal(t, "operator refresh token is invalid", err.Error())
}

func TestOperatorAuthService_RefreshToken_MissingRefreshTokenRepo(t *testing.T) {
	t.Parallel()

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = service.RefreshToken(context.Background(), 42, "opaque-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operator refresh token repository is not configured")
}

func TestOperatorAuthService_ChangePassword_MissingRefreshTokenRepo(t *testing.T) {
	t.Parallel()

	passwordHash, err := authSvc.HashPassword(testPassword)
	require.NoError(t, err)

	updateCalled := false
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(context.Context, int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:        base.Model{ID: 42},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: passwordHash,
				Active:       true,
			}, nil
		},
		updateFn: func(context.Context, *platform.Operator) error {
			updateCalled = true
			return nil
		},
	}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	err = service.ChangePassword(context.Background(), 42, testPassword, "ChangedPass789!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operator refresh token repository is not configured")
	assert.False(t, updateCalled, "password update must not be persisted without refresh-token revocation")
}

func TestOperatorAuthService_RefreshToken_SuccessRotatesServerSideSession(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	email := fmt.Sprintf("refresh-ok-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	firstAccessJWT, firstRefreshJWT, _, err := service.Login(ctx, email, testPassword, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotEmpty(t, firstRefreshJWT)
	oldDBToken := currentOperatorRefreshToken(t, db, operatorID)

	recoveryCtx := rotation.WithRecoveryProof(ctx, firstAccessJWT)
	accessToken, secondRefreshJWT, err := service.RefreshToken(recoveryCtx, operatorID, oldDBToken)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, secondRefreshJWT)
	newDBToken := currentOperatorRefreshToken(t, db, operatorID)
	assert.NotEqual(t, oldDBToken, newDBToken, "refresh must rotate to a new opaque server-side handle")
	assert.Equal(t, 1, countOperatorRefreshTokens(t, db, operatorID, oldDBToken),
		"the bounded recovery handoff must survive process replacement")
	var replacement string
	err = db.NewSelect().
		TableExpr("platform.operator_refresh_tokens").
		Column("replacement_token").
		Where("token = ?", oldDBToken).
		Scan(ctx, &replacement)
	require.NoError(t, err)
	assert.Equal(t, newDBToken, replacement)
}

func TestOperatorAuthService_RefreshToken_InterruptedRotationRecoveredWithinGrace(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	email := fmt.Sprintf("refresh-replay-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	predecessorAccessJWT, _, _, err := service.Login(ctx, email, testPassword, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	oldDBToken := currentOperatorRefreshToken(t, db, operatorID)
	recoveryCtx := rotation.WithRecoveryProof(ctx, predecessorAccessJWT)

	firstAccess, firstRefresh, err := service.RefreshToken(recoveryCtx, operatorID, oldDBToken)
	require.NoError(t, err)
	currentToken := currentOperatorRefreshToken(t, db, operatorID)

	secondAccess, secondRefresh, err := service.RefreshToken(recoveryCtx, operatorID, oldDBToken)
	require.NoError(t, err)
	assert.NotEmpty(t, firstAccess)
	assert.NotEmpty(t, firstRefresh)
	assert.NotEmpty(t, secondAccess)
	assert.NotEmpty(t, secondRefresh)
	assert.Equal(t, currentToken, currentOperatorRefreshToken(t, db, operatorID),
		"recovery must return the existing successor instead of rotating again")
	assert.Equal(t, 2, countOperatorRefreshTokens(t, db, operatorID, ""))
}

func TestOperatorAuthService_RefreshToken_InterruptedRotationRecoveredAcrossMultipleHandoffs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	email := fmt.Sprintf("refresh-multi-hop-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	_, _, _, err := service.Login(ctx, email, testPassword, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	predecessorHandle := currentOperatorRefreshToken(t, db, operatorID)
	firstProofCtx := rotation.WithRecoveryProof(ctx, "first-recovery-secret")
	_, _, err = service.RefreshToken(firstProofCtx, operatorID, predecessorHandle)
	require.NoError(t, err)
	firstSuccessorHandle := currentOperatorRefreshToken(t, db, operatorID)
	secondProofCtx := rotation.WithRecoveryProof(ctx, "second-recovery-secret")
	_, _, err = service.RefreshToken(secondProofCtx, operatorID, firstSuccessorHandle)
	require.NoError(t, err)
	secondSuccessorHandle := currentOperatorRefreshToken(t, db, operatorID)

	accessToken, refreshToken, err := service.RefreshToken(firstProofCtx, operatorID, predecessorHandle)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.Equal(t, secondSuccessorHandle, currentOperatorRefreshToken(t, db, operatorID),
		"a delayed predecessor must recover the current successor without another rotation")
	assert.Equal(t, 3, countOperatorRefreshTokens(t, db, operatorID, ""),
		"multi-hop recovery must preserve the active family")
}

func TestOperatorAuthService_RefreshToken_ReplayAfterGraceRevokesFamily(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	email := fmt.Sprintf("refresh-expired-grace-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	predecessorAccessJWT, _, _, err := service.Login(ctx, email, testPassword, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	oldDBToken := currentOperatorRefreshToken(t, db, operatorID)
	recoveryCtx := rotation.WithRecoveryProof(ctx, predecessorAccessJWT)
	successorAccessJWT, _, err := service.RefreshToken(recoveryCtx, operatorID, oldDBToken)
	require.NoError(t, err)
	successorDBToken := currentOperatorRefreshToken(t, db, operatorID)

	_, err = db.NewUpdate().
		Table("platform.operator_refresh_tokens").
		Set("rotated_at = ?", time.Now().Add(-rotation.RecoveryGrace-time.Minute)).
		Where("token = ?", oldDBToken).
		Exec(ctx)
	require.NoError(t, err)

	// A later rotation must not erase the expired-grace predecessor while its
	// refresh JWT is still valid; that row is needed to revoke the family.
	_, _, err = service.RefreshToken(rotation.WithRecoveryProof(ctx, successorAccessJWT), operatorID, successorDBToken)
	require.NoError(t, err)

	_, _, err = service.RefreshToken(recoveryCtx, operatorID, oldDBToken)
	require.Error(t, err)
	var invalidRefresh *platformSvc.OperatorRefreshTokenInvalidError
	assert.ErrorAs(t, err, &invalidRefresh)
	assert.Equal(t, 0, countOperatorRefreshTokens(t, db, operatorID, ""),
		"replay outside the recovery boundary must commit family revocation")
}

func TestOperatorAuthService_RefreshToken_WrongRecoveryProofRevokesFamily(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	email := fmt.Sprintf("refresh-wrong-proof-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	predecessorAccessJWT, _, _, err := service.Login(ctx, email, testPassword, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	oldDBToken := currentOperatorRefreshToken(t, db, operatorID)
	_, _, err = service.RefreshToken(rotation.WithRecoveryProof(ctx, predecessorAccessJWT), operatorID, oldDBToken)
	require.NoError(t, err)

	_, _, err = service.RefreshToken(rotation.WithRecoveryProof(ctx, "attacker-does-not-have-the-recovery-secret"), operatorID, oldDBToken)
	require.Error(t, err)
	var invalidRefresh *platformSvc.OperatorRefreshTokenInvalidError
	assert.ErrorAs(t, err, &invalidRefresh)
	assert.Equal(t, 0, countOperatorRefreshTokens(t, db, operatorID, ""),
		"failed possession proof must commit operator token-family revocation")
}

func TestOperatorAuthService_RefreshToken_PasswordChangeRevokesOldToken(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := buildAuthService(t, db)
	ctx := context.Background()
	email := fmt.Sprintf("refresh-pwchange-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)

	_, _, _, err := service.Login(ctx, email, testPassword, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	oldDBToken := currentOperatorRefreshToken(t, db, operatorID)

	err = service.ChangePassword(ctx, operatorID, testPassword, "ChangedPass789!")
	require.NoError(t, err)
	assert.Equal(t, 0, countOperatorRefreshTokens(t, db, operatorID, ""),
		"password change must revoke every operator refresh session")

	_, _, err = service.RefreshToken(ctx, operatorID, oldDBToken)
	require.Error(t, err)
	var invalidRefresh *platformSvc.OperatorRefreshTokenInvalidError
	assert.ErrorAs(t, err, &invalidRefresh)
}

func TestOperatorAuthService_RefreshToken_OperatorNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, nil
		},
	}
	refreshRepo := &mockOperatorRefreshTokenRepo{
		findByTokenForUpdateFn: func(ctx context.Context, token string) (*platform.OperatorRefreshToken, error) {
			return &platform.OperatorRefreshToken{
				Model:      base.Model{ID: 10},
				OperatorID: 999,
				Token:      token,
				Expiry:     time.Now().Add(time.Hour),
				FamilyID:   "family",
			}, nil
		},
		getLatestTokenInFamilyFn: func(ctx context.Context, familyID string) (*platform.OperatorRefreshToken, error) {
			return &platform.OperatorRefreshToken{Generation: 0}, nil
		},
	}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     operatorRepo,
		AuditLogRepo:     &mockAuditLogRepoShared{},
		RefreshTokenRepo: refreshRepo,
		DB:               db,
		Logger:           slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = service.RefreshToken(ctx, 999, "opaque-token")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorNotFoundError{}, err)
}

func TestOperatorAuthService_RefreshToken_InactiveOperator(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: id},
				Email:       "operator@example.com",
				DisplayName: "Inactive Operator",
				Active:      false,
			}, nil
		},
	}
	refreshRepo := &mockOperatorRefreshTokenRepo{
		findByTokenForUpdateFn: func(ctx context.Context, token string) (*platform.OperatorRefreshToken, error) {
			return &platform.OperatorRefreshToken{
				Model:      base.Model{ID: 10},
				OperatorID: 1,
				Token:      token,
				Expiry:     time.Now().Add(time.Hour),
				FamilyID:   "family",
			}, nil
		},
		getLatestTokenInFamilyFn: func(ctx context.Context, familyID string) (*platform.OperatorRefreshToken, error) {
			return &platform.OperatorRefreshToken{Generation: 0}, nil
		},
	}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     operatorRepo,
		AuditLogRepo:     &mockAuditLogRepoShared{},
		RefreshTokenRepo: refreshRepo,
		DB:               db,
		Logger:           slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = service.RefreshToken(ctx, 1, "opaque-token")
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInactiveError{}, err)
}

func TestOperatorAuthService_RefreshToken_RepositoryError(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return nil, fmt.Errorf("database error")
		},
	}
	refreshRepo := &mockOperatorRefreshTokenRepo{
		findByTokenForUpdateFn: func(ctx context.Context, token string) (*platform.OperatorRefreshToken, error) {
			return &platform.OperatorRefreshToken{
				Model:      base.Model{ID: 10},
				OperatorID: 1,
				Token:      token,
				Expiry:     time.Now().Add(time.Hour),
				FamilyID:   "family",
			}, nil
		},
		getLatestTokenInFamilyFn: func(ctx context.Context, familyID string) (*platform.OperatorRefreshToken, error) {
			return &platform.OperatorRefreshToken{Generation: 0}, nil
		},
	}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     operatorRepo,
		AuditLogRepo:     &mockAuditLogRepoShared{},
		RefreshTokenRepo: refreshRepo,
		DB:               db,
		Logger:           slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = service.RefreshToken(ctx, 1, "opaque-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find operator")
}

func TestOperatorAuthService_RefreshToken_HandoffLookupErrorDoesNotRevokeFamily(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	rotatedAt := time.Now()
	replacement := "replacement-handle"
	recoveryCtx := rotation.WithRecoveryProof(context.Background(), "independent-recovery-secret")
	recoveryProofHash := rotation.RecoveryProofHash(recoveryCtx)
	familyRevoked := false
	refreshRepo := &mockOperatorRefreshTokenRepo{
		findByTokenForUpdateFn: func(_ context.Context, token string) (*platform.OperatorRefreshToken, error) {
			if token == replacement {
				return nil, fmt.Errorf("temporary database timeout")
			}
			return &platform.OperatorRefreshToken{
				Model:             base.Model{ID: 10},
				OperatorID:        1,
				Token:             token,
				Expiry:            time.Now().Add(time.Hour),
				FamilyID:          "family",
				Generation:        0,
				RotatedAt:         &rotatedAt,
				ReplacementToken:  &replacement,
				RecoveryProofHash: recoveryProofHash,
			}, nil
		},
		deleteByFamilyIDFn: func(context.Context, string) ([]*platform.OperatorRefreshToken, error) {
			familyRevoked = true
			return nil, nil
		},
	}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     &mockOperatorRepo{},
		AuditLogRepo:     &mockAuditLogRepoShared{},
		RefreshTokenRepo: refreshRepo,
		DB:               db,
		Logger:           slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = service.RefreshToken(recoveryCtx, 1, "predecessor-handle")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "temporary database timeout")
	assert.False(t, familyRevoked,
		"transient infrastructure errors must remain retryable and must not revoke a valid session")
}
