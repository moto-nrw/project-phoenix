// Package messageattachments serves the photos and files of chat messages
// (#6685): one upload route and one read route per chat family. The roots
// mount them beside the family's message routes, in the school's portal and
// in the school portal of the Lehrkräfte.
package messageattachments

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/communication"
)

// Units opens the tenant units of the attachment routes. The upload row
// commits in its own unit before the route writes the bytes; a failed write
// discards the row in a new one. The root binds them to a chat family's
// capability, which authorizes the caller.
type Units interface {
	PrepareMessageAttachment(ctx context.Context, upload communication.MessageAttachmentUpload) (*communication.PreparedMessageAttachment, error)
	DiscardMessageAttachment(ctx context.Context, attachmentID int64) error
	OpenMessageAttachment(ctx context.Context, attachmentID int64) (*communication.MessageAttachmentFile, error)
}

// Resource is the attachment routes of one chat family.
type Resource struct {
	units Units
	files common.MessageAttachmentFileStorage
	guard []func(http.Handler) http.Handler
}

// NewResource builds the routes over the family's units and object store.
// guard is the route permission of the family's message routes, if any.
func NewResource(units Units, files common.MessageAttachmentFileStorage, guard ...func(http.Handler) http.Handler) *Resource {
	return &Resource{units: units, files: files, guard: guard}
}

// Register mounts the routes into the protected group of the chat family.
// The routes open their own units, so the group's transaction middleware
// stays unused: the bytes must not be written inside the row's transaction.
func (rs *Resource) Register(r chi.Router, _ common.Middleware) {
	routes := r.With(rs.guard...)
	routes.Post("/attachments", rs.upload)
	routes.Get("/attachments/{attachmentId}/download", rs.download)
}

// upload stores one file for a message the caller is about to send.
func (rs *Resource) upload(w http.ResponseWriter, r *http.Request) {
	files, err := rs.storage()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	parsed, err := common.ParseMessageAttachment(w, r, communication.MaxMessageAttachmentBytes)
	if err != nil {
		common.RenderError(w, r, common.ErrorUpload(err, communication.MaxMessageAttachmentBytes))
		return
	}
	prepared, err := rs.units.PrepareMessageAttachment(r.Context(), communication.MessageAttachmentUpload{
		Filename: parsed.Filename, ContentType: parsed.ContentType, Extension: parsed.Extension,
		SizeBytes: parsed.SizeBytes, Width: parsed.Width, Height: parsed.Height,
	})
	if err != nil {
		common.RenderError(w, r, renderError(err))
		return
	}
	if _, err := files.Save(r.Context(), prepared.TenantID, prepared.StoredName, parsed.Content); err != nil {
		if discardErr := rs.units.DiscardMessageAttachment(r.Context(), prepared.Attachment.ID); discardErr != nil {
			err = errors.Join(err, discardErr)
		}
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("write message attachment: %w", err)))
		return
	}
	common.Respond(w, r, http.StatusCreated, prepared.Attachment, "Attachment stored")
}

// download streams one file to a reader of its conversation.
func (rs *Resource) download(w http.ResponseWriter, r *http.Request) {
	attachmentID, ok := common.ParsePositiveInt64IDWithError(w, r, "attachmentId", "invalid attachment ID")
	if !ok {
		return
	}
	files, err := rs.storage()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	file, err := rs.units.OpenMessageAttachment(r.Context(), attachmentID)
	if err != nil {
		common.RenderError(w, r, renderError(err))
		return
	}
	if !common.ServeMessageAttachment(w, r, files, file.TenantID, file.StoredName, file.Attachment.Filename, file.Attachment.ContentType) {
		common.RenderError(w, r, common.ErrorNotFound(communication.ErrMessageAttachmentNotFound))
	}
}

func (rs *Resource) storage() (common.MessageAttachmentFiles, error) {
	if rs.units == nil || rs.files == nil {
		return nil, errors.New("message attachments are not configured")
	}
	files, err := rs.files()
	if err != nil {
		return nil, fmt.Errorf("message attachment storage unavailable: %w", err)
	}
	return files, nil
}

// errorRules answers the refusals of the attachment routes of both chat
// families with the codes their message routes use.
var errorRules = []common.ErrorRule{
	{Target: communication.ErrMessageAttachmentNotFound, Render: func(err error) render.Renderer { return common.ErrorNotFound(err) }},
	{Target: communication.ErrMessageAttachmentLimit, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, common.CodeFilesAttachmentLimitReached)
	}},
	{Target: communication.ErrMessageAttachmentUnavailable, Render: func(err error) render.Renderer { return common.ErrorInvalidRequest(err) }},
	{Target: communication.ErrParentMessagingDisabled, Render: func(err error) render.Renderer {
		return common.ErrorForbiddenWithCode(err, common.CodeCommunicationParentMessagingDisabled)
	}},
	{Target: communication.ErrStaffMessagingDisabled, Render: func(_ error) render.Renderer {
		return common.ErrorForbiddenMessageWithCode("Der Team-Chat ist für diese Schule nicht eingeschaltet.", common.CodeCommunicationStaffMessagingDisabled)
	}},
	{Target: communication.ErrStaffMessageRecipientNotAvailable, Render: func(_ error) render.Renderer {
		return common.ErrorConflictMessage("Sie können hier keine Dateien senden.")
	}},
	{Target: communication.ErrStaffMessagingNoActor, Render: func(_ error) render.Renderer {
		return common.ErrorForbiddenMessage("Bitte melden Sie sich erneut an.")
	}},
}

var renderError = common.RulesRenderer(errorRules, func(err error) render.Renderer {
	return common.ErrorInternalServerWrap("message attachment request failed", err)
})
