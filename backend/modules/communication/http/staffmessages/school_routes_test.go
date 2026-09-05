// Team-Chat across portals (#2208): a Lehrkraft with a school token and a
// Betreuungskraft with a tenant token read and write the SAME conversation
// through their respective mounts, and each side sees which kind of colleague
// the other is.
package staffmessaging_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	staffmessaging "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type envelope[T any] struct {
	Data T `json:"data"`
}

func decodeData[T any](t *testing.T, body []byte) T {
	t.Helper()
	var env envelope[T]
	require.NoError(t, json.Unmarshal(body, &env), string(body))
	return env.Data
}

func TestSchoolStaffMessagesCrossPortal(t *testing.T) {
	t.Parallel()
	db, serviceFactory := testutil.SetupStaffMessagingModule(t)
	staffMessages := staffmessaging.NewResource(serviceFactory.StaffMessaging, db)
	schoolRouter := chi.NewRouter()
	schoolRouter.Mount("/staff-messages", staffMessages.SchoolRouter())
	tenantRouter := staffMessages.Router()
	tenantID, _ := testpkg.CreateTestTenant(t, db)

	// The chat is off by default (#2598); the school switches it on.
	require.NoError(t, serviceFactory.Settings.SetValue(
		testpkg.TenantContext(tenantID), configModel.KeyStaffMessagingEnabled, true, nil, nil,
	))

	unique := time.Now().UnixNano()
	// A Lehrkraft: staff row (school identity) + lehrkraft system role.
	_, teacherAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Lena", fmt.Sprintf("Lehrkraft-%d", unique))
	testpkg.EnsureAccountTenant(t, db, teacherAccount.ID, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, teacherAccount.ID, tenantID)
	// A Betreuungskraft of the OGS.
	_, ogsAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Ben", fmt.Sprintf("Betreuung-%d", unique))
	testpkg.EnsureAccountTenant(t, db, ogsAccount.ID, tenantID)

	teacherClaims := jwt.AppClaims{
		ID: int(teacherAccount.ID), Sub: teacherAccount.Email,
		Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool,
	}
	ogsClaims := jwt.AppClaims{
		ID: int(ogsAccount.ID), Sub: ogsAccount.Email,
		Roles: []string{"user"}, TenantID: tenantID, Scope: tenant.ScopeTenant,
	}

	// 1. The Lehrkraft sees the OGS colleague in the recipient list, marked as
	//    OGS staff.
	req := httptest.NewRequest(http.MethodGet, "/staff-messages/recipients", nil)
	rec := testutil.ExecuteWithAuth(t, schoolRouter, req, teacherClaims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	recipients := decodeData[[]staffmessaging.RecipientResponse](t, rec.Body.Bytes())
	var ogsRecipient *staffmessaging.RecipientResponse
	for i := range recipients {
		if recipients[i].AccountID == fmt.Sprint(ogsAccount.ID) {
			ogsRecipient = &recipients[i]
		}
	}
	require.NotNil(t, ogsRecipient, "the OGS colleague must be addressable from the school portal")
	assert.Equal(t, "staff", ogsRecipient.RoleKind)

	// 2. The Lehrkraft opens the conversation and writes.
	req = httptest.NewRequest(http.MethodPost, "/staff-messages/threads/open",
		strings.NewReader(fmt.Sprintf(`{"account_id":%q}`, fmt.Sprint(ogsAccount.ID))))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, teacherClaims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	opened := decodeData[staffmessaging.ThreadDetailResponse](t, rec.Body.Bytes())
	require.NotEmpty(t, opened.ThreadID)
	assert.Equal(t, "staff", opened.CounterpartRoleKind)

	req = httptest.NewRequest(http.MethodPost, "/staff-messages/threads/"+opened.ThreadID,
		strings.NewReader(`{"body":"Lena aus der 2a war heute sehr müde."}`))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, teacherClaims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// 3. The Betreuungskraft finds it in the tenant-portal inbox, unread, with
	//    the counterpart marked as Lehrkraft.
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = testutil.ExecuteWithAuth(t, tenantRouter, req, ogsClaims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	inbox := decodeData[[]staffmessaging.InboxThreadResponse](t, rec.Body.Bytes())
	require.Len(t, inbox, 1)
	assert.Equal(t, opened.ThreadID, inbox[0].ThreadID)
	assert.Equal(t, "lehrkraft", inbox[0].CounterpartRoleKind)
	assert.Equal(t, 1, inbox[0].UnreadCount)

	// 4. The Betreuungskraft answers through the tenant mount ...
	req = httptest.NewRequest(http.MethodPost, "/threads/"+opened.ThreadID,
		strings.NewReader(`{"body":"Danke, wir achten drauf."}`))
	req.Header.Set("Content-Type", "application/json")
	rec = testutil.ExecuteWithAuth(t, tenantRouter, req, ogsClaims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// ... and the Lehrkraft reads both messages through the school mount.
	req = httptest.NewRequest(http.MethodGet, "/staff-messages/threads/"+opened.ThreadID, nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, teacherClaims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeData[staffmessaging.ThreadDetailResponse](t, rec.Body.Bytes())
	require.Len(t, detail.Messages, 2)
	assert.Equal(t, fmt.Sprint(ogsAccount.ID), detail.Messages[1].SenderAccountID)

	req = httptest.NewRequest(http.MethodGet, "/staff-messages/unread-count", nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, teacherClaims)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 0, decodeData[map[string]int](t, rec.Body.Bytes())["unread_count"], "reading the thread marks it read")

	// 5. Scope isolation: the same account with a tenant token is refused on
	//    the school mount, and the school token is refused on the tenant mount.
	req = httptest.NewRequest(http.MethodGet, "/staff-messages/", nil)
	rec = testutil.ExecuteWithAuth(t, schoolRouter, req, jwt.AppClaims{
		ID: int(teacherAccount.ID), Sub: teacherAccount.Email, TenantID: tenantID, Scope: tenant.ScopeTenant,
	})
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = testutil.ExecuteWithAuth(t, tenantRouter, req, teacherClaims)
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestSchoolStaffMessagesDisabledSchool(t *testing.T) {
	t.Parallel()
	db, serviceFactory := testutil.SetupStaffMessagingModule(t)
	schoolRouter := chi.NewRouter()
	schoolRouter.Mount("/staff-messages", staffmessaging.NewResource(serviceFactory.StaffMessaging, db).SchoolRouter())
	tenantID, _ := testpkg.CreateTestTenant(t, db)

	_, teacherAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Off", fmt.Sprintf("Lehrkraft-%d", time.Now().UnixNano()))
	testpkg.EnsureAccountTenant(t, db, teacherAccount.ID, tenantID)
	testpkg.AssignLehrkraftSystemRole(t, db, teacherAccount.ID, tenantID)

	req := httptest.NewRequest(http.MethodGet, "/staff-messages/", nil)
	rec := testutil.ExecuteWithAuth(t, schoolRouter, req, jwt.AppClaims{
		ID: int(teacherAccount.ID), Sub: teacherAccount.Email,
		Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool,
	})
	// Same stable code as the tenant portal, so the school page can render the
	// off-state instead of a technical error.
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), staffmessaging.ErrCodeStaffMessagingDisabled)
}
