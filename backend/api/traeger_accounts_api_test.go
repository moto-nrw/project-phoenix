// Router-level tests for the Träger-Büro account management (#4650): the
// invitation by mail and its acceptance, the roles and their rights, the
// last person with the role Verwaltung, the removal that ends the sessions,
// the 403 of a role without the right, the isolation between Träger and the
// ledger of every step.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// newTraegerAccountsPortal mounts /traeger as the serving root does and hands
// back the Identity & Access owner and runtime it was composed over.
func newTraegerAccountsPortal(t *testing.T) (*bun.DB, *traegerPortalHandler, identityaccess.Owner, tenant.UnitOfWork) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	unit := testpkg.TenantRuntime(t, db)
	portal, identity := traegerPortalOn(t, db, unit)
	return db, portal, identity, unit
}

// traegerBuero creates a Träger whose account-management rows the test
// removes itself: the organization fixture deletes an emptied organization
// with foreign-key triggers suspended, so nothing cascades.
func traegerBuero(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	organizationID := traegerOrganization(t, db, name, true)
	t.Cleanup(func() {
		ctx := context.Background()
		for _, statement := range []string{
			`DELETE FROM audit.traeger_account_changes WHERE organization_id = ?`,
			`DELETE FROM auth.traeger_invitations WHERE organization_id = ?`,
			`DELETE FROM auth.organization_account_roles WHERE organization_id = ?`,
			`DELETE FROM platform.organization_accounts WHERE organization_id = ?`,
			`DELETE FROM auth.tokens WHERE organization_id = ?`,
		} {
			_, err := db.ExecContext(ctx, statement, organizationID)
			require.NoError(t, err)
		}
	})
	return organizationID
}

// traegerMember creates an account with an active membership and signs it in.
func traegerMember(t *testing.T, db *bun.DB, router *traegerPortalHandler, organizationID int64, roles ...string) (*testpkg.AccountFixture, traegerLoginAnswer) {
	t.Helper()
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, db, password)
	testpkg.CreateTestTraegerMember(t, db, organizationID, account.ID, roles...)
	code, answer := traegerSignIn(t, router, account.Email, password, organizationID)
	require.Equal(t, http.StatusOK, code)
	return account, answer
}

func traegerCall(t *testing.T, router http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type traegerBueroAnswer struct {
	Members []struct {
		AccountID int64    `json:"account_id"`
		Email     string   `json:"email"`
		Roles     []string `json:"roles"`
	} `json:"members"`
	Invitations []struct {
		ID    int64    `json:"id"`
		Email string   `json:"email"`
		Roles []string `json:"roles"`
	} `json:"invitations"`
}

func traegerBueroList(t *testing.T, router http.Handler, token string) traegerBueroAnswer {
	t.Helper()
	rec := traegerCall(t, router, http.MethodGet, "/traeger/accounts/", token, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[traegerBueroAnswer](t, rec.Body.Bytes())
}

// invite invites the address and returns the link token from the mail.
// traegerInvite invites the address and returns the link token of the mail
// the invitation queued in Delivery's outbox with it.
func traegerInvite(t *testing.T, db *bun.DB, router http.Handler, token, address string, roles ...string) string {
	t.Helper()
	rec := traegerCall(t, router, http.MethodPost, "/traeger/accounts/invitations", token, map[string]any{"email": address, "roles": roles})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	invitation := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes())
	ctx := context.Background()
	const queued = `FROM platform.email_outbox WHERE tenant_id IS NULL AND kind = ? AND payload->>'Type' = 'traeger_invitation' AND (payload->>'ReferenceID')::bigint = ?`
	t.Cleanup(func() {
		_, err := db.ExecContext(ctx, `DELETE `+queued, owners.TransactionalMailKind, invitation.ID)
		require.NoError(t, err)
	})
	var mail struct {
		Recipient string `bun:"recipient"`
		Template  string `bun:"template"`
		Link      string `bun:"link"`
	}
	require.NoError(t, db.NewRaw(`SELECT payload->'Message'->'To'->>'Address' AS recipient, payload->'Message'->>'Template' AS template,
		payload->'Message'->'Content'->>'InvitationURL' AS link `+queued, owners.TransactionalMailKind, invitation.ID).Scan(ctx, &mail), "the invitation queues its mail")
	assert.Equal(t, address, mail.Recipient)
	assert.Equal(t, "traeger-invitation.html", mail.Template)
	link, err := url.Parse(mail.Link)
	require.NoError(t, err)
	assert.Equal(t, "traeger.invalid", link.Host, "the link opens the Träger-Portal")
	assert.Equal(t, "/invite", link.Path)
	return link.Query().Get("token")
}

func traegerLedger(t *testing.T, db *bun.DB, organizationID int64) []string {
	t.Helper()
	var actions []string
	require.NoError(t, db.NewRaw(`SELECT action FROM audit.traeger_account_changes WHERE organization_id = ? ORDER BY id`, organizationID).Scan(context.Background(), &actions))
	return actions
}

func TestTraegerInvitationCreatesAnAccountWithTheInvitedRole(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Einladung")
	_, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	address := fmt.Sprintf("neu-%d@test.local", testpkg.UniqueTestTenantID(t))

	token := traegerInvite(t, db, router, admin.AccessToken, address, identityaccess.TraegerRoleBilling)
	buero := traegerBueroList(t, router, admin.AccessToken)
	require.Len(t, buero.Invitations, 1, "the open invitation is listed")
	assert.Equal(t, address, buero.Invitations[0].Email)
	assert.Equal(t, []string{identityaccess.TraegerRoleBilling}, buero.Invitations[0].Roles)

	rec := traegerCall(t, router, http.MethodGet, "/traeger/invitations/"+token, "", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[struct {
		OrganizationName         string `json:"organization_name"`
		Email                    string `json:"email"`
		RequiresExistingPassword bool   `json:"requires_existing_password"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, "Träger Einladung", preview.OrganizationName)
	assert.Equal(t, address, preview.Email)
	assert.False(t, preview.RequiresExistingPassword, "a new address chooses a password")

	password := testpkg.NewTestPassword()
	rec = traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", map[string]any{"password": password, "confirm_password": testpkg.NewTestPassword()})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "the password is chosen twice")
	assert.Contains(t, rec.Body.String(), "identity.password_mismatch")
	rec = traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", map[string]any{"password": password, "confirm_password": password})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE email = ?`, address)
		require.NoError(t, err)
	})

	code, answer := traegerSignIn(t, router, address, password, 0)
	require.Equal(t, http.StatusOK, code, "the acceptance created the membership")
	claims := traegerTokenClaims(t, answer.AccessToken)
	assert.Equal(t, float64(organizationID), claims["org_id"])
	assert.ElementsMatch(t, []any{identityaccess.TraegerRightOverviewRead, identityaccess.TraegerRightBillingManage,
		identityaccess.TraegerRightFinanceExport, identityaccess.TraegerRightReportsRead}, claims["permissions"], "the role bundles the rights of the ADR")

	rec = traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", map[string]any{"password": password, "confirm_password": password})
	assert.Equal(t, http.StatusGone, rec.Code, "a link is redeemable once")
	assert.Empty(t, traegerBueroList(t, router, admin.AccessToken).Invitations, "the accepted invitation is no longer open")
	assert.Equal(t, []string{"invitation_created", "members_listed", "invitation_accepted", "members_listed"}, traegerLedger(t, db, organizationID))
}

func TestTraegerInvitationOfAnExistingAccountNeedsItsPassword(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Bestandskonto")
	_, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	password := testpkg.NewTestPassword()
	existing := traegerAccount(t, db, password)

	token := traegerInvite(t, db, router, admin.AccessToken, existing.Email, identityaccess.TraegerRoleReadOnly, identityaccess.TraegerRolePersonnel)
	rec := traegerCall(t, router, http.MethodGet, "/traeger/invitations/"+token, "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"requires_existing_password":true`)

	rec = traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", map[string]any{"password": testpkg.NewTestPassword()})
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "the invitation grants access, never another person's credentials")
	assert.Contains(t, rec.Body.String(), "identity.invalid_credentials")
	rec = traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", map[string]any{"password": password})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	code, answer := traegerSignIn(t, router, existing.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, traegerTokenClaims(t, answer.AccessToken)["permissions"], identityaccess.TraegerRightStaffManage)

	rec = traegerCall(t, router, http.MethodPost, "/traeger/accounts/invitations", admin.AccessToken, map[string]any{"email": existing.Email, "roles": []string{identityaccess.TraegerRoleBilling}})
	assert.Equal(t, http.StatusConflict, rec.Code, "a member is not invited again")
	assert.Contains(t, rec.Body.String(), "traeger.already_member")
}

func TestTraegerAccountsRequireTheAccountsRight(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Rechte")
	for _, role := range []string{identityaccess.TraegerRoleReadOnly, identityaccess.TraegerRoleBilling, identityaccess.TraegerRolePersonnel} {
		t.Run(role, func(t *testing.T) {
			_, session := traegerMember(t, db, router, organizationID, role)
			rec := traegerCall(t, router, http.MethodGet, "/traeger/accounts/", session.AccessToken, nil)
			assert.Equal(t, http.StatusForbidden, rec.Code, "a role without the right gets 403")
			rec = traegerCall(t, router, http.MethodPost, "/traeger/accounts/invitations", session.AccessToken, map[string]any{"email": "x@test.local", "roles": []string{identityaccess.TraegerRoleAdministration}})
			assert.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
	assert.Empty(t, traegerLedger(t, db, organizationID), "a refused request changes and reads nothing")
}

func TestTraegerAccountsStayInsideTheTraegerOfTheSession(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	own := traegerBuero(t, db, "Träger Eigen")
	foreign := traegerBuero(t, db, "Träger Fremd")
	_, admin := traegerMember(t, db, router, own, identityaccess.TraegerRoleAdministration)
	foreignMember, foreignAdmin := traegerMember(t, db, router, foreign, identityaccess.TraegerRoleAdministration)
	traegerInvite(t, db, router, foreignAdmin.AccessToken, fmt.Sprintf("fremd-%d@test.local", testpkg.UniqueTestTenantID(t)), identityaccess.TraegerRoleReadOnly)
	foreignInvitation := traegerBueroList(t, router, foreignAdmin.AccessToken).Invitations[0].ID

	buero := traegerBueroList(t, router, admin.AccessToken)
	require.Len(t, buero.Members, 1)
	assert.NotEqual(t, foreignMember.ID, buero.Members[0].AccountID, "no Träger sees the people of another Träger")
	assert.Empty(t, buero.Invitations)

	rec := traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", foreignMember.ID), admin.AccessToken, map[string]any{"roles": []string{identityaccess.TraegerRoleReadOnly}})
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "traeger.member_not_found")
	rec = traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/%d", foreignMember.ID), admin.AccessToken, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/invitations/%d", foreignInvitation), admin.AccessToken, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Len(t, traegerBueroList(t, router, foreignAdmin.AccessToken).Invitations, 1, "the foreign invitation stays open")
}

func TestTraegerKeepsItsLastAdministrator(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Verwaltung")
	first, firstSession := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)

	rec := traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/%d", first.ID), firstSession.AccessToken, nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "the last person with Verwaltung cannot remove themselves")
	assert.Contains(t, rec.Body.String(), "traeger.last_administrator")
	rec = traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", first.ID), firstSession.AccessToken, map[string]any{"roles": []string{identityaccess.TraegerRoleBilling}})
	assert.Equal(t, http.StatusConflict, rec.Code, "nor give the role away")

	second, _ := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	rec = traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/%d", first.ID), firstSession.AccessToken, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, "with a second person holding Verwaltung the first may leave")

	var active int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.organization_accounts WHERE organization_id = ? AND status = 'active'`, organizationID).Scan(context.Background(), &active))
	assert.Equal(t, 1, active)
	var sessions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE organization_id = ? AND account_id = ?`, organizationID, first.ID).Scan(context.Background(), &sessions))
	assert.Zero(t, sessions, "the removal ends the Träger sessions at once")
	ctx := tenant.WithUnitOfWork(context.Background(), unit)
	_, _, err := identity.RefreshTokenWithAudit(ctx, firstSession.RefreshToken, "", "", "")
	assert.Error(t, err, "the removed person's session cannot be renewed")
	assert.NotEqual(t, first.ID, second.ID)
	assert.Equal(t, []string{"member_removed"}, traegerLedger(t, db, organizationID))
}

func TestTraegerRoleChangeReachesTheSessionAtItsNextRefresh(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Rollenwechsel")
	_, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	colleague, session := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleReadOnly)
	assert.NotContains(t, traegerTokenClaims(t, session.AccessToken)["permissions"], identityaccess.TraegerRightStaffManage)

	rec := traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", colleague.ID), admin.AccessToken, map[string]any{"roles": []string{"unknown"}})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "only the Träger roles exist")
	rec = traegerCall(t, router, http.MethodPut, fmt.Sprintf("/traeger/accounts/%d/roles", colleague.ID), admin.AccessToken, map[string]any{"roles": []string{identityaccess.TraegerRolePersonnel, identityaccess.TraegerRoleReadOnly}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"roles":["personnel","read_only"]`)

	ctx := tenant.WithUnitOfWork(context.Background(), unit)
	access, _, err := identity.RefreshTokenWithAudit(ctx, session.RefreshToken, "", "", "")
	require.NoError(t, err)
	assert.Contains(t, traegerTokenClaims(t, access)["permissions"], identityaccess.TraegerRightStaffManage, "the refresh carries the new rights")

	var change struct {
		Before []string `bun:"roles_before,array"`
		After  []string `bun:"roles_after,array"`
	}
	require.NoError(t, db.NewRaw(`SELECT roles_before, roles_after FROM audit.traeger_account_changes WHERE organization_id = ? AND action = 'roles_changed'`, organizationID).
		Scan(context.Background(), &change))
	assert.Equal(t, []string{identityaccess.TraegerRoleReadOnly}, change.Before, "the ledger keeps the roles before")
	assert.Equal(t, []string{identityaccess.TraegerRolePersonnel, identityaccess.TraegerRoleReadOnly}, change.After, "and after the change")
}

func TestTraegerInvitationCanBeRevoked(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Widerruf")
	_, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	address := fmt.Sprintf("widerruf-%d@test.local", testpkg.UniqueTestTenantID(t))
	first := traegerInvite(t, db, router, admin.AccessToken, address, identityaccess.TraegerRoleReadOnly)
	second := traegerInvite(t, db, router, admin.AccessToken, address, identityaccess.TraegerRoleBilling)

	rec := traegerCall(t, router, http.MethodGet, "/traeger/invitations/"+first, "", nil)
	assert.Equal(t, http.StatusGone, rec.Code, "a new invitation spends the old one")
	invitations := traegerBueroList(t, router, admin.AccessToken).Invitations
	require.Len(t, invitations, 1)
	rec = traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/invitations/%d", invitations[0].ID), admin.AccessToken, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = traegerCall(t, router, http.MethodGet, "/traeger/invitations/"+second, "", nil)
	assert.Equal(t, http.StatusGone, rec.Code, "a revoked link opens nothing")
	rec = traegerCall(t, router, http.MethodGet, "/traeger/invitations/unbekannt", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, traegerLedger(t, db, organizationID), "invitation_revoked")
}

func TestTraegerAccountListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Kontenbudget")
	_, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	for range 2 {
		traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleReadOnly)
	}
	list := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		req := httptest.NewRequest(http.MethodGet, "/traeger/accounts/", nil)
		req.Header.Set("Authorization", "Bearer "+admin.AccessToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	three := list()
	traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleBilling)
	traegerMember(t, db, router, organizationID, identityaccess.TraegerRolePersonnel)
	assert.Len(t, list(), len(three), "the member list reads the same statements for any number of people")
	querybudgets.AssertQueryBudget(t, "api.traeger.accounts", three)
}

func TestTraegerInvitationExpiresWithTheSchoolInvitations(t *testing.T) {
	t.Parallel()
	db, router, identity, unit := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Ablauf")
	_, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	expired := traegerInvite(t, db, router, admin.AccessToken, fmt.Sprintf("alt-%d@test.local", testpkg.UniqueTestTenantID(t)), identityaccess.TraegerRoleReadOnly)
	open := traegerInvite(t, db, router, admin.AccessToken, fmt.Sprintf("neu-%d@test.local", testpkg.UniqueTestTenantID(t)), identityaccess.TraegerRoleReadOnly)
	_, err := db.ExecContext(context.Background(), `UPDATE auth.traeger_invitations SET expires_at = NOW() - INTERVAL '1 minute'
		WHERE organization_id = ? AND email LIKE 'alt-%'`, organizationID)
	require.NoError(t, err)

	rec := traegerCall(t, router, http.MethodGet, "/traeger/invitations/"+expired, "", nil)
	assert.Equal(t, http.StatusGone, rec.Code, "an expired link opens nothing")
	_, err = identity.DeleteExpiredSchoolInvitations(tenant.WithUnitOfWork(context.Background(), unit))
	require.NoError(t, err)

	var left []string
	require.NoError(t, db.NewRaw(`SELECT email FROM auth.traeger_invitations WHERE organization_id = ?`, organizationID).Scan(context.Background(), &left))
	assert.Len(t, left, 1, "the hourly cleanup deletes the expired invitation")
	rec = traegerCall(t, router, http.MethodGet, "/traeger/invitations/"+open, "", nil)
	assert.Equal(t, http.StatusOK, rec.Code, "an open invitation stays")
}

// A switched-off account cannot sign in, so it keeps nobody's Träger-Büro
// manageable: it does not count as a person with Verwaltung (quorum #4937).
func TestTraegerSwitchedOffAdministratorDoesNotCount(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Ausgeschaltet")
	usable, session := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	switchedOff, _ := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	_, err := db.ExecContext(context.Background(), `UPDATE auth.accounts SET active = FALSE WHERE id = ?`, switchedOff.ID)
	require.NoError(t, err)

	rec := traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/%d", usable.ID), session.AccessToken, nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "the last person who can sign in keeps Verwaltung")
	assert.Contains(t, rec.Body.String(), "traeger.last_administrator")

	rec = traegerCall(t, router, http.MethodDelete, fmt.Sprintf("/traeger/accounts/%d", switchedOff.ID), session.AccessToken, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code, "the switched-off account itself can be removed")
}

// The invitation link is no way around the login's lockout: refused
// passwords count like failed logins and lock the account (quorum #4937).
func TestTraegerInvitationCountsRefusedPasswordsLikeTheLogin(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Sperre")
	_, admin := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	password := testpkg.NewTestPassword()
	existing := traegerAccount(t, db, password)
	token := traegerInvite(t, db, router, admin.AccessToken, existing.Email, identityaccess.TraegerRoleReadOnly)

	for range 5 {
		rec := traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", map[string]any{"password": testpkg.NewTestPassword()})
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
	var lockedUntil *time.Time
	require.NoError(t, db.NewRaw(`SELECT password_locked_until FROM auth.accounts WHERE id = ?`, existing.ID).Scan(context.Background(), &lockedUntil))
	assert.NotNil(t, lockedUntil, "five refused passwords lock the account like five failed logins")

	rec := traegerCall(t, router, http.MethodPost, "/traeger/invitations/"+token+"/accept", "", map[string]any{"password": password})
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "a locked account is not accepted even with its password")
	rec = traegerCall(t, router, http.MethodGet, "/traeger/invitations/"+token, "", nil)
	assert.Equal(t, http.StatusOK, rec.Code, "the link stays open for a later try")
}

// A switched-off account keeps its access token until it expires; the account
// management refuses it at once (quorum #4937).
func TestTraegerAccountsRefuseASwitchedOffActor(t *testing.T) {
	t.Parallel()
	db, router, _, _ := newTraegerAccountsPortal(t)
	organizationID := traegerBuero(t, db, "Träger Abgeschaltet")
	actor, session := traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	traegerMember(t, db, router, organizationID, identityaccess.TraegerRoleAdministration)
	_, err := db.ExecContext(context.Background(), `UPDATE auth.accounts SET active = FALSE WHERE id = ?`, actor.ID)
	require.NoError(t, err)

	rec := traegerCall(t, router, http.MethodGet, "/traeger/accounts/", session.AccessToken, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "the token alone no longer reads the colleagues")
	assert.Contains(t, rec.Body.String(), "identity.account_inactive")
	rec = traegerCall(t, router, http.MethodPost, "/traeger/accounts/invitations", session.AccessToken, map[string]any{"email": "x@test.local", "roles": []string{identityaccess.TraegerRoleAdministration}})
	assert.Equal(t, http.StatusForbidden, rec.Code, "nor invites anybody")
	assert.Empty(t, traegerLedger(t, db, organizationID), "a refused actor changes and reads nothing")
}
