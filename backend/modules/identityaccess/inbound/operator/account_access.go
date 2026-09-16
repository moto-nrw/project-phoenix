package operator

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Operator endpoints for managing which schools an existing account may
// access (issue #1021). Cross-tenant by nature, therefore operator-only: a
// school admin can pull an account into their own school through the
// invitation flow, but never into someone else's.

// AccountTenantRole is one role an account holds at one school. IDs stay
// decimal strings in JSON so the frontend never loses int64 precision.
type AccountTenantRole struct {
	ID       int64   `json:"id,string"`
	Name     string  `json:"name"`
	IsSystem bool    `json:"is_system"`
	BaseRole *string `json:"base_role,omitempty"`
}

// AccountTenantAccessEntry is one school an account has (or had) access to,
// including the roles it holds there.
type AccountTenantAccessEntry struct {
	TenantID         int64               `json:"tenant_id"`
	SchoolName       string              `json:"school_name"`
	SchoolSlug       string              `json:"school_slug"`
	SchoolActive     bool                `json:"school_active"`
	OrganizationID   int64               `json:"organization_id"`
	OrganizationName string              `json:"organization_name"`
	Status           string              `json:"status"`
	ActivatedAt      *time.Time          `json:"activated_at,omitempty"`
	DeactivatedAt    *time.Time          `json:"deactivated_at,omitempty"`
	HasPerson        bool                `json:"has_person"`
	HasStaff         bool                `json:"has_staff"`
	Roles            []AccountTenantRole `json:"roles"`
}

// RoleOption is the role projection the operator role picker renders.
type RoleOption struct {
	ID       int64  `json:"id,string"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
}

type grantAccountTenantAccessRequest struct {
	SchoolID  int64         `json:"school_id"`
	RoleID    common.JSONID `json:"role_id"`
	FirstName string        `json:"first_name,omitempty"`
	LastName  string        `json:"last_name,omitempty"`
	Position  string        `json:"position,omitempty"`
}

func (req *grantAccountTenantAccessRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)
	if req.SchoolID <= 0 {
		return errors.New("school_id is required")
	}
	if req.RoleID.Int64() <= 0 {
		return errors.New("role_id is required")
	}
	return nil
}

type updateAccountTenantRoleRequest struct {
	RoleID common.JSONID `json:"role_id"`
}

func (req *updateAccountTenantRoleRequest) Bind(_ *http.Request) error {
	if req.RoleID.Int64() <= 0 {
		return errors.New("role_id is required")
	}
	return nil
}

// ListAccountTenantAccess handles GET /operator/accounts/{accountId}/tenants.
func (rs *Resource) ListAccountTenantAccess(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}
	entries, err := rs.identity.ListAccountTenantAccess(r.Context(), accountID)
	rs.respondAccess(w, r, entries, err, http.StatusOK, "Account school access retrieved successfully")
}

// ListAssignableSchoolRoles handles GET /operator/accounts/{accountId}/tenants/{tenantId}/roles.
func (rs *Resource) ListAssignableSchoolRoles(w http.ResponseWriter, r *http.Request) {
	accountID, schoolID, ok := parseAccountTenantParams(w, r)
	if !ok {
		return
	}
	if _, err := rs.identity.ListAccountTenantAccess(r.Context(), accountID); err != nil {
		common.RenderError(w, r, rs.accessError(err))
		return
	}
	roles, err := rs.identity.ListAssignableSchoolRoles(r.Context(), schoolID)
	if err != nil {
		common.RenderError(w, r, rs.accessError(err))
		return
	}
	options := make([]RoleOption, 0, len(roles))
	for _, role := range roles {
		options = append(options, RoleOption{ID: role.ID, Name: role.Name, IsSystem: role.IsSystem})
	}
	common.Respond(w, r, http.StatusOK, options, "Assignable school roles retrieved successfully")
}

// GrantAccountTenantAccess handles POST /operator/accounts/{accountId}/tenants.
func (rs *Resource) GrantAccountTenantAccess(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseInt64IDWithError(w, r, "accountId", "invalid account ID")
	if !ok {
		return
	}
	req := &grantAccountTenantAccessRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, rs.responses.InvalidRequest(err))
		return
	}
	entries, err := rs.identity.GrantAccountTenantAccess(r.Context(), identityaccess.GrantAccountTenantAccess{
		AccountID: accountID, SchoolID: req.SchoolID, RoleID: req.RoleID.Int64(),
		FirstName: req.FirstName, LastName: req.LastName, Position: req.Position,
		OperatorID: operatorIDFromContext(r), ClientIP: clientIP(r),
	})
	rs.respondAccess(w, r, entries, err, http.StatusCreated, "School access granted successfully")
}

// UpdateAccountTenantRole handles PUT /operator/accounts/{accountId}/tenants/{tenantId}.
func (rs *Resource) UpdateAccountTenantRole(w http.ResponseWriter, r *http.Request) {
	accountID, schoolID, ok := parseAccountTenantParams(w, r)
	if !ok {
		return
	}
	req := &updateAccountTenantRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, rs.responses.InvalidRequest(err))
		return
	}
	entries, err := rs.identity.UpdateAccountTenantRole(
		r.Context(), accountID, schoolID, req.RoleID.Int64(), operatorIDFromContext(r), clientIP(r),
	)
	rs.respondAccess(w, r, entries, err, http.StatusOK, "School role updated successfully")
}

// RevokeAccountTenantAccess handles DELETE /operator/accounts/{accountId}/tenants/{tenantId}.
func (rs *Resource) RevokeAccountTenantAccess(w http.ResponseWriter, r *http.Request) {
	accountID, schoolID, ok := parseAccountTenantParams(w, r)
	if !ok {
		return
	}
	entries, err := rs.identity.RevokeAccountTenantAccess(
		r.Context(), accountID, schoolID, operatorIDFromContext(r), clientIP(r),
	)
	rs.respondAccess(w, r, entries, err, http.StatusOK, "School access revoked successfully")
}

func (rs *Resource) respondAccess(w http.ResponseWriter, r *http.Request, entries []identityaccess.AccountTenantAccess, err error, status int, message string) {
	if err != nil {
		common.RenderError(w, r, rs.accessError(err))
		return
	}
	common.Respond(w, r, status, accessEntries(entries), message)
}

// accessEntries keeps the wire shape: an empty listing and an entry
// without roles render empty lists, never null.
func accessEntries(entries []identityaccess.AccountTenantAccess) []AccountTenantAccessEntry {
	result := make([]AccountTenantAccessEntry, 0, len(entries))
	for _, entry := range entries {
		roles := make([]AccountTenantRole, 0, len(entry.Roles))
		for _, role := range entry.Roles {
			roles = append(roles, AccountTenantRole{ID: role.ID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole})
		}
		result = append(result, AccountTenantAccessEntry{
			TenantID: entry.TenantID, SchoolName: entry.SchoolName, SchoolSlug: entry.SchoolSlug, SchoolActive: entry.SchoolActive,
			OrganizationID: entry.OrganizationID, OrganizationName: entry.OrganizationName, Status: entry.Status,
			ActivatedAt: entry.ActivatedAt, DeactivatedAt: entry.DeactivatedAt,
			HasPerson: entry.HasPerson, HasStaff: entry.HasStaff, Roles: roles,
		})
	}
	return result
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

// clientIP renders the request address for the operator audit ledger; an
// absent address stays empty, as the ledger always stored it.
func clientIP(r *http.Request) string {
	if ip := common.ParseClientIP(r); ip != nil {
		return ip.String()
	}
	return ""
}
