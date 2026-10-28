// Group chats in the Team-Chat (#4776): OGS staff start a named group with
// colleagues and Lehrkräfte, the creator and the leadership manage it, every
// member may leave, a new member sees the history from joining, and access
// ends at once with the membership or the account.
package staffmessaging_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	rootAPI "github.com/moto-nrw/project-phoenix/api"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	staffmessaging "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type groupFixture struct {
	db           *bun.DB
	tenantRouter chi.Router
	schoolRouter chi.Router
	tenantID     int64
}

type groupPerson struct {
	id     int64
	claims sessiontoken.AppClaims
}

func (p groupPerson) idString() string { return fmt.Sprint(p.id) }

func setupGroupFixture(t *testing.T) groupFixture {
	t.Helper()
	db, module, setSetting := setupStaffMessagingModule(t)
	resource := staffmessaging.NewResource(module.StaffMessaging)
	schoolRouter := chi.NewRouter()
	schoolRouter.Mount("/staff-messages", resource.SchoolRouter(rootAPI.ProtectedSchoolRoutes))
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	require.NoError(t, setSetting(testpkg.TenantContext(tenantID), settings.KeyStaffMessagingEnabled, true))
	return groupFixture{db: db, tenantRouter: resource.Router(rootAPI.ProtectedTenantRoutes), schoolRouter: schoolRouter, tenantID: tenantID}
}

// person creates a colleague of the fixture school. kind is "staff",
// "admin" (leadership) or "lehrkraft"; a Lehrkraft gets a school token.
func (f groupFixture) person(t *testing.T, first, kind string) groupPerson {
	t.Helper()
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, f.tenantID, first, fmt.Sprintf("Gruppe-%d", time.Now().UnixNano()))
	testpkg.EnsureAccountTenant(t, f.db, account.ID, f.tenantID)
	scope := tenant.ScopeTenant
	switch kind {
	case "lehrkraft":
		testpkg.AssignLehrkraftSystemRole(t, f.db, account.ID, f.tenantID)
		scope = tenant.ScopeSchool
	case "admin":
		assignAdminRole(t, f.db, account.ID, f.tenantID)
	}
	return groupPerson{id: account.ID, claims: sessiontoken.AppClaims{
		ID: int(account.ID), Sub: account.Email, Roles: []string{kind}, TenantID: f.tenantID, Scope: scope,
	}}
}

// assignAdminRole gives the account a school role whose base role is admin:
// the leadership in the colleague classification.
func assignAdminRole(t *testing.T, db *bun.DB, accountID, tenantID int64) {
	t.Helper()
	role := testpkg.CreateTestRoleForTenant(t, db, "leitung", tenantID)
	ctx := t.Context()
	_, err := db.NewRaw(`UPDATE auth.roles SET base_role = 'admin' WHERE id = ?`, role.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO auth.account_roles (account_id, role_id, tenant_id) VALUES (?, ?, ?)`, accountID, role.ID, tenantID).Exec(ctx)
	require.NoError(t, err)
}

func (f groupFixture) do(t *testing.T, who groupPerson, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := f.tenantRouter
	if who.claims.Scope == tenant.ScopeSchool {
		router = f.schoolRouter
		path = "/staff-messages" + path
	}
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return testutil.ExecuteWithAuth(t, router, req, who.claims)
}

func (f groupFixture) inboxRow(t *testing.T, who groupPerson, threadID string) *staffmessaging.InboxThreadResponse {
	t.Helper()
	rec := f.do(t, who, http.MethodGet, "/", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, row := range decodeData[[]staffmessaging.InboxThreadResponse](t, rec.Body.Bytes()) {
		if row.ThreadID == threadID {
			return &row
		}
	}
	return nil
}

func TestStaffGroupChatLifecycle(t *testing.T) {
	t.Parallel()
	f := setupGroupFixture(t)
	ben := f.person(t, "Ben", "staff")
	anna := f.person(t, "Anna", "staff")
	lena := f.person(t, "Lena", "lehrkraft")
	leitung := f.person(t, "Lea", "admin")

	// Ben starts a group with Anna and the Lehrkraft Lena.
	rec := f.do(t, ben, http.MethodPost, "/groups",
		fmt.Sprintf(`{"name":"  Klasse 2a  ","account_ids":[%q,%q]}`, anna.idString(), lena.idString()))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	group := decodeData[staffmessaging.ThreadDetailResponse](t, rec.Body.Bytes())
	threadID := group.ThreadID
	assert.Equal(t, "group", group.Kind)
	assert.Equal(t, "Klasse 2a", group.GroupName, "the name is trimmed")
	assert.Empty(t, group.CounterpartAccountID, "a group has no direct-chat counterpart")
	assert.True(t, group.CanManage, "the creator manages the group")
	assert.Len(t, group.Members, 3)

	// The group shows in Lena's school-portal inbox before anyone wrote.
	row := f.inboxRow(t, lena, threadID)
	require.NotNil(t, row, "a new group shows in the members' inbox")
	assert.Equal(t, "group", row.Kind)
	assert.Equal(t, 3, row.MemberCount)
	assert.Empty(t, row.CounterpartAccountID)

	rec = f.do(t, ben, http.MethodPost, "/threads/"+threadID, `{"body":"Ausflug am Freitag"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	row = f.inboxRow(t, lena, threadID)
	require.NotNil(t, row)
	assert.Equal(t, 1, row.UnreadCount)
	assert.Equal(t, "Ausflug am Freitag", row.LastMessageBody)
	assert.NotEmpty(t, row.LastSenderName, "a group preview names who wrote")

	// Lena reads through the school portal: Ben's message counts her as reader.
	rec = f.do(t, lena, http.MethodGet, "/threads/"+threadID, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeData[staffmessaging.ThreadDetailResponse](t, rec.Body.Bytes())
	assert.False(t, detail.CanManage, "a Lehrkraft does not manage a group she did not start")
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, []string{lena.idString()}, detail.Messages[0].ReadByAccountIDs, "opening the group returns the reader's advanced receipt")

	rec = f.do(t, ben, http.MethodGet, "/threads/"+threadID, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail = decodeData[staffmessaging.ThreadDetailResponse](t, rec.Body.Bytes())
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, 2, detail.Messages[0].ReaderCount, "Anna and Lena could read it, not Ben himself")
	assert.Equal(t, []string{lena.idString()}, detail.Messages[0].ReadByAccountIDs)
	assert.ElementsMatch(t, []string{anna.idString(), lena.idString()}, detail.Messages[0].EligibleReaderAccountIDs)

	// The leadership joins later: no earlier history, nothing unread.
	rec = f.do(t, ben, http.MethodPost, "/threads/"+threadID+"/members", fmt.Sprintf(`{"account_ids":[%q]}`, leitung.idString()))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	row = f.inboxRow(t, leitung, threadID)
	require.NotNil(t, row)
	assert.Equal(t, 0, row.UnreadCount)
	assert.Empty(t, row.LastMessageBody, "the preview hides a message from before joining")
	rec = f.do(t, leitung, http.MethodGet, "/threads/"+threadID, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail = decodeData[staffmessaging.ThreadDetailResponse](t, rec.Body.Bytes())
	assert.Empty(t, detail.Messages, "a new member sees the history from joining")
	assert.True(t, detail.CanManage, "the leadership manages every group it is in")

	// ... and may rename it, unlike Anna.
	rec = f.do(t, anna, http.MethodPut, "/threads/"+threadID+"/name", `{"name":"Nope"}`)
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "communication.staff_group_manage_forbidden")
	rec = f.do(t, leitung, http.MethodPut, "/threads/"+threadID+"/name", `{"name":"Klasse 2a und OGS"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "Klasse 2a und OGS", decodeData[staffmessaging.ThreadDetailResponse](t, rec.Body.Bytes()).GroupName)

	// Removing Anna ends her access at once.
	rec = f.do(t, ben, http.MethodDelete, "/threads/"+threadID+"/members/"+anna.idString(), "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.do(t, anna, http.MethodGet, "/threads/"+threadID, "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = f.do(t, anna, http.MethodPost, "/threads/"+threadID, `{"body":"Noch da?"}`)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Nil(t, f.inboxRow(t, anna, threadID))

	// Lena leaves through the school portal.
	rec = f.do(t, lena, http.MethodPost, "/threads/"+threadID+"/leave", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.do(t, lena, http.MethodGet, "/threads/"+threadID, "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	// The last two leave: the group is gone.
	require.Equal(t, http.StatusOK, f.do(t, ben, http.MethodPost, "/threads/"+threadID+"/leave", "").Code)
	require.Equal(t, http.StatusOK, f.do(t, leitung, http.MethodPost, "/threads/"+threadID+"/leave", "").Code)
	var remaining int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM users.staff_message_threads WHERE id = ?`, threadID).Scan(t.Context(), &remaining))
	assert.Zero(t, remaining)
}

func TestStaffGroupChatRules(t *testing.T) {
	t.Parallel()
	f := setupGroupFixture(t)
	ben := f.person(t, "Ben", "staff")
	anna := f.person(t, "Anna", "staff")
	lena := f.person(t, "Lena", "lehrkraft")

	// A Lehrkraft does not start groups: the school portal has no such route,
	// and the service refuses her with a registered code anyway.
	rec := f.do(t, lena, http.MethodPost, "/groups", fmt.Sprintf(`{"name":"X","account_ids":[%q]}`, ben.idString()))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	lenaTenant := lena
	lenaTenant.claims.Scope = tenant.ScopeTenant
	rec = f.do(t, lenaTenant, http.MethodPost, "/groups", fmt.Sprintf(`{"name":"X","account_ids":[%q]}`, ben.idString()))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "communication.staff_group_create_forbidden")

	// Input rules.
	rec = f.do(t, ben, http.MethodPost, "/groups", fmt.Sprintf(`{"name":"   ","account_ids":[%q]}`, anna.idString()))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "communication.staff_group_name_invalid")
	rec = f.do(t, ben, http.MethodPost, "/groups", fmt.Sprintf(`{"name":"Allein","account_ids":[%q]}`, ben.idString()))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "communication.staff_group_members_required")
	rec = f.do(t, ben, http.MethodPost, "/groups", `{"name":"Team","account_ids":["ungültig"]}`)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "general.input")
	assert.Contains(t, rec.Body.String(), "Ungültige Konto-ID.")

	// No members from another school.
	otherTenant, _ := testpkg.CreateTestTenant(t, f.db)
	_, stranger := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, otherTenant, "Fremd", fmt.Sprintf("Schule-%d", time.Now().UnixNano()))
	testpkg.EnsureAccountTenant(t, f.db, stranger.ID, otherTenant)
	rec = f.do(t, ben, http.MethodPost, "/groups", fmt.Sprintf(`{"name":"Gemischt","account_ids":[%q,"%d"]}`, anna.idString(), stranger.ID))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

	// A deactivated member loses access at once.
	rec = f.do(t, ben, http.MethodPost, "/groups", fmt.Sprintf(`{"name":"Team","account_ids":[%q]}`, anna.idString()))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	threadID := decodeData[staffmessaging.ThreadDetailResponse](t, rec.Body.Bytes()).ThreadID
	_, err := f.db.NewRaw(`UPDATE auth.accounts SET active = FALSE WHERE id = ?`, anna.id).Exec(t.Context())
	require.NoError(t, err)
	rec = f.do(t, anna, http.MethodGet, "/threads/"+threadID, "")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Nil(t, f.inboxRow(t, anna, threadID), "a deactivated member must not see the group in the inbox")
	rec = f.do(t, anna, http.MethodGet, "/unread-count", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Zero(t, decodeData[map[string]int](t, rec.Body.Bytes())["unread_count"], "a deactivated member must not receive group unread badges")

	// Ben keeps writing; the group does not close because one member left.
	rec = f.do(t, ben, http.MethodPost, "/threads/"+threadID, `{"body":"Weiter geht's"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
