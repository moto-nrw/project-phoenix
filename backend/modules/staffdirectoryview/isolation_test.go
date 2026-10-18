package staffdirectoryview_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStaffDirectoryJoinsRealOwnersAndIsolatesAccountAndContactResults(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	directory := api.NewPeopleRepositorySuiteFactory(database).Staff
	type side struct {
		tenantID, staffID, accountID int64
		email                        string
	}
	var sides []side
	for _, name := range []string{"Own", "Foreign"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Contact", name)
			sides = append(sides, side{testpkg.Tenant(t), staff.ID, account.ID, account.Email})
		})
	}
	for _, side := range sides {
		require.NoError(t, testpkg.WithTenantTx(t, context.Background(), db, side.tenantID, func(ctx context.Context, _ bun.Tx) error {
			ids, err := directory.ListAccountIDsByStaffIDs(ctx, []int64{sides[0].staffID, sides[1].staffID})
			require.NoError(t, err)
			require.Equal(t, map[int64]int64{side.staffID: side.accountID}, ids)
			contact, err := directory.GetStaffContactInfo(ctx, side.staffID)
			require.NoError(t, err)
			require.Equal(t, side.staffID, contact.StaffID)
			require.Equal(t, side.accountID, contact.AccountID)
			require.Equal(t, side.email, contact.Email)
			for _, other := range sides {
				if other.tenantID == side.tenantID {
					continue
				}
				_, err = directory.GetStaffContactInfo(ctx, other.staffID)
				require.Error(t, err, "foreign contacts must not be observable")
			}
			return nil
		}))
	}
}
