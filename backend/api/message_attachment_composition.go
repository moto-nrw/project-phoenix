package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	communicationModule "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/http/messageattachments"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	fileCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// newMessageAttachmentRoutes builds the upload and read routes of one chat
// family's photos and files (#6685) behind the family's route permission.
func newMessageAttachmentRoutes(attachments communicationModule.MessageAttachmentCapability, logger *slog.Logger, guard ...func(http.Handler) http.Handler) *messageattachments.Resource {
	return messageattachments.NewResource(messageAttachmentUnits{attachments: attachments}, NewMessageAttachmentStorage(logger), guard...)
}

// schoolMessageAttachmentRouter serves the team chat's attachment routes to
// the school portal of the Lehrkräfte. They sit at the root beside the other
// school-portal streams, because the portal resource registers its own
// routes under /school.
func schoolMessageAttachmentRouter(attachments *messageattachments.Resource) chi.Router {
	router := chi.NewRouter()
	ProtectedSchoolRoutes(router, attachments.Register)
	return router
}

// messageAttachmentUnits opens the tenant units of the attachment routes.
// The upload row commits in its own unit before the route writes the bytes; a
// failed write discards the row in a new one. Opening a file authorizes the
// reader in a unit that ends before the bytes are streamed.
type messageAttachmentUnits struct {
	attachments communicationModule.MessageAttachmentCapability
}

func (u messageAttachmentUnits) PrepareMessageAttachment(ctx context.Context, upload communicationModule.MessageAttachmentUpload) (*communicationModule.PreparedMessageAttachment, error) {
	var prepared *communicationModule.PreparedMessageAttachment
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var prepareErr error
		prepared, prepareErr = u.attachments.PrepareMessageAttachment(txCtx, upload)
		return prepareErr
	})
	return prepared, err
}

func (u messageAttachmentUnits) DiscardMessageAttachment(ctx context.Context, attachmentID int64) error {
	return tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return u.attachments.DiscardMessageAttachment(txCtx, attachmentID)
	})
}

func (u messageAttachmentUnits) OpenMessageAttachment(ctx context.Context, attachmentID int64) (*communicationModule.MessageAttachmentFile, error) {
	var file *communicationModule.MessageAttachmentFile
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var openErr error
		file, openErr = u.attachments.OpenMessageAttachment(txCtx, attachmentID)
		return openErr
	})
	return file, err
}

// NewMessageAttachmentStorage opens the private store of the photos and files
// of chat messages (#6685) through File Storage's document coordinator. The
// backend is looked up on each call, so constructing the routes opens no
// storage.
func NewMessageAttachmentStorage(logger *slog.Logger) common.MessageAttachmentFileStorage {
	return func() (common.MessageAttachmentFiles, error) {
		backend, err := nativeUploads.PrivateUploadsBackend()
		if err != nil {
			return nil, err
		}
		return fileCompose.NewDocumentCoordinator(communicationModule.MessageAttachmentObjectKind, backend, messageAttachmentObjectStore{}, logger), nil
	}
}

// messageAttachmentObjectStore settles nothing: an attachment's cleanup
// intent is queued by the database when its row is deleted and settled by
// the Worker's sweep, not by the coordinator.
type messageAttachmentObjectStore struct{}

func (messageAttachmentObjectStore) MarkFileDeleted(context.Context, int64) error { return nil }
func (messageAttachmentObjectStore) MarkQueuedCleanupCompleteByFilename(context.Context, string) error {
	return nil
}
func (messageAttachmentObjectStore) ActivateQueuedCleanup(context.Context, string) error { return nil }
