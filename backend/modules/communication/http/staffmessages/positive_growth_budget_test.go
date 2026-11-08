package staffmessaging_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	staffmessaging "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestStaffMessagePositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{tenant.ScopeTenant, tenant.ScopeSchool} {
		t.Run(scope, func(t *testing.T) {
			db := testpkg.SetupTestDB(t)
			testpkg.OwnTenant(t)
			module, settings, err := api.NewStaffMessagingTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
			require.NoError(t, err)
			require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), "operations.staff_messaging_enabled", true, nil, nil))
			_, viewer := testpkg.CreateTestStaffWithAccount(t, db, "Viewer", "Budget")
			testpkg.EnsureAccountTenant(t, db, viewer.ID, testpkg.Tenant(t))
			if scope == tenant.ScopeSchool {
				testpkg.AssignLehrkraftSystemRole(t, db, viewer.ID, testpkg.Tenant(t))
			}
			resource := staffmessaging.NewResource(module.StaffMessaging)
			router := resource.Router(api.ProtectedTenantRoutes)
			prefix := "api"
			if scope == tenant.ScopeSchool {
				router = resource.SchoolRouter(api.ProtectedSchoolRoutes)
				prefix = "school"
			}
			router = testpkg.MountedQueryBudgetRouter("/"+prefix+"/staff-messages", router)
			claims := sessiontoken.AppClaims{ID: int(viewer.ID), Sub: viewer.Email, Roles: []string{"user"}, TenantID: testpkg.Tenant(t), Scope: scope}
			token := testutil.MintTestJWT(t, claims)
			counter := testpkg.CaptureQueriesForContext(t, db)
			counter.Stop()
			request := func(method, path, body string, measured bool) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(counter.Context(testpkg.Ctx(t)))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				if measured {
					counter.Reset()
					counter.Start()
				}
				rec := testutil.ExecuteRequestForTest(t, router, req)
				counter.Stop()
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				return rec
			}
			// A foreign-school colleague grows outside the viewer's tenant and must
			// never appear in recipient or inbox DTOs.
			foreignTenant, _ := testpkg.CreateTestTenant(t, db)
			var threadIDs, counterpartIDs []string
			var messageIDs []string
			created := 0
			for _, n := range []int{3, 8} {
				for created < n {
					_, colleague := testpkg.CreateTestStaffWithAccount(t, db, "Colleague", fmt.Sprintf("%02d", created))
					testpkg.EnsureAccountTenant(t, db, colleague.ID, testpkg.Tenant(t))
					_, foreign := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", fmt.Sprintf("%02d", created))
					testpkg.EnsureAccountTenant(t, db, foreign.ID, foreignTenant)
					opened := decodeData[staffmessaging.ThreadDetailResponse](t, request("POST", "/threads/open", fmt.Sprintf(`{"account_id":%q}`, fmt.Sprint(colleague.ID)), false).Body.Bytes())
					threadIDs = append(threadIDs, opened.ThreadID)
					counterpartIDs = append(counterpartIDs, fmt.Sprint(colleague.ID))
					// Grow both the inbox and the message collection of the first thread.
					request("POST", "/threads/"+opened.ThreadID, fmt.Sprintf(`{"body":"inbox-%02d"}`, created), false)
					message := decodeData[staffmessaging.MessageResponse](t, request("POST", "/threads/"+threadIDs[0], fmt.Sprintf(`{"body":"detail-%02d"}`, created), false).Body.Bytes())
					messageIDs = append(messageIDs, message.ID)
					created++
				}
				if scope == tenant.ScopeTenant {
					recipients := decodeData[[]staffmessaging.RecipientResponse](t, request("GET", "/recipients", "", true).Body.Bytes())
					require.Len(t, recipients, n)
					for i, row := range recipients {
						require.Equal(t, counterpartIDs[i], row.AccountID)
						require.Equal(t, fmt.Sprintf("Colleague %02d", i), row.Name)
						require.Equal(t, "staff", row.RoleKind)
					}
					t.Logf("MEASURE %s recipients N%d=%d", prefix, n, counter.Total())
					querybudgets.AssertQueryBudget(t, "api.staff_messages.recipients.positive_growth", counter.Measurement(counter.Queries()))
				}
				inbox := decodeData[[]staffmessaging.InboxThreadResponse](t, request("GET", "/", "", true).Body.Bytes())
				require.Len(t, inbox, n)
				require.Equal(t, threadIDs[0], inbox[0].ThreadID)
				for i, row := range inbox {
					expectedIndex := n - i
					if i == 0 {
						expectedIndex = 0
					}
					require.Equal(t, threadIDs[expectedIndex], row.ThreadID)
					require.Contains(t, threadIDs, row.ThreadID)
					require.Equal(t, "direct", row.Kind)
					require.True(t, row.LastMessageMine)
					require.Zero(t, row.UnreadCount)
				}
				t.Logf("MEASURE %s inbox N%d=%d", prefix, n, counter.Total())
				if scope == tenant.ScopeTenant {
					querybudgets.AssertQueryBudget(t, "api.staff_messages.inbox.positive_growth", counter.Measurement(counter.Queries()))
				} else {
					querybudgets.AssertQueryBudget(t, "school.staff_messages.inbox.positive_growth", counter.Measurement(counter.Queries()))
				}
				detail := decodeData[staffmessaging.ThreadDetailResponse](t, request("GET", "/threads/"+threadIDs[0], "", true).Body.Bytes())
				require.Len(t, detail.Messages, n+1)
				require.Equal(t, threadIDs[0], detail.ThreadID)
				require.Equal(t, counterpartIDs[0], detail.CounterpartAccountID)
				require.Equal(t, "Colleague 00", detail.CounterpartName)
				require.Equal(t, "inbox-00", detail.Messages[0].Body)
				for i, id := range messageIDs {
					require.Equal(t, id, detail.Messages[i+1].ID)
					require.Equal(t, fmt.Sprintf("detail-%02d", i), detail.Messages[i+1].Body)
					require.Equal(t, fmt.Sprint(viewer.ID), detail.Messages[i+1].SenderAccountID)
				}
				t.Logf("MEASURE %s detail N%d=%d", prefix, n, counter.Total())
				if scope == tenant.ScopeTenant {
					querybudgets.AssertQueryBudget(t, "api.staff_messages.thread.positive_growth", counter.Measurement(counter.Queries()))
				} else {
					querybudgets.AssertQueryBudget(t, "school.staff_messages.thread.positive_growth", counter.Measurement(counter.Queries()))
				}
			}
		})
	}
}
