// The operator assigns the first person of a Träger-Büro (ADR 0084, spec
// #4611 user stories 1 and 2, #4649): the membership and the Träger role
// commit together with the operator audit, and the person then signs in to
// the Träger-Portal and sees the schools of the Träger.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	provisioningAPI "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
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

	rec := call(http.MethodPost, members, map[string]string{"email": "niemand@test.local", "role": "administration"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"identity.account_not_found"`, "the person needs an account first")
	rec = call(http.MethodPost, members, map[string]string{"email": account.Email, "role": "chef"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "only the Träger roles of ADR 0084 exist")
	var granted int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.organization_accounts WHERE account_id = ?`, account.ID).Scan(ctx, &granted))
	assert.Zero(t, granted, "a refused role leaves no membership behind")

	rec = call(http.MethodPost, fmt.Sprintf("/organizations/%d/traeger-members", organizationID+100000), map[string]string{"email": account.Email, "role": "administration"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "an unknown Träger is refused before any grant: %s", rec.Body.String())

	rec = call(http.MethodPost, members, map[string]string{"email": account.Email, "role": "administration"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	member := decodeSchoolPortalData[struct {
		AccountID int64    `json:"account_id"`
		Email     string   `json:"email"`
		Status    string   `json:"status"`
		Roles     []string `json:"roles"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, account.ID, member.AccountID)
	assert.Equal(t, "active", member.Status)
	assert.Equal(t, []string{"administration"}, member.Roles)
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
		body := bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"role":"administration"}`, account.Email))
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
		body := bytes.NewBufferString(fmt.Sprintf(`{"email":%q,"role":"read_only"}`, account.Email))
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
