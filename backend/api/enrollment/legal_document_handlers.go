package enrollment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const (
	maxEnrollmentLegalDocumentSize = 10 * 1024 * 1024
	maxEnrollmentLegalDocumentBody = maxEnrollmentLegalDocumentSize + 4096
	enrollmentLegalDocumentDir     = "public/uploads/enrollment-form-legal-documents"
	enrollmentLegalDocumentPrefix  = "/uploads/enrollment-form-legal-documents/"
)

type enrollmentLegalDocumentResponse struct {
	DocumentURL string `json:"document_url"`
}

type legalDocumentReferenceRepository interface {
	HasLegalDocumentReference(ctx context.Context, storedURL, publicURL string) (bool, error)
}

func (rs *Resource) uploadLegalDocument(w http.ResponseWriter, r *http.Request) {
	uploaded, err := common.ParsePDFWithLimits(w, r, "document", maxEnrollmentLegalDocumentSize, maxEnrollmentLegalDocumentBody)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}

	filePath, err := common.SavePDF(uploaded.File, enrollmentLegalDocumentDir, fmt.Sprintf("%d", tenantID))
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	documentURL := enrollmentLegalDocumentPrefix + filepath.Base(filePath)
	common.Respond(w, r, http.StatusOK, enrollmentLegalDocumentResponse{DocumentURL: documentURL}, "Legal document uploaded successfully")
}

func (rs *Resource) deleteLegalDocument(w http.ResponseWriter, r *http.Request) {
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}

	filename := chi.URLParam(r, "filename")
	if err := common.ValidateFilename(filename); err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	storedURL := enrollmentLegalDocumentPrefix + filename
	if !enrollmentLegalDocumentBelongsToTenant(storedURL, tenantID) {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("document does not belong to tenant")))
		return
	}

	if rs.legalDocumentRefs == nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("legal document reference repository is not configured")))
		return
	}

	publicURL := publicEnrollmentLegalDocumentURL(storedURL)
	var referenced bool
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		var refErr error
		referenced, refErr = rs.legalDocumentRefs.HasLegalDocumentReference(ctx, storedURL, publicURL)
		return refErr
	})
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("check legal document references: %w", err)))
		return
	}
	if referenced {
		common.RespondNoContent(w, r)
		return
	}

	path, err := common.ResolveStoredPath("public", storedURL, enrollmentLegalDocumentPrefix)
	if err != nil {
		render.Status(r, http.StatusBadRequest)
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	common.RemoveImage(path)
	common.RespondNoContent(w, r)
}

func enrollmentLegalDocumentBelongsToTenant(storedURL string, tenantID int64) bool {
	if tenantID <= 0 || !strings.HasPrefix(storedURL, enrollmentLegalDocumentPrefix) {
		return false
	}
	filename := strings.TrimPrefix(storedURL, enrollmentLegalDocumentPrefix)
	return strings.HasPrefix(filename, fmt.Sprintf("%d_", tenantID))
}

func publicEnrollmentLegalDocumentURL(storedURL string) string {
	if strings.HasPrefix(storedURL, enrollmentLegalDocumentPrefix) {
		return "/api/public/enrollment-form-legal-documents/" + strings.TrimPrefix(storedURL, enrollmentLegalDocumentPrefix)
	}
	return storedURL
}

// UploadLegalDocument returns the form-template legal document upload handler for tests.
func (rs *Resource) UploadLegalDocument() http.HandlerFunc {
	return rs.uploadLegalDocument
}

// DeleteLegalDocument returns the form-template legal document delete handler for tests.
func (rs *Resource) DeleteLegalDocument() http.HandlerFunc {
	return rs.deleteLegalDocument
}
