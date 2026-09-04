package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpontaneousActivityInstanceUniquenessDownRefusesDuplicateLinkedInstances(t *testing.T) {
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	require.NoError(t, spontaneousActivityInstanceUniquenessUp(ctx, db))

	room := testpkg.CreateTestRoom(t, db, "spontaneous-uniqueness-rollback")
	group := testpkg.CreateTestActivityGroup(t, db, "spontaneous-uniqueness-rollback")
	date := testpkg.Date(2026, 9, 4)
	testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &group.ID,
	})
	testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &group.ID,
		IsSpontaneous:   true,
	})

	err := spontaneousActivityInstanceUniquenessDown(ctx, db)
	require.EqualError(t, err, "cannot restore activity instance template uniqueness: duplicate activity-linked instances exist")

	var indexDefinition string
	require.NoError(t, db.NewRaw(`
		SELECT indexdef
		FROM pg_indexes
		WHERE schemaname = 'schedule'
			AND indexname = 'idx_activity_instances_template_unique'
	`).Scan(ctx, &indexDefinition))
	assert.Contains(t, indexDefinition, "is_spontaneous = false")
}
