package operatordashboard_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/operatordashboard/contract"

	operatordashboard "github.com/moto-nrw/project-phoenix/modules/operatordashboard/internal/adapters/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The consumer's values Map answers in. They differ from the projection's rows
// on purpose, so a test passes only when every mapping is applied.
type (
	countsView       struct{ Organizations, Schools, Accounts int }
	organizationView struct {
		ID       int64
		Schools  int
		Accounts int
	}
	schoolView struct {
		ID, OrganizationID int64
		Accounts           int
		Deleted            bool
	}
	usageView struct {
		TenantID           int64
		Portal             string
		Standalone, Usable int
	}
)

func consumerRows() contract.Rows[countsView, organizationView, schoolView, usageView] {
	return contract.Rows[countsView, organizationView, schoolView, usageView]{
		Counts: func(row contract.Counts) countsView {
			return countsView{Organizations: row.Organizations, Schools: row.Schools, Accounts: row.Accounts}
		},
		Organization: func(row contract.OrganizationSummary) organizationView {
			return organizationView{ID: row.ID, Schools: row.SchoolCount, Accounts: row.AccountCount}
		},
		School: func(row contract.SchoolSummary) schoolView {
			return schoolView{ID: row.ID, OrganizationID: row.OrganizationID, Accounts: row.AccountCount, Deleted: row.DeletedAt != nil}
		},
		PWAUsage: func(row contract.PWAUsageRow) usageView {
			return usageView{TenantID: row.TenantID, Portal: row.Portal, Standalone: row.StandaloneUsers, Usable: row.EligibleUsers}
		},
	}
}

// TestMap_AnswersInConsumerValuesWithoutForeignRows pins the mapped reads the
// owner's composition binds: every row arrives in the consumer's values, and a
// read scoped to one organisation or school never carries another one's rows.
func TestMap_AnswersInConsumerValuesWithoutForeignRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	fix := setupSummariesFixture(t, db)
	mapped := operatordashboard.Map(newProjection(), consumerRows())

	fresh := time.Now()
	assignRoleForTenant(t, db, fix.AccountID1, fix.SchoolA1, roleUser)
	assignRoleForTenant(t, db, fix.AccountID2, fix.SchoolB1, roleUser)
	recordUsageRow(t, db, fix.SchoolA1, fix.AccountID1, "staff", fresh)
	recordUsageRow(t, db, fix.SchoolB1, fix.AccountID2, "staff", fresh)

	withinAdmin(t, db, func(ctx context.Context) error {
		schools, err := mapped.SchoolSummaries(ctx, &fix.OrgA.ID)
		require.NoError(t, err)
		byID := map[int64]schoolView{}
		for _, school := range schools {
			byID[school.ID] = school
			assert.Equal(t, fix.OrgA.ID, school.OrganizationID)
		}
		assert.Equal(t, schoolView{ID: fix.SchoolA1, OrganizationID: fix.OrgA.ID, Accounts: 1}, byID[fix.SchoolA1])
		assert.Equal(t, schoolView{ID: fix.SchoolA2, OrganizationID: fix.OrgA.ID, Accounts: 1}, byID[fix.SchoolA2])
		assert.True(t, byID[fix.SchoolADead].Deleted)
		assert.NotContains(t, byID, fix.SchoolB1, "a school of another organisation must not appear")
		assert.Len(t, schools, 3)

		organizations, err := mapped.OrganizationSummaries(ctx)
		require.NoError(t, err)
		organizationsByID := map[int64]organizationView{}
		for _, organization := range organizations {
			organizationsByID[organization.ID] = organization
		}
		assert.Equal(t, organizationView{ID: fix.OrgA.ID, Schools: 2, Accounts: 1}, organizationsByID[fix.OrgA.ID])
		assert.Equal(t, organizationView{ID: fix.OrgB.ID, Schools: 1, Accounts: 1}, organizationsByID[fix.OrgB.ID])

		usage, err := mapped.PWAUsage(ctx, fix.SchoolA1, 30*24*time.Hour)
		require.NoError(t, err)
		assert.Equal(t, []usageView{{TenantID: fix.SchoolA1, Portal: "staff", Standalone: 1, Usable: 1}}, usage,
			"the usage of another school must not appear")

		counts, err := mapped.Counts(ctx)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, counts.Organizations, 2)
		assert.GreaterOrEqual(t, counts.Schools, 3)
		assert.GreaterOrEqual(t, counts.Accounts, 2)
		return nil
	})
}

// TestMap_KeepsFailedReadsEmpty pins that a failed read maps to nothing
// instead of an empty list the caller could mistake for a result.
func TestMap_KeepsFailedReadsEmpty(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	mapped := operatordashboard.Map(operatordashboard.New(ambientTransaction, nil), consumerRows())

	withinAdmin(t, db, func(ctx context.Context) error {
		counts, err := mapped.Counts(ctx)
		require.ErrorContains(t, err, "active account counts are not bound")
		assert.Zero(t, counts)
		organizations, err := mapped.OrganizationSummaries(ctx)
		require.Error(t, err)
		assert.Nil(t, organizations)
		schools, err := mapped.SchoolSummaries(ctx, nil)
		require.Error(t, err)
		assert.Nil(t, schools)
		return nil
	})
}

func TestMap_RequiresEveryMapping(t *testing.T) {
	t.Parallel()
	rows := consumerRows()
	rows.PWAUsage = nil
	assert.Panics(t, func() { operatordashboard.Map(newProjection(), rows) })
	assert.Panics(t, func() { operatordashboard.Map(nil, consumerRows()) })
}
