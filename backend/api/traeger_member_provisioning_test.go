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
	testpkg.AssertQueryBudget(t, "api.operator.traeger_members", three)
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
// with several roles, in the operator audit. The person accepts with a new
// password and signs in to the Träger-Portal without any school account.
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
	assert.Equal(t, []string{"administration", "billing"}, invitation.Roles, "several roles in one invitation")
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
		AND changes->>'email' = ? AND changes->'roles' = '["administration", "billing"]'::jsonb`, operatorID, invitation.ID, address).Scan(ctx, &audited))
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

	live := traegerOrganization(t, db, "Träger an", true)
	_, err := identity.InviteTraegerAccountForOperator(ctx, live, "an@test.local", []string{"administration"})
	require.ErrorIs(t, err, identityaccess.ErrTraegerTransactionRequired)
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.traeger_invitations WHERE organization_id = ?`, live).Scan(ctx, &stored))
	assert.Zero(t, stored, "outside the operator's unit nothing is stored")
}
