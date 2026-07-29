package operator

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// Operator endpoints for managing which schools an existing account may access
// (issue #1021). Cross-tenant by nature, therefore operator-only: a school
// admin can pull an account into their own school through the invitation flow,
// but never into someone else's.

type grantAccountTenantAccessRequest struct {
	SchoolID  int64  `json:"school_id"`
	RoleID    int64  `json:"role_id"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Position  string `json:"position,omitempty"`
}

func (req *grantAccountTenantAccessRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)
	if req.SchoolID <= 0 {
		return errors.New("school_id is required")
	}
	if req.RoleID <= 0 {
		return errors.New("role_id is required")
	}
	return nil
}

type updateAccountTenantRoleRequest struct {
	RoleID int64 `json:"role_id"`
}

func (req *updateAccountTenantRoleRequest) Bind(_ *http.Request) error {
	if req.RoleID <= 0 {
		return errors.New("role_id is required")
	}
	return nil
}

// ListAccountTenantAccess handles GET /operator/accounts/{accountId}/tenants.
func (rs *ProvisioningResource) ListAccountTenantAccess(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}

	entries, err := rs.service.ListAccountTenantAccess(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, accountTenantAccessErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, entries, "Account school access retrieved successfully")
}

// ListAssignableSchoolRoles handles GET /operator/accounts/{accountId}/tenants/{tenantId}/roles.
func (rs *ProvisioningResource) ListAssignableSchoolRoles(w http.ResponseWriter, r *http.Request) {
	accountID, schoolID, ok := parseAccountTenantParams(w, r)
	if !ok {
		return
	}
	if _, err := rs.service.ListAccountTenantAccess(r.Context(), accountID); err != nil {
		common.RenderError(w, r, accountTenantAccessErrorRenderer(err))
		return
	}
	roles, err := rs.service.ListAssignableSchoolRoles(r.Context(), schoolID)
	if err != nil {
		common.RenderError(w, r, accountTenantAccessErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, roles, "Assignable school roles retrieved successfully")
}

// GrantAccountTenantAccess handles POST /operator/accounts/{accountId}/tenants.
func (rs *ProvisioningResource) GrantAccountTenantAccess(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}

	req := &grantAccountTenantAccessRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	entries, err := rs.service.GrantAccountTenantAccess(
		r.Context(),
		accountID,
		req.SchoolID,
		platformSvc.GrantAccountTenantAccessRequest{
			RoleID:    req.RoleID,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Position:  req.Position,
		},
		operatorIDFromContext(r),
		getClientIP(r),
	)
	if err != nil {
		common.RenderError(w, r, accountTenantAccessErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, entries, "School access granted successfully")
}

// UpdateAccountTenantRole handles PUT /operator/accounts/{accountId}/tenants/{tenantId}.
func (rs *ProvisioningResource) UpdateAccountTenantRole(w http.ResponseWriter, r *http.Request) {
	accountID, schoolID, ok := parseAccountTenantParams(w, r)
	if !ok {
		return
	}

	req := &updateAccountTenantRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}

	entries, err := rs.service.UpdateAccountTenantRole(
		r.Context(), accountID, schoolID, req.RoleID, operatorIDFromContext(r), getClientIP(r),
	)
	if err != nil {
		common.RenderError(w, r, accountTenantAccessErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, entries, "School role updated successfully")
}

// RevokeAccountTenantAccess handles DELETE /operator/accounts/{accountId}/tenants/{tenantId}.
func (rs *ProvisioningResource) RevokeAccountTenantAccess(w http.ResponseWriter, r *http.Request) {
	accountID, schoolID, ok := parseAccountTenantParams(w, r)
	if !ok {
		return
	}

	entries, err := rs.service.RevokeAccountTenantAccess(
		r.Context(), accountID, schoolID, operatorIDFromContext(r), getClientIP(r),
	)
	if err != nil {
		common.RenderError(w, r, accountTenantAccessErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, entries, "School access revoked successfully")
}

func parseAccountTenantParams(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return 0, 0, false
	}
	schoolID, ok := common.ParseInt64IDWithError(w, r, "tenantId", "invalid school ID")
	if !ok {
		return 0, 0, false
	}
	return accountID, schoolID, true
}

func operatorIDFromContext(r *http.Request) int64 {
	return int64(jwt.ClaimsFromCtx(r.Context()).ID)
}

// accountTenantAccessErrorRenderer surfaces the validation messages verbatim —
// unlike the generic provisioning renderer, which collapses them to "invalid
// input data". The operator needs to know WHY a role was rejected.
func accountTenantAccessErrorRenderer(err error) render.Renderer {
	var accountNotFound *platformSvc.AccountNotFoundError
	var accessNotFound *platformSvc.AccountTenantAccessNotFoundError
	var invalidData *platformSvc.InvalidDataError

	switch {
	case errors.As(err, &accountNotFound):
		return ErrNotFound("Account not found")
	case errors.As(err, &accessNotFound):
		return ErrNotFound("Account has no access to this school")
	case errors.As(err, &invalidData):
		return ErrInvalidRequest(invalidData.Err)
	default:
		return ProvisioningErrorRenderer(err)
	}
}
