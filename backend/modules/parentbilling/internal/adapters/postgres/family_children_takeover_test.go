package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A registration's number is stated before the contract and taken over
// with it (#6335). An older statement never silently replaces a newer one,
// a newer one does, a time in the future counts as now, and the other
// Träger's family under the same address keeps its number. The test runs
// below row level security.
func TestFamilyChildrenStatementKeepsTheNewerOne(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Familie Übernahme A")
	other := newCareScope(t, db, "Träger Familie Übernahme B")
	store := NewStore()
	rollback := errors.New("roll the rows back")
	const family = "eva.uebernahme@example.org"

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		contracts := map[int64]int64{}
		for _, scope := range []careScope{other, own} {
			var contract int64
			require.NoError(t, tx.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id)
				VALUES (?, ?, ?) RETURNING id`, scope.organization, scope.school, scope.contract).Scan(ctx, &contract))
			contracts[scope.organization] = contract
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.family_child_counts (organization_id, family_email,
			care_contract_id, stated_children, stated_at) VALUES (?, ?, ?, 7, NOW() + interval '1 day')`,
			other.organization, family, contracts[other.organization])
		require.NoError(t, err)

		_, applied, err := store.StateFamilyChildren(ctx, own.organization, 0, family, contracts[own.organization], 3, nil)
		require.NoError(t, err)
		require.True(t, applied, "the first statement of the family")

		older := time.Now().Add(-48 * time.Hour)
		_, applied, err = store.StateFamilyChildren(ctx, own.organization, 0, family, contracts[own.organization], 2, &older)
		require.NoError(t, err)
		assert.False(t, applied, "an older statement does not replace the newer one")
		count, found, err := store.FamilyChildCount(ctx, own.organization, family)
		require.NoError(t, err)
		require.True(t, found)
		require.NotNil(t, count.StatedChildren)
		assert.Equal(t, 3, *count.StatedChildren)

		future := time.Now().Add(240 * time.Hour)
		_, applied, err = store.StateFamilyChildren(ctx, own.organization, 0, family, contracts[own.organization], 4, &future)
		require.NoError(t, err)
		require.True(t, applied)
		count, _, err = store.FamilyChildCount(ctx, own.organization, family)
		require.NoError(t, err)
		require.NotNil(t, count.StatedAt)
		assert.Equal(t, 4, *count.StatedChildren)
		var databaseNow time.Time
		require.NoError(t, tx.NewRaw(`SELECT clock_timestamp()`).Scan(ctx, &databaseNow))
		assert.False(t, count.StatedAt.After(databaseNow), "a time in the future counts as now")

		var foreign int
		require.NoError(t, tx.NewRaw(`SELECT stated_children FROM parent_billing.family_child_counts WHERE organization_id = ?`,
			other.organization).Scan(ctx, &foreign))
		assert.Equal(t, 7, foreign, "the other Träger's family keeps its number")

		require.NoError(t, store.InsertFamilyChildEvents(ctx, own.organization, parentbilling.FamilyChildEvent{
			Action: parentbilling.FamilyChildEventState, Actor: parentbilling.FamilyChildActorParent,
			Channel: parentbilling.FamilyChannelLink, Children: count.StatedChildren,
		}, []parentbilling.FamilyChildTarget{{ContractID: contracts[own.organization], CountID: &count.ID}}))
		var channel string
		require.NoError(t, tx.NewRaw(`SELECT channel FROM parent_billing.family_child_count_events WHERE organization_id = ? AND care_contract_id = ?`,
			own.organization, contracts[own.organization]).Scan(ctx, &channel))
		assert.Equal(t, parentbilling.FamilyChannelLink, channel, "the trail names the channel")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
