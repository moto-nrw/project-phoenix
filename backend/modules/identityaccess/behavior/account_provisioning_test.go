package behavior_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The address is normalized before the row is written, so a capitalized
// input can never create a second account for the same person — the unique
// index on auth.accounts(email) only protects the stored spelling.
func TestRegisterSchoolAccountNormalisesTheAddress(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	provisioning := setupAuthFactory(t, db).AccountAuthentication()

	unique := time.Now().UnixNano()
	mixed := fmt.Sprintf("  Register-%d@Test.Local  ", unique)

	provisioned, err := provisioning.RegisterSchoolAccount(context.Background(), identityaccess.SchoolAccountRegistration{
		Email:    mixed,
		Username: fmt.Sprintf("register-normalised-%d", unique),
		Password: testPassword,
	})
	require.NoError(t, err)
	testpkg.OwnTestAccount(t, db, provisioned.Account.ID)

	assert.Equal(t, strings.ToLower(strings.TrimSpace(mixed)), provisioned.Account.Email)

	var stored string
	require.NoError(t, db.NewSelect().
		ColumnExpr("email").
		TableExpr("auth.accounts").
		Where("id = ?", provisioned.Account.ID).
		Scan(context.Background(), &stored))
	assert.Equal(t, strings.ToLower(strings.TrimSpace(mixed)), stored)
}

// A registration without an address must fail before it writes anything: an
// account row with an empty e-mail is one nobody can sign in to and nobody
// can invite again, because every lookup matches on the address.
func TestRegisterSchoolAccountRefusesAnEmptyAddress(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	provisioning := setupAuthFactory(t, db).AccountAuthentication()

	_, err := provisioning.RegisterSchoolAccount(context.Background(), identityaccess.SchoolAccountRegistration{
		Username: fmt.Sprintf("register-nameless-%d", time.Now().UnixNano()),
		Password: testPassword,
	})
	require.Error(t, err)

	var written int
	require.NoError(t, db.NewSelect().
		ColumnExpr("count(*)").
		TableExpr("auth.accounts").
		Where("email = ''").
		Scan(context.Background(), &written))
	assert.Zero(t, written, "a refused registration writes no account")
}

// Linking an account to a school must never revive a membership an
// offboarding deactivated. The invitation flow reactivates deliberately —
// it goes out to the person again; the admin link does not, so a removed
// staff member cannot be given access back through it silently (#3332).
func TestLinkSchoolAccountLeavesADeactivatedMembershipAlone(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	provisioning := setupAuthFactory(t, db).AccountAuthentication()
	tenantID := testpkg.Tenant(t)

	account := testpkg.CreateTestAccount(t, db, "link-offboarded")
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	_, err := db.ExecContext(context.Background(),
		`UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW()
		 WHERE account_id = ? AND tenant_id = ?`, account.ID, tenantID)
	require.NoError(t, err)

	_, err = provisioning.LinkSchoolAccount(context.Background(), identityaccess.SchoolAccountLink{
		TenantID: tenantID, Email: account.Email,
	})
	require.NoError(t, err)

	var status string
	var deactivated *time.Time
	require.NoError(t, db.NewSelect().
		ColumnExpr("status, deactivated_at").
		TableExpr("auth.account_tenants").
		Where("account_id = ? AND tenant_id = ?", account.ID, tenantID).
		Scan(context.Background(), &status, &deactivated))
	assert.Equal(t, "inactive", status, "the link must not reactivate the membership")
	assert.NotNil(t, deactivated, "the offboarding stamp must survive the link")
}
