package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The numbers of the families keep to their Träger below row level security
// (#5918): the Träger transaction drops its request role, so each assertion
// holds only because the adapter names the Träger in its own statement. Both
// Träger know a family under the same address, the case a missing filter
// would mix. Removing one filter turns its assertion red.
func TestFamilyChildrenFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Familie A")
	other := newCareScope(t, db, "Träger Familie B")
	store := NewStore()
	rollback := errors.New("roll the rows back")
	const family = "eva.familie@example.org"

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		// The other Träger's row comes first, so a read without the filter
		// meets it.
		contracts := map[int64]int64{}
		for _, scope := range []careScope{other, own} {
			var contract int64
			require.NoError(t, tx.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id)
				VALUES (?, ?, ?) RETURNING id`, scope.organization, scope.school, scope.contract).Scan(ctx, &contract))
			contracts[scope.organization] = contract
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.family_child_counts (organization_id, family_email,
				care_contract_id, stated_children, stated_at) VALUES (?, ?, ?, 2, NOW())`, scope.organization, family, contract)
			require.NoError(t, err)
		}

		counts, err := store.FamilyChildCounts(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, counts, 1, "FamilyChildCounts of the Träger")
		assert.Equal(t, contracts[own.organization], counts[0].ContractID)

		count, found, err := store.FamilyChildCount(ctx, own.organization, family)
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, contracts[own.organization], count.ContractID, "FamilyChildCount answers the own family, not the other Träger's")

		// Even a foreign event carrying this row's ID must stay outside history.
		for _, event := range []struct {
			organization int64
			children     int
		}{{own.organization, 3}, {other.organization, 9}} {
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.family_child_count_events
				(organization_id, care_contract_id, family_child_count_id, action, actor, children, valid_from)
				VALUES (?, ?, ?, 'confirm', 'office', ?, '2029-09-01')`, event.organization, contracts[event.organization], count.ID, event.children)
			require.NoError(t, err)
		}
		counts, err = store.FamilyChildCounts(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, counts, 1)
		number, valid := counts[0].ConfirmedIn(calendar.NewDate(2029, 9, 1))
		require.True(t, valid)
		assert.Equal(t, 3, number, "confirmation history also names its Träger below RLS")

		input := parentbilling.FamilyChildrenInput{ContractID: contracts[other.organization], Children: 3, ValidFrom: calendar.NewDate(2029, 9, 1)}
		_, err = tx.ExecContext(ctx, "SAVEPOINT foreign_contract")
		require.NoError(t, err)
		_, err = store.ConfirmFamilyChildren(ctx, own.organization, 0, "neu@example.org", input)
		assert.ErrorIs(t, err, parentbilling.ErrFamilyChildrenNotFound, "a contract of another Träger takes no number")
		_, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT foreign_contract")
		require.NoError(t, err)

		removed, err := store.DeleteFamilyChildren(ctx, own.organization, family)
		require.NoError(t, err)
		assert.Equal(t, contracts[own.organization], removed.ContractID, "DeleteFamilyChildren removes the own family")
		_, err = store.DeleteFamilyChildren(ctx, own.organization, family)
		assert.ErrorIs(t, err, parentbilling.ErrFamilyChildrenNotFound, "the other Träger's family is not the own")

		_, err = store.StateFamilyChildren(ctx, own.organization, 0, family, contracts[own.organization], 2)
		require.NoError(t, err)
		counts, err = store.FamilyChildCounts(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, counts, 1)
		_, valid = counts[0].ConfirmedIn(calendar.NewDate(2029, 9, 1))
		assert.False(t, valid, "a fresh statement does not revive removed confirmations")

		var kept int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.family_child_counts WHERE organization_id = ?`, other.organization).
			Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger keeps its number")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
