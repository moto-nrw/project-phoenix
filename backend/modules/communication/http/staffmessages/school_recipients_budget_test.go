package staffmessaging_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	staffmessaging "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestSchoolStaffRecipientsQueryBudget(t *testing.T) {
	t.Parallel()
	runSchoolStaffRecipientsBudget(t, true)
}

func TestSchoolStaffRecipientsInactiveQueryBudget(t *testing.T) {
	t.Parallel()
	runSchoolStaffRecipientsBudget(t, false)
}

// N counts each growing fixture set, excluding the school-scope viewer:
// active recipients (when populated), disabled accounts, inactive mappings,
// deleted staff memberships and foreign-school staff. These are real rows,
// not repeated calls over the same fixture size.
func runSchoolStaffRecipientsBudget(t *testing.T, populated bool) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	module, settings, err := api.NewStaffMessagingTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), "operations.staff_messaging_enabled", true, nil, nil))
	_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "Self", "Viewer")
	testpkg.EnsureAccountTenant(t, db, viewer.ID, testpkg.Tenant(t))
	testpkg.AssignLehrkraftSystemRole(t, db, viewer.ID, testpkg.Tenant(t))
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	router := module.SchoolStaffMessagingRouterForTests()
	claims := sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, Roles: []string{"lehrkraft"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool}
	token := testutil.MintTestJWT(t, claims) // No permission is required on this public school-scope surface.
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	expected := []staffmessaging.RecipientResponse{}
	created, populatedStatements := 0, 0
	for _, target := range []int{0, 3, 8} {
		for created < target {
			if populated {
				recipient := createSchoolRecipient(t, db, created)
				expected = append([]staffmessaging.RecipientResponse{recipient}, expected...)
			}
			createExcludedSchoolRecipients(t, db, foreignTenant, created)
			created++
		}
		req := httptest.NewRequest(http.MethodGet, "/school/staff-messages/recipients", nil).WithContext(counter.Context(testpkg.Ctx(t)))
		req.Header.Set("Authorization", "Bearer "+token)
		counter.Reset()
		counter.Start()
		rec := testutil.ExecuteRequestForTest(t, router, req)
		counter.Stop()
		assertSchoolRecipientResponse(t, rec, expected)
		assertSchoolRecipientReadOnly(t, counter)
		queries, err := json.Marshal(counter.Queries())
		require.NoError(t, err)
		t.Logf("school_staff_recipients populated=%t n=%d total=%d queries_json=%s", populated, target, counter.Total(), queries)
		if populated && target > 0 {
			querybudgets.AssertQueryBudget(t, "school.staff_messages.recipients.list", counter.Measurement(counter.Queries()))
		} else {
			querybudgets.AssertQueryBudget(t, "school.staff_messages.recipients.empty", counter.Measurement(counter.Queries()))
		}
		switch target {
		case 3:
			populatedStatements = counter.Total()
		case 8:
			require.Equal(t, populatedStatements, counter.Total(), "growing recipient and exclusion sets must not add roundtrips")
		}
	}
}

func TestSchoolStaffRecipientsScopeErrors(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, _, err := api.NewStaffMessagingTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "School", "Viewer")
	router := module.SchoolStaffMessagingRouterForTests()
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	for _, scope := range []string{tenant.ScopeTenant, tenant.ScopeParent, tenant.ScopeOrg, tenant.ScopePlatform, tenant.ScopeCaterer} {
		claims := sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, Roles: []string{"user"}, TenantID: testpkg.Tenant(t), Scope: scope}
		req := httptest.NewRequest(http.MethodGet, "/school/staff-messages/recipients", nil).WithContext(counter.Context(testpkg.Ctx(t)))
		req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
		counter.Reset()
		counter.Start()
		rec := testutil.ExecuteRequestForTest(t, router, req)
		counter.Stop()
		require.Equal(t, http.StatusUnauthorized, rec.Code, "scope=%q body=%s", scope, rec.Body.String())
		require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
		require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen","title":"Unauthorized","status":401,"detail":"token unauthorized","instance":"","code":"general.permission"}`, rec.Body.String())
		require.Empty(t, counter.Queries(), "wrong-portal rejection precedes the tenant transaction")
	}
}

func TestSchoolStaffRecipientsDisabledError(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, settings, err := api.NewStaffMessagingTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), "operations.staff_messaging_enabled", false, nil, nil))
	_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "School", "Viewer")
	testpkg.EnsureAccountTenant(t, db, viewer.ID, testpkg.Tenant(t))
	claims := sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, Roles: []string{"lehrkraft"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeSchool}
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	req := httptest.NewRequest(http.MethodGet, "/school/staff-messages/recipients", nil).WithContext(counter.Context(testpkg.Ctx(t)))
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	counter.Reset()
	counter.Start()
	rec := testutil.ExecuteRequestForTest(t, module.SchoolStaffMessagingRouterForTests(), req)
	counter.Stop()
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
	require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-zugriff-pruefen","title":"Forbidden","status":403,"detail":"Der Team-Chat ist für diese Schule nicht eingeschaltet.","instance":"","code":"communication.staff_messaging_disabled"}`, rec.Body.String())
	assertSchoolRecipientReadOnly(t, counter)
	require.Empty(t, counter.Selects("users.persons"), "disabled feature must not load recipients")
}

func createSchoolRecipient(t *testing.T, db *bun.DB, index int) staffmessaging.RecipientResponse {
	t.Helper()
	// Descending insertion order must not determine the ascending wire order.
	name := fmt.Sprintf("Recipient-%02d", 8-index)
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "School", name)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	kind := "staff"
	if index%2 == 0 {
		testpkg.AssignLehrkraftSystemRole(t, db, account.ID, testpkg.Tenant(t))
		kind = "lehrkraft"
	}
	return staffmessaging.RecipientResponse{AccountID: fmt.Sprint(account.ID), Name: "School " + name, RoleKind: kind}
}

func createExcludedSchoolRecipients(t *testing.T, db *bun.DB, foreignTenant int64, index int) {
	t.Helper()
	name := fmt.Sprintf("Recipient-%02d", index)
	_, disabled := testpkg.CreateTestStaffWithAccount(t, db, "Disabled", name)
	testpkg.EnsureAccountTenant(t, db, disabled.ID, testpkg.Tenant(t))
	_, err := db.NewRaw(`UPDATE auth.accounts SET active = false WHERE id = ?`, disabled.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	_, inactive := testpkg.CreateTestStaffWithAccount(t, db, "Inactive", name)
	testpkg.EnsureAccountTenant(t, db, inactive.ID, testpkg.Tenant(t))
	_, err = db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive' WHERE account_id = ? AND tenant_id = ?`, inactive.ID, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	deleted, account := testpkg.CreateTestStaffWithAccount(t, db, "Deleted", name)
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
	_, err = db.NewRaw(`UPDATE users.staff_school_memberships SET deleted_at = NOW() WHERE id = ? AND tenant_id = ?`, deleted.ID, testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	_, foreign := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", name)
	testpkg.EnsureAccountTenant(t, db, foreign.ID, foreignTenant)
}

func assertSchoolRecipientResponse(t *testing.T, rec *httptest.ResponseRecorder, expected []staffmessaging.RecipientResponse) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body, err := json.Marshal(struct {
		Status  string                             `json:"status"`
		Data    []staffmessaging.RecipientResponse `json:"data"`
		Message string                             `json:"message"`
	}{"success", expected, "Recipients retrieved"})
	require.NoError(t, err)
	require.JSONEq(t, string(body), rec.Body.String(), "complete ordered DTOs, self exclusion and tenant isolation")
}

func assertSchoolRecipientReadOnly(t *testing.T, counter *testpkg.QueryCounter) {
	t.Helper()
	require.Zero(t, counter.WriteRows())
	for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, counter.Operation(operation))
	}
}
