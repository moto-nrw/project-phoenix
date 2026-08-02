package staff

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
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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
	case modelBase.IsNoRows(err):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// --- handlers --------------------------------------------------------------

// listStaffDocuments serves GET /{id}/documents?category=x.
func (rs *Resource) listStaffDocuments(w http.ResponseWriter, r *http.Request) {
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
		rs.getLogger().Warn("staff document cleanup retry lookup failed",
			"staff_id", id,
			"error", err,
		)
	} else {
		tenantID := tenant.FromContext(r.Context())
		for _, document := range documents {
			docID, storedName := document.ID, document.FilenameStored
			tenant.RegisterAfterCommit(r.Context(), func() {
				rs.cleanupStaffDocumentFile(tenantID, id, docID, storedName, "retry")
			})
		}
	}
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

// uploadStaffDocument serves POST /{id}/documents (multipart: file +
// category). The file is written before its metadata because multipart input
// cannot be replayed after commit; a rollback hook removes it if that metadata
// transaction fails to commit.
func (rs *Resource) uploadStaffDocument(w http.ResponseWriter, r *http.Request) {
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

	dir := staffDocumentsDir(tenant.FromContext(r.Context()))
	filePath, err := common.SaveNamedFile(uploaded.File, dir, storedName)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	tenant.RegisterAfterRollback(r.Context(), func() {
		if err := rs.removeUploadedDocument(filePath); err != nil {
			rs.getLogger().Error("staff document cleanup failed after upload rollback",
				"staff_id", id,
				"error", err,
			)
		}
	})

	info, err := rs.StaffDocumentService.CreateStaffDocument(r.Context(), usersSvc.CreateStaffDocumentInput{
		StaffID:         id,
		Category:        r.FormValue("category"),
		FilenameDisplay: uploaded.Filename,
		FilenameStored:  storedName,
		SizeBytes:       fileSizeOnDisk(filePath),
		ContentType:     uploaded.ContentType,
	}, actor)
	if err != nil {
		if err := rs.removeUploadedDocument(filePath); err != nil {
			rs.getLogger().Warn("staff document cleanup failed after upload error",
				"staff_id", id,
				"error", err,
			)
		}
		renderDocumentError(w, r, err)
		return
	}

	resp := newStaffDocumentResponse(info)
	common.Respond(w, r, http.StatusCreated, &resp, "Staff document uploaded successfully")
}

// downloadStaffDocument serves GET /{id}/documents/{documentId}/download.
// The service refuses sensitive categories without an access-log row.
func (rs *Resource) downloadStaffDocument(w http.ResponseWriter, r *http.Request) {
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
func (rs *Resource) deleteStaffDocument(w http.ResponseWriter, r *http.Request) {
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
		if !modelBase.IsNoRows(err) {
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
		tenantID := tenant.FromContext(r.Context())
		tenant.RegisterAfterCommit(r.Context(), func() {
			rs.cleanupStaffDocumentFile(tenantID, id, doc.ID, doc.FilenameStored, "delete")
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

// fileSizeOnDisk returns the stored file's size; 0 on stat errors (the
// metadata row still records the upload — size is display data only).
func fileSizeOnDisk(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// removeStoredDocument deletes one stored file. Callers invoke it only after
// their metadata transaction commits, so a failed transaction never destroys
// bytes that still back a visible document.
func (rs *Resource) removeStoredDocument(r *http.Request, storedName string) error {
	return rs.removeStoredDocumentForTenant(tenant.FromContext(r.Context()), storedName)
}

func (rs *Resource) removeStoredDocumentForTenant(tenantID int64, storedName string) error {
	if common.ValidateFilename(storedName) != nil {
		return errors.New("stored document filename failed validation on delete")
	}
	dir := staffDocumentsDir(tenantID)
	if err := os.Remove(filepath.Join(dir, storedName)); err != nil && !os.IsNotExist(err) {
		return errors.New("failed to remove stored document")
	}
	return nil
}

func (rs *Resource) cleanupStaffDocumentFile(tenantID, staffID, documentID int64, storedName, source string) {
	if err := rs.removeStoredDocumentForTenant(tenantID, storedName); err != nil {
		rs.getLogger().Warn("staff document cleanup failed",
			"staff_id", staffID,
			"document_id", documentID,
			"source", source,
			"error", err,
		)
		return
	}
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	if err := rs.StaffDocumentService.MarkStaffDocumentFileDeleted(ctx, documentID); err != nil {
		rs.getLogger().Error("staff document cleanup status update failed",
			"staff_id", staffID,
			"document_id", documentID,
			"source", source,
			"error", err,
		)
	}
}

func (rs *Resource) removeUploadedDocument(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return errors.New("failed to remove stored document")
	}
	return nil
}
