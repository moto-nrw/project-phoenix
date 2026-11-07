//go:build testhooks

package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type staffRecipientDTO struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	RoleKind  string `json:"role_kind"`
}

func TestPopulatedStaffRecipientsRealRoot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, settingModule, err := api.NewStaffMessagingTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	require.NoError(t, settingModule.Settings.SetValue(testpkg.Ctx(t), "operations.staff_messaging_enabled", true, nil, nil))
	_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "Self", "Viewer")
	testpkg.EnsureAccountTenant(t, db, viewer.ID, testpkg.Tenant(t))
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	_, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, otherTenant, "Foreign", "Recipient")
	testpkg.EnsureAccountTenant(t, db, foreignAccount.ID, otherTenant)
	router := api.NewPeopleStaffRecipientsBudgetTestRouter(module)
	claims := routetest.Claims{ID: int(viewer.ID), Sub: viewer.Email, Roles: []string{"user"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeTenant}
	token := routetest.MintTestJWT(t, claims)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	expected := []staffRecipientDTO{}
	var smallStatements int
	for _, target := range []int{3, 12} {
		for len(expected) < target {
			// Insert names in descending order so SQL, not insertion order,
			// determines the response order.
			name := fmt.Sprintf("Recipient-%02d", 12-len(expected))
			_, account := testpkg.CreateTestStaffWithAccount(t, db, "Staff", name)
			testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
			expected = append([]staffRecipientDTO{{AccountID: fmt.Sprint(account.ID), Name: "Staff " + name, RoleKind: "staff"}}, expected...)
		}
		req := httptest.NewRequest(http.MethodGet, "/recipients", nil).WithContext(counter.Context(testpkg.Ctx(t)))
		req.Header.Set("Authorization", "Bearer "+token)
		counter.Reset()
		counter.Start()
		rec := routetest.ExecuteRequestForTest(t, router, req)
		counter.Stop()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var response struct {
			Data []staffRecipientDTO `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Equal(t, expected, response.Data, "DTOs, sorted names, self exclusion and tenant isolation stay unchanged")
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
			require.Empty(t, counter.Operation(operation))
		}
		queries, err := json.Marshal(counter.Queries())
		require.NoError(t, err)
		t.Logf("populated_staff_recipients n=%d total=%d selects=%d persons=%d queries_json=%s", target, counter.Total(), len(counter.Operation("SELECT")), len(counter.Selects("users.persons")), queries)
		querybudgets.AssertQueryBudget(t, "modules.peopledirectory.staff_recipients.list", counter.Queries())
		if target == 3 {
			smallStatements = counter.Total()
		} else {
			require.Equal(t, smallStatements, counter.Total(), "recipient growth must not add roundtrips")
		}
	}
}

func TestStaffRecipientScopeErrorsRealRoot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	repositories, err := api.NewStaffMessagingTestRepositories(testpkg.NewDatabase(t, db), owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db)))
	require.NoError(t, err)
	viewer := testpkg.CreateTestAccount(t, db, "scope-recipients-viewer")
	rows, err := repositories.Read.ListAddressableStaff(context.Background(), viewer.ID)
	require.ErrorIs(t, err, tenant.ErrTenantRequired)
	require.Nil(t, rows)
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()
	rows, err = repositories.Read.ListAddressableStaff(ctx, viewer.ID)
	require.ErrorContains(t, err, "transaction is required", "database/scope resolution precedes cancellation")
	require.Nil(t, rows)
	_, err = testpkg.TenantQuery1(t, ctx, db, repositories.Read.ListAddressableStaff, viewer.ID)
	require.ErrorIs(t, err, context.Canceled)
}

func TestEmptyStaffRecipientCancellationAfterResolutionRealRoot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	viewer := testpkg.CreateTestAccount(t, db, "cancelled-recipients-viewer")
	lookupFailure := errors.New("active staff lookup failed")
	for _, lookupErr := range []error{nil, lookupFailure} {
		// Only the two external owner lookups are controlled here, so the
		// cancellation lands after resolution, at the removed SQL boundary.
		err := testpkg.WithinTenantTransaction(testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			ctx, cancel := context.WithCancel(txCtx)
			defer cancel()
			colleagues := owners.NewCommunicationStaffColleagues(testpkg.NewDatabase(t, db),
				func(context.Context) ([]int64, error) { return nil, nil },
				peopleCompose.StaffMessageIdentity{ActiveSchoolAccounts: func(context.Context, int64, []int64) ([]int64, error) {
					cancel()
					return nil, lookupErr
				}})
			rows, err := colleagues.ListAddressableStaff(ctx, viewer.ID)
			require.Nil(t, rows)
			return err
		})
		if lookupErr == nil {
			require.ErrorIs(t, err, context.Canceled)
			require.ErrorContains(t, err, "list messageable staff")
		} else {
			require.ErrorIs(t, err, lookupFailure, "owner lookup failure still takes precedence over cancellation")
			require.NotErrorIs(t, err, context.Canceled)
		}
	}
}

func TestEmptyStaffRecipientsRealRoot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, settingModule, err := api.NewStaffMessagingTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	require.NoError(t, settingModule.Settings.SetValue(testpkg.Ctx(t), "operations.staff_messaging_enabled", true, nil, nil))
	viewer := testpkg.CreateTestAccount(t, db, "empty-recipients-viewer")
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	_, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, otherTenant, "Foreign", "Recipient")
	testpkg.EnsureAccountTenant(t, db, foreignAccount.ID, otherTenant)
	router := api.NewPeopleStaffRecipientsBudgetTestRouter(module)
	claims := routetest.Claims{ID: int(viewer.ID), Sub: viewer.Email, Roles: []string{"user"}, TenantID: testpkg.Tenant(t), Scope: tenant.ScopeTenant}
	token := routetest.MintTestJWT(t, claims)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()

	count := 0
	for _, target := range []int{0, 3, 12} {
		for count < target {
			_, account := testpkg.CreateTestStaffWithAccount(t, db, "Inactive", fmt.Sprintf("Recipient-%02d", count))
			testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))
			_, err := db.NewRaw(`UPDATE auth.accounts SET active = false WHERE id = ?`, account.ID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			count++
		}
		req := httptest.NewRequest(http.MethodGet, "/recipients", nil).WithContext(counter.Context(testpkg.Ctx(t)))
		req.Header.Set("Authorization", "Bearer "+token)
		counter.Reset()
		counter.Start()
		rec := routetest.ExecuteRequestForTest(t, router, req)
		counter.Stop()
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var response struct {
			Data []staffRecipientDTO `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.NotNil(t, response.Data, "the empty recipient list remains an array")
		require.Empty(t, response.Data, "inactive staff and staff in another tenant are not recipients")
		require.Zero(t, counter.WriteRows())
		require.Empty(t, counter.Operation("INSERT"))
		require.Empty(t, counter.Operation("UPDATE"))
		require.Empty(t, counter.Operation("DELETE"))
		queries, err := json.Marshal(counter.Queries())
		require.NoError(t, err)
		t.Logf("empty_staff_recipients n=%d total=%d selects=%d persons=%d queries_json=%s", target, counter.Total(), len(counter.Operation("SELECT")), len(counter.Selects("users.persons")), queries)
		if target == 0 {
			querybudgets.AssertQueryBudget(t, "modules.peopledirectory.staff_recipients.empty_no_staff", counter.Queries())
		} else {
			querybudgets.AssertQueryBudget(t, "modules.peopledirectory.staff_recipients.empty_inactive_staff", counter.Queries())
		}
		assert.Empty(t, counter.Matching(func(query string) bool {
			return strings.Contains(query, "users.persons") && strings.Contains(query, "1 = 0")
		}), "an empty active-staff set has no person identities to hydrate")
	}
}
