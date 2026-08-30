package platform_test

// IssueTokensForAuthenticatedOperator is the post-MFA-verify token-mint path
// for operators. 0% in the coverage profile — none of the existing tests
// exercise the "MFA already proved identity" branch.

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

const issueOperatorSentinelID int64 = 70010001

func TestOperatorAuthService_IssueTokensForAuthenticatedOperator_HappyPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(_ context.Context, id int64) (*platform.Operator, error) {
			op := &platform.Operator{
				Email:       "ops@example.test",
				DisplayName: "Test Operator",
				Active:      true,
			}
			op.ID = id
			return op, nil
		},
	}
	var auditCalls int
	refreshRepo := &mockOperatorRefreshTokenRepo{}
	auditRepo := &mockAuditLogRepoShared{
		createFn: func(_ context.Context, _ *platform.OperatorAuditLog) error {
			auditCalls++
			return nil
		},
	}
	svc, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:     operatorRepo,
		AuditLogRepo:     auditRepo,
		RefreshTokenRepo: refreshRepo,
		DB:               &bun.DB{},
		Logger:           slog.Default(),
	})
	require.NoError(t, err)

	access, refresh, err := svc.IssueTokensForAuthenticatedOperator(ctx, issueOperatorSentinelID, "127.0.0.1", "ua")
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)
	assert.Equal(t, 1, auditCalls,
		"successful post-MFA mint must still write an audit row so login events stay visible")
	require.Len(t, refreshRepo.created, 1)
	assert.NotEqual(t, "operator-refresh-70010001", refreshRepo.created[0].Token,
		"operator refresh sessions must use random server-side handles, not deterministic operator IDs")
}

func TestOperatorAuthService_IssueTokensForAuthenticatedOperator_NotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.Operator, error) {
			return nil, nil // not-found is nil/nil per repo contract
		},
	}
	auditRepo := &mockAuditLogRepoShared{}
	svc, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: auditRepo,
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = svc.IssueTokensForAuthenticatedOperator(ctx, issueOperatorSentinelID, "127.0.0.1", "ua")
	require.Error(t, err)
	var notFound *platformSvc.OperatorNotFoundError
	assert.ErrorAs(t, err, &notFound, "missing operator must surface as OperatorNotFoundError")
}

func TestOperatorAuthService_IssueTokensForAuthenticatedOperator_RepoError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.Operator, error) {
			return nil, fmt.Errorf("database unavailable")
		},
	}
	svc, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = svc.IssueTokensForAuthenticatedOperator(ctx, issueOperatorSentinelID, "127.0.0.1", "ua")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database unavailable",
		"underlying repo errors must propagate so callers can log them")
}

func TestOperatorAuthService_IssueTokensForAuthenticatedOperator_Inactive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(_ context.Context, id int64) (*platform.Operator, error) {
			op := &platform.Operator{
				Model:       base.Model{ID: id},
				Email:       "inactive@example.test",
				DisplayName: "Inactive Operator",
				Active:      false,
			}
			return op, nil
		},
	}
	svc, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: operatorRepo,
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           &bun.DB{},
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	_, _, err = svc.IssueTokensForAuthenticatedOperator(ctx, issueOperatorSentinelID, "127.0.0.1", "ua")
	require.Error(t, err)
	var inactive *platformSvc.OperatorInactiveError
	assert.ErrorAs(t, err, &inactive, "inactive operators must never receive a session token")
}
