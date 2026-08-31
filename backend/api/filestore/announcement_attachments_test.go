// Router-level tests for the attachments of an Elternmitteilung (#2890):
// upload, list, download and delete on a draft, the refusal to change a
// published announcement, the file-type gate, and the cleanup intent that
// keeps the bytes reachable after the row is soft-deleted.
package filestore_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// attachmentContext extends the file-storage harness with the attachment
// router and an announcement to hang files off.
type attachmentContext struct {
	*apiContext
	announcementID int64
}

func setupAttachmentRoute(t *testing.T) *attachmentContext {
	t.Helper()
	c := setupFileStoreRoute(t)
	c.router.Mount("/announcement-attachments", c.resource.AnnouncementAttachmentRouter())
	t.Cleanup(func() {
		if pubDir, err := common.ResolvePublicDir(); err == nil {
			_ = os.RemoveAll(filepath.Join(pubDir, "uploads", "announcement-attachments",
				fmt.Sprint(testpkg.Tenant(t))))
		}
	})
	announcement := testpkg.CreateTestParentAnnouncement(t, c.db, c.admin, "Ausflug")
	return &attachmentContext{apiContext: c, announcementID: announcement.ID}
}

func (c *attachmentContext) uploadAttachment(filename string, content []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(c.t, err)
	_, err = part.Write(content)
	require.NoError(c.t, err)
	require.NoError(c.t, writer.Close())
	return c.do(http.MethodPost,
		fmt.Sprintf("/announcement-attachments/%d", c.announcementID),
		&buf, writer.FormDataContentType(), c.admin, permissions.AdminWildcard)
}

func (c *attachmentContext) listAttachments() (*httptest.ResponseRecorder, attachmentListBody) {
	rec := c.do(http.MethodGet,
		fmt.Sprintf("/announcement-attachments/%d", c.announcementID),
		nil, "", c.admin, permissions.AdminWildcard)
	var envelope struct {
		Data attachmentListBody `json:"data"`
	}
	if rec.Code == http.StatusOK {
		require.NoError(c.t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	}
	return rec, envelope.Data
}

type attachmentListBody struct {
	Attachments []struct {
		ID          string `json:"id"`
		Filename    string `json:"filename"`
		SizeBytes   int64  `json:"size_bytes"`
		ContentType string `json:"content_type"`
	} `json:"attachments"`
	MaxCount int   `json:"max_count"`
	MaxBytes int64 `json:"max_bytes"`
	Editable bool  `json:"editable"`
}

func TestAnnouncementAttachmentLifecycle(t *testing.T) {
	t.Parallel()
	c := setupAttachmentRoute(t)

	rec := c.uploadAttachment("Einverständnis.pdf", fakePDF)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	listRec, list := c.listAttachments()
	require.Equal(t, http.StatusOK, listRec.Code)
	require.Len(t, list.Attachments, 1)
	assert.Equal(t, "Einverständnis.pdf", list.Attachments[0].Filename,
		"the name the school typed is what the parents see")
	assert.Equal(t, "application/pdf", list.Attachments[0].ContentType)
	assert.True(t, list.Editable, "a draft's attachments can still be changed")
	// The limits travel with the list so the form can state them before
	// somebody picks a file.
	assert.Positive(t, list.MaxCount)
	assert.Positive(t, list.MaxBytes)

	attachmentID := list.Attachments[0].ID

	// The download carries the display name and is served as an attachment.
	dlRec := c.do(http.MethodGet,
		fmt.Sprintf("/announcement-attachments/%d/%s/download", c.announcementID, attachmentID),
		nil, "", c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusOK, dlRec.Code)
	assert.Equal(t, fakePDF, dlRec.Body.Bytes())
	assert.Contains(t, dlRec.Header().Get("Content-Disposition"), "attachment")
	assert.Equal(t, "nosniff", dlRec.Header().Get("X-Content-Type-Options"))

	delRec := c.do(http.MethodDelete,
		fmt.Sprintf("/announcement-attachments/%d/%s", c.announcementID, attachmentID),
		nil, "", c.admin, permissions.AdminWildcard)
	require.Equal(t, http.StatusOK, delRec.Code, delRec.Body.String())

	_, afterDelete := c.listAttachments()
	assert.Empty(t, afterDelete.Attachments, "a deleted attachment leaves the list")
}

func TestAnnouncementAttachmentRejectsUnknownFileType(t *testing.T) {
	t.Parallel()
	c := setupAttachmentRoute(t)

	// An executable renamed to .pdf must not pass: the gate reads the magic
	// bytes, not the extension.
	rec := c.uploadAttachment("harmlos.pdf", []byte("MZ\x90\x00 this is not a pdf"))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "nicht erlaubt",
		"the rejection reaches the school in German, naming the allowed formats")

	// A bare ZIP is refused too — the OOXML parts are what prove a DOCX.
	rec = c.uploadAttachment("liste.xlsx", []byte("PK\x03\x04 not really an office file"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAnnouncementAttachmentPublishedAnnouncementIsFixed(t *testing.T) {
	t.Parallel()
	c := setupAttachmentRoute(t)

	// One file goes on while it is still a draft.
	require.Equal(t, http.StatusCreated, c.uploadAttachment("Brief.pdf", fakePDF).Code)
	_, before := c.listAttachments()
	require.Len(t, before.Attachments, 1)

	testpkg.PublishTestParentAnnouncement(t, c.db, c.announcementID)

	// After publication the attachments are part of what the parents were
	// shown — and, for an Elternbrief, of what they confirmed. Neither adding
	// nor removing one may still happen; the correction path is zurückziehen,
	// ändern, erneut veröffentlichen.
	rec := c.uploadAttachment("Nachtrag.pdf", fakePDF)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "zurück",
		"the message says what to do, not just that it failed")

	delRec := c.do(http.MethodDelete,
		fmt.Sprintf("/announcement-attachments/%d/%s", c.announcementID, before.Attachments[0].ID),
		nil, "", c.admin, permissions.AdminWildcard)
	assert.Equal(t, http.StatusConflict, delRec.Code)

	// The file itself stays readable — immutability is not invisibility.
	listRec, after := c.listAttachments()
	require.Equal(t, http.StatusOK, listRec.Code)
	require.Len(t, after.Attachments, 1)
	assert.False(t, after.Editable, "the UI must be able to say the file is fixed now")
}

func TestAnnouncementAttachmentUnknownAnnouncementIsNotFound(t *testing.T) {
	t.Parallel()
	c := setupAttachmentRoute(t)

	rec := c.do(http.MethodGet,
		fmt.Sprintf("/announcement-attachments/%d", c.announcementID+9_000_000),
		nil, "", c.admin, permissions.AdminWildcard)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAnnouncementAttachmentRequiresAdmin(t *testing.T) {
	t.Parallel()
	c := setupAttachmentRoute(t)

	// Who may not write an Elternmitteilung may not attach a file to one.
	rec := c.do(http.MethodGet,
		fmt.Sprintf("/announcement-attachments/%d", c.announcementID),
		nil, "", c.member, "students:read")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
