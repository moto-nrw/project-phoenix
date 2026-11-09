package care_test

// Ambient entries of ADR 0161 point 7 for the parent child writes (#5862):
// called from no unit, from an open administrative unit and from a school
// unit of the same and of another school, the bundled write ends like the
// child check followed by its own school unit, the way before the handoff.

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/require"
)

// writeOutcome classifies a write as the caller sees it. A refused child is
// the 403 care.child_not_linked of the write routes.
func writeOutcome(err error, school string) string {
	switch {
	case errors.Is(err, parentportal.ErrChildNotLinked):
		return "child not linked"
	case err != nil:
		return "failed"
	default:
		return "wrote in " + school
	}
}

func TestPermittedChildWriteEndsLikeTheStandaloneWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	children := childResolver(db, api.NewRepositoryTestFactory(database), api.NewPeopleRepositorySuiteFactory(database))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var schoolB int64
	t.Run("school B", func(t *testing.T) {
		schoolB = testpkg.OwnTenant(t)
		testpkg.EnsureTestTenant(t, db, schoolB)
	})

	stranger := testpkg.CreateTestParentGuardianChain(t, db)
	for _, account := range []struct {
		name string
		id   int64
		want map[string]string
	}{
		{"guardian", chain.AccountID, map[string]string{"no unit": "wrote in " + fmt.Sprint(chain.TenantID)}},
		// Another family's account: the child check refuses it in every
		// entry, also inside an open administrative unit, where the handoff
		// alone would refuse the unit before the check.
		{"stranger", stranger.AccountID, map[string]string{"no unit": "child not linked", "administrative unit": "child not linked"}},
	} {
		outcomes := map[string]string{}
		for name, entry := range ambientEntries(t, chain.TenantID, schoolB) {
			var standalone, bundled string
			entry(ctx, func(ctx context.Context) {
				var school string
				child, err := children.ResolvePermittedChild(ctx, account.id, chain.StudentID, portalAccess)
				if err == nil {
					id, idErr := tenant.NewTenantID(child.TenantID)
					require.NoError(t, idErr)
					err = tenant.WithinTenant(ctx, id, func(txCtx context.Context) error { return schoolRead(t, &school)(txCtx, child.TenantID) })
				}
				standalone = writeOutcome(err, school)
			})
			entry(ctx, func(ctx context.Context) {
				var school string
				err := children.WithPermittedChildForWrite(ctx, account.id, chain.StudentID, portalAccess, nil, func(txCtx context.Context, child *parentportal.Child) error {
					return schoolRead(t, &school)(txCtx, child.TenantID)
				})
				bundled = writeOutcome(err, school)
			})
			require.Equal(t, standalone, bundled, "%s, %s", account.name, name)
			outcomes[name] = bundled
		}
		for name, want := range account.want {
			require.Equal(t, want, outcomes[name], "%s, %s", account.name, name)
		}
		t.Logf("ambient outcomes, %s: %v", account.name, outcomes)
	}
}
