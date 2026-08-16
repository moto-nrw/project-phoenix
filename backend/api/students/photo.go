package students

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// publicPhotoBaseDir + StudentPhotoStoredURLPrefix must stay aligned —
// ResolveStoredPath and BuildStudentPhotoServeURL both key off them.
const (
	maxStudentPhotoFile = 5 * 1024 * 1024 // 5 MiB advertised file limit
	maxStudentPhotoBody = maxStudentPhotoFile + 1024
	publicPhotoBaseDir  = "public/uploads/student-photos"
)

const (
	msgPhotosFeatureDisabled = "Kinderfotos sind in dieser Schule nicht aktiviert"                  //nolint:staticcheck // ST1005: user-facing German message
	msgConsentWithdrawnRetry = "Einwilligung der Eltern wurde widerrufen — bitte erneut bestätigen" //nolint:staticcheck // ST1005: user-facing German message
	msgConsentRequiredFirst  = "Einwilligung der Eltern muss vor dem Upload bestätigt werden"       //nolint:staticcheck // ST1005: user-facing German message
	msgPhotoForbiddenForAcct = "dieses Foto ist für diesen Account nicht freigegeben"
	msgPhotoBelongsToOther   = "dieses Foto gehört nicht zu diesem Kind"
	msgPhotoNotFound         = "kein Foto hinterlegt"
)

// uploadStudentPhoto handles POST /api/students/{id}/photo. Mounted without
// TenantTxMiddleware so a slow 5 MiB upload doesn't pin a bun pool connection;
// the service opens its own short tx.
func (rs *Resource) uploadStudentPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}

	uploaded, err := common.ParseImageWithLimits(w, r, "photo", maxStudentPhotoFile, maxStudentPhotoBody)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	consentAck := r.FormValue("consent_acknowledged") == "true"

	// {tenantID}_{studentID} prefix prevents cross-tenant collisions and
	// makes orphaned files traceable.
	prefix := fmt.Sprintf("%d_%d", tenantID, id)
	filePath, err := common.SaveImage(uploaded.File, publicPhotoBaseDir, prefix, uploaded.ContentType)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	newStoredURL := common.StudentPhotoStoredURLPrefix + filepath.Base(filePath)

	if err := rs.StudentPhotos.CommitUpload(r.Context(), userService.CommitUploadRequest{
		StudentID:    id,
		NewStoredURL: newStoredURL,
		ConsentAck:   consentAck,
	}); err != nil {
		// Keep disk and DB consistent on rejection.
		common.RemoveImage(filePath)
		mapPhotoUploadError(w, r, err)
		return
	}

	servedURL := common.BuildStudentPhotoServeURL(id, newStoredURL)
	common.Respond(w, r, http.StatusOK, map[string]string{"photo_url": servedURL}, "Foto hochgeladen")
}

// deleteStudentPhoto is idempotent — returns 200 even when no photo was set.
func (rs *Resource) deleteStudentPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}
	clearedURL, err := rs.StudentPhotos.CommitDelete(r.Context(), id)
	if err != nil {
		mapPhotoDeleteError(w, r, err)
		return
	}
	if clearedURL == "" {
		common.Respond(w, r, http.StatusOK, nil, "Kein Foto vorhanden")
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Foto entfernt")
}

// serveStudentPhoto streams bytes after the auth tx commits so the pool
// connection releases before the file stream. private, no-cache so feature
// disable / consent withdrawal / deletion take effect on next load.
func (rs *Resource) serveStudentPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}
	filename := chi.URLParam(r, "filename")

	storedURL, err := rs.StudentPhotos.LookupForRead(r.Context(), id, filename)
	if err != nil {
		mapPhotoReadError(w, r, err)
		return
	}

	resolvedPath, resolveErr := common.ResolveStoredPath("public", storedURL, common.StudentPhotoStoredURLPrefix)
	if resolveErr != nil {
		// 403 not 500 — ResolveStoredPath rejected a path-traversal attempt.
		renderError(w, r, common.ErrorForbidden(errors.New("could not resolve stored photo path")))
		return
	}
	common.ServeImage(w, r, filepath.Dir(resolvedPath), filepath.Base(resolvedPath), "private, no-cache")
}

// mapPhotoUploadError maps service sentinels to HTTP status.
func mapPhotoUploadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, userService.ErrPhotoFeatureDisabled),
		errors.Is(err, userService.ErrPhotoFeatureDisabledMid):
		render.Status(r, http.StatusForbidden)
		renderError(w, r, common.ErrorForbidden(errors.New(msgPhotosFeatureDisabled))) //nolint:staticcheck // ST1005: user-facing German message
	case errors.Is(err, userService.ErrPhotoStudentNotFound):
		renderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, userService.ErrPhotoStudentForbidden),
		errors.Is(err, userService.ErrPhotoStudentReassigned):
		renderError(w, r, common.ErrorForbidden(errors.New("insufficient permissions to update this student's photo")))
	case errors.Is(err, userService.ErrPhotoConsentRequired):
		renderError(w, r, common.ErrorInvalidRequest(errors.New(msgConsentRequiredFirst))) //nolint:staticcheck // ST1005: user-facing German message
	case errors.Is(err, userService.ErrPhotoConsentWithdrawn):
		// 409 — consent flipped between request and commit; frontend re-prompts.
		renderError(w, r, common.ErrorConflictMessage(msgConsentWithdrawnRetry))
	case errors.Is(err, userService.ErrPhotoNoTenant):
		renderError(w, r, common.ErrorInvalidRequest(err))
	default:
		renderError(w, r, common.ErrorInternalServer(err))
	}
}

func mapPhotoDeleteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, userService.ErrPhotoFeatureDisabled):
		render.Status(r, http.StatusForbidden)
		renderError(w, r, common.ErrorForbidden(errors.New(msgPhotosFeatureDisabled))) //nolint:staticcheck // ST1005: user-facing German message
	case errors.Is(err, userService.ErrPhotoStudentNotFound):
		renderError(w, r, common.ErrorNotFound(errors.New("student not found")))
	case errors.Is(err, userService.ErrPhotoStudentForbidden),
		errors.Is(err, userService.ErrPhotoStudentReassigned):
		renderError(w, r, common.ErrorForbidden(errors.New("insufficient permissions to delete this student's photo")))
	case errors.Is(err, userService.ErrPhotoNoTenant):
		renderError(w, r, common.ErrorInvalidRequest(err))
	default:
		renderError(w, r, common.ErrorInternalServer(err))
	}
}

func mapPhotoReadError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, userService.ErrPhotoFeatureDisabled):
		render.Status(r, http.StatusForbidden)
		renderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, userService.ErrPhotoStudentNotFound):
		renderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, userService.ErrPhotoStudentForbidden):
		renderError(w, r, common.ErrorForbidden(errors.New(msgPhotoForbiddenForAcct)))
	case errors.Is(err, userService.ErrPhotoNotSet):
		renderError(w, r, common.ErrorNotFound(errors.New(msgPhotoNotFound)))
	case errors.Is(err, userService.ErrPhotoFilenameMismatch):
		renderError(w, r, common.ErrorForbidden(errors.New(msgPhotoBelongsToOther)))
	case errors.Is(err, userService.ErrPhotoNoTenant):
		renderError(w, r, common.ErrorInvalidRequest(err))
	default:
		renderError(w, r, common.ErrorInternalServer(err))
	}
}
