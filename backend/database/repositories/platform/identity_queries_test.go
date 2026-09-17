package platform_test

import (
	"testing"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestPlatformReadsFailClosedWithoutMembershipQuery pins that the school
// lookup and the operator account counts report an error when composed
// without the Identity & Access membership query (#2721), instead of
// counting or listing nothing.
func TestPlatformReadsFailClosedWithoutMembershipQuery(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "unbound-membership")

	_, err := platformRepo.NewSchoolRepository(db).FindActiveByAccountID(ctx, account.ID)
	require.ErrorContains(t, err, "active membership query is not bound")

	summaries := platformRepo.NewOperatorSummariesRepository(db)
	_, err = summaries.Stats(ctx)
	require.ErrorContains(t, err, "active membership query is not bound")
	_, err = summaries.OrganizationSummaries(ctx)
	require.ErrorContains(t, err, "active membership query is not bound")
	_, err = summaries.SchoolSummaries(ctx)
	require.ErrorContains(t, err, "active membership query is not bound")
	_, err = summaries.SchoolSummariesByOrganization(ctx, testpkg.Tenant(t))
	require.ErrorContains(t, err, "active membership query is not bound")
}
