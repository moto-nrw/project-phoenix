package platform_test

import (
	"context"
	"testing"

	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The retained MFA and passkey exchanges end in the operator token issue,
// which the service root binds to Identity & Access (#3252). This pins the
// retained error shapes those routes render.
func TestIntegration_OperatorTokenIssue_KeepsTheRetainedErrorShapes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := buildAuthService(t, db)
	ctx := context.Background()
	operator := testpkg.CreateTestOperator(t, db)

	access, refresh, err := service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	require.NoError(t, err)
	assert.NotEmpty(t, access)
	assert.NotEmpty(t, refresh)

	_, _, err = service.IssueTokensForAuthenticatedOperator(ctx, operator.ID+1_000_000, "127.0.0.1", "ua")
	var notFound *platformSvc.OperatorNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, operator.ID+1_000_000, notFound.OperatorID)

	_, err = db.NewUpdate().Table("platform.operators").Set("active = false").Where("id = ?", operator.ID).Exec(ctx)
	require.NoError(t, err)
	_, _, err = service.IssueTokensForAuthenticatedOperator(ctx, operator.ID, "127.0.0.1", "ua")
	var inactive *platformSvc.OperatorInactiveError
	require.ErrorAs(t, err, &inactive)
	assert.Equal(t, operator.ID, inactive.OperatorID)
}
