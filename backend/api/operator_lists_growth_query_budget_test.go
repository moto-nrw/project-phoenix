//go:build testhooks

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
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	provisioningHTTP "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/inbound/operator"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/inbound/operator"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// These route-sized routers use the serving root's real owners, operator JWT
// chain (including persisted session/active-operator checks), access audits and
// owner transactions/RLS. Serve-host routing and its outer middleware are not
// measured. Fixture writes and token issuance are outside each captured request.
func operatorListBudgetRouter(t *testing.T, db *bun.DB) (chi.Router, identityaccess.Owner, string, int64) {
	t.Helper()
	provisioning, identity := buildOperatorProvisioningWithIdentity(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	auth, err := NewConfiguredSessionAuth(viper.GetString("auth_jwt_secret"), viper.GetDuration("auth_jwt_expiry"), viper.GetDuration("auth_jwt_refresh_expiry"))
	require.NoError(t, err)
	token, _, err := identity.IssueTokensForAuthenticatedOperator(provisioningContext(t, db), operatorID, "127.0.0.1", "operator-list-budget")
	require.NoError(t, err)
	routes := newOperatorIdentity(auth, identity).routes()
	resource := provisioningHTTP.NewProvisioningResource(provisioningHTTP.ProvisioningConfig{Service: provisioning})
	router := chi.NewRouter()
	router.Use(routes.sessions.Operator(ReadOnlyPreviewMiddleware, SecurityPrincipalMiddleware)...)
	router.Route("/operator", func(r chi.Router) {
		mountOperatorProvisioningRoutes(r, resource, nil)
		module, lifecycle, err := NewOperatorSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		mountOperatorSchoolRoutes(r, resource, routes.mfaAdmin, &operatorSchoolSettings{
			values:                 settingsHTTP.NewSettingsResource(settingsHTTP.SettingsConfig{Settings: module.OperatorSchoolSettings(module.SettingsSideEffects.Dispatch), OperatorID: operatorIDFromSession}),
			bookingAuthorityImpact: newBookingAuthorityImpact(lifecycle),
		}, nil)
		routes.mountAccountTenantAccessRoutes(r, resource)
		routes.mountPasskeyRoutes(r)
		routes.mountTrustedDeviceRoutes(r)
	})
	return router, identity, token, operatorID
}

func operatorBudgetRows(t *testing.T, ctx context.Context, router http.Handler, token, path string) []map[string]any {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, "%s: %s", path, response.Body.String())
	var body json.RawMessage = response.Body.Bytes()
	if len(body) > 0 && body[0] == '{' {
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(body, &envelope))
		body = envelope.Data
	}
	var rows []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	require.NoError(t, decoder.Decode(&rows), path)
	return rows
}

func operatorBudgetIDs(t *testing.T, rows []map[string]any, field string) []string {
	t.Helper()
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		require.Contains(t, row, field)
		ids = append(ids, fmt.Sprint(row[field]))
	}
	return ids
}

func TestOperatorDirectoryListsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.EnsureTestTenant(t, db, testpkg.Tenant(t))
	router, _, token, operatorID := operatorListBudgetRouter(t, db)
	provisioning := buildOperatorProvisioning(t, db)
	org, school := provisionTestSchool(t, db, provisioning, operatorID, "list-budget")
	ctx := provisioningContext(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	type route struct {
		path, scenario, field string
		expected              *[]string
	}
	var organizations, schools, accounts, persons, devices, roles []string
	cases := []route{
		{"/operator/accounts", "operator.lists.accounts", "account_id", &accounts},
		{"/operator/organizations/", "operator.lists.organizations", "id", &organizations},
		{"/operator/organizations/summaries", "operator.lists.organization_summaries", "id", &organizations},
		{fmt.Sprintf("/operator/organizations/%d/accounts", org), "operator.lists.organization_accounts", "account_id", &accounts},
		{fmt.Sprintf("/operator/organizations/%d/devices", org), "operator.lists.organization_devices", "id", &devices},
		{fmt.Sprintf("/operator/organizations/%d/persons", org), "operator.lists.organization_persons", "id", &persons},
		{fmt.Sprintf("/operator/organizations/%d/schools", org), "operator.lists.organization_schools", "id", &schools},
		{"/operator/roles", "operator.lists.roles", "id", &roles},
		{"/operator/schools/", "operator.lists.schools", "id", &schools},
		{fmt.Sprintf("/operator/schools/%d/accounts", school), "operator.lists.school_accounts", "account_id", &accounts},
		{fmt.Sprintf("/operator/schools/%d/devices", school), "operator.lists.school_devices", "id", &devices},
		{fmt.Sprintf("/operator/schools/%d/persons", school), "operator.lists.school_persons", "id", &persons},
	}
	baseline := map[string]int{}
	for _, tc := range cases {
		baseline[tc.path] = len(operatorBudgetRows(t, ctx, router, token, tc.path))
	}
	for _, n := range []int{3, 8} {
		for len(accounts) < n {
			i := len(accounts)
			otherOrg, _ := provisionTestSchool(t, db, provisioning, operatorID, fmt.Sprintf("list-other-%d", i))
			organizations = append(organizations, fmt.Sprint(otherOrg))
			input := newInput(provisioning.CreateSchool)
			input.OrganizationID, input.Active = org, true
			input.Name = fmt.Sprintf("Budget school %d", i)
			input.Slug = fmt.Sprintf("list-school-%d-%d", school, i)
			input.Subdomain = input.Slug
			input.Email = input.Slug + "@example.test"
			created, err := provisioning.CreateSchool(ctx, input, operatorID, provisioningTestClientIP)
			require.NoError(t, err)
			schools = append(schools, fmt.Sprint(created.ID))
			account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("list-account-%d", i))
			testpkg.EnsureAccountTenant(t, db, account.ID, school)
			accounts = append(accounts, fmt.Sprint(account.ID))
			person := testpkg.CreateTestPersonForTenant(t, db, school, "Budget", fmt.Sprintf("Person%d", i))
			persons = append(persons, fmt.Sprint(person.ID))
			device := testpkg.CreateTestDeviceForTenant(t, db, school, fmt.Sprintf("LIST-%d-%d", school, i))
			devices = append(devices, fmt.Sprint(device.ID))
			role := testpkg.CreateTestSystemRole(t, db, "user")
			roles = append(roles, fmt.Sprint(role.ID))
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/N%d", tc.scenario, n), func(t *testing.T) {
				counter.Reset()
				rows := operatorBudgetRows(t, counter.Context(ctx), router, token, tc.path)
				statements := counter.Queries()
				delta := n
				// Global schools grow by 2N (N in each organization group).
				// Accounts have two school mappings; organization devices include
				// the new schools' web-manual devices as well as our N devices.
				if tc.scenario == "operator.lists.schools" || tc.scenario == "operator.lists.accounts" || tc.scenario == "operator.lists.organization_devices" {
					delta = 2 * n
				}
				require.Len(t, rows, baseline[tc.path]+delta)
				ids := operatorBudgetIDs(t, rows, tc.field)
				for _, id := range *tc.expected {
					require.Contains(t, ids, id)
				}
				t.Logf("MEASURE %s N%d statements=%d", tc.scenario, n, len(statements))
				assertOperatorListBudget(t, tc.scenario, counter.Measurement(statements))
			})
		}
	}
}

func TestOperatorIdentityListsPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	router, _, token, operatorID := operatorListBudgetRouter(t, db)
	provisioning := buildOperatorProvisioning(t, db)
	_, school := provisionTestSchool(t, db, provisioning, operatorID, "identity-list")
	account := testpkg.CreateTestAccount(t, db, "list-tenant-owner")
	ctx := provisioningContext(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	var tenants, roles, passkeys, devices []string
	rolesPath := fmt.Sprintf("/operator/accounts/%d/tenants/%d/roles", account.ID, school)
	roleBaseline := len(operatorBudgetRows(t, ctx, router, token, rolesPath))
	for _, n := range []int{3, 8} {
		for len(tenants) < n {
			i := len(tenants)
			tenantID := testpkg.Tenant(t)
			if i > 0 {
				_, tenantID = provisionTestSchool(t, db, provisioning, operatorID, fmt.Sprintf("identity-tenant-%d", i))
			}
			testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
			tenants = append(tenants, fmt.Sprint(tenantID))
			role := testpkg.CreateTestRoleForTenant(t, db, "list-user", school)
			roles = append(roles, fmt.Sprint(role.ID))
			// The rows the listings read; their records are owner-private.
			var keyID, deviceID int64
			require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_passkey_credentials (operator_id, user_handle, credential_id, credential_json, name) VALUES (?, ?, ?, ?, ?) RETURNING id`,
				operatorID, []byte("budget-operator"), []byte(fmt.Sprintf("budget-%d-%d", operatorID, i)), `{"id":"registered"}`, fmt.Sprintf("Key %d", i)).Scan(ctx, &keyID))
			passkeys = append(passkeys, fmt.Sprint(keyID))
			require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_mfa_trusted_devices (operator_id, token_hash, expires_at) VALUES (?, ?, ?) RETURNING id`,
				operatorID, fmt.Sprintf("budget-token-%d-%d", operatorID, i), time.Now().Add(time.Hour)).Scan(ctx, &deviceID))
			devices = append(devices, fmt.Sprint(deviceID))
		}
		for _, tc := range []struct {
			path, scenario, field string
			ids                   []string
			baseline              int
		}{
			{fmt.Sprintf("/operator/accounts/%d/tenants", account.ID), "operator.lists.account_tenants", "tenant_id", tenants, 0},
			{fmt.Sprintf("/operator/accounts/%d/tenants/", account.ID), "operator.lists.account_tenants_slash", "tenant_id", tenants, 0},
			{rolesPath, "operator.lists.account_tenant_roles", "id", roles, roleBaseline},
			{"/operator/auth/passkeys", "operator.lists.passkeys", "id", passkeys, 0},
			{"/operator/auth/passkeys/", "operator.lists.passkeys_slash", "id", passkeys, 0},
			{"/operator/auth/mfa/trusted-devices", "operator.lists.trusted_devices", "id", devices, 0},
		} {
			t.Run(fmt.Sprintf("%s/N%d", tc.scenario, n), func(t *testing.T) {
				counter.Reset()
				rows := operatorBudgetRows(t, counter.Context(ctx), router, token, tc.path)
				statements := counter.Queries()
				require.Len(t, rows, tc.baseline+n)
				ids := operatorBudgetIDs(t, rows, tc.field)
				for _, id := range tc.ids {
					require.Contains(t, ids, id)
				}
				t.Logf("MEASURE %s N%d statements=%d", tc.scenario, n, len(statements))
				assertOperatorListBudget(t, tc.scenario, counter.Measurement(statements))
			})
		}
	}
}

// N counts children without bookings, which must be reported as blocking.
// This is not a claim about the cost of every possible booking history.
func TestOperatorBookingImpactPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	router, _, token, _ := operatorListBudgetRouter(t, db)
	ctx := provisioningContext(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	var students []string
	for _, n := range []int{3, 8} {
		for len(students) < n {
			student := testpkg.CreateTestStudent(t, db, "Budget", fmt.Sprintf("Child%d", len(students)), "2a")
			students = append(students, fmt.Sprint(student.ID))
		}
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			path := fmt.Sprintf("/operator/schools/%d/settings/booking-authority-impact", testpkg.Tenant(t))
			request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(counter.Context(ctx))
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			counter.Reset()
			testpkg.MeasuredHTTPRequest(counter, router, response, request)
			statements := counter.Queries()
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var body struct {
				Data struct {
					Blocking []map[string]any `json:"blocking_children"`
					Planned  []map[string]any `json:"planned_completions"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Len(t, body.Data.Blocking, n)
			require.Empty(t, body.Data.Planned)
			require.ElementsMatch(t, students, operatorBudgetIDs(t, body.Data.Blocking, "student_id"))
			t.Logf("MEASURE operator.lists.booking_authority_impact N%d statements=%d", n, len(statements))
			querybudgets.AssertQueryBudget(t, "operator.lists.booking_authority_impact", counter.Measurement(statements))
		})
	}
}

// Literal calls keep every table case visible to the budget-register ratchet.
func assertOperatorListBudget(t *testing.T, scenario string, statements querybudgets.Measurement) {
	t.Helper()
	switch scenario {
	case "operator.lists.account_tenant_roles":
		querybudgets.AssertQueryBudget(t, "operator.lists.account_tenant_roles", statements)
	case "operator.lists.account_tenants":
		querybudgets.AssertQueryBudget(t, "operator.lists.account_tenants", statements)
	case "operator.lists.account_tenants_slash":
		querybudgets.AssertQueryBudget(t, "operator.lists.account_tenants_slash", statements)
	case "operator.lists.accounts":
		querybudgets.AssertQueryBudget(t, "operator.lists.accounts", statements)
	case "operator.lists.organization_accounts":
		querybudgets.AssertQueryBudget(t, "operator.lists.organization_accounts", statements)
	case "operator.lists.organization_devices":
		querybudgets.AssertQueryBudget(t, "operator.lists.organization_devices", statements)
	case "operator.lists.organization_persons":
		querybudgets.AssertQueryBudget(t, "operator.lists.organization_persons", statements)
	case "operator.lists.organization_schools":
		querybudgets.AssertQueryBudget(t, "operator.lists.organization_schools", statements)
	case "operator.lists.organization_summaries":
		querybudgets.AssertQueryBudget(t, "operator.lists.organization_summaries", statements)
	case "operator.lists.organizations":
		querybudgets.AssertQueryBudget(t, "operator.lists.organizations", statements)
	case "operator.lists.passkeys":
		querybudgets.AssertQueryBudget(t, "operator.lists.passkeys", statements)
	case "operator.lists.passkeys_slash":
		querybudgets.AssertQueryBudget(t, "operator.lists.passkeys_slash", statements)
	case "operator.lists.roles":
		querybudgets.AssertQueryBudget(t, "operator.lists.roles", statements)
	case "operator.lists.school_accounts":
		querybudgets.AssertQueryBudget(t, "operator.lists.school_accounts", statements)
	case "operator.lists.school_devices":
		querybudgets.AssertQueryBudget(t, "operator.lists.school_devices", statements)
	case "operator.lists.school_persons":
		querybudgets.AssertQueryBudget(t, "operator.lists.school_persons", statements)
	case "operator.lists.schools":
		querybudgets.AssertQueryBudget(t, "operator.lists.schools", statements)
	case "operator.lists.trusted_devices":
		querybudgets.AssertQueryBudget(t, "operator.lists.trusted_devices", statements)
	default:
		t.Fatalf("unknown operator list scenario: %s", scenario)
	}
}
