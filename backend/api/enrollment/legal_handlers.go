package enrollment

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// PublicLegalTextsResponse carries the tenant-scoped legal documents
// (Markdown) the public enrollment form shows behind the AGB and
// Datenschutz consent checkboxes. Empty strings mean the admin hasn't
// configured a document → the form renders a plain consent label
// without a clickable "view document" link.
type PublicLegalTextsResponse struct {
	AGB          string `json:"agb"`
	DSGVO        string `json:"dsgvo"`
	EmailContact string `json:"email_contact"`
	Photo        string `json:"photo"`
}

// publicLegalTexts serves the tenant's AGB + Datenschutz Markdown for
// the parent form. Slug-gated, no JWT — same access shape as the
// captcha-config endpoint. The texts are tenant-wide (not phase-
// specific), so no phaseId param.
func (rs *Resource) publicLegalTexts(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil || rs.SchoolRepo == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("legal texts endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	out := PublicLegalTextsResponse{}
	// legalErr captures a genuine settings/DB/JSON resolve failure so we
	// can return a 500 instead of the 404 path. These texts sit behind
	// legally relevant consents — on a real failure the endpoint must
	// fail rather than let the form fall back to plain consent labels.
	var legalErr error
	resolveErr := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, schoolErr := rs.SchoolRepo.FindBySlug(adminCtx, slug)
		if schoolErr != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}
		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)
		return tenant.WithTenantTx(tenantCtx, rs.db, school.ID, func(txCtx context.Context, _ bun.Tx) error {
			texts, err := rs.RequestService.LegalTexts(txCtx)
			if err != nil {
				legalErr = err
				return err
			}
			out.AGB = texts.AGB
			out.DSGVO = texts.DSGVO
			out.EmailContact = texts.EmailContact
			out.Photo = texts.Photo
			return nil
		})
	})
	if resolveErr != nil {
		if legalErr != nil {
			common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("resolve legal texts: %w", legalErr)))
			return
		}
		renderPublicEnrollmentError(w, r, resolveErr)
		return
	}

	common.Respond(w, r, http.StatusOK, out, "Legal texts retrieved")
}
