package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	settingsModule "github.com/moto-ogs/project-phoenix/modules/settings"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Photos and files in the guardian's conversations (#6685): a guardian
// uploads for their child, sends the upload with a message and reads it; a
// guardian of another family reaches neither the file nor the upload.

func (r mealWriteRig) uploadAttachment(t *testing.T, f mealWriteFamily, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/parent/me/messages/children/%d/attachments", f.StudentID), &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+f.token)
	return testutil.ExecuteRequest(r.router, req)
}

type parentAttachmentWire struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Width       int    `json:"width"`
}

func TestGuardianMessageCarriesUploadedPhotoForTheirConversationOnly(t *testing.T) {
	t.Parallel()
	r := newMealWriteRig(t)
	require.NoError(t, r.setSetting(testpkg.Ctx(t), settingsModule.KeyParentNotesEnabled, true))
	family, stranger := r.mealFamily(t), r.mealFamily(t)

	var photo bytes.Buffer
	require.NoError(t, png.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 12, 8))))
	uploaded := r.uploadAttachment(t, family, "zettel.png", photo.Bytes())
	require.Equal(t, http.StatusCreated, uploaded.Code, uploaded.Body.String())
	var stored struct{ Data parentAttachmentWire }
	require.NoError(t, json.Unmarshal(uploaded.Body.Bytes(), &stored))
	require.Equal(t, "zettel.png", stored.Data.Filename)
	require.Equal(t, 12, stored.Data.Width)
	download := func(f mealWriteFamily) string {
		return fmt.Sprintf("/parent/me/messages/children/%d/attachments/%s/download", f.StudentID, stored.Data.ID)
	}

	// The unsent upload is its uploader's alone, and nobody else can send it.
	require.Equal(t, http.StatusOK, r.serve(t, download(family), family.token).Code)
	require.Equal(t, http.StatusNotFound, r.serve(t, download(stranger), stranger.token).Code)
	stolen := r.call(t, testpkg.Ctx(t), writeCall{http.MethodPost, fmt.Sprintf("/parent/me/messages/children/%d", stranger.StudentID),
		map[string]any{"body": "x", "attachment_ids": []string{stored.Data.ID}}}, stranger.token)
	require.Equal(t, http.StatusBadRequest, stolen.Code, stolen.Body.String())

	// The guardian sends the photo without text.
	sent := r.call(t, testpkg.Ctx(t), writeCall{http.MethodPost, fmt.Sprintf("/parent/me/messages/children/%d", family.StudentID),
		map[string]any{"body": "", "attachment_ids": []string{stored.Data.ID}}}, family.token)
	require.Equal(t, http.StatusCreated, sent.Code, sent.Body.String())
	var view struct {
		Data struct {
			Messages []struct {
				Body        string                 `json:"body"`
				Attachments []parentAttachmentWire `json:"attachments"`
			} `json:"messages"`
		}
	}
	require.NoError(t, json.Unmarshal(sent.Body.Bytes(), &view))
	require.Len(t, view.Data.Messages, 1)
	require.Equal(t, []parentAttachmentWire{stored.Data}, view.Data.Messages[0].Attachments)

	// Its conversation reads the file; another family still does not, not
	// even under the right child.
	got := r.serve(t, download(family)+"?inline=1", family.token)
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())
	require.Equal(t, photo.Len(), got.Body.Len())
	require.Equal(t, http.StatusNotFound, r.serve(t, download(stranger), stranger.token).Code)
	require.Equal(t, http.StatusForbidden, r.serve(t, download(family), stranger.token).Code)

	// A message without text and without a file stays refused.
	empty := r.call(t, testpkg.Ctx(t), writeCall{http.MethodPost, fmt.Sprintf("/parent/me/messages/children/%d", family.StudentID), map[string]any{"body": " "}}, family.token)
	require.Equal(t, http.StatusBadRequest, empty.Code, empty.Body.String())
}
