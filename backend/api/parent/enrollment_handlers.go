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
// The handler resolves the slug to a tenant via admin-tx, verifies the
// calling account is mapped to that tenant via auth.account_tenants
// (defense against an authenticated parent stamping rows on schools
// they have no relationship with), then runs the existing
// RequestService.Submit with GuardianAccountID stamped from claims.ID
// and the originating IP captured for rate-limiting. Captcha is
// skipped — the JWT is the trust signal.
func (rs *Resource) submitParentEnrollment(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent submit not configured")))
		return
	}
	if rs.AuthService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent submit: account tenant repo missing")))
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

// runParentEnrollmentSubmit resolves the slug to a tenant, verifies the caller
// is mapped to it, and runs the submit — all inside one admin-tx so the
// service's inner TxHandler reuses this transaction. Failures inside the submit
// (parse/submit) are captured in submitErr but do not roll the closure back,
// matching the pre-existing behavior; only tenant-resolve failures return an
// error from the closure.
func (rs *Resource) runParentEnrollmentSubmit(r *http.Request, accountID int64, slug string, wireReq *enrollmentAPI.SubmitEnrollmentRequest) parentSubmitOutcome {
	var out parentSubmitOutcome
	out.resolveErr = tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err := rs.SchoolService.GetSchoolBySlug(adminCtx, slug)
		if err != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		// Tenant-mapping check: the parent JWT identifies the actor but
		// does NOT prove they belong at this school. Without this guard,
		// any authenticated parent could stamp guardian_account_id on
		// requests for arbitrary tenants.
		mapped, mapErr := rs.AuthService.VerifyAccountTenantMembership(adminCtx, accountID, school.ID)
		if mapErr != nil {
			return fmt.Errorf("verify tenant membership: %w", mapErr)
		}
		if !mapped {
			out.forbidden = true
			return nil
		}

		out.result, out.submitErr = rs.submitEnrollmentForTenant(adminCtx, school.ID, accountID, wireReq, getClientIP(r))
		return nil
	})
	return out
}

// submitEnrollmentForTenant binds the wire request for the resolved tenant,
// stamps the guardian account id, and forwards to RequestService.Submit under
// the tenant context. A parse failure returns before the service call.
func (rs *Resource) submitEnrollmentForTenant(adminCtx context.Context, schoolID, accountID int64, wireReq *enrollmentAPI.SubmitEnrollmentRequest, clientIP string) (*enrollmentService.SubmitResult, error) {
	serviceReq, parseErr := enrollmentAPI.BuildServiceRequest(wireReq, schoolID, clientIP)
	if parseErr != nil {
		return nil, parseErr
	}
	serviceReq.GuardianAccountID = &accountID
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
		common.RenderError(w, r, common.ErrorForbidden(errors.New("account is not a member of this school")))
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
