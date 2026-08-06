package students

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	apiDocuments "github.com/moto-nrw/project-phoenix/api/common/documents"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Child documents (#777). The bytes go through the storage backend under
// student-documents/{tenant_id}/{uuid}; only these handlers can reach them
// (backend public/ has no static file server). Multipart parsing, content
// validation and the storage dance live here, authority and audit live in the
// document service — the same split as staff documents and student photos.

const (
	// maxStudentDocumentFile is the advertised upload cap; the multipart body
	// cap adds headroom for boundaries and headers.
	maxStudentDocumentFile = 10 * 1024 * 1024
	maxStudentDocumentBody = maxStudentDocumentFile + 4096
	// studentDocumentKind is the storage key prefix for child documents.
	studentDocumentKind = "student-documents"
)

// studentDocumentCoordinator builds the storage coordinator for child
// documents. It is constructed per request rather than stored on the Resource
// so a bare test Resource stays usable.
func (rs *Resource) studentDocumentCoordinator() (*apiDocuments.Coordinator, error) {
	backend, err := common.UploadsBackend()
	if err != nil {
		return nil, err
	}
	return &apiDocuments.Coordinator{
		Kind:             studentDocumentKind,
		Backend:          backend,
		Store:            rs.StudentDocumentService,
		NewTenantContext: func(tenantID int64) context.Context { return tenant.WithTenantID(context.Background(), tenantID) },
		Logger:           rs.getLogger(),
	}, nil
}

// studentDocumentActor builds the service actor from the JWT: account id for
// audit rows, display name for the change history, roles for the access log,
// permissions for the per-category checks.
func studentDocumentActor(r *http.Request) (usersSvc.StudentDocumentActor, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return usersSvc.StudentDocumentActor{}, errors.New("invalid token: no account id")
	}
	return usersSvc.StudentDocumentActor{
		AccountID: int64(claims.ID),
		// The audit row snapshots the editor's name so the history still
		// reads correctly after a rename or an account deletion.
		Name:        strings.TrimSpace(strings.TrimSpace(claims.FirstName) + " " + strings.TrimSpace(claims.LastName)),
		Role:        strings.Join(claims.Roles, ","),
		Permissions: jwt.PermissionsFromCtx(r.Context()),
	}, nil
}

// --- wire types ------------------------------------------------------------

// StudentDocumentResponse is one document on the wire.
type StudentDocumentResponse struct {
	ID            int64     `json:"id"`
	StudentID     int64     `json:"student_id"`
	Category      string    `json:"category"`
	CategoryLabel string    `json:"category_label"`
	Filename      string    `json:"filename"`
	SizeBytes     int64     `json:"size_bytes"`
	ContentType   string    `json:"content_type"`
	UploadedAt    time.Time `json:"uploaded_at"`
	Sensitive     bool      `json:"sensitive"`
}

// StudentDocumentListResponse is the list GET: the documents the caller may
// see plus the caller's visible categories (drives the filter and the upload
// category picker).
type StudentDocumentListResponse struct {
	StudentID         int64                     `json:"student_id"`
	Documents         []StudentDocumentResponse `json:"documents"`
	VisibleCategories []StudentDocumentCategory `json:"visible_categories"`
}

// StudentDocumentCategory is one selectable category with its German label.
type StudentDocumentCategory struct {
	Value     string `json:"value"`
	Label     string `json:"label"`
	Sensitive bool   `json:"sensitive"`
}

func newStudentDocumentResponse(doc *userModels.StudentDocument) StudentDocumentResponse {
	return StudentDocumentResponse{
		ID:            doc.ID,
		StudentID:     doc.StudentID,
		Category:      doc.Category,
		CategoryLabel: studentDocumentLabel(doc.Category),
		Filename:      doc.FilenameDisplay,
		SizeBytes:     doc.SizeBytes,
		ContentType:   doc.ContentType,
		UploadedAt:    doc.CreatedAt,
		Sensitive:     isSensitiveStudentDocumentCategory(doc.Category),
	}
}

func studentDocumentLabel(category string) string {
	if label := userModels.StudentDocumentCategoryLabels[category]; label != "" {
		return label
	}
	return category
}

func isSensitiveStudentDocumentCategory(category string) bool {
	return userModels.IsHealthStudentDocumentCategory(category) ||
		userModels.IsLegalStudentDocumentCategory(category)
}

// renderStudentDocumentError maps document service errors onto HTTP responses.
func renderStudentDocumentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usersSvc.ErrStudentDocumentForbidden),
		errors.Is(err, usersSvc.ErrStudentDocumentNoAccess):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, usersSvc.ErrStudentDocumentInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case modelBase.IsNoRows(err):
		common.RenderError(w, r, common.ErrorNotFound(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// --- handlers --------------------------------------------------------------

// listStudentDocuments serves GET /{id}/documents?category=x.
func (rs *Resource) listStudentDocuments(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStudentID)
	if !ok {
		return
	}
	actor, err := studentDocumentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	docs, visible, err := rs.StudentDocumentService.ListStudentDocuments(r.Context(), id, r.URL.Query().Get("category"), actor)
	if err != nil {
		renderStudentDocumentError(w, r, err)
		return
	}

	// Retry only objects whose prior post-commit removal did not finish. The
	// hooks run after this read transaction commits, so list queries never do
	// storage I/O inside their tenant transaction.
	rs.retryStudentDocumentCleanups(r.Context(), id, actor, "retry")

	resp := &StudentDocumentListResponse{
		StudentID:         id,
		Documents:         make([]StudentDocumentResponse, 0, len(docs)),
		VisibleCategories: make([]StudentDocumentCategory, 0, len(visible)),
	}
	for _, doc := range docs {
		resp.Documents = append(resp.Documents, newStudentDocumentResponse(doc))
	}
	for _, category := range visible {
		resp.VisibleCategories = append(resp.VisibleCategories, StudentDocumentCategory{
			Value:     category,
			Label:     studentDocumentLabel(category),
			Sensitive: isSensitiveStudentDocumentCategory(category),
		})
	}
	common.Respond(w, r, http.StatusOK, resp, "Student documents retrieved successfully")
}

// retryStudentDocumentCleanups schedules removal of objects whose unlink did
// not finish, for the child in the URL only.
//
// It deliberately does NOT touch queued upload intents, for two reasons. They
// are tenant-wide and uncapped, so a graduate purge could leave hundreds
// eligible and hand the whole backlog to whoever next opens any child's tab —
// hundreds of sequential unlinks and transactions inside one request. And the
// intent scan takes its rows FOR UPDATE SKIP LOCKED to keep a concurrent
// upload from having its bytes deleted, a lock that is released at COMMIT,
// which is before an after-commit hook removes anything. The scheduler owns
// that work instead: it removes inside the same transaction that holds the
// lock, and it reaches every tenant within five minutes.
func (rs *Resource) retryStudentDocumentCleanups(ctx context.Context, studentID int64, actor usersSvc.StudentDocumentActor, source string) {
	coordinator, err := rs.studentDocumentCoordinator()
	if err != nil {
		rs.getLogger().Warn("student document cleanup retry unavailable", "error", err)
		return
	}
	tenantID := tenant.FromContext(ctx)

	documents, err := rs.StudentDocumentService.ListDeletedStudentDocumentsPendingFileCleanup(ctx, studentID, actor)
	if err != nil {
		rs.getLogger().Warn("student document cleanup retry lookup failed",
			"student_id", studentID, "error", err)
		return
	}
	for _, document := range documents {
		docID, storedName := document.ID, document.FilenameStored
		tenant.RegisterAfterCommit(ctx, func() {
			coordinator.CleanupDocument(tenantID, studentID, docID, storedName, source)
		})
	}
}

// uploadStudentDocument serves POST /{id}/documents (multipart: file +
// category). The object is written before its metadata because multipart
// input cannot be replayed after commit; CreateStudentDocument commits its
// metadata transaction before returning, and this handler disposes of the
// object on failure.
func (rs *Resource) uploadStudentDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStudentID)
	if !ok {
		return
	}
	actor, err := studentDocumentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}
	coordinator, err := rs.studentDocumentCoordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	uploaded, err := common.ParseDocumentWithLimits(w, r, "file", maxStudentDocumentFile, maxStudentDocumentBody)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	storedName, err := apiDocuments.NewStoredName(common.DocumentFileExtension(uploaded.ContentType))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if err := rs.StudentDocumentService.QueueStudentDocumentFileCleanup(r.Context(), id, storedName); err != nil {
		rs.getLogger().Error("student document upload cleanup intent failed",
			"student_id", id, "error", err)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// The queued intent becomes eligible for the cleanup scheduler after a
	// fixed delay, so every step that can still persist metadata for this
	// object must be bounded well inside that delay: a stalled write or
	// metadata transaction would otherwise commit a document row after the
	// scheduler had already removed its bytes. The bookkeeping calls below
	// deliberately stay on the request context so they still run once the
	// upload deadline has fired.
	uploadCtx, cancelUpload := context.WithTimeout(r.Context(), usersSvc.StudentDocumentUploadDeadline)
	defer cancelUpload()

	size, err := coordinator.Save(uploadCtx, tenantID, storedName, uploaded.File)
	if err != nil {
		// The intent is activated, not settled. A failed write removes its own
		// partial object only best-effort, so declaring the bytes gone here
		// would strand a half-written child document that nothing can reach
		// again: no metadata row, no eligible intent, no scheduler path.
		// Activating costs one no-op removal when the object really is gone.
		if activateErr := rs.StudentDocumentService.ActivateQueuedCleanup(r.Context(), storedName); activateErr != nil {
			rs.getLogger().Error("student document cleanup intent activation failed after write error",
				"student_id", id, "error", activateErr)
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	doc, err := rs.StudentDocumentService.CreateStudentDocument(uploadCtx, usersSvc.CreateStudentDocumentInput{
		StudentID:       id,
		Category:        r.FormValue("category"),
		FilenameDisplay: uploaded.Filename,
		FilenameStored:  storedName,
		SizeBytes:       size,
		ContentType:     uploaded.ContentType,
	}, actor)
	if err != nil {
		// Only the two request-shaped rejections are decided before the
		// metadata transaction opens, so only they prove the row never landed
		// and let the bytes go immediately. Anything deeper — including a
		// commit whose acknowledgement never arrived — is in doubt and is left
		// to the queued intent.
		rejectedBeforeCommit := errors.Is(err, usersSvc.ErrStudentDocumentInvalid) ||
			errors.Is(err, usersSvc.ErrStudentDocumentForbidden)
		coordinator.ReleaseFailedUpload(r.Context(), tenantID, id, storedName, rejectedBeforeCommit, err)
		renderStudentDocumentError(w, r, err)
		return
	}

	resp := newStudentDocumentResponse(doc)
	common.Respond(w, r, http.StatusCreated, &resp, "Student document uploaded successfully")
}

// downloadStudentDocument serves GET /{id}/documents/{documentId}/download.
// The service refuses sensitive categories without an access-log row.
func (rs *Resource) downloadStudentDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStudentID)
	if !ok {
		return
	}
	docID, ok := common.ParseInt64IDWithError(w, r, "documentId", "invalid document ID")
	if !ok {
		return
	}
	actor, err := studentDocumentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	coordinator, err := rs.studentDocumentCoordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	doc, err := rs.StudentDocumentService.ResolveStudentDocumentDownload(r.Context(), id, docID, actor)
	if err != nil {
		renderStudentDocumentError(w, r, err)
		return
	}

	if !coordinator.Serve(w, r, tenant.FromContext(r.Context()), doc.FilenameStored, doc.FilenameDisplay, doc.ContentType) {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("document file not found")))
	}
}

// deleteStudentDocument serves DELETE /{id}/documents/{documentId}: audited
// soft delete of the metadata row, then removal of the bytes. The row
// survives as the trail; the content does not.
func (rs *Resource) deleteStudentDocument(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStudentID)
	if !ok {
		return
	}
	docID, ok := common.ParseInt64IDWithError(w, r, "documentId", "invalid document ID")
	if !ok {
		return
	}
	actor, err := studentDocumentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	coordinator, err := rs.studentDocumentCoordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	doc, err := rs.StudentDocumentService.DeleteStudentDocument(r.Context(), id, docID, actor)
	if err != nil {
		if !modelBase.IsNoRows(err) {
			renderStudentDocumentError(w, r, err)
			return
		}
		// Already soft-deleted: the row is gone from the normal view but its
		// bytes may still be there, so re-resolve and retry the removal
		// instead of reporting a 404 the caller cannot act on.
		doc, err = rs.StudentDocumentService.ResolveStudentDocumentCleanup(r.Context(), id, docID, actor)
		if err != nil {
			renderStudentDocumentError(w, r, err)
			return
		}
	}

	if !doc.IsFileDeleted() {
		tenantID := tenant.FromContext(r.Context())
		documentID, storedName := doc.ID, doc.FilenameStored
		tenant.RegisterAfterCommit(r.Context(), func() {
			coordinator.CleanupDocument(tenantID, id, documentID, storedName, "delete")
		})
	}
	common.Respond(w, r, http.StatusOK, map[string]int64{"student_id": id}, "Student document deleted successfully")
}
