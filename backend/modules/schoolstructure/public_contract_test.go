package schoolstructure_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestPublicSchoolStructureContractReadsGroupsByID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	igel := testpkg.CreateTestEducationGroup(t, db, "Igel")
	fuchs := testpkg.CreateTestEducationGroup(t, db, "Fuchs")

	err = testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		_, err := module.FindGroup(ctx, 0)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidGroup)

		found, err := module.FindGroup(ctx, igel.ID)
		require.NoError(t, err)
		assert.Equal(t, igel.ID, found.ID)
		assert.Equal(t, igel.Name, found.Name)

		listed, err := module.ListGroupsByID(ctx, []int64{igel.ID, fuchs.ID})
		require.NoError(t, err)
		require.Len(t, listed, 2)
		assert.Equal(t, []string{fuchs.Name, igel.Name}, []string{listed[0].Name, listed[1].Name}, "sorted by name")
		return nil
	})
	require.NoError(t, err)
}
