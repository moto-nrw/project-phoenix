package enrollment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// PublicLegalTextsResponse carries the tenant-scoped legal documents
// (Markdown) and the derived legal blocks the public enrollment form renders.
type PublicLegalTextsResponse struct {
	AGB          string `json:"agb"`
	DSGVO        string `json:"dsgvo"`
	EmailContact string `json:"email_contact"`
	Photo        string `json:"photo"`
	// TermsEnabled reflects enrollment.legal_terms_enabled. The AGB block is
	// rendered only when this is true and the AGB text is non-empty.
	TermsEnabled bool                           `json:"terms_enabled"`
	Blocks       []enrollmentService.LegalBlock `json:"blocks"`
}

// publicLegalTexts serves the tenant's AGB + Datenschutz Markdown for
// the parent form. Slug-gated, no JWT — same access shape as the
// captcha-config endpoint. When phaseId is present, template-owned
// legal_blocks on that phase's selected form schema override the tenant
// standard blocks.
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
	var phaseID int64
	if phaseParam := chi.URLParam(r, "phaseId"); phaseParam != "" {
		parsed, err := strconv.ParseInt(phaseParam, 10, 64)
		if err != nil || parsed <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("phaseId is required")))
			return
		}
		phaseID = parsed
	}

	out := PublicLegalTextsResponse{}
	// legalErr captures a genuine settings/DB/JSON resolve failure so we
	// can return a 500 instead of the 404 path. These texts sit behind
	// legally relevant blocks — on a real failure the endpoint must fail
	// rather than let the form collect an incomplete legal state.
	var legalErr error
	resolveErr := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, schoolErr := rs.SchoolRepo.FindBySlug(adminCtx, slug)
		if schoolErr != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}
		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)
		return tenant.WithTenantTx(tenantCtx, rs.db, school.ID, func(txCtx context.Context, _ bun.Tx) error {
			var (
				texts enrollmentService.LegalTexts
				err   error
			)
			if phaseID > 0 {
				texts, err = rs.RequestService.LegalTextsForPhase(txCtx, phaseID)
			} else {
				texts, err = rs.RequestService.LegalTexts(txCtx)
			}
			if err != nil {
				legalErr = err
				return err
			}
			out.AGB = texts.AGB
			out.DSGVO = texts.DSGVO
			out.EmailContact = texts.EmailContact
			out.Photo = texts.Photo
			out.TermsEnabled = texts.TermsEnabled
			out.Blocks = texts.Blocks
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
