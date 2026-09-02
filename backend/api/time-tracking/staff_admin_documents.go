package timetracking

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Staff documents (#1424, Dokumente tab phase 1). File bytes live on the
// filesystem under a per-tenant directory with UUID filenames; only these
// handlers can reach them (backend public/ has no static file server). The
// handlers own multipart parsing, magic-number validation and disk I/O —
// authority, metadata and audit live in the document service, mirroring the
// student-photo split.

const (
	// maxStaffDocumentFile is the advertised upload cap (10 MB per #1424);
	// the multipart body cap adds headroom for boundaries and headers.
	maxStaffDocumentFile  = 10 * 1024 * 1024
	maxStaffDocumentBody  = maxStaffDocumentFile + 4096
	staffDocumentsBaseDir = "public/uploads/staff-documents"
)

// staffDocumentsDir is the per-tenant document directory. Save, serve and
// remove all derive it from the same ResolvePublicDir resolution so the
// three operations can never disagree about where the bytes live (the
// relative fallback only applies when no public dir exists at all).
func staffDocumentsDir(tenantID int64) string {
	base := staffDocumentsBaseDir
	if pubDir, err := common.ResolvePublicDir(); err == nil {
		base = filepath.Join(pubDir, "uploads", "staff-documents")
	}
	return filepath.Join(base, strconv.FormatInt(tenantID, 10))
}

// documentActor builds the service actor from the JWT: account id for audit
// rows, comma-joined roles for the access log, permissions for the
// per-category checks.
func documentActor(r *http.Request) (usersSvc.StaffDocumentActor, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return usersSvc.StaffDocumentActor{}, errors.New("invalid token: no account id")
	}
	return usersSvc.StaffDocumentActor{
		AccountID:   int64(claims.ID),
		Role:        strings.Join(claims.Roles, ","),
		Permissions: jwt.PermissionsFromCtx(r.Context()),
	}, nil
}

// --- wire types ------------------------------------------------------------

// StaffDocumentResponse is one document on the wire.
type StaffDocumentResponse struct {
	ID          int64          `json:"id"`
	StaffID     int64          `json:"staff_id"`
	Category    string         `json:"category"`
	Filename    string         `json:"filename"`
	SizeBytes   int64          `json:"size_bytes"`
	ContentType string         `json:"content_type"`
	UploadedAt  time.Time      `json:"uploaded_at"`
	RetainUntil *timezone.Date `json:"retain_until,omitempty"`
	ReviewDue   *timezone.Date `json:"review_due,omitempty"`
}

// StaffDocumentListResponse is the list GET: documents the caller may see
// plus the caller's visible categories (drives the filter and the upload
// category picker).
type StaffDocumentListResponse struct {
	StaffID           int64                   `json:"staff_id"`
	Documents         []StaffDocumentResponse `json:"documents"`
	VisibleCategories []string                `json:"visible_categories"`
}

func newStaffDocumentResponse(info *usersSvc.StaffDocumentInfo) StaffDocumentResponse {
	doc := info.Document
	return StaffDocumentResponse{
		ID:          doc.ID,
		StaffID:     doc.StaffID,
		Category:    doc.Category,
		Filename:    doc.FilenameDisplay,
		SizeBytes:   doc.SizeBytes,
		ContentType: doc.ContentType,
		UploadedAt:  doc.CreatedAt,
		RetainUntil: info.RetainUntil,
		ReviewDue:   info.ReviewDue,
	}
}

// renderDocumentError maps document service errors onto HTTP responses.
func renderDocumentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usersSvc.ErrStaffDocumentForbidden):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, usersSvc.ErrStaffDocumentInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case common.IsNotFound(err):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// --- handlers --------------------------------------------------------------

// listStaffDocuments serves GET /{id}/documents?category=x.
func (rs *StaffAdminResource) listStaffDocuments(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	actor, err := documentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	infos, visible, err := rs.StaffDocumentService.ListStaffDocuments(r.Context(), id, r.URL.Query().Get("category"), actor)
	if err != nil {
		renderDocumentError(w, r, err)
		return
	}
	// Retry only files whose prior post-commit unlink did not finish. The hooks
	// run after this read transaction commits, so list queries never do I/O in
	// their tenant transaction.
	if documents, err := rs.StaffDocumentService.ListDeletedStaffDocumentsPendingFileCleanup(r.Context(), id, actor); err != nil {
		rs.logger.Warn("staff document cleanup retry lookup failed",
			"staff_id", id,
			"error", err,
		)
	} else {
		cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(r.Context()))
		for _, document := range documents {
			docID, storedName := document.ID, document.FilenameStored
			tenant.RegisterAfterCommit(r.Context(), func() {
				rs.cleanupStaffDocumentFile(cleanupCtx, id, docID, storedName, "retry")
			})
		}
	}
	rs.retryQueuedStaffDocumentCleanups(r.Context(), "retry")
	resp := &StaffDocumentListResponse{
		StaffID:           id,
		Documents:         make([]StaffDocumentResponse, 0, len(infos)),
		VisibleCategories: visible,
	}
	for _, info := range infos {
		resp.Documents = append(resp.Documents, newStaffDocumentResponse(info))
	}
	common.Respond(w, r, http.StatusOK, resp, "Staff documents retrieved successfully")
}

// RetryQueuedStaffDocumentCleanups is the seam the composition root binds the
// membership adapter's document-directory read to (#2667). The directory
// route lives with the membership adapter now, but the file bytes and their
// cleanup bookkeeping stayed here, so the adapter calls back into this
// resource. It is a thin export on purpose: the behaviour must remain exactly
// what the old api/staff listDocumentDirectory hook did, which was one call to
// retryQueuedStaffDocumentCleanups(ctx, "directory") — offboarded pass first,
// then the queued intents, both registered as after-commit work.
func (rs *StaffAdminResource) RetryQueuedStaffDocumentCleanups(ctx context.Context, source string) {
	rs.retryQueuedStaffDocumentCleanups(ctx, source)
}

// retryQueuedStaffDocumentCleanups retries every orphaned upload in the
// current tenant. It intentionally does not look up staff records: metadata
// creation can fail after a concurrent offboarding, but its uploaded bytes
// must remain eligible for deletion.
func (rs *StaffAdminResource) retryQueuedStaffDocumentCleanups(ctx context.Context, source string) {
	rs.retryOffboardedStaffDocumentCleanups(ctx, source)
	cleanups, err := rs.StaffDocumentService.ListQueuedStaffDocumentFileCleanups(ctx)
	if err != nil {
		rs.logger.Warn("staff document orphan cleanup retry lookup failed",
			"source", source,
			"error", err,
		)
		return
	}
	cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(ctx))
	for _, cleanup := range cleanups {
		staffID, cleanupID, storedName := cleanup.StaffID, cleanup.ID, cleanup.FilenameStored
		tenant.RegisterAfterCommit(ctx, func() {
			rs.cleanupQueuedStaffDocumentFile(cleanupCtx, staffID, cleanupID, storedName, source)
		})
	}
}

// retryOffboardedStaffDocumentCleanups retries files for soft-deleted staff
// records without resolving those records through an active-staff route.
func (rs *StaffAdminResource) retryOffboardedStaffDocumentCleanups(ctx context.Context, source string) {
	documents, err := rs.StaffDocumentService.ListOffboardedStaffDocumentsPendingFileCleanup(ctx)
	if err != nil {
		rs.logger.Warn("offboarded staff document cleanup retry lookup failed",
			"source", source,
			"error", err,
		)
		return
	}
	cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(ctx))
	for _, document := range documents {
		staffID, docID, storedName := document.StaffID, document.ID, document.FilenameStored
		tenant.RegisterAfterCommit(ctx, func() {
			rs.cleanupStaffDocumentFile(cleanupCtx, staffID, docID, storedName, source)
		})
	}
}

// uploadStaffDocument serves POST /{id}/documents (multipart: file +
// category). The file is written before its metadata because multipart input
// cannot be replayed after commit; CreateStaffDocument commits its metadata
// transaction before returning, and this handler removes the file on failure.
func (rs *StaffAdminResource) uploadStaffDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	actor, err := documentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	uploaded, err := common.ParseDocumentWithLimits(w, r, "file", maxStaffDocumentFile, maxStaffDocumentBody)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	storedName, err := newStoredDocumentName(uploaded.ContentType)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if err := rs.StaffDocumentService.QueueStaffDocumentFileCleanup(r.Context(), id, storedName); err != nil {
		rs.logger.Error("staff document upload cleanup intent failed",
			"staff_id", id,
			"error", err,
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// The queued intent becomes eligible for the cleanup scheduler after a fixed
	// delay, so every step that can still persist metadata for this file must be
	// bounded well inside that delay: a stalled write or metadata transaction
	// would otherwise commit a document row after the scheduler had already
	// removed its bytes. The bookkeeping calls below deliberately stay on the
	// request context so they still run once the upload deadline has fired.
	uploadCtx, cancelUpload := context.WithTimeout(r.Context(), usersSvc.StaffDocumentUploadDeadline)
	defer cancelUpload()

	dir := staffDocumentsDir(tenant.FromContext(r.Context()))
	filePath, err := common.SavePrivateNamedFile(uploaded.File, dir, storedName)
	if err != nil {
		if completeErr := rs.StaffDocumentService.MarkQueuedStaffDocumentFileCleanupCompleteByFilename(r.Context(), storedName); completeErr != nil {
			rs.logger.Warn("staff document cleanup intent completion failed after upload write error",
				"staff_id", id,
				"error", completeErr,
			)
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	info, err := rs.StaffDocumentService.CreateStaffDocument(uploadCtx, usersSvc.CreateStaffDocumentInput{
		StaffID:         id,
		Category:        r.FormValue("category"),
		FilenameDisplay: uploaded.Filename,
		FilenameStored:  storedName,
		SizeBytes:       fileSizeOnDisk(filePath),
		ContentType:     uploaded.ContentType,
	}, actor)
	if err != nil {
		rs.releaseFailedDocumentUpload(r.Context(), id, filePath, storedName, err)
		renderDocumentError(w, r, err)
		return
	}

	resp := newStaffDocumentResponse(info)
	common.Respond(w, r, http.StatusCreated, &resp, "Staff document uploaded successfully")
}

// downloadStaffDocument serves GET /{id}/documents/{documentId}/download.
// The service refuses sensitive categories without an access-log row.
func (rs *StaffAdminResource) downloadStaffDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	docID, ok := common.ParseInt64IDWithError(w, r, "documentId", "invalid document ID")
	if !ok {
		return
	}
	actor, err := documentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	doc, err := rs.StaffDocumentService.ResolveStaffDocumentDownload(r.Context(), id, docID, actor)
	if err != nil {
		renderDocumentError(w, r, err)
		return
	}

	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": doc.FilenameDisplay,
	}))
	w.Header().Set("Content-Type", doc.ContentType)
	common.ServeFile(w, r, staffDocumentsDir(tenant.FromContext(r.Context())), doc.FilenameStored, "private, no-store")
}

// deleteStaffDocument serves DELETE /{id}/documents/{documentId}: audited
// soft delete of the metadata row, then removal of the file bytes. The row
// survives as the trail; the content does not.
func (rs *StaffAdminResource) deleteStaffDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	docID, ok := common.ParseInt64IDWithError(w, r, "documentId", "invalid document ID")
	if !ok {
		return
	}
	actor, err := documentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	doc, err := rs.StaffDocumentService.DeleteStaffDocument(r.Context(), id, docID, actor)
	if err != nil {
		if !common.IsNotFound(err) {
			renderDocumentError(w, r, err)
			return
		}
		doc, err = rs.StaffDocumentService.ResolveStaffDocumentCleanup(r.Context(), id, docID, actor)
		if err != nil {
			renderDocumentError(w, r, err)
			return
		}
	}

	if doc.FileDeletedAt == nil {
		cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(r.Context()))
		tenant.RegisterAfterCommit(r.Context(), func() {
			rs.cleanupStaffDocumentFile(cleanupCtx, id, doc.ID, doc.FilenameStored, "delete")
		})
	}
	common.Respond(w, r, http.StatusOK, &stammdatenAck{StaffID: id}, "Staff document deleted successfully")
}

// --- helpers ---------------------------------------------------------------

// newStoredDocumentName generates the on-disk UUID filename for a validated
// content type.
func newStoredDocumentName(contentType string) (string, error) {
	ext := common.DocumentFileExtension(contentType)
	if ext == "" {
		return "", errors.New("unsupported document content type")
	}
	id, err := uuid.NewV4()
	if err != nil {
		return "", errors.New("failed to generate document filename")
	}
	return id.String() + ext, nil
}

// isUnresolvedUploadError reports whether an upload failure leaves the metadata
// transaction's outcome undecided, in which case the file must not be removed
// by this request.
func isUnresolvedUploadError(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// releaseFailedDocumentUpload disposes of the bytes of an upload whose metadata
// could not be persisted, and settles its queued cleanup intent: completed when
// the file is gone, re-activated when removal failed so the scheduler retries.
// A cancelled or timed-out metadata transaction is the one case that touches
// neither — its commit may still have landed, and deleting the file here would
// strand a committed document row. The intent decides instead: a committed
// transaction marks it complete (the file must stay), a rolled-back one leaves
// it queued for the scheduler.
func (rs *StaffAdminResource) releaseFailedDocumentUpload(ctx context.Context, staffID int64, filePath, storedName string, cause error) {
	if isUnresolvedUploadError(cause) {
		rs.logger.Warn("staff document upload interrupted, leaving cleanup to the queued intent",
			"staff_id", staffID,
			"error", cause,
		)
		return
	}
	if err := rs.removeUploadedDocument(filePath); err != nil {
		rs.logger.Error("staff document cleanup failed after upload error",
			"staff_id", staffID,
			"error", err,
		)
		if activateErr := rs.StaffDocumentService.ActivateQueuedStaffDocumentFileCleanup(ctx, storedName); activateErr != nil {
			rs.logger.Error("staff document cleanup intent activation failed",
				"staff_id", staffID,
				"error", activateErr,
			)
		}
		return
	}
	if completeErr := rs.StaffDocumentService.MarkQueuedStaffDocumentFileCleanupCompleteByFilename(ctx, storedName); completeErr != nil {
		rs.logger.Warn("staff document cleanup intent completion failed",
			"staff_id", staffID,
			"error", completeErr,
		)
	}
}

// fileSizeOnDisk returns the stored file's size; 0 on stat errors (the
// metadata row still records the upload — size is display data only).
func fileSizeOnDisk(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (rs *StaffAdminResource) removeStoredDocumentForTenant(tenantID int64, storedName string) error {
	if common.ValidateFilename(storedName) != nil {
		return errors.New("stored document filename failed validation on delete")
	}
	dir := staffDocumentsDir(tenantID)
	if err := os.Remove(filepath.Join(dir, storedName)); err != nil && !os.IsNotExist(err) {
		return errors.New("failed to remove stored document")
	}
	return nil
}

func (rs *StaffAdminResource) cleanupStaffDocumentFile(ctx context.Context, staffID, documentID int64, storedName, source string) {
	tenantID := tenant.FromContext(ctx)
	if err := rs.removeStoredDocumentForTenant(tenantID, storedName); err != nil {
		rs.logger.Warn("staff document cleanup failed",
			"staff_id", staffID,
			"document_id", documentID,
			"source", source,
			"error", err,
		)
		return
	}
	if err := rs.StaffDocumentService.MarkStaffDocumentFileDeleted(ctx, documentID); err != nil {
		rs.logger.Error("staff document cleanup status update failed",
			"staff_id", staffID,
			"document_id", documentID,
			"source", source,
			"error", err,
		)
	}
}

func (rs *StaffAdminResource) cleanupQueuedStaffDocumentFile(ctx context.Context, staffID, cleanupID int64, storedName, source string) {
	tenantID := tenant.FromContext(ctx)
	if err := rs.removeStoredDocumentForTenant(tenantID, storedName); err != nil {
		rs.logger.Warn("staff document orphan cleanup failed",
			"staff_id", staffID,
			"cleanup_id", cleanupID,
			"source", source,
			"error", err,
		)
		return
	}
	if err := rs.StaffDocumentService.MarkQueuedStaffDocumentFileCleanupComplete(ctx, cleanupID); err != nil {
		rs.logger.Error("staff document orphan cleanup status update failed",
			"staff_id", staffID,
			"cleanup_id", cleanupID,
			"source", source,
			"error", err,
		)
	}
}

func (rs *StaffAdminResource) removeUploadedDocument(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return errors.New("failed to remove stored document")
	}
	return nil
}

// ResolveEditorStaffID exposes the JWT-account-to-staff resolution for the
// composition root, which needs the acting staff member for offboarding
// tombstones (#2667).
func (rs *StaffAdminResource) ResolveEditorStaffID(ctx context.Context) (int64, error) {
	return rs.resolveEditorStaffID(ctx)
}

// QueueOffboardedStaffDocumentCleanup registers the after-commit file
// removals for an offboarded staff member: every document whose bytes are
// still stored and every queued upload intent. It runs inside the
// offboarding transaction so a rolled-back offboarding queues nothing.
func (rs *StaffAdminResource) QueueOffboardedStaffDocumentCleanup(ctx context.Context, staffID int64) error {
	cleanupCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(ctx))
	documents, err := rs.StaffDocumentService.ListStaffDocumentsPendingFileCleanup(ctx, staffID)
	if err != nil {
		return err
	}
	for _, document := range documents {
		docID, storedName := document.ID, document.FilenameStored
		tenant.RegisterAfterCommit(ctx, func() {
			rs.cleanupStaffDocumentFile(cleanupCtx, staffID, docID, storedName, "offboarding")
		})
	}
	cleanups, err := rs.StaffDocumentService.ListQueuedStaffDocumentFileCleanup(ctx, staffID)
	if err != nil {
		return err
	}
	for _, cleanup := range cleanups {
		cleanupID, storedName := cleanup.ID, cleanup.FilenameStored
		tenant.RegisterAfterCommit(ctx, func() {
			rs.cleanupQueuedStaffDocumentFile(cleanupCtx, staffID, cleanupID, storedName, "offboarding")
		})
	}
	return nil
}
