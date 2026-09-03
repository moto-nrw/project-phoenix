package enrollment_test

import (
	"context"
	"testing"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormSchemaRepository_ListByIDs(t *testing.T) {
	t.Parallel()
	db, repo, tenantID, creator := setupSchemaRepoTest(t)
	defer wipeSchemas(db, tenantID, creator)
	first := &enrollmentModels.FormSchema{Name: uniqueSchemaName("batch-a"), Version: 1, Fields: validFields(), IsActive: true, CreatedBy: creator}
	second := &enrollmentModels.FormSchema{Name: uniqueSchemaName("batch-b"), Version: 1, Fields: validFields(), IsActive: true, CreatedBy: creator}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if err := repo.Create(ctx, first); err != nil {
			return err
		}
		return repo.Create(ctx, second)
	}))

	var rows []*enrollmentModels.FormSchema
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		rows, err = repo.ListByIDs(ctx, []int64{second.ID, first.ID, 9_999_999})
		return err
	}))
	require.Len(t, rows, 2)
	assert.ElementsMatch(t, []int64{first.ID, second.ID}, []int64{rows[0].ID, rows[1].ID})
}
