package parent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/api/enrollment"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// getEnrollmentProfile returns guardian + linked-children data for the
// calling parent within the requested tenant. Tenant comes from the
// path slug (admin-tx resolves it); account from claims.ID. The
// repository call runs under WithTenantTx via the loader so RLS
// narrows reads to that tenant — a parent who has no profile in this
// school just gets claims-derived guardian fields and an empty
// children list.
func (rs *Resource) getEnrollmentProfile(w http.ResponseWriter, r *http.Request) {
	if rs.GuardianProfileLoader == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent enrollment profile not wired")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}
	accountID := int64(claims.ID)

	slug := strings.TrimSpace(chi.URLParam(r, "tenantSlug"))
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	schoolID, err := rs.resolveSchoolID(r.Context(), slug)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}

	loaded, err := rs.GuardianProfileLoader.LoadForTenant(r.Context(), accountID, schoolID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	resp := common.BuildGuardianProfileResponse(claims, loaded)
	common.Respond(w, r, http.StatusOK, resp, "Profile retrieved")
}

// resolveSchoolID wraps SchoolRepo.FindBySlug in WithAdminTx so the
// cross-tenant lookup is allowed. Returns an error when the slug
// doesn't resolve or the school is soft-deleted.
func (rs *Resource) resolveSchoolID(ctx context.Context, slug string) (int64, error) {
	var out int64
	err := tenant.WithAdminTx(ctx, rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, findErr := rs.SchoolService.GetSchoolBySlug(adminCtx, slug)
		if findErr != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}
		out = school.ID
		return nil
	})
	if err != nil {
		return 0, err
	}
	return out, nil
}

// submitParentEnrollment handles a parent-authenticated submission.
// The handler resolves the slug to a tenant via admin-tx, resolves the
// caller's guardian submit facts for that school, then runs the
// existing RequestService.Submit with GuardianAccountID stamped from
// claims.ID and the originating IP captured for rate-limiting. Captcha
// is skipped — the JWT is the trust signal.
//
// Authorization (#1663): a parent does NOT need an existing tenant
// mapping — applying to a new school is the point of the picker; the
// phase's audience config (enforced in RequestService.Submit via
// GuardianSubmitEligible) decides whether unlinked parents qualify.
// What IS blocked here: an account whose guardian relationships at the
// school all lack parent_portal.enrollment.submit — an explicitly
// revoked permission must not be bypassable via the parent portal
// (see .claude/rules/guardian-parent-permissions.md).
func (rs *Resource) submitParentEnrollment(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent submit not configured")))
		return
	}
	if rs.ParentService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent submit: parent service missing")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}
	accountID := int64(claims.ID)

	slug := strings.TrimSpace(chi.URLParam(r, "tenantSlug"))
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	wireReq, err := decodeParentEnrollmentBody(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	out := rs.runParentEnrollmentSubmit(r, accountID, slug, wireReq)
	rs.respondParentEnrollment(w, r, out)
}

// decodeParentEnrollmentBody reuses the public submit wire shape. The parent
// path has no captcha (the JWT is the trust signal) and never sends child ids;
// both extra fields decode inertly. Bind() defaults nil maps/slices.
func decodeParentEnrollmentBody(r *http.Request) (*enrollmentAPI.SubmitEnrollmentRequest, error) {
	wireReq := &enrollmentAPI.SubmitEnrollmentRequest{}
	if err := json.NewDecoder(r.Body).Decode(wireReq); err != nil {
		return nil, err
	}
	_ = wireReq.Bind(r)
	if wireReq.LateInviteToken == "" {
		wireReq.LateInviteToken = strings.TrimSpace(r.URL.Query().Get("late_invite"))
	}
	return wireReq, nil
}

// parentSubmitOutcome captures the four mutually-distinguished results of the
// admin-tx submit closure so the post-tx mapping can pick the right response.
type parentSubmitOutcome struct {
	result     *enrollmentService.SubmitResult
	submitErr  error
	forbidden  bool
	resolveErr error
}

// runParentEnrollmentSubmit resolves the slug to a tenant, resolves the
// caller's guardian submit facts, and runs the submit — all inside one
// admin-tx so the service's inner TxHandler reuses this transaction.
// Failures inside the submit (parse/submit) are captured in submitErr but do
// not roll the closure back, matching the pre-existing behavior; only
// tenant-resolve failures return an error from the closure.
func (rs *Resource) runParentEnrollmentSubmit(r *http.Request, accountID int64, slug string, wireReq *enrollmentAPI.SubmitEnrollmentRequest) parentSubmitOutcome {
	var out parentSubmitOutcome
	out.resolveErr = tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err := rs.SchoolService.GetSchoolBySlug(adminCtx, slug)
		if err != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		// Guardian facts for (account, school): a parent whose guardian
		// relationships at this school all lack the submit permission is
		// blocked outright — an admin revoked it deliberately. Accounts
		// with no guardian rows here (new school) pass; the phase's
		// audience config decides their eligibility in Submit (#1663).
		status, stErr := rs.ParentService.GetEnrollmentSubmitStatus(adminCtx, accountID, school.ID)
		if stErr != nil {
			return fmt.Errorf("resolve enrollment submit status: %w", stErr)
		}
		if status.HasGuardianLink && !status.HasSubmitPermission {
			out.forbidden = true
			return nil
		}

		out.result, out.submitErr = rs.submitEnrollmentForTenant(adminCtx, school.ID, accountID, status.HasSubmitPermission, wireReq, getClientIP(r))
		return nil
	})
	return out
}

// submitEnrollmentForTenant binds the wire request for the resolved tenant,
// stamps the guardian account id + submit eligibility, and forwards to
// RequestService.Submit under the tenant context. A parse failure returns
// before the service call.
func (rs *Resource) submitEnrollmentForTenant(adminCtx context.Context, schoolID, accountID int64, submitEligible bool, wireReq *enrollmentAPI.SubmitEnrollmentRequest, clientIP string) (*enrollmentService.SubmitResult, error) {
	serviceReq, parseErr := enrollmentAPI.BuildServiceRequest(wireReq, schoolID, clientIP)
	if parseErr != nil {
		return nil, parseErr
	}
	serviceReq.GuardianAccountID = &accountID
	serviceReq.GuardianSubmitEligible = submitEligible
	return rs.RequestService.Submit(tenant.WithTenantID(adminCtx, schoolID), serviceReq)
}

// respondParentEnrollment maps the submit outcome to the HTTP response. The
// priority (resolve → forbidden → submit → success) matches the original flow.
func (rs *Resource) respondParentEnrollment(w http.ResponseWriter, r *http.Request, out parentSubmitOutcome) {
	if out.resolveErr != nil {
		common.RenderError(w, r, common.ErrorNotFound(out.resolveErr))
		return
	}
	if out.forbidden {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("enrollment submission is not permitted for this account at this school")))
		return
	}
	if out.submitErr != nil {
		enrollmentAPI.MapSubmitError(w, r, out.submitErr)
		return
	}

	resp := enrollmentAPI.SubmitEnrollmentResponse{
		RequestID: strconv.FormatInt(out.result.Request.ID, 10),
		StatusURL: out.result.StatusURL,
		Warnings:  out.result.Warnings,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Enrollment submitted")
}
