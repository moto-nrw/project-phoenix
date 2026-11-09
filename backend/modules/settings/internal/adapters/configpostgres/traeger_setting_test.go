package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// FindByOrganization names the Träger it reads for, not only the row-level
// security of its transaction (ADR 0086 section 5). The test reads in the
// administrative transaction, where row-level security does not narrow the
// rows to one Träger, so only the predicate on organization_id keeps another
// Träger's values out: the DATEV numbers of Träger A must not reach the
// export check of Träger B (#4694). Without the predicate the test fails.
func TestTraegerSettingRepositoryFindByOrganizationNamesTheTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := NewTraegerSettingRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)
	slug := func(name string) string { return fmt.Sprintf("%s-%d", name, testpkg.UniqueTestTenantID(t)) }
	withValues := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "DATEV Träger A", Slug: slug("datev-a"), Active: true}).ID
	without := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "DATEV Träger B", Slug: slug("datev-b"), Active: true}).ID
	keys := []string{config.KeyParentBillingDatevConsultantNumber, config.KeyParentBillingDatevRevenueAccount}
	for _, key := range keys {
		_, err := db.NewInsert().Model(&config.TraegerSettingValue{
			OrganizationID: withValues, SettingKey: key, Value: []byte(`"4400"`),
		}).ModelTableExpr(tableTraegerSettingValues).Exec(context.Background())
		require.NoError(t, err)
	}

	read := func(organizationID int64) []*config.TraegerSettingValue {
		var values []*config.TraegerSettingValue
		require.NoError(t, testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error {
			var err error
			values, err = repo.FindByOrganization(txCtx, organizationID, keys)
			return err
		}))
		return values
	}
	assert.Len(t, read(withValues), 2, "the Träger reads its own values")
	assert.Empty(t, read(without), "another Träger's values stay out without help from row-level security")
}
