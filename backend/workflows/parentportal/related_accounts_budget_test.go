package parentportal_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/parentfixture"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelatedAccountsQueryBudget(t *testing.T) {
	t.Parallel()
	svc, _, db := buildRelAcctService(t, parentService.ParentInviteModeDirect, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	ctx := testpkg.Ctx(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	requestCtx := counter.Context(testpkg.WithPackageTenantRuntime(context.Background()))
	linked := 1
	for _, n := range []int{1, 3, 12} {
		for linked < n {
			profile := testpkg.CreateTestGuardianProfile(t, db, fmt.Sprintf("related-budget-%d", linked))
			link := &parentfixture.GuardianLinkFixture{
				StudentID: chain.StudentID, GuardianProfileID: profile.ID,
				RelationshipType: "relative", GuardianRole: "pickup_only", EmergencyPriority: linked,
			}
			link.TenantID = testpkg.Tenant(t)
			require.NoError(t, repos.CreateGuardianLinkFixture(ctx, link))
			linked++
		}
		links, err := testpkg.TenantQuery1(t, ctx, db, repos.StudentGuardian.FindByStudentID, chain.StudentID)
		require.NoError(t, err)
		counter.Reset()
		accounts, err := svc.ListRelatedAccounts(requestCtx, chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		require.Len(t, accounts, n)
		for i, account := range accounts {
			assert.Equal(t, links[i].GuardianProfileID, account.GuardianProfileID, "link order")
			assert.Equal(t, account.GuardianProfileID == chain.GuardianProfileID, account.IsSelf)
			if account.IsSelf {
				assert.Equal(t, parentService.RelatedAccountActive, account.Status)
			} else {
				assert.Equal(t, parentService.RelatedAccountNoAccount, account.Status)
			}
		}
		assert.Len(t, counter.Matching(func(query string) bool {
			return strings.Contains(query, "from users.guardian_profiles")
		}), 1, "load linked profiles in one batch")
		t.Logf("N=%d total=%d profiles=%d invitations=%d SQL=%q", n, len(counter.Queries()), len(counter.Selects("users.guardian_profiles")), len(counter.Selects("auth.guardian_invitations")), counter.Queries())
		switch n {
		case 1:
			querybudgets.AssertQueryBudget(t, "workflows.parentportal.related_accounts.list.n1", counter.Measurement(counter.Queries()))
		case 3:
			querybudgets.AssertQueryBudget(t, "workflows.parentportal.related_accounts.list.n3", counter.Measurement(counter.Queries()))
		case 12:
			querybudgets.AssertQueryBudget(t, "workflows.parentportal.related_accounts.list.n12", counter.Measurement(counter.Queries()))
		}
		assert.Empty(t, counter.Operation("INSERT"))
		assert.Empty(t, counter.Operation("UPDATE"))
		assert.Empty(t, counter.Operation("DELETE"))
	}
}

// Cancellation at the profile boundary must still abort the request. The
// batch reader's internal diagnostic prefix need not match the single reader.
func TestRelatedAccountsProfileReadError(t *testing.T) {
	t.Parallel()
	svc, _, db := buildRelAcctService(t, parentService.ParentInviteModeDirect, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	profiles := owners.NewParentGuardianProfileReads(repos.GuardianProfile)
	svc.ChildFlows.GuardianProfileRepo = cancelledRelatedProfiles(profiles)
	_, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.ErrorIs(t, err, context.Canceled)
	assert.EqualError(t, err, "failed to find guardian profiles by ids: context canceled")
}

type cancelledProfileReads[T any] struct{ guardianProfiles[T] }

func cancelledRelatedProfiles[T any](source guardianProfiles[T]) cancelledProfileReads[T] {
	return cancelledProfileReads[T]{source}
}

func (r cancelledProfileReads[T]) FindByID(ctx context.Context, id int64) (*T, error) {
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	return r.guardianProfiles.FindByID(ctx, id)
}

func (r cancelledProfileReads[T]) FindByIDs(ctx context.Context, ids []int64) (map[int64]*T, error) {
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	return r.guardianProfiles.FindByIDs(ctx, ids)
}

func TestRelatedAccountsMissingProfileRetainsError(t *testing.T) {
	t.Parallel()
	svc, _, db := buildRelAcctService(t, parentService.ParentInviteModeDirect, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	repos := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	profiles := owners.NewParentGuardianProfileReads(repos.GuardianProfile)
	svc.ChildFlows.GuardianProfileRepo = missingRelatedProfiles(profiles)
	accounts, err := svc.ListRelatedAccounts(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.ErrorIs(t, err, parentService.ErrGuardianProfileNotFound)
	assert.ErrorIs(t, err, peopledirectory.ErrGuardianProfileNotFound)
	assert.EqualError(t, err, peopledirectory.ErrGuardianProfileNotFound.Error())
	assert.Nil(t, accounts)
}

type missingProfileReads[T any] struct{ guardianProfiles[T] }

func missingRelatedProfiles[T any](source guardianProfiles[T]) missingProfileReads[T] {
	return missingProfileReads[T]{source}
}

func (r missingProfileReads[T]) FindByIDs(context.Context, []int64) (map[int64]*T, error) {
	return map[int64]*T{}, nil
}

func (r missingProfileReads[T]) FindByID(context.Context, int64) (*T, error) {
	return nil, parentService.WithGuardianProfileMissingCause(peopledirectory.ErrGuardianProfileNotFound)
}
