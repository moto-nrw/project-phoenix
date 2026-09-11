package operator

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// MFAAdminStateResponse mirrors the tenant-side shape so the same
// frontend modal can consume either surface.
type MFAAdminStateResponse struct {
	Enrolled bool   `json:"enrolled"`
	Override string `json:"override"`
}

// MFAAdminResetRequest is the body for DELETE /operator/schools/{id}/accounts/{accountId}/mfa.
type MFAAdminResetRequest struct {
	Reason string `json:"reason"`
}

// Bind enforces a non-empty reason >= 3 chars, matching the tenant-side
// validation. The audit trail relies on a non-trivial reason being
// present.
func (req *MFAAdminResetRequest) Bind(_ *http.Request) error {
	req.Reason = strings.TrimSpace(req.Reason)
	return validation.ValidateStruct(req,
		validation.Field(&req.Reason, validation.Required, validation.Length(3, 0)),
	)
}

// MFAAdminOverrideSetRequest is the body for the override-set endpoint.
// Mirrors the tenant request struct so the frontend payload is identical.
type MFAAdminOverrideSetRequest struct {
	Override string `json:"override"`
	Reason   string `json:"reason"`
}

// Bind validates the override value against the allow-list and requires
// a non-trivial reason.
func (req *MFAAdminOverrideSetRequest) Bind(_ *http.Request) error {
	req.Override = strings.TrimSpace(req.Override)
	req.Reason = strings.TrimSpace(req.Reason)
	if !authSvc.IsValidMFAAdminOverride(req.Override) {
		return errors.New("override must be one of: none, force_off, force_on")
	}
	return validation.ValidateStruct(req,
		validation.Field(&req.Reason, validation.Required, validation.Length(3, 0)),
	)
}

// requireMFAAdminDeps is the first-line guard for the new endpoints:
// both the tenant MFA service and the account-tenant repo must be wired.
// Returns false (with a 503 already written) when either is missing.
func (rs *ProvisioningResource) requireMFAAdminDeps(w http.ResponseWriter, r *http.Request) bool {
	if rs.TenantMFAService == nil {
		common.RenderError(w, r, ErrInternal("MFA admin endpoints are not configured"))
		return false
	}
	return true
}

// resolveOperatorMFAAdminTarget parses the path params, verifies the
// account is mapped to the given school via auth.account_tenants, and
// returns the parsed IDs. Writes the response and returns ok=false on
// validation/lookup failure.
func (rs *ProvisioningResource) resolveOperatorMFAAdminTarget(w http.ResponseWriter, r *http.Request) (schoolID, accountID int64, ok bool) {
	schoolID, ok = common.ParseInt64IDWithError(w, r, "id", "invalid school ID")
	if !ok {
		return 0, 0, false
	}
	accountID, ok = common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return 0, 0, false
	}
	exists, err := rs.TenantMFAService.AccountBelongsToTenant(r.Context(), accountID, schoolID)
	if err != nil {
		common.RenderError(w, r, ErrInternal("failed to verify account membership"))
		return 0, 0, false
	}
	if !exists {
		common.RenderError(w, r, ErrNotFound("account is not a member of this school"))
		return 0, 0, false
	}
	return schoolID, accountID, true
}

// withTenantContext injects the school as the tenant for the duration of
// the request. Required so the MFA service's audit append
// (recordAuthEvent) sees a tenantID and actually writes the audit row;
// without this the operator flow would silently drop the audit event.
func withTenantContext(r *http.Request, schoolID int64) *http.Request {
	return r.WithContext(tenant.WithTenantID(r.Context(), schoolID))
}

// GetSchoolAccountMFAState returns the enrolled + override snapshot for
// the modal to render the right action set.
func (rs *ProvisioningResource) GetSchoolAccountMFAState(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFAAdminDeps(w, r) {
		return
	}
	schoolID, accountID, ok := rs.resolveOperatorMFAAdminTarget(w, r)
	if !ok {
		return
	}
	enrolled, err := rs.TenantMFAService.HasEnrollment(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrInternal("failed to read MFA enrollment"))
		return
	}
	// The school MFA modal shows the *tenant-scoped* override that
	// affects logins to this school. The platform-wide ("operator
	// account-wide") row lives behind its own endpoint
	// (/operator/accounts/{id}/mfa/global-override) so the school
	// view never mixes per-school and account-wide state.
	override, err := rs.TenantMFAService.GetTenantMFAOverride(r.Context(), accountID, schoolID)
	if err != nil {
		// Read-side failure shouldn't block the modal from rendering —
		// fall back to "none" and let the admin still trigger writes.
		override = authSvc.MFAAdminOverrideNone
	}
	common.Respond(w, r, http.StatusOK, &MFAAdminStateResponse{
		Enrolled: enrolled,
		Override: override,
	}, "MFA state retrieved successfully")
}

// ResetSchoolAccountMFA wipes enrollment + trusted devices for the
// target account, audited with actor_type=operator.
func (rs *ProvisioningResource) ResetSchoolAccountMFA(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFAAdminDeps(w, r) {
		return
	}
	schoolID, accountID, ok := rs.resolveOperatorMFAAdminTarget(w, r)
	if !ok {
		return
	}
	operatorID := operatorIDFromClaims(r)
	if operatorID == 0 {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	req := &MFAAdminResetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	ctx := withTenantContext(r, schoolID).Context()
	if err := rs.TenantMFAService.OperatorAdminDisable(ctx, operatorID, schoolID, accountID, req.Reason); err != nil {
		if errors.Is(err, authSvc.ErrMFAPermissionDenied) {
			common.RenderError(w, r, ErrForbidden("Permission denied"))
			return
		}
		common.RenderError(w, r, ErrInternal("MFA admin disable failed"))
		return
	}
	common.RespondNoContent(w, r)
}

// SetSchoolAccountMFAOverride flips the per-account override
// (none / force_off / force_on). Trusted-device revocation is part of
// the service path for force_off.
func (rs *ProvisioningResource) SetSchoolAccountMFAOverride(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFAAdminDeps(w, r) {
		return
	}
	schoolID, accountID, ok := rs.resolveOperatorMFAAdminTarget(w, r)
	if !ok {
		return
	}
	operatorID := operatorIDFromClaims(r)
	if operatorID == 0 {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	req := &MFAAdminOverrideSetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	ctx := withTenantContext(r, schoolID).Context()
	if err := rs.TenantMFAService.OperatorSetMFAOverride(ctx, operatorID, schoolID, accountID, req.Override, req.Reason); err != nil {
		if errors.Is(err, authSvc.ErrMFAInvalidOverride) {
			common.RenderError(w, r, ErrInvalidRequest(err))
			return
		}
		if errors.Is(err, authSvc.ErrMFAPermissionDenied) {
			common.RenderError(w, r, ErrForbidden("Permission denied"))
			return
		}
		common.RenderError(w, r, ErrInternal("MFA override failed"))
		return
	}
	common.RespondNoContent(w, r)
}

// operatorIDFromClaims extracts the operator account id from the JWT
// claims on the request. Returns 0 when the request is unauthenticated
// — callers must reject in that case.
func operatorIDFromClaims(r *http.Request) int64 {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return 0
	}
	return int64(claims.ID)
}

// MFAGlobalOverrideStateResponse mirrors MFAAdminStateResponse for the
// account-wide ("operator emergency switch") read. Enrolled is
// duplicated from the per-school endpoint so the operator's account-
// level modal can render without two round-trips; Override holds the
// platform-wide auth.mfa_overrides row's value or "none" when no
// global row is present.
type MFAGlobalOverrideStateResponse struct {
	Enrolled bool   `json:"enrolled"`
	Override string `json:"override"`
}

// GetAccountMFAGlobalOverride returns the platform-wide override row
// for an account — used by the operator's account-level modal to render
// the "Account-weit deaktivieren" emergency surface.
//
// Account-scope (not school-scope): the operator route lives directly
// under /operator/accounts/{accountId}/mfa/global-override; there is
// no schoolId in the path because the action by definition spans every
// tenant the account belongs to. (#1430 review round 2.)
func (rs *ProvisioningResource) GetAccountMFAGlobalOverride(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFAAdminDeps(w, r) {
		return
	}
	accountID, ok := rs.resolveOperatorAccountTarget(w, r)
	if !ok {
		return
	}
	enrolled, err := rs.TenantMFAService.HasEnrollment(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrInternal("failed to read MFA enrollment"))
		return
	}
	override, err := rs.TenantMFAService.GetGlobalMFAOverride(r.Context(), accountID)
	if err != nil {
		override = authSvc.MFAAdminOverrideNone
	}
	common.Respond(w, r, http.StatusOK, &MFAGlobalOverrideStateResponse{
		Enrolled: enrolled,
		Override: override,
	}, "MFA global override retrieved successfully")
}

// SetAccountMFAGlobalOverride writes (or clears) the platform-wide
// override row. Force_off here is the explicit "mailbox lockout
// emergency switch" — applies across every tenant the account belongs
// to and revokes every trusted device. Tenant admins intentionally
// cannot reach this surface; only the operator dashboard exposes it.
func (rs *ProvisioningResource) SetAccountMFAGlobalOverride(w http.ResponseWriter, r *http.Request) {
	if !rs.requireMFAAdminDeps(w, r) {
		return
	}
	accountID, ok := rs.resolveOperatorAccountTarget(w, r)
	if !ok {
		return
	}
	operatorID := operatorIDFromClaims(r)
	if operatorID == 0 {
		common.RenderError(w, r, ErrUnauthorized())
		return
	}
	req := &MFAAdminOverrideSetRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	if err := rs.TenantMFAService.OperatorSetGlobalMFAOverride(r.Context(), operatorID, accountID, req.Override, req.Reason); err != nil {
		if errors.Is(err, authSvc.ErrMFAInvalidOverride) {
			common.RenderError(w, r, ErrInvalidRequest(err))
			return
		}
		if errors.Is(err, authSvc.ErrMFAPermissionDenied) {
			common.RenderError(w, r, ErrForbidden("Permission denied"))
			return
		}
		common.RenderError(w, r, ErrInternal("MFA global override failed"))
		return
	}
	common.RespondNoContent(w, r)
}

// resolveOperatorAccountTarget parses {accountId} from the path. Same
// shape as resolveOperatorMFAAdminTarget but without a schoolId — the
// account-wide endpoints are deliberately decoupled from any school.
func (rs *ProvisioningResource) resolveOperatorAccountTarget(w http.ResponseWriter, r *http.Request) (int64, bool) {
	accountIDStr := chi.URLParam(r, "accountId")
	accountID, err := strconv.ParseInt(accountIDStr, 10, 64)
	if err != nil || accountID <= 0 {
		common.RenderError(w, r, ErrInvalidRequest(errors.New(common.MsgInvalidAccountID)))
		return 0, false
	}
	return accountID, true
}
