package enrollmenthttp

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (rs *HolidayPeriodResource) uploadFormDocument(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	period, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	uploaded, err := common.ParsePDFWithLimits(w, r, "document", maxEnrollmentLegalDocumentSize, maxEnrollmentLegalDocumentBody)
	if r.MultipartForm != nil {
		defer func() { _ = r.MultipartForm.RemoveAll() }()
	}
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithDetails(err, common.CodeEnrollmentLegalDocumentInvalid, map[string]any{"max_mb": 10}))
		return
	}
	defer common.CloseFile(uploaded.File)
	content, err := io.ReadAll(io.LimitReader(uploaded.File, maxEnrollmentLegalDocumentSize+1))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	doc, err := rs.periods.UploadHolidayFormDocument(r.Context(), reviewer.OrganizationID, period, reviewer.AccountID, uploaded.Filename, content)
	if err != nil {
		renderHolidayDocumentError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, doc, "Holiday form document uploaded")
}

func (rs *HolidayPeriodResource) getFormDocument(w http.ResponseWriter, r *http.Request) {
	org, ok := traegerOf(w, r)
	if !ok {
		return
	}
	period, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	doc, err := rs.periods.HolidayFormDocument(r.Context(), org, period, chi.URLParam(r, "documentId"))
	if err != nil {
		renderHolidayDocumentError(w, r, err)
		return
	}
	writeHolidayPDF(w, doc)
}

func (rs *HolidayPeriodResource) deleteFormDocument(w http.ResponseWriter, r *http.Request) {
	org, ok := traegerOf(w, r)
	if !ok {
		return
	}
	period, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.periods.DeleteHolidayFormDocument(r.Context(), org, period, chi.URLParam(r, "documentId")); err != nil {
		renderHolidayDocumentError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rs *HolidayRegistrationResource) document(w http.ResponseWriter, r *http.Request) {
	link, err := rs.resolveLink(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	var doc capability.HolidayFormDocument
	err = withinTraeger(r.Context(), link.OrganizationID, func(ctx context.Context) error {
		var loadErr error
		doc, loadErr = rs.registrations.PublicHolidayFormDocument(ctx, link, chi.URLParam(r, "documentId"))
		return loadErr
	})
	if err != nil {
		renderHolidayDocumentError(w, r, err)
		return
	}
	writeHolidayPDF(w, doc)
}

func writeHolidayPDF(w http.ResponseWriter, doc capability.HolidayFormDocument) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": doc.Filename}))
	w.Header().Set("Content-Length", strconv.Itoa(len(doc.Content)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(doc.Content)
}

func renderHolidayDocumentError(w http.ResponseWriter, r *http.Request, err error) {
	tenant.MarkRollback(r.Context())
	switch {
	case errors.Is(err, capability.ErrHolidayFormDocumentNotFound), errors.Is(err, capability.ErrHolidayPeriodNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayPeriodNotFound))
	case errors.Is(err, capability.ErrHolidayFormDocumentReferenced):
		common.RenderError(w, r, common.ErrorConflict(err))
	default:
		renderHolidayFormError(w, r, err)
	}
}
