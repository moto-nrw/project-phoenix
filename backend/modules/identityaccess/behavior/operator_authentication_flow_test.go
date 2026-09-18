package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The operator MFA and passkey exchanges end in the operator token issue.
// This pins the outcomes those routes classify on: a live operator receives
// a pair, an unknown one is not found and a deactivated one is refused.
func TestIntegration_OperatorTokenIssue_ReportsTheOperatorOutcomes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := newOperatorMFATestModule(t, db)
	service := module.Auth
	ctx := context.Background()
	operator := testpkg.CreateTestOperator(t, db)

	access, refresh, err := service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)

	_, _, err = service.IssueTokensForAuthenticatedOperator(ctx, operator.ID+1_000_000, "127.0.0.1", "ua")
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound)

	_, err = db.NewUpdate().Table("platform.operators").Set("active = false").Where("id = ?", operator.ID).Exec(ctx)
	require.NoError(t, err)
	_, _, err = service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.ErrorIs(t, err, identityaccess.ErrOperatorInactive)
}
