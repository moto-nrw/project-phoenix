// Router-level tests of the names of the Träger-Büro (#4693): the invitee
// names themselves at the acceptance, a person with "Konten verwalten" names
// an active colleague, every member reads and names themselves without that
// right, and each name lands in the ledger and the history.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type traegerNameChange struct {
	Actor      int64   `bun:"actor_account_id"`
	Target     *int64  `bun:"target_account_id"`
	TargetName *string `bun:"target_name"`
}

// traegerNameChanges reads the entries of the ledger that set a name.
func traegerNameChanges(t *testing.T, db *bun.DB, organizationID int64, action string) []traegerNameChange {
	t.Helper()
	var changes []traegerNameChange
	require.NoError(t, db.NewRaw(`SELECT actor_account_id, target_account_id, target_name FROM audit.traeger_account_changes
		WHERE organization_id = ? AND action = ? ORDER BY id`, organizationID, action).Scan(context.Background(), &changes))
	return changes
}

// assertNameRefused expects the field error of an invalid name.
func assertNameRefused(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeGeneralInput)
	assert.Contains(t, rec.Body.String(), `"name"`, "the error names the field")
}

func traegerMemberName(t *testing.T, buero traegerBueroAnswer, accountID int64) string {
	t.Helper()
	for _, member := range buero.Members {
		if member.AccountID == accountID {
			return member.Name
		}
	}
	t.Fatalf("account %d is no member", accountID)
	return ""
}

func TestTraegerInvitationAcceptanceNamesTheInvitee(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	router, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	organizationID := traegerBuero(t, db, "Träger Name Einladung")
	_, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleLeadership)
	address := fmt.Sprintf("name-%d@test.local", testpkg.UniqueTestTenantID(t))
	token := traegerInvite(t, db, router, admin.AccessToken, address, identityaccess.TraegerRoleBilling)
	password := testpkg.NewTestPassword()

	for _, name := range []any{nil, "   ", "Jana\u0007Krause", strings.Repeat("ä", identityaccess.TraegerMemberNameMaxLength+1)} {
		body := map[string]any{"password": password, "confirm_password": password}
		if name != nil {
			body["name"] = name
		}
		rec := traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", body)
		assertNameRefused(t, rec)
	}
	rec := traegerCall(t, router, http.MethodGet, "/traeger/invitations/"+token, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, "a refused name leaves the link open")

	rec = traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", map[string]any{
		"name": "  Jana Krause ", "password": password, "confirm_password": password,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE email = ?`, address)
		require.NoError(t, err)
	})
	var invitee int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.accounts WHERE email = ?`, address).Scan(context.Background(), &invitee))
	assert.Equal(t, "Jana Krause", traegerMemberName(t, traegerBueroList(t, router, admin.AccessToken), invitee), "the trimmed name is listed")
	accepted := traegerNameChanges(t, db, organizationID, identityaccess.TraegerAccountInvitationAccepted)
	require.Len(t, accepted, 1)
	require.NotNil(t, accepted[0].TargetName)
	assert.Equal(t, "Jana Krause", *accepted[0].TargetName, "the ledger keeps the name of the acceptance")
}

func TestTraegerMemberNameNeedsTheAccountsRightAndAnActiveMember(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	router, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	organizationID := traegerBuero(t, db, "Träger Name Verwalten")
	leader, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleLeadership)
	billing, clerk := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleBilling)
	leaving, _ := traegerMember(t, db, router, organizationID, identityaccess.TraegerRolePersonnel)
	stranger := traegerAccount(t, db, testpkg.NewTestPassword())
	path := func(accountID int64) string { return fmt.Sprintf("/traeger/accounts/%d/name", accountID) }

	rec := traegerCall(t, router, http.MethodPut, path(leader.ID), clerk.AccessToken, map[string]any{"name": "Chefin"})
	assert.Equal(t, http.StatusForbidden, rec.Code, "the Abrechnung names nobody else")
	rec = traegerCall(t, router, http.MethodPut, path(billing.ID), admin.AccessToken, map[string]any{"name": ""})
	assertNameRefused(t, rec)

	rec = traegerCall(t, router, http.MethodPut, path(billing.ID), admin.AccessToken, map[string]any{"name": "Tobias Engel"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	member := decodeSchoolPortalData[struct {
		AccountID int64  `json:"account_id"`
		Name      string `json:"name"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, billing.ID, member.AccountID)
	assert.Equal(t, "Tobias Engel", member.Name)
	assert.Equal(t, "Tobias Engel", traegerMemberName(t, traegerBueroList(t, router, admin.AccessToken), billing.ID))

	rec = traegerCall(t, router, http.MethodPut, path(stranger.ID), admin.AccessToken, map[string]any{"name": "Fremd"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "no member of this Träger-Büro")
	assert.Contains(t, rec.Body.String(), common.CodeTraegerMemberNotFound)
	rec = traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/%d", leaving.ID), admin.AccessToken, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = traegerCall(t, router, http.MethodPut, path(leaving.ID), admin.AccessToken, map[string]any{"name": "Weg"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "a removed person takes no new name")

	changes := traegerNameChanges(t, db, organizationID, identityaccess.TraegerAccountNameChanged)
	require.Len(t, changes, 1, "only the successful change is written")
	assert.Equal(t, leader.ID, changes[0].Actor)
	require.NotNil(t, changes[0].Target)
	assert.Equal(t, billing.ID, *changes[0].Target)
	require.NotNil(t, changes[0].TargetName)
	assert.Equal(t, "Tobias Engel", *changes[0].TargetName)
	history := traegerHistory(t, router, admin.AccessToken, 0)
	require.NotEmpty(t, history.Entries)
	var named []string
	for _, entry := range history.Entries {
		if entry.Action == identityaccess.TraegerAccountNameChanged {
			named = append(named, entry.TargetName)
		}
	}
	assert.Equal(t, []string{"Tobias Engel"}, named, "the history shows the new name")
}

func TestTraegerMembersNameThemselvesWithoutTheAccountsRight(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	router, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	organizationID := traegerBuero(t, db, "Träger Eigener Name")
	clerk, session := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleBilling)
	type profile struct {
		AccountID int64  `json:"account_id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
	}
	read := func() profile {
		t.Helper()
		rec := traegerCall(t, router, http.MethodGet, "/traeger/profile/", session.AccessToken, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return decodeSchoolPortalData[profile](t, rec.Body.Bytes())
	}
	assert.Equal(t, profile{AccountID: clerk.ID, Email: clerk.Email}, read(), "no name yet")

	rec := traegerCall(t, router, http.MethodPut, "/traeger/profile/name", session.AccessToken, map[string]any{"name": "Jana\nKrause"})
	assertNameRefused(t, rec)
	rec = traegerCall(t, router, http.MethodPut, "/traeger/profile/name", session.AccessToken, map[string]any{"name": "Jana Krause"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, profile{AccountID: clerk.ID, Email: clerk.Email, Name: "Jana Krause"}, decodeSchoolPortalData[profile](t, rec.Body.Bytes()))
	assert.Equal(t, "Jana Krause", read().Name)
	rec = traegerCall(t, router, http.MethodGet, "/traeger/accounts/", session.AccessToken, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "naming oneself opens no member list")

	changes := traegerNameChanges(t, db, organizationID, identityaccess.TraegerAccountNameChanged)
	require.Len(t, changes, 1)
	assert.Equal(t, clerk.ID, changes[0].Actor, "the person named themselves")
	require.NotNil(t, changes[0].Target)
	assert.Equal(t, clerk.ID, *changes[0].Target)
	require.NotNil(t, changes[0].TargetName)
	assert.Equal(t, "Jana Krause", *changes[0].TargetName)

	_, err := db.ExecContext(context.Background(), `UPDATE platform.organization_accounts SET status = 'inactive', deactivated_at = NOW()
		WHERE organization_id = ? AND account_id = ?`, organizationID, clerk.ID)
	require.NoError(t, err)
	rec = traegerCall(t, router, http.MethodPut, "/traeger/profile/name", session.AccessToken, map[string]any{"name": "Noch da"})
	assert.Equal(t, http.StatusForbidden, rec.Code, "the token of a removed person names nobody")
	assert.Contains(t, rec.Body.String(), common.CodeTraegerNoMembership)
}
