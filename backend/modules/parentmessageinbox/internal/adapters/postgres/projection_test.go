package parentinbox

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/parentmessageinbox/contract"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// projectionForSchool reads as the given school without an ambient
// transaction, so the tenant predicate rather than RLS decides what is seen.
func projectionForSchool(db *bun.DB, schoolID int64) *Projection {
	return New(func(context.Context) (bun.IDB, int64, error) { return db, schoolID, nil }, testpkg.GuardianLinks{})
}

func TestFindThreadHeaderAnswersOnlyTheThreadsSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var threadID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
		VALUES (?, ?, ?) RETURNING id`, chain.TenantID, chain.StudentID, chain.AccountID).Scan(ctx, &threadID))
	otherSchool, _ := testpkg.CreateTestTenant(t, db)

	header, err := projectionForSchool(db, chain.TenantID).FindThreadHeader(ctx, threadID)
	require.NoError(t, err)
	assert.Equal(t, &contract.ParentThreadHeader{StudentName: "Felix Schneider", GuardianName: "Sabine Schneider", RelationshipType: "parent"}, header)

	foreign, err := projectionForSchool(db, otherSchool).FindThreadHeader(ctx, threadID)
	require.NoError(t, err)
	assert.Nil(t, foreign, "another school must not read the thread")
}
