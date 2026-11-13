package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The children of a family (#5918) are rows without a name: the number the
// parents stated with their address as the family's key, and the trail of
// every read and change. They hang on the contract the number was stated or
// confirmed for and go with it by cascade, so the erasure of a retired demo
// school (1.15.805, 1.15.808) removes them without a statement of its own.

// leaveFamilyChildren states a number for the family of the school's
// contract and records it in the trail. It answers the rows it wrote.
func leaveFamilyChildren(t *testing.T, db *bun.DB, schoolID int64, family string) (count, event int64) {
	t.Helper()
	ctx := t.Context()
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.family_child_counts (organization_id, family_email, care_contract_id,
			stated_children, stated_at, confirmed_children, valid_from, confirmed_at)
		SELECT organization_id, ?1, id, 3, NOW(), 3, DATE '2026-09-01', NOW()
		FROM care_contract.contracts WHERE school_id = ?0 ORDER BY id LIMIT 1 RETURNING id`, schoolID, family).Scan(ctx, &count))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.family_child_count_events (organization_id, care_contract_id,
			family_child_count_id, action, actor, children)
		SELECT organization_id, care_contract_id, id, 'state', 'parent', 3 FROM parent_billing.family_child_counts WHERE id = ?0
		RETURNING id`, count).Scan(ctx, &event))
	return count, event
}

func TestDemoSchoolQueueErasesTheFamilyChildrenOfTheRetiredSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	openID := schoolOfOrganization(t, db, schoolID, false)
	accounts := leaveVisitorTraces(t, db, schoolID)
	leaveVisitorTraces(t, db, openID)
	count, event := leaveFamilyChildren(t, db, schoolID, "besuch@example.org")
	openCount, openEvent := leaveFamilyChildren(t, db, openID, "familie@example.org")

	retireVisitorOrder(t, db, schoolID, accounts)

	assert.Zero(t, remainingRows(t, db, "parent_billing.family_child_counts", []int64{count}), "no number of the retired school's family")
	assert.Zero(t, remainingRows(t, db, "parent_billing.family_child_count_events", []int64{event}), "no entry of its trail")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.family_child_counts", []int64{openCount}), "the open school keeps its family's number")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.family_child_count_events", []int64{openEvent}))
}
