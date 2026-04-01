package platform_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestOperatorAuthService_Login_OperatorNotFound(t *testing.T) {
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

	operator, err := service.UpdateProfile(ctx, 1, "New Name", "operator@example.com", nil)
	require.NoError(t, err)
	assert.NotNil(t, operator)
	assert.Equal(t, "New Name", operator.DisplayName)
}

func TestOperatorAuthService_UpdateProfile_EmptyName(t *testing.T) {
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

	_, err = service.UpdateProfile(ctx, 1, "", "operator@example.com", nil)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_UpdateProfile_WhitespaceName(t *testing.T) {
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

	_, err = service.UpdateProfile(ctx, 1, "   ", "operator@example.com", nil)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_UpdateProfile_NameTooLong(t *testing.T) {
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

	_, err = service.UpdateProfile(ctx, 1, longName, "operator@example.com", nil)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_UpdateProfile_OperatorNotFound(t *testing.T) {
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

	_, err = service.UpdateProfile(ctx, 999, "New Name", "operator@example.com", nil)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorNotFoundError{}, err)
}

func TestOperatorAuthService_UpdateProfile_UpdateError(t *testing.T) {
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

	_, err = service.UpdateProfile(ctx, 1, "New Name", "operator@example.com", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update operator profile")
}

func TestOperatorAuthService_UpdateProfile_EmailChange_Success(t *testing.T) {
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: 1},
				Email:       "old@example.com",
				DisplayName: "Operator",
			}, nil
		},
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return nil, nil // no conflict
		},
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
			assert.Equal(t, "new@example.com", operator.Email)
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

	operator, err := service.UpdateProfile(ctx, 1, "Operator", "new@example.com", nil)
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", operator.Email)
}

func TestOperatorAuthService_UpdateProfile_EmailConflict(t *testing.T) {
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: 1},
				Email:       "old@example.com",
				DisplayName: "Operator",
			}, nil
		},
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{ID: 2},
				Email: "taken@example.com",
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

	_, err = service.UpdateProfile(ctx, 1, "Operator", "taken@example.com", nil)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.ConflictError{}, err)
}

func TestOperatorAuthService_UpdateProfile_InvalidEmailFormat(t *testing.T) {
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

	_, err = service.UpdateProfile(ctx, 1, "Name", "not-an-email", nil)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_UpdateProfile_EmptyEmail_KeepsCurrent(t *testing.T) {
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: 1},
				Email:       "existing@example.com",
				DisplayName: "Old Name",
			}, nil
		},
		updateFn: func(ctx context.Context, op *platform.Operator) error {
			assert.Equal(t, "existing@example.com", op.Email, "email should remain unchanged")
			assert.Equal(t, "New Name", op.DisplayName)
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

	result, err := service.UpdateProfile(ctx, 1, "New Name", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "existing@example.com", result.Email)
}

func TestOperatorAuthService_UpdateProfile_SameEmail_NoConflictCheck(t *testing.T) {
	ctx := context.Background()
	findByEmailCalled := false
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: 1},
				Email:       "same@example.com",
				DisplayName: "Old Name",
			}, nil
		},
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			findByEmailCalled = true
			return nil, nil
		},
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
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

	operator, err := service.UpdateProfile(ctx, 1, "New Name", "same@example.com", nil)
	require.NoError(t, err)
	assert.Equal(t, "New Name", operator.DisplayName)
	assert.False(t, findByEmailCalled, "FindByEmail should not be called when email is unchanged")
}

func TestOperatorAuthService_UpdateProfile_EmailDisplayNameStripped(t *testing.T) {
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: 1},
				Email:       "old@example.com",
				DisplayName: "Operator",
			}, nil
		},
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			assert.Equal(t, "new@example.com", email, "display name should be stripped from email")
			return nil, nil
		},
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
			assert.Equal(t, "new@example.com", operator.Email, "stored email should not contain display name")
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

	// mail.ParseAddress accepts "Name <addr>" — we must extract .Address
	operator, err := service.UpdateProfile(ctx, 1, "Operator", "Admin <new@example.com>", nil)
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", operator.Email)
}

func TestOperatorAuthService_UpdateProfile_EmailTooLong(t *testing.T) {
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

	longLocal := ""
	for i := 0; i < 250; i++ {
		longLocal += "a"
	}
	longEmail := longLocal + "@example.com"

	_, err = service.UpdateProfile(ctx, 1, "Name", longEmail, nil)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidDataError{}, err)
}

func TestOperatorAuthService_UpdateProfile_FindByEmailError(t *testing.T) {
	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: 1},
				Email:       "old@example.com",
				DisplayName: "Operator",
			}, nil
		},
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return nil, fmt.Errorf("database connection error")
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

	_, err = service.UpdateProfile(ctx, 1, "Operator", "new@example.com", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check email uniqueness")
}

func TestOperatorAuthService_UpdateProfile_EmailChange_AuditLog(t *testing.T) {
	ctx := context.Background()
	auditLogCalled := false
	var auditEntry *platform.OperatorAuditLog

	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: 1},
				Email:       "old@example.com",
				DisplayName: "Operator",
			}, nil
		},
		findByEmailFn: func(ctx context.Context, email string) (*platform.Operator, error) {
			return nil, nil
		},
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			auditLogCalled = true
			auditEntry = entry
			return nil
		},
	}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	clientIP := net.ParseIP("192.168.1.1")
	_, err = service.UpdateProfile(ctx, 1, "Operator", "new@example.com", clientIP)
	require.NoError(t, err)

	assert.True(t, auditLogCalled, "audit log should be created for email change")
	assert.Equal(t, int64(1), auditEntry.OperatorID)
	assert.Equal(t, platform.ActionUpdate, auditEntry.Action)
	assert.Equal(t, platform.ResourceOperator, auditEntry.ResourceType)
	assert.Equal(t, clientIP, auditEntry.RequestIP)
	assert.Contains(t, string(auditEntry.Changes), "old@example.com")
	assert.Contains(t, string(auditEntry.Changes), "new@example.com")
}

func TestOperatorAuthService_UpdateProfile_NoAuditLog_WhenEmailUnchanged(t *testing.T) {
	ctx := context.Background()
	auditLogCalled := false

	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model:       base.Model{ID: 1},
				Email:       "same@example.com",
				DisplayName: "Old Name",
			}, nil
		},
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
			return nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(ctx context.Context, entry *platform.OperatorAuditLog) error {
			auditLogCalled = true
			return nil
		},
	}

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, err = service.UpdateProfile(ctx, 1, "New Name", "same@example.com", nil)
	require.NoError(t, err)
	assert.False(t, auditLogCalled, "audit log should not be created when email is unchanged")
}

func TestOperatorAuthService_ChangePassword_OperatorNotFound(t *testing.T) {
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
	// Set JWT secret for token generation BEFORE creating service
	oldSecret := viper.GetString("auth_jwt_secret")
	viper.Set("auth_jwt_secret", "test-secret-key-for-jwt-tokens-that-is-long-enough")
	defer viper.Set("auth_jwt_secret", oldSecret)

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

	// Create service AFTER setting env var
	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
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
}

func TestOperatorAuthService_Login_WrongPassword(t *testing.T) {
	// Set JWT secret (even though we won't reach token generation)
	oldSecret := viper.GetString("auth_jwt_secret")
	viper.Set("auth_jwt_secret", "test-secret-key-for-jwt-tokens-that-is-long-enough")
	defer viper.Set("auth_jwt_secret", oldSecret)

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

	_, _, _, err = service.Login(ctx, "operator@example.com", "WrongPassword123!", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.IsType(t, &platformSvc.InvalidCredentialsError{}, err)
}

func TestOperatorAuthService_ValidateOperator_Success(t *testing.T) {
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
	ctx := context.Background()

	// Create real password hash for old password
	oldPasswordHash, err := authSvc.HashPassword("OldPass1!")
	require.NoError(t, err)

	var capturedNewHash string
	var updateCalled bool

	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 42,
				},
				Email:        "operator@example.com",
				DisplayName:  "Test Operator",
				PasswordHash: oldPasswordHash,
				Active:       true,
			}, nil
		},
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
			updateCalled = true
			capturedNewHash = operator.PasswordHash
			assert.NotEqual(t, oldPasswordHash, operator.PasswordHash, "New hash should differ from old hash")
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

	err = service.ChangePassword(ctx, 42, "OldPass1!", "NewPass1!")
	require.NoError(t, err)
	assert.True(t, updateCalled, "Update should be called")
	assert.NotEmpty(t, capturedNewHash, "New password hash should be set")
}

func TestOperatorAuthService_ChangePassword_WrongCurrentPassword(t *testing.T) {
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
	ctx := context.Background()

	// Create real password hash
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
		updateFn: func(ctx context.Context, operator *platform.Operator) error {
			return fmt.Errorf("database update failed")
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

	err = service.ChangePassword(ctx, 42, "OldPass1!", "NewPass1!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update password")
}

func TestOperatorAuthService_Login_AuditLogError(t *testing.T) {
	// Set JWT secret for token generation
	oldSecret := viper.GetString("auth_jwt_secret")
	viper.Set("auth_jwt_secret", "test-secret-key-for-jwt-tokens-that-is-long-enough")
	defer viper.Set("auth_jwt_secret", oldSecret)

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
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditLogRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	// Login should succeed even if audit log fails (it just logs the error)
	accessToken, refreshToken, operator, err := service.Login(ctx, "operator@example.com", "Test1234%", net.ParseIP("127.0.0.1"))
	require.NoError(t, err, "Login should succeed despite audit log failure")
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.NotNil(t, operator)
}

func TestOperatorAuthService_RefreshToken_Success(t *testing.T) {
	oldSecret := viper.GetString("auth_jwt_secret")
	viper.Set("auth_jwt_secret", "test-secret-key-for-jwt-tokens-that-is-long-enough")
	defer viper.Set("auth_jwt_secret", oldSecret)

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

	accessToken, refreshToken, err := service.RefreshToken(ctx, 42)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
}

func TestOperatorAuthService_RefreshToken_OperatorNotFound(t *testing.T) {
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

	_, _, err = service.RefreshToken(ctx, 999)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorNotFoundError{}, err)
}

func TestOperatorAuthService_RefreshToken_InactiveOperator(t *testing.T) {
	oldSecret := viper.GetString("auth_jwt_secret")
	viper.Set("auth_jwt_secret", "test-secret-key-for-jwt-tokens-that-is-long-enough")
	defer viper.Set("auth_jwt_secret", oldSecret)

	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(ctx context.Context, id int64) (*platform.Operator, error) {
			return &platform.Operator{
				Model: base.Model{
					ID: 1,
				},
				Email:       "operator@example.com",
				DisplayName: "Inactive Operator",
				Active:      false,
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

	_, _, err = service.RefreshToken(ctx, 1)
	require.Error(t, err)
	assert.IsType(t, &platformSvc.OperatorInactiveError{}, err)
}

func TestOperatorAuthService_RefreshToken_RepositoryError(t *testing.T) {
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

	_, _, err = service.RefreshToken(ctx, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find operator")
}
