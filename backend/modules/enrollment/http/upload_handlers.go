package enrollmenthttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// The files parents attach to an enrollment (#4783): the public upload of the
// form and the staff view of one file.

const (
	maxEnrollmentUploadSize = int64(capability.MaxFileFieldSizeMB) * 1024 * 1024
	maxEnrollmentUploadBody = maxEnrollmentUploadSize + 64*1024
)

// UploadService is the intake's upload and the staff open of an upload; the
// root opens the tenant unit of each call.
type UploadService interface {
	PrepareUpload(ctx context.Context, input capability.UploadInput) (*capability.PreparedUpload, error)
	DiscardUpload(ctx context.Context, tenantID, uploadID int64) error
	OpenRequestUpload(ctx context.Context, access capability.UploadAccess) (*capability.RequestUpload, error)
}

// UploadListing lists the uploads of a request in the caller's unit.
type UploadListing interface {
	ListRequestUploads(ctx context.Context, requestID int64) ([]capability.RequestUpload, error)
}

// UploadFileStore writes and serves the private objects of uploads.
type UploadFileStore interface {
	Save(ctx context.Context, tenantID int64, storedName string, source io.Reader) (int64, error)
	ServeInline(w http.ResponseWriter, r *http.Request, tenantID int64, storedName, displayName, contentType string) bool
}

// UploadFileStorage opens the object store of uploads for one request. The
// root binds File Storage's document coordinator and looks the backend up on
// each call, so building the routes opens no storage.
type UploadFileStorage func(kind string) (UploadFileStore, error)

// UploadReceipt is the public answer to an upload: the token the form sends
// with the submission and the file as the parent sees it.
type UploadReceipt struct {
	Token       string `json:"token"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

// AdminRequestUpload is one live upload of a request in the staff detail.
type AdminRequestUpload struct {
	ID          string    `json:"id"`
	FieldKey    string    `json:"field_key"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// uploadEnrollmentFile stores one file for a file field of the public form.
// The row commits before the bytes are written, so a crash in between leaves
// an unclaimed row that expires, never bytes nobody points at.
func (rs *Resource) uploadEnrollmentFile(w http.ResponseWriter, r *http.Request) {
	if rs.Uploads == nil || rs.UploadFiles == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment uploads not configured")))
		return
	}
	file, input, err := parseEnrollmentUpload(w, r)
	if err != nil {
		renderUploadRefusal(w, r, err)
		return
	}
	defer common.CloseFile(file.File)
	schoolID, err := rs.resolvePublicTenantID(r.Context(), strings.TrimSpace(chi.URLParam(r, "tenantSlug")))
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentFormNotFound))
		return
	}
	input.TenantID = schoolID
	files, err := rs.UploadFiles(capability.RequestUploadStorageKind)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("enrollment upload storage unavailable: %w", err)))
		return
	}
	receipt, err := rs.storeEnrollmentUpload(r.Context(), files, input, file.File)
	if err != nil {
		renderUploadFailure(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, receipt, "Upload stored")
}

// storeEnrollmentUpload lets the intake commit the row, writes the bytes and
// discards the row when the write fails.
func (rs *Resource) storeEnrollmentUpload(ctx context.Context, files UploadFileStore, input capability.UploadInput, content io.Reader) (*UploadReceipt, error) {
	return storeEnrollmentUpload(ctx, rs.Uploads, files, input, content)
}

func storeEnrollmentUpload(ctx context.Context, uploads UploadService, files UploadFileStore, input capability.UploadInput, content io.Reader) (*UploadReceipt, error) {
	prepared, err := uploads.PrepareUpload(ctx, input)
	if err != nil {
		return nil, err
	}
	upload := prepared.Upload
	if _, err := files.Save(ctx, input.TenantID, upload.StoredName, content); err != nil {
		if discardErr := uploads.DiscardUpload(ctx, input.TenantID, upload.ID); discardErr != nil {
			err = errors.Join(err, discardErr)
		}
		return nil, fmt.Errorf("write enrollment upload: %w", err)
	}
	return &UploadReceipt{Token: prepared.Token, Filename: upload.Filename, SizeBytes: upload.SizeBytes, ContentType: upload.ContentType}, nil
}

// parseEnrollmentUpload reads the multipart upload: the content type comes
// from the bytes (PDF, PNG, JPEG), the extension must agree with it, and the
// size is measured, not taken from the header.
func parseEnrollmentUpload(w http.ResponseWriter, r *http.Request) (*common.UploadedFile, capability.UploadInput, error) {
	uploaded, err := common.ParseDocumentWithLimits(w, r, "file", maxEnrollmentUploadSize, maxEnrollmentUploadBody)
	if err != nil {
		return nil, capability.UploadInput{}, err
	}
	extension := common.DocumentFileExtension(uploaded.ContentType)
	size, sizeErr := uploaded.File.Seek(0, io.SeekEnd)
	if _, rewindErr := uploaded.File.Seek(0, io.SeekStart); sizeErr != nil || rewindErr != nil {
		common.CloseFile(uploaded.File)
		return nil, capability.UploadInput{}, errors.New("cannot measure upload")
	}
	phaseID, phaseErr := strconv.ParseInt(strings.TrimSpace(r.FormValue("phase_id")), 10, 64)
	if !extensionMatchesContent(uploaded.Filename, uploaded.ContentType) {
		common.CloseFile(uploaded.File)
		return nil, capability.UploadInput{}, &uploadContentMismatch{filename: uploaded.Filename}
	}
	if phaseErr != nil || phaseID <= 0 {
		common.CloseFile(uploaded.File)
		return nil, capability.UploadInput{}, &capability.UploadRejection{MaxMB: capability.MaxFileFieldSizeMB}
	}
	return uploaded, capability.UploadInput{
		PhaseID: phaseID, FieldKey: r.FormValue("field_key"), Filename: uploaded.Filename,
		ContentType: normalizeUploadContentType(uploaded.ContentType), Extension: extension,
		SizeBytes: size, RemoteIP: remoteIPFromRequest(r),
		LateInviteToken: r.FormValue("late_invite_token"), StatusToken: r.FormValue("status_token"),
	}, nil
}

// extensionMatchesContent reports whether the file name's extension names
// the detected content. A PDF renamed to .png, or a file without a known
// extension, is refused even though the bytes alone would pass.
func extensionMatchesContent(filename, contentType string) bool {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".pdf":
		return contentType == "application/pdf"
	case ".png":
		return contentType == "image/png"
	case ".jpg", ".jpeg":
		return contentType == "image/jpeg" || contentType == "image/jpg"
	default:
		return false
	}
}

func normalizeUploadContentType(contentType string) string {
	if contentType == "image/jpg" {
		return "image/jpeg"
	}
	return contentType
}

type uploadContentMismatch struct{ filename string }

func (e *uploadContentMismatch) Error() string { return "file content does not match its extension" }

// renderUploadRefusal answers a file the transport refused with the field's
// upload code and the general size limit.
func renderUploadRefusal(w http.ResponseWriter, r *http.Request, err error) {
	var mismatch *uploadContentMismatch
	if common.IsUploadTypeNotAllowed(err) && r.MultipartForm != nil {
		for _, header := range r.MultipartForm.File["file"] {
			switch strings.ToLower(filepath.Ext(header.Filename)) {
			case ".pdf", ".png", ".jpg", ".jpeg":
				mismatch = &uploadContentMismatch{filename: header.Filename}
			}
			break
		}
	} else {
		errors.As(err, &mismatch)
	}
	if mismatch != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithDetails(err, common.CodeEnrollmentUploadContentMismatch,
			map[string]any{"filename": mismatch.filename}))
		return
	}
	common.RenderError(w, r, common.ErrorInvalidRequestWithDetails(err, common.CodeEnrollmentUploadInvalid,
		map[string]any{"max_mb": capability.MaxFileFieldSizeMB}))
}

// renderUploadFailure maps the owner's refusals of an upload.
func renderUploadFailure(w http.ResponseWriter, r *http.Request, err error) {
	var rejection *capability.UploadRejection
	switch {
	case errors.As(err, &rejection):
		common.RenderError(w, r, common.ErrorInvalidRequestWithDetails(err, common.CodeEnrollmentUploadInvalid,
			map[string]any{"max_mb": rejection.MaxMB, "allowed_types": rejection.AllowedTypes}))
	case errors.Is(err, capability.ErrUploadFieldNotFound),
		errors.Is(err, capability.ErrInvalidSubmission),
		errors.Is(err, capability.ErrEnrollmentDisabled),
		errors.Is(err, capability.ErrEnrollmentWindowClosed),
		errors.Is(err, capability.ErrLateInviteInvalid),
		errors.Is(err, capability.ErrPhaseAudienceRestricted):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentFormNotFound))
	case errors.Is(err, capability.ErrUploadRateLimited):
		w.Header().Set("Retry-After", "3600")
		common.RenderError(w, r, common.ErrorTooManyRequestsWithCode(err, common.CodeEnrollmentUploadRateLimited))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// openAdminRequestUpload serves one upload of a request inline. The access
// is recorded before a byte leaves; without the record nothing is served.
func (rs *Resource) openAdminRequestUpload(w http.ResponseWriter, r *http.Request) {
	if rs.Uploads == nil || rs.UploadFiles == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment uploads not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	uploadID, ok := common.ParsePositiveInt64IDWithError(w, r, "uploadId", "invalid upload id")
	if !ok {
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	access := capability.UploadAccess{
		RequestID: requestID, UploadID: uploadID,
		ActorAccountID: int64(claims.ID), ActorRole: strings.Join(claims.Roles, ","),
	}
	upload, err := rs.Uploads.OpenRequestUpload(r.Context(), access)
	if err != nil {
		if errors.Is(err, capability.ErrUploadNotFound) {
			common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentUploadNotFound))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	files, err := rs.UploadFiles(capability.RequestUploadStorageKind)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("enrollment upload storage unavailable: %w", err)))
		return
	}
	if !files.ServeInline(w, r, upload.TenantID, upload.StoredName, upload.Filename, upload.ContentType) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(errors.New("upload object missing"), common.CodeEnrollmentUploadNotFound))
	}
}

// attachAdminUploads adds the live uploads of the request to the detail and
// whether the caller may open them.
func (rs *Resource) attachAdminUploads(ctx context.Context, detail *AdminRequestDetail, requestID int64) error {
	detail.Uploads = []AdminRequestUpload{}
	if rs.UploadList == nil {
		return nil
	}
	principal, err := common.CurrentPrincipal(ctx)
	detail.CanViewUploads = err == nil && principal.HasPermission(permissions.EnrollmentUploadsRead)
	uploads, err := rs.UploadList.ListRequestUploads(ctx, requestID)
	if err != nil {
		return err
	}
	for _, upload := range uploads {
		detail.Uploads = append(detail.Uploads, AdminRequestUpload{
			ID: strconv.FormatInt(upload.ID, 10), FieldKey: upload.FieldKey, Filename: upload.Filename,
			SizeBytes: upload.SizeBytes, ContentType: upload.ContentType, UploadedAt: upload.CreatedAt,
		})
	}
	return nil
}
