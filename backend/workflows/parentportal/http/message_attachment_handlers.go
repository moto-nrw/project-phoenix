package parent

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Photos and files in the guardian's conversations (#6685).

// The limits mirror Communication's: one file up to 10 MB, five files per
// message. The owner enforces both again; these bound the request.
const (
	maxMessageAttachmentBytes int64 = 10 << 20
	maxMessageAttachments           = 5
)

// uploadChildMessageAttachment stores one file for a message the guardian is
// about to send about the child. The row commits before the bytes are
// written, so a crash in between leaves an expiring row, never bytes nobody
// points at.
func (rs *Resource) uploadChildMessageAttachment(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parseStudentID(w, r)
	if !ok {
		return
	}
	files, err := rs.messageAttachmentFiles()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	parsed, err := common.ParseMessageAttachment(w, r, maxMessageAttachmentBytes)
	if err != nil {
		common.RenderError(w, r, common.ErrorUpload(err, maxMessageAttachmentBytes))
		return
	}
	stored, err := rs.ParentService.PrepareChildMessageAttachment(r.Context(), accountID, studentID, parentService.MessageAttachmentUpload{
		Filename: parsed.Filename, ContentType: parsed.ContentType, Extension: parsed.Extension,
		SizeBytes: parsed.SizeBytes, Width: parsed.Width, Height: parsed.Height,
	})
	if err != nil {
		renderMessageAttachmentError(w, r, err)
		return
	}
	if _, err := files.Save(r.Context(), stored.TenantID, stored.StoredName, parsed.Content); err != nil {
		if discardErr := rs.ParentService.DiscardChildMessageAttachment(r.Context(), accountID, studentID, stored.Attachment.ID); discardErr != nil {
			err = errors.Join(err, discardErr)
		}
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("write message attachment: %w", err)))
		return
	}
	common.Respond(w, r, http.StatusCreated, stored.Attachment, "Attachment stored")
}

// downloadChildMessageAttachment streams one file of the guardian's
// conversations about the child. A file of another family answers 404.
func (rs *Resource) downloadChildMessageAttachment(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parseStudentID(w, r)
	if !ok {
		return
	}
	attachmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "attachmentId", "invalid attachment ID")
	if !ok {
		return
	}
	files, err := rs.messageAttachmentFiles()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	stored, err := rs.ParentService.OpenChildMessageAttachment(r.Context(), accountID, studentID, attachmentID)
	if err != nil {
		renderMessageAttachmentError(w, r, err)
		return
	}
	if !common.ServeMessageAttachment(w, r, files, stored.TenantID, stored.StoredName, stored.Attachment.Filename, stored.Attachment.ContentType) {
		common.RenderError(w, r, common.ErrorNotFound(parentService.ErrMessageAttachmentNotFound))
	}
}

func (rs *Resource) messageAttachmentFiles() (common.MessageAttachmentFiles, error) {
	if rs.MessageAttachmentFiles == nil {
		return nil, errors.New("message attachments are not configured")
	}
	files, err := rs.MessageAttachmentFiles()
	if err != nil {
		return nil, fmt.Errorf("message attachment storage unavailable: %w", err)
	}
	return files, nil
}

// renderMessageAttachmentError answers the refusals of the attachment store
// and leaves the child and permission refusals to the shared write errors.
func renderMessageAttachmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, parentService.ErrMessageAttachmentNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrMessageAttachmentLimit):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeFilesAttachmentLimitReached))
	case errors.Is(err, parentService.ErrMessageAttachmentUnavailable):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		renderParentWriteError(w, r, err)
	}
}

// parseMessageAttachmentIDs reads the attachment_ids of a send request and
// answers a malformed list itself.
func parseMessageAttachmentIDs(w http.ResponseWriter, r *http.Request, raw []string) ([]int64, bool) {
	ids, err := common.ParseMessageAttachmentIDs(raw, maxMessageAttachments)
	if errors.Is(err, common.ErrTooManyMessageAttachments) {
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeFilesAttachmentLimitReached))
		return nil, false
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil, false
	}
	return ids, true
}

// messagePreview is the list text of a message: its body, or the name of its
// first file for a message without text.
func messagePreview(body string, attachments json.RawMessage) string {
	if strings.TrimSpace(body) != "" || len(attachments) == 0 {
		return body
	}
	var files []parentService.MessageAttachment
	if err := json.Unmarshal(attachments, &files); err != nil || len(files) == 0 {
		return body
	}
	return "📎 " + files[0].Filename
}
