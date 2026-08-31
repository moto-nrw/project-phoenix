package filestore

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	apiDocuments "github.com/moto-nrw/project-phoenix/api/common/documents"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/filestore"
	filestoreSvc "github.com/moto-nrw/project-phoenix/services/filestore"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Anhänge an Elternmitteilungen (#2890).
//
// Warum diese Routen hier liegen und nicht bei der Mitteilung: die Bytes einer
// hochgeladenen Datei sind Sache dieses Moduls — Storage-Schlüssel,
// Magic-Byte-Prüfung, Cleanup-Intents, Audit-Spur. Die Mitteilung steuert nur
// bei, wer die Datei sehen darf, und tut das über einen Port
// (services/filestore.AnnouncementAudience). Beide Router werden im
// Kompositions-Wurzelpunkt gemountet.
//
// Der Anhang geht bewusst NICHT mit der E-Mail raus. Elternmitteilungen kennen
// zwei E-Mail-Empfängerkreise, und "alle Bezugspersonen" erreicht auch Adressen
// ohne Portalzugang. Ein Anhang an eine solche Adresse würde genau die
// Zugangskontrolle umgehen, die "nur mit Portalzugang" herstellt — die E-Mail
// verweist deshalb nur auf das Portal.

const (
	// attachmentStorageKind is the storage key prefix of the attachments. It
	// is deliberately distinct from the school storage: the two have separate
	// tables, separate cleanup intents and separate audiences, and a shared
	// prefix would let one module's sweep reach the other's objects.
	attachmentStorageKind = "announcement-attachments"
)

// AnnouncementAttachmentResponse is one attachment on the wire.
type AnnouncementAttachmentResponse struct {
	ID          string    `json:"id"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// AnnouncementAttachmentListResponse is the attachment list of one
// announcement, together with the limits the UI must state BEFORE somebody
// picks a file — an error message after a failed upload is too late.
type AnnouncementAttachmentListResponse struct {
	Attachments []AnnouncementAttachmentResponse `json:"attachments"`
	MaxCount    int                              `json:"max_count"`
	MaxBytes    int64                            `json:"max_bytes"`
	// Editable is false once the announcement is published: attachments are
	// then fixed, and the UI says so instead of offering a button that 409s.
	Editable bool `json:"editable"`
}

func newAttachmentResponse(attachment *filestore.AnnouncementAttachment) AnnouncementAttachmentResponse {
	return AnnouncementAttachmentResponse{
		ID:          idString(attachment.ID),
		Filename:    attachment.FilenameDisplay,
		SizeBytes:   attachment.SizeBytes,
		ContentType: attachment.ContentType,
		UploadedAt:  attachment.CreatedAt,
	}
}

func newAttachmentListResponse(attachments []*filestore.AnnouncementAttachment, editable bool) *AnnouncementAttachmentListResponse {
	resp := &AnnouncementAttachmentListResponse{
		Attachments: make([]AnnouncementAttachmentResponse, 0, len(attachments)),
		MaxCount:    filestore.MaxAnnouncementAttachments,
		MaxBytes:    maxFile,
		Editable:    editable,
	}
	for _, attachment := range attachments {
		resp.Attachments = append(resp.Attachments, newAttachmentResponse(attachment))
	}
	return resp
}

// AnnouncementAttachmentRouter serves the staff side under
// /api/announcement-attachments. It is gated on the same permission as the
// announcements themselves (#1669): who may write an Elternmitteilung may
// attach a file to it.
//
// Upload and download skip withTx so a slow body or file stream does not pin a
// pool connection; those handlers open their own short transactions.
func (rs *Resource) AnnouncementAttachmentRouter() chi.Router {
	r := chi.NewRouter()
	common.ProtectedTenantGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {
		admin := common.RequiresPermission(permissions.AdminWildcard)

		r.With(admin, withTx).Get("/{announcementId}", rs.listAnnouncementAttachments)
		r.With(admin).Post("/{announcementId}", rs.uploadAnnouncementAttachment)
		r.With(admin).Get("/{announcementId}/{attachmentId}/download", rs.downloadAnnouncementAttachment)
		r.With(admin, withTx).Delete("/{announcementId}/{attachmentId}", rs.deleteAnnouncementAttachment)
	})
	return r
}

// ParentAnnouncementAttachmentRouter serves the guardian side. It is mounted
// at the root rather than under /parent, which is a catch-all mount, and is
// authenticated with ParentMiddleware — same shape as the parent SSE stream.
//
// No tenant transaction: a parent token is cross-tenant, so the school is
// resolved from the announcement AFTER the audience check, and the row lookup
// then runs under that school.
func (rs *Resource) ParentAnnouncementAttachmentRouter() chi.Router {
	r := chi.NewRouter()
	common.ProtectedParentGroup(r, func(r chi.Router) {
		r.Get("/{announcementId}", rs.listParentAnnouncementAttachments)
		r.Get("/{announcementId}/{attachmentId}/download", rs.downloadParentAnnouncementAttachment)
	})
	return r
}

// attachmentCoordinator builds the storage coordinator for attachments. Its
// Store is the attachment half of the service — using the folder-file store
// here would settle the wrong cleanup intents.
func (rs *Resource) attachmentCoordinator() (*apiDocuments.Coordinator, error) {
	backend, err := common.UploadsBackend()
	if err != nil {
		return nil, err
	}
	return &apiDocuments.Coordinator{
		Kind:    attachmentStorageKind,
		Backend: backend,
		Store:   rs.Service.AttachmentStore(),
		Logger:  rs.getLogger(),
	}, nil
}

// renderAttachmentError maps the attachment service errors onto HTTP.
func renderAttachmentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, filestoreSvc.ErrAttachmentNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, filestoreSvc.ErrAttachmentAnnouncementPublished):
		common.RenderError(w, r, common.ErrorConflictWithCode(
			errors.New("Die Mitteilung ist schon veröffentlicht. Ziehen Sie sie zurück, um Anhänge zu ändern."),
			"announcement_published"))
	case errors.Is(err, filestoreSvc.ErrAttachmentLimitReached):
		common.RenderError(w, r, common.ErrorConflictWithCode(
			errors.New("Diese Mitteilung hat schon die höchstmögliche Zahl an Anhängen."),
			"attachment_limit_reached"))
	default:
		renderError(w, r, err)
	}
}

// --- staff handlers -----------------------------------------------------------

func (rs *Resource) listAnnouncementAttachments(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	attachments, err := rs.Service.ListAttachments(r.Context(), announcementID)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	// Whether the announcement may still be changed decides what the UI
	// offers; asking for it separately would race with the list.
	editable := rs.Service.AuthorizeAttachmentUpload(r.Context(), announcementID) == nil
	common.Respond(w, r, http.StatusOK, newAttachmentListResponse(attachments, editable), "Attachments retrieved successfully")
}

func (rs *Resource) uploadAnnouncementAttachment(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}
	coordinator, err := rs.attachmentCoordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	uploaded, err := common.ParseOfficeFileWithLimits(w, r, "file", maxFile, maxBody)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(common.GermanUploadError(err, maxFile)))
		return
	}
	defer common.CloseFile(uploaded.File)

	// Authorize BEFORE anything is written, so a request that was never
	// allowed costs neither disk nor a cleanup row.
	if err := rs.Service.AuthorizeAttachmentUpload(r.Context(), announcementID); err != nil {
		renderAttachmentError(w, r, err)
		return
	}

	storedName, err := apiDocuments.NewStoredName(common.DocumentFileExtension(uploaded.ContentType))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if err := rs.Service.QueueAttachmentCleanup(r.Context(), announcementID, storedName); err != nil {
		rs.getLogger().Error("announcement attachment cleanup intent failed",
			"announcement_id", announcementID,
			"error", err)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	uploadCtx, cancelUpload := context.WithTimeout(r.Context(), filestoreSvc.UploadDeadline)
	defer cancelUpload()

	size, ok := rs.writeAttachmentObject(w, r, uploadCtx, coordinator, tenantID, announcementID, storedName, uploaded)
	if !ok {
		return
	}

	attachment, err := rs.Service.CreateAttachment(uploadCtx, filestoreSvc.CreateAttachmentInput{
		AnnouncementID:  announcementID,
		FilenameDisplay: uploaded.Filename,
		FilenameStored:  storedName,
		SizeBytes:       size,
		ContentType:     uploaded.ContentType,
	}, actor)
	if err != nil {
		// Only request-shaped rejections prove the row never landed; anything
		// deeper is left to the queued intent (see the coordinator).
		rejectedBeforeCommit := errors.Is(err, filestoreSvc.ErrInvalid) ||
			errors.Is(err, filestoreSvc.ErrAttachmentNotFound) ||
			errors.Is(err, filestoreSvc.ErrAttachmentAnnouncementPublished) ||
			errors.Is(err, filestoreSvc.ErrAttachmentLimitReached)
		coordinator.ReleaseFailedUpload(r.Context(), tenantID, announcementID, storedName, rejectedBeforeCommit, err)
		renderAttachmentError(w, r, err)
		return
	}

	resp := newAttachmentResponse(attachment)
	common.Respond(w, r, http.StatusCreated, &resp, "Attachment uploaded successfully")
}

// writeAttachmentObject stores the upload bytes within the upload deadline and
// reports whether the request may continue. On a failed or late write the
// queued intent is activated rather than settled: a half-written object must
// stay reachable for the scheduler.
func (rs *Resource) writeAttachmentObject(w http.ResponseWriter, r *http.Request, uploadCtx context.Context, coordinator *apiDocuments.Coordinator, tenantID, announcementID int64, storedName string, uploaded *common.UploadedFile) (int64, bool) {
	size, err := coordinator.Save(uploadCtx, tenantID, storedName, uploaded.File)
	if err == nil && uploadCtx.Err() != nil {
		err = errors.New("upload exceeded its deadline")
	}
	if err == nil {
		return size, true
	}
	if activateErr := rs.Service.ActivateQueuedAttachmentCleanup(r.Context(), storedName); activateErr != nil {
		rs.getLogger().Error("announcement attachment cleanup intent activation failed after write error",
			"announcement_id", announcementID,
			"error", activateErr)
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
	return 0, false
}

func (rs *Resource) downloadAnnouncementAttachment(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	attachmentID, ok := common.ParseInt64IDWithError(w, r, "attachmentId", "invalid attachment ID")
	if !ok {
		return
	}
	coordinator, err := rs.attachmentCoordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	attachment, err := rs.Service.ResolveAttachmentDownload(r.Context(), announcementID, attachmentID)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	rs.serveAttachment(w, r, tenant.FromContext(r.Context()), attachment, coordinator)
}

func (rs *Resource) deleteAnnouncementAttachment(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	attachmentID, ok := common.ParseInt64IDWithError(w, r, "attachmentId", "invalid attachment ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	coordinator, err := rs.attachmentCoordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	attachment, err := rs.Service.DeleteAttachment(r.Context(), announcementID, attachmentID, actor)
	if err != nil {
		if !modelBase.IsNoRows(err) {
			renderAttachmentError(w, r, err)
			return
		}
		// Already soft-deleted: retry the byte removal instead of a 404 the
		// caller cannot act on.
		attachment, err = rs.Service.ResolveAttachmentCleanup(r.Context(), announcementID, attachmentID)
		if err != nil {
			renderAttachmentError(w, r, err)
			return
		}
	}

	if !attachment.IsFileDeleted() {
		tenantID := tenant.FromContext(r.Context())
		id, storedName := attachment.ID, attachment.FilenameStored
		cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(r.Context()))
		cleanupCtx = tenant.ContextWithoutAfterCommitHooks(cleanupCtx)
		tenant.RegisterAfterCommit(r.Context(), func() {
			coordinator.CleanupDocument(cleanupCtx, tenantID, announcementID, id, storedName, "delete")
		})
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"announcement_id": idString(announcementID)}, "Attachment deleted successfully")
}

// --- parent handlers ----------------------------------------------------------

// parentAccountID reads the guardian account from the parent token.
func parentAccountID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token: no account id")))
		return 0, false
	}
	return int64(claims.ID), true
}

func (rs *Resource) listParentAnnouncementAttachments(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	accountID, ok := parentAccountID(w, r)
	if !ok {
		return
	}
	_, attachments, err := rs.Service.ListAttachmentsForGuardian(r.Context(), accountID, announcementID)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	// The parent side never edits attachments; the flag exists so both
	// responses share one shape.
	common.Respond(w, r, http.StatusOK, newAttachmentListResponse(attachments, false), "Attachments retrieved successfully")
}

func (rs *Resource) downloadParentAnnouncementAttachment(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := common.ParseInt64IDWithError(w, r, "announcementId", "invalid announcement ID")
	if !ok {
		return
	}
	attachmentID, ok := common.ParseInt64IDWithError(w, r, "attachmentId", "invalid attachment ID")
	if !ok {
		return
	}
	accountID, ok := parentAccountID(w, r)
	if !ok {
		return
	}
	coordinator, err := rs.attachmentCoordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	tenantID, attachment, err := rs.Service.ResolveGuardianAttachmentDownload(r.Context(), accountID, announcementID, attachmentID)
	if err != nil {
		renderAttachmentError(w, r, err)
		return
	}
	rs.serveAttachment(w, r, tenantID, attachment, coordinator)
}

// serveAttachment streams one attachment. ?inline=1 opens PDFs and images in
// the browser instead of downloading; office files ignore it and download
// either way.
func (rs *Resource) serveAttachment(w http.ResponseWriter, r *http.Request, tenantID int64, attachment *filestore.AnnouncementAttachment, coordinator *apiDocuments.Coordinator) {
	served := false
	if r.URL.Query().Get("inline") == "1" {
		served = coordinator.ServeInline(w, r, tenantID, attachment.FilenameStored, attachment.FilenameDisplay, attachment.ContentType)
	} else {
		served = coordinator.Serve(w, r, tenantID, attachment.FilenameStored, attachment.FilenameDisplay, attachment.ContentType)
	}
	if !served {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("attachment not found")))
	}
}
