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

	// Reuse the public submit wire shape. The parent path has no captcha
	// (the JWT is the trust signal) and never sends child ids; both extra
	// fields decode inertly. Bind() defaults nil maps/slices.
	wireReq := &enrollmentAPI.SubmitEnrollmentRequest{}
	if err := json.NewDecoder(r.Body).Decode(wireReq); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	_ = wireReq.Bind(r)
	if wireReq.LateInviteToken == "" {
		wireReq.LateInviteToken = strings.TrimSpace(r.URL.Query().Get("late_invite"))
	}

	var (
		result    *enrollmentService.SubmitResult
		submitErr error
		forbidden bool
	)
	resolveErr := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err := rs.SchoolService.GetSchoolBySlug(adminCtx, slug)
		if err != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		// Tenant-mapping check: the parent JWT identifies the actor but
		// does NOT prove they belong at this school. Without this
		// guard, any authenticated parent could stamp
		// guardian_account_id on requests for arbitrary tenants.
		mapped, mapErr := rs.AuthService.VerifyAccountTenantMembership(adminCtx, accountID, school.ID)
		if mapErr != nil {
			return fmt.Errorf("verify tenant membership: %w", mapErr)
		}
		if !mapped {
			forbidden = true
			return nil
		}

		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)

		serviceReq, parseErr := enrollmentAPI.BuildServiceRequest(wireReq, school.ID, getClientIP(r))
		if parseErr != nil {
			submitErr = parseErr
			return nil
		}
		serviceReq.GuardianAccountID = &accountID
		res, err := rs.RequestService.Submit(tenantCtx, serviceReq)
		if err != nil {
			submitErr = err
			return nil
		}
		result = res
		return nil
	})
	if resolveErr != nil {
		common.RenderError(w, r, common.ErrorNotFound(resolveErr))
		return
	}
	if forbidden {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("account is not a member of this school")))
		return
	}
	if submitErr != nil {
		enrollmentAPI.MapSubmitError(w, r, submitErr)
		return
	}

	resp := enrollmentAPI.SubmitEnrollmentResponse{
		RequestID: strconv.FormatInt(result.Request.ID, 10),
		StatusURL: result.StatusURL,
		Warnings:  result.Warnings,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Enrollment submitted")
}
