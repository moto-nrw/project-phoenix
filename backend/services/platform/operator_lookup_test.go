package platform_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The operator lookup stays on the retained service (#3252); these cases
// are carried over unchanged from the former operator auth service tests.

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

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
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

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
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

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
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

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
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

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
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

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
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
