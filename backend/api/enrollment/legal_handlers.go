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
	AGB            string `json:"agb"`
	AGBDocumentURL string `json:"agb_document_url"`
	AGBDisplayMode string `json:"agb_display_mode"`
	DSGVO          string `json:"dsgvo"`
	EmailContact   string `json:"email_contact"`
	Photo          string `json:"photo"`
	// Standard blocks render only when their toggle is enabled and their
	// matching text is non-empty.
	TermsEnabled        bool                           `json:"terms_enabled"`
	DSGVOEnabled        bool                           `json:"dsgvo_enabled"`
	EmailContactEnabled bool                           `json:"email_contact_enabled"`
	PhotoEnabled        bool                           `json:"photo_enabled"`
	Blocks              []enrollmentService.LegalBlock `json:"blocks"`
}

// publicLegalTexts serves the tenant's AGB + Datenschutz Markdown for
// the parent form. Slug-gated, no JWT — same access shape as the
// captcha-config endpoint. When phaseId is present, template-owned
// legal_blocks on that phase's selected form schema override the tenant
// standard blocks.
func (rs *Resource) publicLegalTexts(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil || rs.SchoolService == nil || rs.db == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("legal texts endpoint not wired")))
		return
	}

	slug := chi.URLParam(r, "tenantSlug")
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}
	phaseID, ok := parseOptionalPhaseIDParam(w, r)
	if !ok {
		return
	}

	lateInviteToken := lateInviteTokenFromRequest(r)
	// legalErr captures a genuine settings/DB/JSON resolve failure so we
	// can return a 500 instead of the 404 path. These texts sit behind
	// legally relevant blocks — on a real failure the endpoint must fail
	// rather than let the form collect an incomplete legal state. The
	// public phase-gate sentinels (unknown phase, disabled tenant/phase,
	// closed window) are NOT failures: they fall through to the same
	// controlled 404/disabled mapping every other public enrollment
	// endpoint uses.
	out := PublicLegalTextsResponse{}
	var legalErr error
	schoolID, resolveErr := rs.resolvePublicTenantID(r.Context(), slug)
	if resolveErr == nil {
		out, resolveErr, legalErr = rs.resolveTenantLegalTexts(r.Context(), schoolID, phaseID, lateInviteToken)
	}
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

// parseOptionalPhaseIDParam reads the optional phaseId path param. A missing
// param returns (0, true); a malformed one renders a 400 and returns ok=false.
func parseOptionalPhaseIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	phaseParam := chi.URLParam(r, "phaseId")
	if phaseParam == "" {
		return 0, true
	}
	parsed, err := strconv.ParseInt(phaseParam, 10, 64)
	if err != nil || parsed <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("phaseId is required")))
		return 0, false
	}
	return parsed, true
}

// resolveTenantLegalTexts resolves the legal texts inside a tenant tx. It
// returns the response, the tx/resolve error (for the controlled 404 path),
// and a separate hard error when the failure is a genuine settings/DB/JSON
// fault rather than a public phase-gate sentinel (for the 500 path).
func (rs *Resource) resolveTenantLegalTexts(ctx context.Context, schoolID, phaseID int64, lateInviteToken string) (PublicLegalTextsResponse, error, error) {
	var out PublicLegalTextsResponse
	var legalErr error
	txErr := tenant.WithTenantTx(ctx, rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
		texts, err := rs.resolvePublicLegalTexts(txCtx, phaseID, lateInviteToken)
		if err != nil {
			if !isPublicLegalGateError(err) {
				legalErr = err
			}
			return err
		}
		out = legalTextsToResponse(texts)
		return nil
	})
	return out, txErr, legalErr
}

// resolvePublicLegalTexts loads the phase-scoped legal texts when a phase is
// selected, or the tenant-wide legal settings otherwise.
func (rs *Resource) resolvePublicLegalTexts(ctx context.Context, phaseID int64, lateInviteToken string) (enrollmentService.LegalTexts, error) {
	if phaseID > 0 {
		return rs.RequestService.LegalTextsForPhaseWithLateInvite(ctx, phaseID, lateInviteToken)
	}
	return rs.RequestService.LegalTexts(ctx)
}

// isPublicLegalGateError reports whether err is a public phase-gate sentinel
// (unknown phase, disabled tenant/phase, closed window, invalid late invite)
// rather than a genuine settings/DB/JSON failure. Gate sentinels take the
// controlled 404/disabled mapping; everything else is a 500.
func isPublicLegalGateError(err error) bool {
	return errors.Is(err, enrollmentService.ErrInvalidSubmission) ||
		errors.Is(err, enrollmentService.ErrEnrollmentDisabled) ||
		errors.Is(err, enrollmentService.ErrEnrollmentWindowClosed) ||
		errors.Is(err, enrollmentService.ErrLateInviteInvalid)
}

func legalTextsToResponse(texts enrollmentService.LegalTexts) PublicLegalTextsResponse {
	return PublicLegalTextsResponse{
		AGB:                 texts.AGB,
		AGBDocumentURL:      texts.AGBDocumentURL,
		AGBDisplayMode:      texts.AGBDisplayMode,
		DSGVO:               texts.DSGVO,
		EmailContact:        texts.EmailContact,
		Photo:               texts.Photo,
		TermsEnabled:        texts.TermsEnabled,
		DSGVOEnabled:        texts.DSGVOEnabled,
		EmailContactEnabled: texts.EmailContactEnabled,
		PhotoEnabled:        texts.PhotoEnabled,
		Blocks:              texts.Blocks,
	}
}
