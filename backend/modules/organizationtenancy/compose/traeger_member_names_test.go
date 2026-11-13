package compose

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestTraegerMemberNamesStayWithTheirTraeger pins the names of #4693: a
// person who belongs to two Träger carries a name at each, and a read for
// one Träger answers only that Träger's names, in the administrative
// transaction of Identity & Access and in the Träger transaction of the
// export protocol alike. A removed person keeps naming their past steps;
// only an active membership takes a new name.
func TestTraegerMemberNamesStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	newTraeger := func(name string) int64 {
		return testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("traeger-namen-%d", testpkg.UniqueTestTenantID(t)),
		}).ID
	}
	traegerA, traegerB := newTraeger("Träger Namen A"), newTraeger("Träger Namen B")
	shared := testpkg.CreateTestAccount(t, db, "traeger-namen-beide").ID
	onlyB := testpkg.CreateTestAccount(t, db, "traeger-namen-nur-b").ID
	removed := testpkg.CreateTestAccount(t, db, "traeger-namen-entfernt").ID
	unnamed := testpkg.CreateTestAccount(t, db, "traeger-namen-ohne").ID
	testpkg.CreateTestTraegerMember(t, db, traegerA, shared)
	testpkg.CreateTestTraegerMember(t, db, traegerB, shared)
	testpkg.CreateTestTraegerMember(t, db, traegerB, onlyB)
	testpkg.CreateTestTraegerMember(t, db, traegerA, removed)
	testpkg.CreateTestTraegerMember(t, db, traegerA, unnamed)
	names := NewTraegerMemberNames(func(Observation) {})

	set := func(organization, account int64, name string) bool {
		t.Helper()
		var ok bool
		require.NoError(t, testpkg.WithinAdminTransaction(ctx, func(txCtx context.Context) error {
			var err error
			ok, err = names.SetTraegerMemberName(txCtx, organization, account, name)
			return err
		}))
		return ok
	}
	require.True(t, set(traegerA, shared, "A-Name"))
	require.True(t, set(traegerB, shared, "B-Name"))
	require.True(t, set(traegerB, onlyB, "Nur-B"))
	require.True(t, set(traegerA, removed, "Ehemalig"))
	_, err := db.ExecContext(context.Background(), `UPDATE platform.organization_accounts SET status = 'inactive', deactivated_at = NOW()
		WHERE organization_id = ? AND account_id = ?`, traegerA, removed)
	require.NoError(t, err)
	assert.False(t, set(traegerA, removed, "Neu"), "an inactive membership takes no new name")
	assert.False(t, set(traegerA, onlyB, "Fremd"), "no membership, no name")

	everyone := []int64{shared, onlyB, removed, unnamed}
	want := []organizationtenancy.TraegerMemberName{{AccountID: shared, Name: "A-Name"}, {AccountID: removed, Name: "Ehemalig"}}
	if shared > removed {
		want[0], want[1] = want[1], want[0]
	}
	require.NoError(t, testpkg.WithinAdminTransaction(ctx, func(txCtx context.Context) error {
		read, err := names.ListTraegerMemberNames(txCtx, traegerA, everyone)
		require.NoError(t, err)
		assert.Equal(t, want, read, "only A's names, the removed person's too, without a person who has none")
		read, err = names.ListTraegerMemberNames(txCtx, traegerB, []int64{shared})
		require.NoError(t, err)
		assert.Equal(t, []organizationtenancy.TraegerMemberName{{AccountID: shared, Name: "B-Name"}}, read)
		return nil
	}))
	require.NoError(t, testpkg.WithinTraeger(ctx, traegerA, func(txCtx context.Context) error {
		read, err := names.ListTraegerMemberNames(txCtx, traegerA, everyone)
		require.NoError(t, err)
		assert.Equal(t, want, read, "the Träger transaction reads its own names")
		read, err = names.ListTraegerMemberNames(txCtx, traegerB, everyone)
		require.NoError(t, err)
		assert.Empty(t, read, "the Träger transaction of A reads no name of B")
		return nil
	}))
	read, err := names.ListTraegerMemberNames(ctx, traegerA, nil)
	require.NoError(t, err)
	assert.Empty(t, read, "an empty set reads nothing")

	_, err = db.ExecContext(context.Background(), `UPDATE platform.organization_accounts SET display_name = ' A' WHERE organization_id = ? AND account_id = ?`, traegerA, shared)
	assert.ErrorContains(t, err, "chk_organization_accounts_display_name", "the table keeps a name trimmed")
}
