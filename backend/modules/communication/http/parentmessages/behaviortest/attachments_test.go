// Photos and files in the messages to guardians (#6685): staff upload, send
// the upload in a conversation or a group thread, and read it under the
// right that reads the messages.
package messaging_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	compose "github.com/moto-ogs/project-phoenix/modules/communication/composition"
	messaging "github.com/moto-ogs/project-phoenix/modules/communication/http/parentmessages"
	parentStore "github.com/moto-ogs/project-phoenix/modules/communication/parentstore"
	deliveryPolicy "github.com/moto-ogs/project-phoenix/modules/delivery/devices"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	settingsValues "github.com/moto-ogs/project-phoenix/modules/settings"
	"github.com/moto-ogs/project-phoenix/realtime"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type attachmentWire struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
}

// attachmentFixture mounts the message routes with their attachment routes
// the way the server does.
func attachmentFixture(t *testing.T) (chi.Router, testpkg.ParentChain) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	database := testpkg.NewDatabase(t, db)
	repos := api.NewParentMessagingSuiteRepositories(database)
	people := api.NewPeopleRepositorySuiteFactory(database)
	accounts := api.NewAccountSuiteRepositories(database)
	directory := api.NewTestPersonDirectory(database, api.TestPersonDirectorySources{Persons: people.Person, Students: people.Student, Staff: people.Staff, AccountExists: api.AccountExists(accounts.Profile)})
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	require.NoError(t, settings.Settings.SetValue(testpkg.Ctx(t), "operations.parent_notes_enabled", true, nil, nil))
	persons := owners.ParentMessagePeopleWithLeaders{ParentMessagePeople: owners.BindParentMessagePeople(directory), ParentMessageLeadership: owners.BindParentMessageLeadership(people.Staff, owners.NewPeopleIdentityAccess(database, nil))}
	svc, groups, attachments, handling := compose.NewParentMessaging(compose.ParentMessagingConfig{ThreadRepo: repos.ParentMessageThread, MessageRepo: parentStore.NewParentMessageRepository(db), ReadRepo: repos.ParentMessageRead, Persons: persons, UserContext: owners.StaffStudentReads{}, Session: api.CommunicationSession{}, Settings: settingsValues.NewParentMessagingValues(settings.Settings), Broadcaster: realtime.NewHub(slog.Default()), DB: db, Logger: slog.Default(), Observe: func(compose.Observation) {}, GroupAudience: owners.NewParentGroupAudience(database)}, owners.CommunicationRequestRuntime{}, deliveryPolicy.ParentDeliveryPolicy{})
	routes := api.NewMessageAttachmentRoutesForTests(attachments, common.RequiresPermission(permissions.UsersRead))
	return messaging.NewResource(svc, groups, handling).Router(api.ProtectedTenantRoutesWith(routes.Register)), testpkg.CreateTestParentGuardianChain(t, db)
}

func staffToken(t *testing.T, name string, grants ...string) string {
	t.Helper()
	_, staff := testpkg.CreateTestStaffWithAccount(t, testpkg.SetupTestDB(t), name, "Anhang")
	testpkg.EnsureAccountTenant(t, testpkg.SetupTestDB(t), staff.ID, testpkg.Tenant(t))
	return testutil.MintTestJWT(t, sessiontoken.AppClaims{ID: int(staff.ID), Sub: staff.Email, Roles: []string{"staff"}, Permissions: grants, TenantID: testpkg.Tenant(t)})
}

func call(t *testing.T, router chi.Router, token, method, path, contentType string, body *bytes.Buffer) *httptest.ResponseRecorder {
	t.Helper()
	if body == nil {
		body = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, body).WithContext(testpkg.Ctx(t))
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return testutil.ExecuteRequestForTest(t, router, req)
}

func uploadPhoto(t *testing.T, router chi.Router, token, filename string) attachmentWire {
	t.Helper()
	var photo, body bytes.Buffer
	require.NoError(t, png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 4, 4))))
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(photo.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	rec := call(t, router, token, http.MethodPost, "/attachments", writer.FormDataContentType(), &body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var out struct{ Data attachmentWire }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out.Data
}

func TestStaffSendPhotosToGuardiansAndReadThemWithTheMessages(t *testing.T) {
	t.Parallel()
	router, chain := attachmentFixture(t)
	sender, colleague, outsider := staffToken(t, "Sena", "admin:*"), staffToken(t, "Cem", "admin:*"), staffToken(t, "Olga")
	post := func(path, payload string) *httptest.ResponseRecorder {
		return call(t, router, sender, http.MethodPost, path, "application/json", bytes.NewBufferString(payload))
	}

	// A photo without text opens the conversation with the guardian.
	photo := uploadPhoto(t, router, sender, "basteln.png")
	download := "/attachments/" + photo.ID + "/download"
	require.Equal(t, http.StatusNotFound, call(t, router, colleague, http.MethodGet, download, "", nil).Code, "an unsent upload is its uploader's alone")
	opened := post("/threads", fmt.Sprintf(`{"student_id":%q,"guardian_account_id":%q,"body":"","attachment_ids":[%q]}`, fmt.Sprint(chain.StudentID), fmt.Sprint(chain.AccountID), photo.ID))
	require.Equal(t, http.StatusCreated, opened.Code, opened.Body.String())
	var thread struct {
		Data messaging.ThreadDetailResponse
	}
	require.NoError(t, json.Unmarshal(opened.Body.Bytes(), &thread))
	require.Len(t, thread.Data.Messages, 1)
	var carried []attachmentWire
	require.NoError(t, json.Unmarshal(thread.Data.Messages[0].Attachments, &carried))
	require.Equal(t, []attachmentWire{photo}, carried)

	// Whoever reads the conversation reads its photo; the route's right
	// keeps everybody else out.
	require.Equal(t, http.StatusOK, call(t, router, colleague, http.MethodGet, download+"?inline=1", "", nil).Code)
	require.Equal(t, http.StatusForbidden, call(t, router, outsider, http.MethodGet, download, "", nil).Code)
	inbox := call(t, router, colleague, http.MethodGet, "/", "", nil)
	require.Equal(t, http.StatusOK, inbox.Code, inbox.Body.String())
	require.Contains(t, inbox.Body.String(), "📎 basteln.png")

	// A group thread carries files the same way.
	file := uploadPhoto(t, router, sender, "elternbrief.png")
	group := post("/group-threads", fmt.Sprintf(`{"target_type":"class","target_id":"1a","target_label":"Klasse 1a","title":"Ausflug","body":"Bitte lesen","attachment_ids":[%q]}`, file.ID))
	require.Equal(t, http.StatusCreated, group.Code, group.Body.String())
	require.True(t, strings.Contains(group.Body.String(), `"filename":"elternbrief.png"`), group.Body.String())
	require.Equal(t, http.StatusOK, call(t, router, colleague, http.MethodGet, "/attachments/"+file.ID+"/download", "", nil).Code)

	// A sent upload rides on no second message.
	again := post("/threads/"+thread.Data.ThreadID, fmt.Sprintf(`{"body":"nochmal","attachment_ids":[%q],"handled_up_to_message_id":%q}`, photo.ID, thread.Data.Messages[0].ID))
	require.Equal(t, http.StatusBadRequest, again.Code, again.Body.String())
}
