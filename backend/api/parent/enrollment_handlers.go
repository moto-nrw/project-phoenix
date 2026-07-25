package parent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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

// getEnrollmentBootstrap serves the enrollment form-load payload for the
// authenticated parents portal. It mirrors the anonymous public
// /form-bootstrap endpoint. Tenant comes from the path slug (admin-tx
// resolves it); the parent JWT (scope=parent, enforced by ParentMiddleware)
// is the authorization boundary. Captcha is skipped — the JWT is the
// anti-bot signal.
//
// Audience gate (#1663): audience-restricted (linked_parents) phases — which
// the public gate refuses — load only for a caller whose guardian link at
// this school grants parent_portal.enrollment.submit (the same fact the
// picker and submit handler resolve). Everyone else gets the public gate,
// which still serves open / new_students phases but 404s linked_parents, so
// a guessed phase id at another school cannot leak a hidden phase's form.
// The real per-phase eligibility is still enforced on submit
// (GuardianSubmitEligible + audience rules).
func (rs *Resource) getEnrollmentBootstrap(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent enrollment bootstrap not configured")))
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
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "phaseId", "phaseId is required")
	if !ok {
		return
	}

	if rs.ParentService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent enrollment bootstrap: parent service missing")))
		return
	}

	schoolID, err := rs.resolveSchoolID(r.Context(), slug)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}

	// Restricted (linked_parents) phases must only load for a caller who
	// could actually submit — the same active guardian-link +
	// parent_portal.enrollment.submit fact the picker and submit handler use.
	// Without this, any parent-scoped JWT could bootstrap another school's
	// hidden phase by guessing its id and read its form schema, offerings,
	// and legal metadata (#1663). We can't cheaply know the phase audience
	// before loading, so we resolve the caller's submit status and pick the
	// matching gate: an eligible guardian gets the enrollee loader (which
	// serves restricted phases); anyone else falls back to the public loader,
	// which serves open / new_students phases but 404s linked_parents exactly
	// as the anonymous public endpoint does. A valid late-invite token still
	// lifts the restriction through the public gate.
	status, statusErr := rs.ParentService.GetEnrollmentSubmitStatus(r.Context(), accountID, schoolID)
	if statusErr != nil {
		common.RenderError(w, r, common.ErrorInternalServer(statusErr))
		return
	}
	allowRestricted := status != nil && status.HasSubmitPermission

	var data *enrollmentService.PublicFormBootstrapData
	lateInviteToken := strings.TrimSpace(r.URL.Query().Get("late_invite"))
	loadErr := tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
		var loaded *enrollmentService.PublicFormBootstrapData
		var e error
		if allowRestricted {
			loaded, e = rs.RequestService.LoadEnrolleeFormBootstrap(txCtx, phaseID, time.Now(), lateInviteToken)
		} else {
			loaded, e = rs.RequestService.LoadPublicFormBootstrap(txCtx, phaseID, time.Now(), lateInviteToken)
		}
		data = loaded
		return e
	})
	if loadErr != nil {
		enrollmentAPI.RenderPublicEnrollmentBootstrapError(w, r, loadErr)
		return
	}

	resp := enrollmentAPI.BuildPublicEnrollmentFormBootstrapResponse(data, enrollmentAPI.PublicCaptchaConfigResponse{})
	common.Respond(w, r, http.StatusOK, resp, "Parent enrollment form bootstrap retrieved")
}

// resolveSchoolID resolves the {tenantSlug} path segment to a school id inside
// WithAdminTx so the cross-tenant lookup is allowed. Returns an error when the
// identifier doesn't resolve or the school is soft-deleted.
//
// The lookup is by SUBDOMAIN, not slug (#1663): the parents portal reaches this
// route through links built from platform.schools.subdomain, which is the same
// identifier /auth/tenant/resolve accepts (the enroll page resolves tenant
// metadata through it in the same render). platform.schools.slug is only unique
// per organization, so it is not usable as a global routing key at all.
func (rs *Resource) resolveSchoolID(ctx context.Context, slug string) (int64, error) {
	var out int64
	err := tenant.WithAdminTx(ctx, rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, findErr := rs.SchoolService.GetSchoolBySubdomain(adminCtx, slug)
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
// Guardian parent-portal permissions are relationship-scoped (per child),
// so this handler does NOT apply an account-wide denial: the absence of
// parent_portal.enrollment.submit on one existing relationship (e.g. a
// pickup-only link) must not block a new-child application to an open or
// new_students phase. The real per-child gate lives in the service: an
// existing_students re-enrollment that matches a specific student is
// authorized against THAT student's own relationship (ErrChildEnrollment
// NotPermitted). The school-wide submit fact is forwarded only as the
// GuardianSubmitEligible audience flag for linked_parents phases (see
// .claude/rules/guardian-parent-permissions.md).
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

// parentSubmitOutcome captures the mutually-distinguished results of the
// admin-tx submit closure so the post-tx mapping can pick the right response.
type parentSubmitOutcome struct {
	result     *enrollmentService.SubmitResult
	submitErr  error
	resolveErr error
}

// runParentEnrollmentSubmit resolves the slug to a tenant, resolves the
// caller's guardian submit facts, and runs the submit, all inside one
// admin-tx so the service's inner TxHandler reuses this transaction.
// A submit failure is both captured in submitErr AND returned from the closure
// so WithAdminTx rolls the shared transaction back: the service can fail (e.g.
// the #1663 ambiguous existing-student match) only after enrollment.requests /
// request_guardians rows are already inserted in this same tx, and returning
// nil would commit those orphans while the handler responds with an error.
// respondParentEnrollment uses the dedicated submitErr field to tell a
// rolled-back submit apart from a genuine tenant-resolve failure.
func (rs *Resource) runParentEnrollmentSubmit(r *http.Request, accountID int64, slug string, wireReq *enrollmentAPI.SubmitEnrollmentRequest) parentSubmitOutcome {
	var out parentSubmitOutcome
	out.resolveErr = tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		// By subdomain, matching resolveSchoolID and /auth/tenant/resolve —
		// see resolveSchoolID for why slug is not a routing key (#1663).
		school, err := rs.SchoolService.GetSchoolBySubdomain(adminCtx, slug)
		if err != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		// Guardian facts for (account, school). We resolve ONLY the school-wide
		// submit fact here and forward it as the GuardianSubmitEligible audience
		// flag (linked_parents phases require it). We deliberately do NOT apply an
		// account-wide denial: guardian parent-portal permissions are per-child,
		// so a missing submit permission on one relationship (e.g. pickup-only)
		// must not block a new-child application the phase accepts anonymously
		// (#1663). Re-enrollment of a SPECIFIC existing child is authorized
		// per-student inside Submit (ErrChildEnrollmentNotPermitted).
		status, stErr := rs.ParentService.GetEnrollmentSubmitStatus(adminCtx, accountID, school.ID)
		if stErr != nil {
			return fmt.Errorf("resolve enrollment submit status: %w", stErr)
		}

		out.result, out.submitErr = rs.submitEnrollmentForTenant(adminCtx, school.ID, accountID, status.HasSubmitPermission, wireReq, getClientIP(r))
		// Return the submit error so WithAdminTx rolls back any rows the
		// service already inserted before failing (see the function doc).
		// nil is returned on success. respondParentEnrollment maps the
		// failure via the dedicated submitErr field, not resolveErr.
		return out.submitErr
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

// respondParentEnrollment maps the submit outcome to the HTTP response.
// Priority: submit → resolve → success. submitErr is checked first because a
// submit failure now rolls the admin-tx back, so WithAdminTx also surfaces that
// same error as resolveErr; the dedicated submitErr field lets us map it to its
// real status (e.g. 400 ambiguous, 403 not-permitted) instead of the resolve
// 404. Per-child authorization failures (ErrChildEnrollmentNotPermitted) travel
// through submitErr and MapSubmitError renders them as 403.
func (rs *Resource) respondParentEnrollment(w http.ResponseWriter, r *http.Request, out parentSubmitOutcome) {
	if out.submitErr != nil {
		enrollmentAPI.MapSubmitError(w, r, out.submitErr)
		return
	}
	if out.resolveErr != nil {
		common.RenderError(w, r, common.ErrorNotFound(out.resolveErr))
		return
	}

	resp := enrollmentAPI.SubmitEnrollmentResponse{
		RequestID: strconv.FormatInt(out.result.Request.ID, 10),
		StatusURL: out.result.StatusURL,
		Warnings:  out.result.Warnings,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Enrollment submitted")
}
