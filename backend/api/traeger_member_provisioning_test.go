// The operator assigns or invites the first person of a Träger-Büro (ADR
// 0084, spec #4611 user stories 1 and 2, #4649, #5144): the membership and
// the Träger roles, or the invitation, commit together with the operator
// audit, and the person then signs in to the Träger-Portal and sees the
// schools of the Träger.
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

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

func TestOperatorAssignsTheFirstPersonOfATraegerBuero(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, schoolID := provisionTestSchool(t, db, service, operatorID, "traeger-buero")
	password := testpkg.NewTestPassword()
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("buero-%d@test.local", testpkg.UniqueTestTenantID(t)), password)

	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		var payload bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&payload).Encode(body))
		}
		req := httptest.NewRequest(method, path, &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	members := fmt.Sprintf("/organizations/%d/traeger-members", organizationID)

	rec := call(http.MethodPost, members, map[string]any{"email": "niemand@test.local", "roles": []string{"administration"}})
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"identity.account_not_found"`, "the person needs an account first")
	rec = call(http.MethodPost, members, map[string]any{"email": account.Email, "roles": []string{"administration", "chef"}})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "only the Träger roles of ADR 0084 exist")
	rec = call(http.MethodPost, members, map[string]any{"email": account.Email, "roles": []string{}})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "at least one role")
	assert.Contains(t, rec.Body.String(), `"field":"roles"`)
	var granted int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.organization_accounts WHERE account_id = ?`, account.ID).Scan(ctx, &granted))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.organization_account_roles WHERE account_id = ?`, account.ID).Scan(ctx, &granted))
	assert.Zero(t, granted, "a refused role leaves no membership and no other role behind")

	rec = call(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members", organizationID+100000), map[string]any{"email": account.Email, "roles": []string{"administration"}})
	assert.Equal(t, http.StatusNotFound, rec.Code, "an unknown Träger is refused before any grant: %s", rec.Body.String())

	rec = call(http.MethodPost, members, map[string]any{"email": account.Email, "roles": []string{"billing", "administration", "billing"}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	member := decodeSchoolPortalData[struct {
		AccountID int64    `json:"account_id"`
		Email     string   `json:"email"`
		Status    string   `json:"status"`
		Roles     []string `json:"roles"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, account.ID, member.AccountID)
	assert.Equal(t, "active", member.Status)
	assert.ElementsMatch(t, []string{"administration", "billing"}, member.Roles, "several roles in one assignment (#5144)")
	var audited int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'grant' AND resource_type = 'traeger_member' AND resource_id = ?`, operatorID, account.ID).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "the grant is in the operator audit")

	rec = call(http.MethodGet, members, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), account.Email)
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'view' AND changes->>'view' = 'traeger_members'`, operatorID).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "reading the addresses is in the operator audit")

	traeger, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	code, answer := traegerSignIn(t, traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	listed, schools := traegerSchools(t, traeger, answer.AccessToken)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	require.Len(t, schools, 1)
	assert.Equal(t, schoolID, schools[0].ID, "the assigned person sees the school of the Träger")
}

func TestOperatorTraegerMemberListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := buildOperatorProvisioning(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-budget")
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	member := func() {
		account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("buero-%d@test.local", testpkg.UniqueTestTenantID(t)))
		testpkg.CreateTestTraegerMember(t, db, organizationID, account.ID, "read_only")
	}
	list := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/organizations/%d/traeger-members", organizationID), nil)
		req = req.WithContext(counter.Context(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"})))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	for range 3 {
		member()
	}
	three := list()
	member()
	member()
	assert.Len(t, list(), len(three), "the member list reads the same statements for any number of members")
	querybudgets.AssertQueryBudget(t, "api.operator.traeger_members", three)
}

// The erasure of unused accounts locks an account before it anonymizes it. A
// grant that meets that lock waits, and once the erasure commits it finds no
// live account: the erased account gets no membership and no Träger role.
func TestOperatorTraegerGrantLosesToAConcurrentErasure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-erasure")
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("erased-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())

	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)

	erasure, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = erasure.Rollback() }()
	var lockedID int64
	require.NoError(t, erasure.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID).Scan(ctx, &lockedID))

	granted := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		body := bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"roles":["administration"]}`, account.Email))
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members", organizationID), body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		granted <- rec
	}()

	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(ctx, &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 20*time.Millisecond, "the grant waits for the erasure's lock")
	_, err = erasure.ExecContext(ctx, `UPDATE auth.accounts SET personal_data_erased_at = now() WHERE id = ?`, account.ID)
	require.NoError(t, err)
	require.NoError(t, erasure.Commit())

	rec := <-granted
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"identity.account_not_found"`)
	var rows int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM platform.organization_accounts WHERE account_id = ?)
		+ (SELECT count(*) FROM auth.organization_account_roles WHERE account_id = ?)`, account.ID, account.ID).Scan(ctx, &rows))
	assert.Zero(t, rows, "an erased account gets neither membership nor Träger role")
}

// Two grants of the same account to two Träger must queue, not deadlock. The
// first grant is played by hand: it holds the key-share lock of its
// membership insert and then locks the account for its role. The second grant
// runs through the operator route in between; it must take the account lock
// before any key share, so the first one gets its lock and both commit.
func TestOperatorTraegerGrantsOfOneAccountQueueWithoutDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-queue")
	account := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("queue-%d@test.local", testpkg.UniqueTestTenantID(t)), testpkg.NewTestPassword())

	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)

	first, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = first.Rollback() }()
	var lockedID int64
	require.NoError(t, first.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR KEY SHARE`, account.ID).Scan(ctx, &lockedID))

	granted := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		body := bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"roles":["read_only"]}`, account.Email))
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members", organizationID), body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		granted <- rec
	}()

	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(ctx, &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 20*time.Millisecond, "the second grant waits for the first one's lock")
	require.NoError(t, first.NewRaw(`SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, account.ID).Scan(ctx, &lockedID),
		"the first grant gets the account lock; no deadlock aborts it")
	require.NoError(t, first.Commit())

	rec := <-granted
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

// The moto team invites the first person of a Träger-Büro without a moto
// account (spec #4611 user story 2, #5144): the Träger-Büro's own invitation
// with several roles, in the operator audit. The first person also gets
// Leitung (#5471). The person accepts with a new password and signs in to
// the Träger-Portal without any school account.
func TestOperatorInvitesTheFirstPersonOfATraegerBuero(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, schoolID := provisionTestSchool(t, db, service, operatorID, "traeger-einladung")
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	call := func(path string, body any) *httptest.ResponseRecorder {
		var payload bytes.Buffer
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
		req := httptest.NewRequest(http.MethodPost, path, &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	invitations := fmt.Sprintf("/organizations/%d/traeger-members/invitations", organizationID)
	address := fmt.Sprintf("erste-%d@test.local", testpkg.UniqueTestTenantID(t))

	rec := call(invitations, map[string]any{"email": "keine-adresse", "roles": []string{"administration"}})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"field":"email"`)
	rec = call(invitations, map[string]any{"email": address, "roles": []string{}})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "at least one role: %s", rec.Body.String())
	rec = call(invitations, map[string]any{"email": address, "roles": []string{"chef"}})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "only the Träger roles of ADR 0084: %s", rec.Body.String())
	rec = call(fmt.Sprintf("/organizations/%d/traeger-members/invitations", organizationID+100000), map[string]any{"email": address, "roles": []string{"administration"}})
	assert.Equal(t, http.StatusNotFound, rec.Code, "an unknown Träger: %s", rec.Body.String())

	rec = call(invitations, map[string]any{"email": address, "roles": []string{"billing", "administration"}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	invitation := decodeSchoolPortalData[struct {
		ID    int64    `json:"id"`
		Email string   `json:"email"`
		Roles []string `json:"roles"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, address, invitation.Email)
	assert.Equal(t, []string{"administration", "billing", "leadership"}, invitation.Roles, "several roles in one invitation, Leitung for the first person")
	assert.NotContains(t, rec.Body.String(), "token", "the link stays in the mail")
	var stored struct {
		CreatedBy *int64 `bun:"created_by"`
		Open      bool   `bun:"open"`
	}
	require.NoError(t, db.NewRaw(`SELECT created_by, used_at IS NULL AND revoked_at IS NULL AS open
		FROM auth.traeger_invitations WHERE id = ? AND organization_id = ?`, invitation.ID, organizationID).Scan(ctx, &stored))
	assert.Nil(t, stored.CreatedBy, "no account of the Träger-Büro invited")
	assert.True(t, stored.Open)
	var audited int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'create' AND resource_type = 'traeger_invitation' AND resource_id = ?
		AND changes->>'email' = ? AND changes->'roles' = '["administration", "billing", "leadership"]'::jsonb`, operatorID, invitation.ID, address).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "the invitation is in the operator audit with the operator as actor")

	var link string
	require.NoError(t, db.NewRaw(`SELECT payload->'Message'->'Content'->>'InvitationURL' FROM platform.email_outbox
		WHERE tenant_id IS NULL AND kind = ? AND payload->>'Type' = 'traeger_invitation' AND (payload->>'ReferenceID')::bigint = ?`,
		owners.TransactionalMailKind, invitation.ID).Scan(ctx, &link), "the Träger-Büro's invitation mail is queued")
	parsed, err := url.Parse(link)
	require.NoError(t, err)
	assert.Equal(t, "/invite", parsed.Path, "the link opens the acceptance of the Träger-Portal")

	traeger, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	password := testpkg.NewTestPassword()
	accepted := traegerCall(t, traeger, http.MethodPost, "/traeger/invitations/"+parsed.Query().Get("token")+"/accept", "", map[string]any{"password": password, "confirm_password": password})
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	code, answer := traegerSignIn(t, traeger, address, password, 0)
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, traegerTokenClaims(t, answer.AccessToken)["permissions"], identityaccess.TraegerRightAccountsManage, "the first person manages the accounts")
	listed, schools := traegerSchools(t, traeger, answer.AccessToken)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	require.Len(t, schools, 1)
	assert.Equal(t, schoolID, schools[0].ID)
	var schoolAccounts int
	require.NoError(t, db.NewRaw(`SELECT (SELECT count(*) FROM auth.account_tenants at JOIN auth.accounts a ON a.id = at.account_id WHERE a.email = ?)
		+ (SELECT count(*) FROM users.persons p JOIN auth.accounts a ON a.id = p.account_id WHERE a.email = ?)`, address, address).Scan(ctx, &schoolAccounts))
	assert.Zero(t, schoolAccounts, "the person has no school account and no person record")

	rec = call(invitations, map[string]any{"email": address, "roles": []string{"read_only"}})
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"traeger.already_member"`, "a member is not invited again")

	rec = call(invitations, map[string]any{"email": "zweite-" + address, "roles": []string{"read_only"}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"roles":["read_only"]`, "with a Leitung in the Träger-Büro the invitation keeps the chosen roles")
}

// A deactivated Träger gets no invitation and no mail (#5144). The identity
// command never opens a unit of its own: it refuses without the operator's
// administrative transaction.
func TestOperatorDoesNotInviteIntoADeactivatedTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, identity := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID := traegerOrganization(t, db, "Träger aus", false)
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	body := bytes.NewBufferString(`{"email":"aus@test.local","roles":["administration"]}`)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members/invitations", organizationID), body)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
	rec := httptest.NewRecorder()
	operator.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	var stored int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.traeger_invitations WHERE organization_id = ?`, organizationID).Scan(ctx, &stored))
	assert.Zero(t, stored)

	// An invitation from before the deactivation is listed as expired: its
	// link cannot be redeemed. It is neither sent again nor withdrawn (#5749).
	var invitationID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.traeger_invitations (organization_id, email, token, roles, expires_at)
		VALUES (?, 'vorher@test.local', ?, ARRAY['read_only']::VARCHAR(32)[], now() + interval '1 day') RETURNING id`,
		organizationID, fmt.Sprintf("vorher-%d", testpkg.UniqueTestTenantID(t))).Scan(ctx, &invitationID))
	operatorCall := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	invitations := fmt.Sprintf("/organizations/%d/traeger-members/invitations", organizationID)
	rec = operatorCall(http.MethodGet, invitations)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"expired":true`, rec.Body.String())
	rec = operatorCall(http.MethodPost, fmt.Sprintf("%s/%d/resend", invitations, invitationID))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	rec = operatorCall(http.MethodDelete, fmt.Sprintf("%s/%d", invitations, invitationID))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	live := traegerOrganization(t, db, "Träger an", true)
	_, err := identity.InviteTraegerAccountForOperator(ctx, live, "an@test.local", []string{"administration"})
	require.ErrorIs(t, err, identityaccess.ErrTraegerTransactionRequired)
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.traeger_invitations WHERE organization_id = ?`, live).Scan(ctx, &stored))
	assert.Zero(t, stored, "outside the operator's unit nothing is stored")
}

// Two invitations of the same address into an empty Träger-Büro must queue:
// there is no Verwaltung row to lock yet. The first invitation is played by
// hand: it holds the address lock and has inserted its open invitation. The
// operator's invitation of the same address waits for it, then replaces it
// instead of failing on the one-open-invitation index (#5144).
func TestOperatorInvitationsOfOneAddressQueueInAnEmptyTraegerBuero(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-einladung-queue")
	address := fmt.Sprintf("queue-%d@test.local", testpkg.UniqueTestTenantID(t))
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)

	first, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = first.Rollback() }()
	_, err = first.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("traeger_invitation:%d:%s", organizationID, address))
	require.NoError(t, err)
	_, err = first.ExecContext(ctx, `INSERT INTO auth.traeger_invitations (organization_id, email, token, roles, expires_at)
		VALUES (?, ?, ?, ARRAY['read_only']::VARCHAR(32)[], now() + interval '1 day')`, organizationID, address, fmt.Sprintf("first-%d", testpkg.UniqueTestTenantID(t)))
	require.NoError(t, err)

	invited := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		body := bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"roles":["administration"]}`, address))
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members/invitations", organizationID), body)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		invited <- rec
	}()

	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(ctx, &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 20*time.Millisecond, "the second invitation waits for the first one")
	require.NoError(t, first.Commit())

	rec := <-invited
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var open []string
	require.NoError(t, db.NewRaw(`SELECT array_to_string(roles, ',') FROM auth.traeger_invitations
		WHERE organization_id = ? AND email = ? AND used_at IS NULL AND revoked_at IS NULL`, organizationID, address).Scan(ctx, &open))
	assert.Equal(t, []string{"administration,leadership"}, open, "the later invitation replaces the earlier one, with Leitung in an empty Träger-Büro (#5471)")
}

// The operator sees the open invitations of a Träger-Büro, sends one again
// and withdraws one (#5749). An expired invitation stays listed until the
// cleanup deletes it; a withdrawn link can no longer be accepted, and an
// accepted invitation leaves the list for the member list.
func TestOperatorManagesTheOpenInvitationsOfATraegerBuero(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-offen")
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		var payload bytes.Buffer
		if body != nil {
			require.NoError(t, json.NewEncoder(&payload).Encode(body))
		}
		req := httptest.NewRequest(method, path, &payload)
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		return rec
	}
	type listed struct {
		ID        int64     `json:"id"`
		Email     string    `json:"email"`
		Roles     []string  `json:"roles"`
		ExpiresAt time.Time `json:"expires_at"`
		Expired   bool      `json:"expired"`
	}
	invitations := fmt.Sprintf("/organizations/%d/traeger-members/invitations", organizationID)
	list := func() []listed {
		rec := call(http.MethodGet, invitations, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "token", "the link stays in the mail")
		return decodeSchoolPortalData[[]listed](t, rec.Body.Bytes())
	}
	mailLink := func(invitationID int64) string {
		var link string
		require.NoError(t, db.NewRaw(`SELECT payload->'Message'->'Content'->>'InvitationURL' FROM platform.email_outbox
			WHERE tenant_id IS NULL AND kind = ? AND payload->>'Type' = 'traeger_invitation' AND (payload->>'ReferenceID')::bigint = ?`,
			owners.TransactionalMailKind, invitationID).Scan(ctx, &link))
		parsed, err := url.Parse(link)
		require.NoError(t, err)
		return parsed.Query().Get("token")
	}
	assert.Empty(t, list(), "nobody invited yet")

	first := fmt.Sprintf("erste-%d@test.local", testpkg.UniqueTestTenantID(t))
	second := "zweite-" + first
	rec := call(http.MethodPost, invitations, map[string]any{"email": first, "roles": []string{"administration"}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = call(http.MethodPost, invitations, map[string]any{"email": second, "roles": []string{"read_only"}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	open := list()
	require.Len(t, open, 2, "both invitations appear at once")
	byEmail := map[string]listed{open[0].Email: open[0], open[1].Email: open[1]}
	assert.Equal(t, []string{"administration", "leadership"}, byEmail[first].Roles)
	assert.Equal(t, []string{"leadership", "read_only"}, byEmail[second].Roles, "in an empty Träger-Büro every invitation adds Leitung")
	assert.False(t, byEmail[first].Expired)
	var audited int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'view' AND changes->>'view' = 'traeger_invitations'`, operatorID).Scan(ctx, &audited))
	assert.Positive(t, audited, "reading the addresses is in the operator audit")

	// The expired invitation stays listed and marked until the cleanup.
	_, err := db.ExecContext(ctx, `UPDATE auth.traeger_invitations SET expires_at = now() - interval '1 minute' WHERE id = ?`, byEmail[second].ID)
	require.NoError(t, err)
	open = list()
	require.Len(t, open, 2)
	for _, invitation := range open {
		assert.Equal(t, invitation.Email == second, invitation.Expired, invitation.Email)
	}

	// Sending again replaces the expired link by a fresh one with the same roles.
	staleToken := mailLink(byEmail[second].ID)
	rec = call(http.MethodPost, fmt.Sprintf("%s/%d/resend", invitations, byEmail[second].ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resent := decodeSchoolPortalData[listed](t, rec.Body.Bytes())
	assert.NotEqual(t, byEmail[second].ID, resent.ID)
	assert.Equal(t, second, resent.Email)
	assert.Equal(t, byEmail[second].Roles, resent.Roles, "the same roles as before")
	assert.False(t, resent.Expired)
	assert.True(t, resent.ExpiresAt.After(time.Now()))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'resend' AND resource_type = 'traeger_invitation' AND resource_id = ?`, operatorID, resent.ID).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "sending again is in the operator audit")
	freshToken := mailLink(resent.ID)
	assert.NotEqual(t, staleToken, freshToken, "the new mail carries a new link")
	open = list()
	require.Len(t, open, 2, "the old invitation gives way to the new one")
	rec = call(http.MethodPost, fmt.Sprintf("%s/%d/resend", invitations, byEmail[second].ID), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "a replaced invitation is not sent again: %s", rec.Body.String())

	// A withdrawn invitation leaves the list and its link no longer works.
	firstToken := mailLink(byEmail[first].ID)
	rec = call(http.MethodDelete, fmt.Sprintf("%s/%d", invitations, byEmail[first].ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'revoke' AND resource_type = 'traeger_invitation' AND resource_id = ? AND changes->>'email' = ?`,
		operatorID, byEmail[first].ID, first).Scan(ctx, &audited))
	assert.Equal(t, 1, audited, "the withdrawal is in the operator audit")
	open = list()
	require.Len(t, open, 1)
	assert.Equal(t, second, open[0].Email)
	rec = call(http.MethodDelete, fmt.Sprintf("%s/%d", invitations, byEmail[first].ID), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "withdrawn once: %s", rec.Body.String())
	rec = call(http.MethodDelete, fmt.Sprintf("/organizations/%d/traeger-members/invitations/%d", organizationID+100000, resent.ID), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "only the invitations of this Träger: %s", rec.Body.String())

	traeger, _ := traegerPortalOn(t, db, testpkg.TenantRuntime(t, db))
	password := testpkg.NewTestPassword()
	refused := traegerCall(t, traeger, http.MethodPost, "/traeger/invitations/"+firstToken+"/accept", "", map[string]any{"password": password, "confirm_password": password})
	assert.Equal(t, http.StatusGone, refused.Code, "a withdrawn invitation cannot be accepted: %s", refused.Body.String())
	refused = traegerCall(t, traeger, http.MethodPost, "/traeger/invitations/"+staleToken+"/accept", "", map[string]any{"password": password, "confirm_password": password})
	assert.Equal(t, http.StatusGone, refused.Code, "the replaced link cannot be accepted: %s", refused.Body.String())

	// Accepting moves the person from the invitations to the members.
	accepted := traegerCall(t, traeger, http.MethodPost, "/traeger/invitations/"+freshToken+"/accept", "", map[string]any{"password": password, "confirm_password": password})
	require.Equal(t, http.StatusOK, accepted.Code, accepted.Body.String())
	assert.Empty(t, list(), "an accepted invitation leaves the list")
	members := call(http.MethodGet, fmt.Sprintf("/organizations/%d/traeger-members", organizationID), nil)
	require.Equal(t, http.StatusOK, members.Code, members.Body.String())
	assert.Contains(t, members.Body.String(), second, "the person now stands among the members")
}

func TestOperatorTraegerInvitationListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-einladung-budget")
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	invite := func() {
		_, err := db.ExecContext(ctx, `INSERT INTO auth.traeger_invitations (organization_id, email, token, roles, expires_at)
			VALUES (?, ?, ?, ARRAY['read_only']::VARCHAR(32)[], now() + interval '1 day')`,
			organizationID, fmt.Sprintf("budget-%d@test.local", testpkg.UniqueTestTenantID(t)), fmt.Sprintf("budget-%d", testpkg.UniqueTestTenantID(t)))
		require.NoError(t, err)
	}
	list := func() []string {
		counter := testpkg.CaptureQueriesForContext(t, db)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/organizations/%d/traeger-members/invitations", organizationID), nil)
		req = req.WithContext(counter.Context(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"})))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	for range 3 {
		invite()
	}
	three := list()
	invite()
	invite()
	assert.Len(t, list(), len(three), "the invitation list reads the same statements for any number of invitations")
	querybudgets.AssertQueryBudget(t, "api.operator.traeger_invitations", three)
}

// A resend that meets another resend of the same invitation waits for the
// address lock, then finds the invitation replaced and sends nothing: the
// first resend's new link stays valid (#5749). The first resend is played by
// hand: it holds the address lock, has spent the invitation and inserted its
// replacement.
func TestOperatorResendDoesNotSpendAConcurrentReplacement(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-resend-queue")
	address := fmt.Sprintf("resend-%d@test.local", testpkg.UniqueTestTenantID(t))
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	var original int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.traeger_invitations (organization_id, email, token, roles, expires_at)
		VALUES (?, ?, ?, ARRAY['read_only']::VARCHAR(32)[], now() + interval '1 day') RETURNING id`,
		organizationID, address, fmt.Sprintf("original-%d", testpkg.UniqueTestTenantID(t))).Scan(ctx, &original))

	first, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = first.Rollback() }()
	_, err = first.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("traeger_invitation:%d:%s", organizationID, address))
	require.NoError(t, err)
	_, err = first.ExecContext(ctx, `UPDATE auth.traeger_invitations SET revoked_at = now() WHERE id = ?`, original)
	require.NoError(t, err)
	var replacement int64
	require.NoError(t, first.NewRaw(`INSERT INTO auth.traeger_invitations (organization_id, email, token, roles, expires_at)
		VALUES (?, ?, ?, ARRAY['read_only']::VARCHAR(32)[], now() + interval '1 day') RETURNING id`,
		organizationID, address, fmt.Sprintf("replacement-%d", testpkg.UniqueTestTenantID(t))).Scan(ctx, &replacement))

	resent := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members/invitations/%d/resend", organizationID, original), nil)
		req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
		rec := httptest.NewRecorder()
		operator.ServeHTTP(rec, req)
		resent <- rec
	}()

	require.Eventually(t, func() bool {
		var waiting int
		err := db.NewRaw(`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(ctx, &waiting)
		return err == nil && waiting > 0
	}, 10*time.Second, 20*time.Millisecond, "the second resend waits for the first one")
	require.NoError(t, first.Commit())

	rec := <-resent
	assert.Equal(t, http.StatusNotFound, rec.Code, "the invitation was already replaced: %s", rec.Body.String())
	var open []int64
	require.NoError(t, db.NewRaw(`SELECT id FROM auth.traeger_invitations
		WHERE organization_id = ? AND email = ? AND used_at IS NULL AND revoked_at IS NULL`, organizationID, address).Scan(ctx, &open))
	assert.Equal(t, []int64{replacement}, open, "the first resend's new link stays valid")
}

// Sending again keeps the stored roles: a read-only invitation gains no
// Leitung, also in a Träger-Büro without a Leitung (#5749).
func TestOperatorResendKeepsTheInvitedRoles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, _ := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, _ := provisionTestSchool(t, db, service, operatorID, "traeger-resend-rollen")
	var invitationID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.traeger_invitations (organization_id, email, token, roles, expires_at)
		VALUES (?, ?, ?, ARRAY['read_only']::VARCHAR(32)[], now() + interval '1 day') RETURNING id`,
		organizationID, fmt.Sprintf("lesen-%d@test.local", testpkg.UniqueTestTenantID(t)), fmt.Sprintf("lesen-%d", testpkg.UniqueTestTenantID(t))).Scan(ctx, &invitationID))
	resource := provisioningAPI.NewProvisioningResource(provisioningAPI.ProvisioningConfig{Service: service})
	operator := chi.NewRouter()
	mountOperatorProvisioningRoutes(operator, resource, nil)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members/invitations/%d/resend", organizationID, invitationID), nil)
	req = req.WithContext(context.WithValue(ctx, sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(operatorID), Scope: "platform"}))
	rec := httptest.NewRecorder()
	operator.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"roles":["read_only"]`)
}
