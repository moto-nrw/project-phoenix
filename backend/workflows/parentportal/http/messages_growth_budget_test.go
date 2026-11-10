package parent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	portalCompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
	parentHTTP "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
	"github.com/stretchr/testify/require"
)

func TestParentPortalMessagesPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	repos := api.NewRepositoryTestFactory(database)
	people := api.NewPeopleRepositorySuiteFactory(database)
	messages := api.NewParentMessagingSuiteRepositories(database)
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	profiles, err := api.NewParentPortalCareProfiles(database)
	require.NoError(t, err)
	messageRecords, err := api.NewParentMessagingTestRepositories(database)
	require.NoError(t, err)
	messageRepo := messageRecords.Message
	broadcaster := testpkg.NewRecordingBroadcaster()
	portal := portalCompose.New(portalCompose.Dependencies{
		GuardianPolicy: owners.NewParentGuardianPolicy(), DeparturePolicy: owners.ParentDeparturePolicy(), Fingerprints: owners.SecurityFingerprints{}, Declarations: owners.ParentDeclarationPolicy(), Locales: owners.ParentPortalLocales(), Runtime: owners.ParentPortalRuntime{}, Logger: slog.Default(),
		ChildRepo: repos.ParentChildReads(), StudentRepo: owners.NewParentStudentReads(people.Student), GuardianProfileRepo: owners.NewParentGuardianProfileReads(people.GuardianProfile), Settings: settings.Settings,
		CareProfiles: profiles, People: owners.MustNewPeopleDirectory(database), MessagingEnabled: owners.BindParentMessagingGate(settings.Settings, slog.Default()),
		MessageThreadRepo: owners.BindParentMessageThreadAccess(messages.ParentMessageThread), MessageRepo: owners.BindParentMessageReads(messageRepo), MessageReadRepo: owners.BindParentMessageInboxReads(messages.ParentMessageRead),
		Conversations: api.NewParentConversationCore(messages.ParentMessageThread, messageRepo, messages.ParentMessageRead, broadcaster, slog.Default()), GroupThreads: api.NewParentGroupThreadTestPort(database),
	})
	router := parentHTTP.NewResource(parentHTTP.ResourceConfig{Parent: portal}).Router(func(r chi.Router) {
		r.Use(api.RequestSettingsCacheMiddleware, api.RequestIdentityCacheMiddleware)
		api.ParentSessionChain(r)
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestParentGuardianChain(t, db)
	_, staff := testpkg.CreateTestStaffWithAccount(t, db, "Budget", "Staff")
	// Two audiences are the maximum per child/guardian. Grow messages N3/N8,
	// keep both positive thread rows, and add unrelated families at each N.
	var threads []*communication.PortalParentMessageThread
	for _, audience := range []string{"team", "leadership"} {
		thread, err := testpkg.TenantQuery4(t, testpkg.Ctx(t), db, messages.ParentMessageThread.GetOrCreate, testpkg.Tenant(t), chain.StudentID, chain.AccountID, audience)
		require.NoError(t, err)
		threads = append(threads, thread)
	}
	claims := sessiontoken.AppClaims{ID: int(chain.AccountID), Sub: fmt.Sprint(chain.AccountID), Roles: []string{"parent"}, Scope: "parent"}
	token := testutil.MintTestJWT(t, claims)
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	var lastGroupID int64
	created := 0
	for _, n := range []int{3, 8} {
		for created < n {
			for _, thread := range threads {
				message := &communication.PortalParentMessage{ThreadID: thread.ID, StudentID: chain.StudentID, SenderAccountID: staff.ID, SenderKind: "staff", SenderName: "Budget Staff", Body: fmt.Sprintf("message-%02d", created), Kind: "message"}
				message.TenantID = testpkg.Tenant(t)
				require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, messageRepo.Create, message))
			}
			testpkg.CreateTestParentGuardianChain(t, db)
			created++
		}
		staffCtx := context.WithValue(testpkg.Ctx(t), sessiontoken.CtxClaims, sessiontoken.AppClaims{ID: int(staff.ID)})
		staffCtx = context.WithValue(staffCtx, sessiontoken.CtxPermissions, []string{"admin:*"})
		var groupID int64
		require.NoError(t, testpkg.WithinTenantContext(t, staffCtx, db, testpkg.Tenant(t), func(ctx context.Context) error {
			var e error
			groupID, e = api.CreateParentGroupClassThreadForTest(ctx, database, staff.ID, "1a", "Growth", "group-00")
			return e
		}))
		// Persist real messages and matching recipient fixtures. The route
		// keeps its production read capability.
		lastGroupID = groupID
		for i := 1; i < n; i++ {
			var messageID int64
			require.NoError(t, db.NewRaw(`INSERT INTO users.parent_group_messages (tenant_id,thread_id,sender_account_id,sender_name,body) VALUES (?,?,?,?,?) RETURNING id`, testpkg.Tenant(t), groupID, staff.ID, "Budget Staff", fmt.Sprintf("group-%02d", i)).Scan(testpkg.Ctx(t), &messageID))
			_, err := db.NewRaw(`INSERT INTO users.parent_group_message_recipients (tenant_id,thread_id,message_id,student_id,guardian_account_id) VALUES (?,?,?,?,?)`, testpkg.Tenant(t), groupID, messageID, chain.StudentID, chain.AccountID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
		}

		paths := []struct{ name, path string }{{"conversation", fmt.Sprintf("/me/messages/children/%d", chain.StudentID)}, {"threads", fmt.Sprintf("/me/messages/children/%d/threads", chain.StudentID)}, {"group_thread", fmt.Sprintf("/me/messages/group-threads/%d", groupID)}}
		for _, entry := range paths {
			// A parent request is bound to no school (ProtectedParentGroup); a
			// bound context would make the conversation's read marker take the
			// ambient way of two units (ADR 0161, #5862).
			reqCtx := testpkg.Ctx(t)
			if entry.name == "conversation" {
				reqCtx = context.Background()
			}
			req := httptest.NewRequest("GET", entry.path, nil).WithContext(counter.Context(testpkg.WithPackageTenantRuntime(reqCtx)))
			req.Header.Set("Authorization", "Bearer "+token)
			counter.Reset()
			counter.Start()
			rec := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/parent", router), req)
			counter.Stop()
			require.Equal(t, 200, rec.Code, rec.Body.String())
			t.Logf("MEASURE parent_portal %s N%d=%d", entry.name, n, counter.Total())
			switch entry.name {
			case "conversation":
				var out struct{ Data parentHTTP.ThreadViewResponse }
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data.Messages, n)
				require.Equal(t, fmt.Sprint(chain.StudentID), out.Data.StudentID)
				for i, row := range out.Data.Messages {
					require.Equal(t, fmt.Sprintf("message-%02d", i), row.Body)
					require.NotEmpty(t, row.ID)
				}
			case "threads":
				var out struct {
					Data []parentHTTP.ThreadSummaryResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data, 2)
				for _, row := range out.Data {
					require.Equal(t, fmt.Sprint(chain.StudentID), row.StudentID)
					require.Contains(t, []string{"team", "leadership"}, row.Audience)
				}
			case "group_thread":
				var out struct {
					Data parentHTTP.GroupThreadViewResponse
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
				require.Len(t, out.Data.Messages, n)
				require.Len(t, out.Data.Children, 1)
				for i, row := range out.Data.Messages {
					require.Equal(t, fmt.Sprintf("group-%02d", i), row.Body)
					require.NotEmpty(t, row.ID)
				}
			}
			switch entry.name {
			case "conversation":
				querybudgets.AssertQueryBudget(t, "parent.messages.conversation.positive_growth", counter.Measurement(counter.Queries()))
			case "threads":
				querybudgets.AssertQueryBudget(t, "parent.messages.threads.positive_growth", counter.Measurement(counter.Queries()))
			case "group_thread":
				querybudgets.AssertQueryBudget(t, "parent.messages.group_thread.positive_growth", counter.Measurement(counter.Queries()))
			}
		}
	}
	// Another family cannot read the child's conversations, even with a valid
	// parent token and active school membership.
	otherToken := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(other.AccountID), Sub: fmt.Sprint(other.AccountID), Roles: []string{"parent"}, Scope: "parent"})
	for _, suffix := range []string{"", "/threads"} {
		req := httptest.NewRequest("GET", fmt.Sprintf("/me/messages/children/%d%s", chain.StudentID, suffix), nil).WithContext(testpkg.WithPackageTenantRuntime(testpkg.Ctx(t)))
		req.Header.Set("Authorization", "Bearer "+otherToken)
		rec := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/parent", router), req)
		require.Equal(t, 403, rec.Code, rec.Body.String())
	}

	t.Run("foreign-school", func(t *testing.T) {
		require.Same(t, db, testpkg.SetupTestDB(t))
		testpkg.OwnTenant(t)
		foreign := testpkg.CreateTestParentGuardianChain(t, db)
		foreignToken := testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(foreign.AccountID), Sub: foreign.Email, Roles: []string{"parent"}, Scope: "parent"})
		for _, path := range []string{fmt.Sprintf("/me/messages/children/%d", chain.StudentID), fmt.Sprintf("/me/messages/children/%d/threads", chain.StudentID), fmt.Sprintf("/me/messages/group-threads/%d", lastGroupID)} {
			req := httptest.NewRequest("GET", path, nil).WithContext(testpkg.WithPackageTenantRuntime(testpkg.Ctx(t)))
			req.Header.Set("Authorization", "Bearer "+foreignToken)
			rec := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/parent", router), req)
			if strings.Contains(path, "group-threads") {
				require.Equal(t, 404, rec.Code, rec.Body.String())
			} else {
				require.Equal(t, 403, rec.Code, rec.Body.String())
			}
		}
	})
	links, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, people.StudentGuardian.FindByStudentID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(testpkg.Ctx(t), db, links[0].ID, map[string]interface{}{}))
	for _, suffix := range []string{"", "/threads"} {
		req := httptest.NewRequest("GET", fmt.Sprintf("/me/messages/children/%d%s", chain.StudentID, suffix), nil).WithContext(testpkg.WithPackageTenantRuntime(testpkg.Ctx(t)))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := testutil.ExecuteRequestForTest(t, testpkg.MountedQueryBudgetRouter("/parent", router), req)
		require.Equal(t, 403, rec.Code, rec.Body.String())
	}
}
