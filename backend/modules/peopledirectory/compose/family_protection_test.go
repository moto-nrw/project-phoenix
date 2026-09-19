package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestFamilyProtectionReadsLatestEventForRequestedChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	protected := testpkg.CreateTestStudent(t, db, "Protected", "Child", "1a")
	released := testpkg.CreateTestStudent(t, db, "Released", "Child", "1a")
	unset := testpkg.CreateTestStudent(t, db, "Unset", "Child", "1a")
	account := testpkg.CreateTestAccount(t, db, "family-protection-owner@example.test")
	for _, event := range []struct {
		studentID int64
		enabled   bool
	}{{protected.ID, true}, {released.ID, true}, {released.ID, false}} {
		_, err = db.NewRaw(
			"INSERT INTO users.student_family_protection_events (tenant_id, student_id, enabled, reason, actor_account_id) VALUES (?, ?, ?, ?, ?)",
			testpkg.Tenant(t), event.studentID, event.enabled, "Fixture", account.ID,
		).Exec(ctx)
		require.NoError(t, err)
	}

	current, err := module.CurrentFamilyProtection(ctx, []int64{protected.ID, released.ID, unset.ID, protected.ID})
	require.NoError(t, err)
	require.Equal(t, map[int64]bool{protected.ID: true, released.ID: false}, current)
	current, err = module.CurrentFamilyProtection(ctx, []int64{unset.ID})
	require.NoError(t, err)
	require.Empty(t, current, "an unprotected child must not expand the read to other children")
	current, err = module.CurrentFamilyProtection(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, current)
}

func TestFamilyProtectionDoesNotReadAnotherTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Foreign", "Child", "2a")
	account := testpkg.CreateTestAccount(t, db, "family-protection-tenant@example.test")
	_, err = db.NewRaw(
		"INSERT INTO users.student_family_protection_events (tenant_id, student_id, enabled, reason, actor_account_id) VALUES (?, ?, ?, ?, ?)",
		otherTenant, foreign.ID, true, "Fixture", account.ID,
	).Exec(ctx)
	require.NoError(t, err)

	current, err := module.CurrentFamilyProtection(ctx, []int64{foreign.ID})
	require.NoError(t, err)
	require.Empty(t, current)

	_, err = module.CurrentFamilyProtection(context.Background(), []int64{foreign.ID})
	require.Error(t, err, "this privacy read must not fall back to an administrative read")
	_, err = module.CurrentFamilyProtection(ctx, []int64{foreign.ID, 0})
	require.ErrorIs(t, err, peopledirectory.ErrInvalidStudent)
}
