// Photos and files in the Team-Chat (#6685): a colleague uploads, sends the
// upload with a message, and only the participants of that conversation read
// the file.
package staffmessaging_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	staffmessaging "github.com/moto-nrw/project-phoenix/modules/communication/http/staffmessages"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type attachmentWire struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

func setupAttachmentFixture(t *testing.T) groupFixture {
	t.Helper()
	db, module, setSetting := setupStaffMessagingModule(t)
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	require.NoError(t, setSetting(testpkg.TenantContext(tenantID), settings.KeyStaffMessagingEnabled, true))
	return groupFixture{db: db, tenantRouter: module.TenantStaffMessagingRouterWithAttachmentsForTests(), tenantID: tenantID}
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func (f groupFixture) upload(t *testing.T, who groupPerson, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/attachments", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return testutil.ExecuteWithAuth(t, f.tenantRouter, req, who.claims)
}

func TestStaffMessageCarriesUploadedPhotoForParticipantsOnly(t *testing.T) {
	t.Parallel()
	f := setupAttachmentFixture(t)
	anna, ben, clara := f.person(t, "Anna", "staff"), f.person(t, "Ben", "staff"), f.person(t, "Clara", "staff")
	opened := f.do(t, anna, http.MethodPost, "/threads/open", fmt.Sprintf(`{"account_id":%q}`, ben.idString()))
	require.Equal(t, http.StatusOK, opened.Code, opened.Body.String())
	thread := decodeData[staffmessaging.ThreadDetailResponse](t, opened.Body.Bytes())

	photo := testPNG(t, 40, 30)
	uploaded := f.upload(t, anna, "ausflug.png", photo)
	require.Equal(t, http.StatusCreated, uploaded.Code, uploaded.Body.String())
	attachment := decodeData[attachmentWire](t, uploaded.Body.Bytes())
	require.Equal(t, "ausflug.png", attachment.Filename)
	require.Equal(t, "image/png", attachment.ContentType)
	require.Equal(t, 40, attachment.Width)
	require.Equal(t, 30, attachment.Height)
	download := "/attachments/" + attachment.ID + "/download"

	// An unsent upload belongs to its uploader alone.
	require.Equal(t, http.StatusOK, f.do(t, anna, http.MethodGet, download, "").Code)
	require.Equal(t, http.StatusNotFound, f.do(t, ben, http.MethodGet, download, "").Code)

	// A message may consist of the photo alone.
	sent := f.do(t, anna, http.MethodPost, "/threads/"+thread.ThreadID, fmt.Sprintf(`{"body":"","attachment_ids":[%q]}`, attachment.ID))
	require.Equal(t, http.StatusOK, sent.Code, sent.Body.String())
	message := decodeData[staffmessaging.MessageResponse](t, sent.Body.Bytes())
	var carried []attachmentWire
	require.NoError(t, json.Unmarshal(message.Attachments, &carried))
	require.Equal(t, []attachmentWire{attachment}, carried)

	// Both participants read the file, a third colleague does not.
	got := f.do(t, ben, http.MethodGet, download+"?inline=1", "")
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())
	require.Equal(t, "inline", got.Header().Get("Content-Disposition"))
	require.Equal(t, http.StatusNotFound, f.do(t, clara, http.MethodGet, download, "").Code)

	// The conversation and the inbox show it.
	read := f.do(t, ben, http.MethodGet, "/threads/"+thread.ThreadID, "")
	require.Equal(t, http.StatusOK, read.Code, read.Body.String())
	messages := decodeData[staffmessaging.ThreadDetailResponse](t, read.Body.Bytes()).Messages
	require.Len(t, messages, 1)
	require.JSONEq(t, string(message.Attachments), string(messages[0].Attachments))
	require.Equal(t, "📎 ausflug.png", f.inboxRow(t, ben, thread.ThreadID).LastMessageBody)

	// A sent upload cannot ride on a second message, nor a foreign one on any.
	again := f.do(t, anna, http.MethodPost, "/threads/"+thread.ThreadID, fmt.Sprintf(`{"body":"nochmal","attachment_ids":[%q]}`, attachment.ID))
	require.Equal(t, http.StatusBadRequest, again.Code, again.Body.String())
	foreign := decodeData[attachmentWire](t, f.upload(t, clara, "fremd.png", photo).Body.Bytes())
	stolen := f.do(t, anna, http.MethodPost, "/threads/"+thread.ThreadID, fmt.Sprintf(`{"body":"x","attachment_ids":[%q]}`, foreign.ID))
	require.Equal(t, http.StatusBadRequest, stolen.Code, stolen.Body.String())

	// A message without text and without a file stays refused.
	require.Equal(t, http.StatusBadRequest, f.do(t, anna, http.MethodPost, "/threads/"+thread.ThreadID, `{"body":" "}`).Code)
}

func TestStaffMessageAttachmentUploadRefusesDisguisedContent(t *testing.T) {
	t.Parallel()
	f := setupAttachmentFixture(t)
	anna := f.person(t, "Anna", "staff")
	refused := f.upload(t, anna, "skript.png", []byte("<html><script>alert(1)</script></html>"))
	require.Equal(t, http.StatusBadRequest, refused.Code, refused.Body.String())
	require.Contains(t, refused.Body.String(), `"code":"files.file_type_not_allowed"`)
	mismatch := f.upload(t, anna, "bild.pdf", testPNG(t, 2, 2))
	require.Equal(t, http.StatusBadRequest, mismatch.Code, mismatch.Body.String())
}
