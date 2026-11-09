package messaging_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	compose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/http/parentmessages"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { testpkg.PerTestTenants(); testpkg.Run(m) }

func TestParentMessagePositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	repos := api.NewParentMessagingSuiteRepositories(database)
	people := api.NewPeopleRepositorySuiteFactory(database)
	accounts := api.NewAccountSuiteRepositories(database)
	directory := api.NewTestPersonDirectory(database, api.TestPersonDirectorySources{Persons: people.Person, Students: people.Student, Staff: people.Staff, AccountExists: api.AccountExists(accounts.Profile)})
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), "operations.parent_notes_enabled", true, nil, nil))
	identity := owners.NewPeopleIdentityAccess(database, nil)
	persons := owners.ParentMessagePeopleWithLeaders{ParentMessagePeople: owners.BindParentMessagePeople(directory), ParentMessageLeadership: owners.BindParentMessageLeadership(people.Staff, identity)}
	svc, groups := compose.NewParentMessaging(compose.ParentMessagingConfig{ThreadRepo: repos.ParentMessageThread, MessageRepo: parentStore.NewParentMessageRepository(db), ReadRepo: repos.ParentMessageRead, Persons: persons, UserContext: owners.StaffStudentReads{}, Session: api.CommunicationSession{}, Settings: settingsValues.NewParentMessagingValues(settings.Settings), Broadcaster: realtime.NewHub(slog.Default()), DB: db, Logger: slog.Default(), Observe: func(compose.Observation) {}, GroupAudience: owners.NewParentGroupAudience(database)}, owners.CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	_, staff := testpkg.CreateTestStaffWithAccount(t, db, "Budget", "Staff")
	testpkg.EnsureAccountTenant(t, db, staff.ID, testpkg.Tenant(t))
	token := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(staff.ID), Sub: staff.Email, Roles: []string{"admin"}, Permissions: []string{"admin:*"}, TenantID: testpkg.Tenant(t)})
	router := messaging.NewResource(svc, groups).Router(api.ProtectedTenantRoutes)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	request := func(method, path, body string, measure bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(counter.Context(testpkg.Ctx(t)))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if measure {
			counter.Reset()
			counter.Start()
		}
		rec := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/api/messages", router), req)
		counter.Stop()
		if measure {
			require.Equal(t, 200, rec.Code, rec.Body.String())
		} else {
			require.Contains(t, []int{200, 201}, rec.Code, rec.Body.String())
		}
		return rec
	}
	var threadIDs, guardianIDs []string
	var groupID string
	created := 0
	for _, n := range []int{3, 8} {
		for created < n {
			guardian := chain
			if created > 0 {
				guardian = testpkg.CreateTestCoGuardianForStudent(t, db, chain.StudentID, "Guardian", fmt.Sprintf("%02d", created))
			}
			opened := request("POST", "/threads", fmt.Sprintf(`{"student_id":%q,"guardian_account_id":%q,"body":"message-%02d"}`, fmt.Sprint(chain.StudentID), fmt.Sprint(guardian.AccountID), created), false)
			var detail struct {
				Data messaging.ThreadDetailResponse
			}
			require.NoError(t, json.Unmarshal(opened.Body.Bytes(), &detail))
			threadIDs = append(threadIDs, detail.Data.ThreadID)
			guardianIDs = append(guardianIDs, fmt.Sprint(guardian.AccountID))
			if created > 0 {
				request("POST", "/threads/"+threadIDs[0], fmt.Sprintf(`{"body":"detail-%02d"}`, created), false)
			}
			// Grow the class audience with independent families, not the same IDs.
			if created > 0 {
				testpkg.CreateTestParentGuardianChain(t, db)
			}
			created++
		}
		groupRec := request("POST", "/group-threads", `{"target_type":"class","target_id":"1a","target_label":"Class 1a","title":"Growth","body":"group-00"}`, false)
		var groupOut struct {
			Data messaging.GroupThreadDetailResponse
		}
		require.NoError(t, json.Unmarshal(groupRec.Body.Bytes(), &groupOut))
		groupID = groupOut.Data.Thread.ID
		for i := 1; i < n; i++ {
			request("POST", "/group-threads/"+groupID+"/messages", fmt.Sprintf(`{"body":"group-%02d"}`, i), false)
		}
		for _, entry := range []struct{ name, path string }{{"guardians", fmt.Sprintf("/students/%d/guardians", chain.StudentID)}, {"student_threads", fmt.Sprintf("/students/%d/threads", chain.StudentID)}, {"thread", "/threads/" + threadIDs[0]}, {"group_audience", "/group-threads/audience?target_type=class&target_id=1a"}, {"group_thread", "/group-threads/" + groupID}} {
			rec := request("GET", entry.path, "", true)
			t.Logf("MEASURE parent_messages %s N%d=%d", entry.name, n, counter.Total())
			switch entry.name {
			case "guardians":
				var out struct{ Data []messaging.GuardianResponse }
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data, n)
				for _, row := range out.Data {
					require.Contains(t, guardianIDs, row.AccountID)
				}
			case "student_threads":
				var out struct {
					Data []messaging.InboxThreadResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data, n)
				for _, row := range out.Data {
					require.Contains(t, threadIDs, row.ThreadID)
					require.Equal(t, fmt.Sprint(chain.StudentID), row.StudentID)
				}
			case "thread":
				var out struct {
					Data messaging.ThreadDetailResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data.Messages, n)
				require.Equal(t, threadIDs[0], out.Data.ThreadID)
				for i, row := range out.Data.Messages {
					body := fmt.Sprintf("detail-%02d", i)
					if i == 0 {
						body = "message-00"
					}
					require.Equal(t, body, row.Body)
					require.NotEmpty(t, row.ID)
				}
			case "group_audience":
				var out struct {
					Data []messaging.GroupChildResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data, n)
				for _, row := range out.Data {
					require.Equal(t, "1a", row.SchoolClass)
					require.True(t, row.Reachable)
				}
			case "group_thread":
				var out struct {
					Data messaging.GroupThreadDetailResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data.Messages, n)
				require.Len(t, out.Data.Children, n)
				for i, row := range out.Data.Messages {
					require.Equal(t, fmt.Sprintf("group-%02d", i), row.Body)
					require.NotEmpty(t, row.ID)
				}
			}
			switch entry.name {
			case "guardians":
				querybudgets.AssertQueryBudget(t, "api.parent_messages.guardians.positive_growth", counter.Measurement(counter.Queries()))
			case "student_threads":
				querybudgets.AssertQueryBudget(t, "api.parent_messages.student_threads.positive_growth", counter.Measurement(counter.Queries()))
			case "thread":
				querybudgets.AssertQueryBudget(t, "api.parent_messages.thread.positive_growth", counter.Measurement(counter.Queries()))
			case "group_audience":
				querybudgets.AssertQueryBudget(t, "api.parent_messages.group_audience.positive_growth", counter.Measurement(counter.Queries()))
			case "group_thread":
				querybudgets.AssertQueryBudget(t, "api.parent_messages.group_thread.positive_growth", counter.Measurement(counter.Queries()))
			}
		}
	}
	// RLS: another school's authenticated administrator cannot read a thread.
	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	_, foreign := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenant, "Foreign", "Staff")
	testpkg.EnsureAccountTenant(t, db, foreign.ID, foreignTenant)
	foreignToken := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(foreign.ID), Sub: foreign.Email, Roles: []string{"admin"}, Permissions: []string{"admin:*"}, TenantID: foreignTenant})
	for _, path := range []string{"/threads/" + threadIDs[0], "/group-threads/" + groupID} {
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+foreignToken)
		rec := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/api/messages", router), req)
		require.Equal(t, 404, rec.Code, rec.Body.String())
	}
}
