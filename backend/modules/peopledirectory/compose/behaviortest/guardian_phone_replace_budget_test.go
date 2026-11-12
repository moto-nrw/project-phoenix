package compose_test

import (
	"strings"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestGuardianPhoneReplacementQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	owner := phoneReplacementOwner(t, db)
	ctx := testpkg.Ctx(t)
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Budget", "Phone", "replace-budget")
	counter := testpkg.CaptureQueries(t, db)
	for _, n := range []int{3, 8} {
		counter.Reset()
		require.NoError(t, owner.ReplaceGuardianPhones(ctx, guardian.ID, replacementInput(n)))
		queries := counter.Matching(func(q string) bool { return strings.Contains(q, "users.guardian_phone_numbers") })
		t.Logf("N%d: business SQL=%d total SQL=%d", n, len(queries), len(counter.Queries()))
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "modules.peopledirectory.guardian_phones.replace_3", counter.Measurement(queries))
		} else {
			querybudgets.AssertQueryBudget(t, "modules.peopledirectory.guardian_phones.replace_8", counter.Measurement(queries))
		}
		require.Len(t, replacementPhones(t, db, guardian.ID), n)
	}
}
