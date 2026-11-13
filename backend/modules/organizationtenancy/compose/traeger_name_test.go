package compose

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestTraegerNameReadsOnlyTheOwnTraeger pins the read of #6437: in its
// Träger transaction a Träger reads its own name, also beneath RLS as the
// table owner, and the store refuses to name another Träger there.
func TestTraegerNameReadsOnlyTheOwnTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	newTraeger := func(name string) int64 {
		return testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("traeger-eigener-name-%d", testpkg.UniqueTestTenantID(t)),
		}).ID
	}
	own, foreign := newTraeger("Träger Eigener Name"), newTraeger("Träger Fremder Name")
	names := NewTraegerMasterData()

	require.NoError(t, testpkg.WithinTraeger(ctx, own, func(txCtx context.Context) error {
		name, err := names.TraegerName(txCtx, own)
		require.NoError(t, err)
		assert.Equal(t, "Träger Eigener Name", name)
		_, err = names.TraegerName(txCtx, foreign)
		assert.Error(t, err, "the Träger transaction names no other Träger")
		require.NoError(t, testpkg.WithoutRequestRole(txCtx))
		name, err = names.TraegerName(txCtx, own)
		require.NoError(t, err)
		assert.Equal(t, "Träger Eigener Name", name, "beneath RLS it still reads only its own Träger")
		return nil
	}))
}
