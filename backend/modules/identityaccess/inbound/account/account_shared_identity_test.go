package account_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The login address and the second factor hold for a whole account. A school
// administrator changes them only while their school alone holds the account
// (#6862); these tests drive the two routes through the composed router.

const sharedAccountCode = "identity.account_shared_across_schools"

// localAccount is an account the fixture maps to the test's school only.
func (e *accountIsolationEnv) localAccount(t *testing.T, prefix string) (int64, string) {
	t.Helper()
	account := testpkg.CreateTestAccount(t, e.tc.db, prefix)
	testpkg.OwnAccountSecurityNotices(t, e.tc.db, account.ID)
	return account.ID, account.Email
}

// accountAtBothSchools is active at the test's school and at a foreign one.
func (e *accountIsolationEnv) accountAtBothSchools(t *testing.T, prefix string) (int64, string) {
	t.Helper()
	id, address := e.localAccount(t, prefix)
	testpkg.MapAccountToTenant(t, e.tc.db, id, e.foreignTenantID)
	return id, address
}

// traegerMember is active at the test's school and in its Träger-Büro.
func (e *accountIsolationEnv) traegerMember(t *testing.T, prefix string) (int64, string) {
	t.Helper()
	id, address := e.localAccount(t, prefix)
	var organizationID int64
	require.NoError(t, e.tc.db.NewSelect().Column("organization_id").TableExpr("platform.schools").
		Where("id = ?", e.tenantID).Scan(context.Background(), &organizationID))
	testpkg.CreateTestTraegerMember(t, e.tc.db, organizationID, id)
	return id, address
}

func (e *accountIsolationEnv) storedAddress(t *testing.T, accountID int64) string {
	t.Helper()
	var address string
	require.NoError(t, testpkg.SetupTestDB(t).NewSelect().Column("email").TableExpr("auth.accounts").
		Where("id = ?", accountID).Scan(context.Background(), &address))
	return address
}

// noticeRecipients lists the addresses the account's notices of one kind go
// to, in any outbox: the school's or the global one. It reads the package's
// pool, the database the env's resource writes to.
func (e *accountIsolationEnv) noticeRecipients(t *testing.T, kind string, accountID int64) []string {
	t.Helper()
	recipients := []string{}
	require.NoError(t, testpkg.SetupTestDB(t).NewRaw(`SELECT recipient->>'address' FROM platform.email_outbox
		WHERE related_entity_type = ? AND related_entity_id = ? ORDER BY id`, kind, accountID).
		Scan(context.Background(), &recipients))
	return recipients
}

func requireSharedAccountRefusal(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	var body struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, sharedAccountCode, body.Code)
}

func TestSchoolAdminCannotChangeLoginAddressOfSharedAccount(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)

	for name, shared := range map[string]func(*testing.T, string) (int64, string){
		"another school": e.accountAtBothSchools,
		"Träger-Büro":    e.traegerMember,
	} {
		t.Run(name, func(t *testing.T) {
			id, address := shared(t, "shared-login-address")
			rr := e.execute(t, e.claims, http.MethodPut, fmt.Sprintf("/auth/accounts/%d", id),
				map[string]string{"email": fmt.Sprintf("taken-over-%d@example.test", id)})

			requireSharedAccountRefusal(t, rr)
			assert.Equal(t, address, e.storedAddress(t, id), "the login address stays")
			assert.Empty(t, e.noticeRecipients(t, "account_login_address_changed", id))
		})
	}
}

func TestSchoolAdminChangesLoginAddressOfOwnAccountAndPreviousAddressLearnsOfIt(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	id, previous := e.localAccount(t, "own-login-address")
	path := fmt.Sprintf("/auth/accounts/%d", id)

	// A name-only edit keeps the address and tells nobody.
	assertAccountActionStatus(t, e, e.claims, http.MethodPut, path,
		map[string]string{"email": previous, "username": fmt.Sprintf("own-login-%d", id)}, http.StatusNoContent)
	assert.Empty(t, e.noticeRecipients(t, "account_login_address_changed", id))

	changed := fmt.Sprintf("own-login-changed-%d@example.test", id)
	assertAccountActionStatus(t, e, e.claims, http.MethodPut, path, map[string]string{"email": changed}, http.StatusNoContent)

	assert.Equal(t, changed, e.storedAddress(t, id))
	assert.Equal(t, []string{previous}, e.noticeRecipients(t, "account_login_address_changed", id),
		"the notice goes to the address the login left, once")
}

func TestSchoolAdminCannotResetSecondFactorOfSharedAccount(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	// The second-factor routes are served by the composed flows.
	e.tc.resource.MFAService = setupAuthModuleOn(t, e.tc.db).MFA

	for name, shared := range map[string]func(*testing.T, string) (int64, string){
		"another school": e.accountAtBothSchools,
		"Träger-Büro":    e.traegerMember,
	} {
		t.Run(name, func(t *testing.T) {
			id, _ := shared(t, "shared-second-factor")
			rr := e.execute(t, e.claims, http.MethodDelete, fmt.Sprintf("/auth/accounts/%d/mfa", id),
				map[string]string{"reason": "lost mailbox"})

			requireSharedAccountRefusal(t, rr)
			assert.Empty(t, e.noticeRecipients(t, "account_mfa_reset", id))

			// The override of the administrator's own school stays open.
			assertAccountActionStatus(t, e, e.claims, http.MethodPut, fmt.Sprintf("/auth/accounts/%d/mfa/override", id),
				map[string]string{"override": "force_off", "reason": "lost mailbox"}, http.StatusNoContent)
		})
	}
}

func TestSchoolAdminResetsSecondFactorOfOwnAccountAndHolderLearnsOfIt(t *testing.T) {
	t.Parallel()
	e := newAccountIsolationEnv(t)
	// The second-factor routes are served by the composed flows.
	e.tc.resource.MFAService = setupAuthModuleOn(t, e.tc.db).MFA
	id, address := e.localAccount(t, "own-second-factor")

	assertAccountActionStatus(t, e, e.claims, http.MethodDelete, fmt.Sprintf("/auth/accounts/%d/mfa", id),
		map[string]string{"reason": "lost mailbox"}, http.StatusNoContent)

	assert.Equal(t, []string{address}, e.noticeRecipients(t, "account_mfa_reset", id))
}
