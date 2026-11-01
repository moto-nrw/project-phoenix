package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestStaffForExistingAccountInTenant gives one account a staff identity
// in another school, for cross-school authorization and command-scope tests.
func CreateTestStaffForExistingAccountInTenant(tb testing.TB, db *bun.DB, tenantID, accountID int64, firstName, lastName string) *Staff {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	person := &Person{FirstName: firstName, LastName: lastName, AccountID: &accountID}
	person.TenantID = tenantID
	_, err := db.NewInsert().Model(person).ModelTableExpr("users.persons").Returning("id").Exec(ctx)
	require.NoError(tb, err)
	staff := &Staff{PersonID: person.ID}
	staff.TenantID = tenantID
	require.NoError(tb, insertTestStaff(ctx, db, staff))
	EnsureAccountTenant(tb, db, accountID, tenantID)
	return staff
}
