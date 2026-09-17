package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The role and permission routes call the Identity & Access role
// administration directly (#3314).

// toRoleResponse maps a role to its API response representation.
func toRoleResponse(role identityaccess.Role) *RoleResponse {
	return &RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		BaseRole:    role.BaseRole,
		CreatedAt:   role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
	}
}

// toPermissionResponse maps a permission to its API response representation.
func toPermissionResponse(permission identityaccess.Permission) *PermissionResponse {
	return &PermissionResponse{
		ID:          permission.ID,
		Name:        permission.Name,
		Description: permission.Description,
		Resource:    permission.Resource,
		Action:      permission.Action,
		CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
	}
}

func toPermissionResponses(permissions []identityaccess.Permission) []*PermissionResponse {
	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		responses = append(responses, toPermissionResponse(permission))
	}
	return responses
}

// createRole handles creating a new role
func (rs *Resource) createRole(w http.ResponseWriter, r *http.Request) {
	req := &CreateRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	role, err := rs.Sessions.CreateRole(r.Context(), req.Name, req.Description, req.BaseRole)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, toRoleResponse(role), "Role created successfully")
}

// getRoleByID handles getting a role by ID
func (rs *Resource) getRoleByID(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	role, err := rs.Sessions.GetRole(r.Context(), int64(id))
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("role not found")))
		return
	}

	// Get permissions for the role
	permissions, _ := rs.Sessions.GetRolePermissions(r.Context(), int64(id))
	permissionNames := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionNames = append(permissionNames, perm.FullName())
	}

	resp := toRoleResponse(role)
	resp.Permissions = permissionNames

	common.Respond(w, r, http.StatusOK, resp, "Role retrieved successfully")
}

// updateRole handles updating a role
func (rs *Resource) updateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	req := &UpdateRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	role, err := rs.Sessions.GetRole(r.Context(), int64(id))
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("role not found")))
		return
	}

	role.Name = req.Name
	role.Description = req.Description
	// Preserve existing base_role when the caller omits the field.
	// System roles must never carry a base_role — ignore silently.
	if req.BaseRole != nil && !role.IsSystem {
		role.BaseRole = req.BaseRole
	}

	if err := rs.Sessions.UpdateRole(r.Context(), role); err != nil {
		common.RenderError(w, r, renderRoleMutationError(err))
		return
	}

	common.RespondNoContent(w, r)
}

// deleteRole handles deleting a role
func (rs *Resource) deleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	if err := rs.Sessions.DeleteRole(r.Context(), int64(id)); err != nil {
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Rolle kann nicht gelöscht werden: Rolle ist aktuell Konten zugewiesen"))
			return
		}
		common.RenderError(w, r, renderRoleMutationError(err))
		return
	}

	common.RespondNoContent(w, r)
}

// renderRoleMutationError maps role administration errors to appropriate HTTP responses.
func renderRoleMutationError(err error) render.Renderer {
	var roleErr *identityaccess.AuthenticationError
	if errors.As(err, &roleErr) {
		if errors.Is(roleErr.Err, identityaccess.ErrSystemRoleImmutable) {
			return common.ErrorForbidden(roleErr.Err)
		}
		if errors.Is(roleErr.Err, identityaccess.ErrRoleNotFound) {
			return common.ErrorNotFound(roleErr.Err)
		}
		if errors.Is(roleErr.Err, identityaccess.ErrPermissionNotFound) {
			return common.ErrorNotFound(roleErr.Err)
		}
	}
	// Store failures that carry sql.ErrNoRows (an update that matched no
	// tenant-owned row) → 404
	if errors.Is(err, sql.ErrNoRows) {
		return common.ErrorNotFound(errors.New("role not found"))
	}
	return common.ErrorInternalServer(err)
}

// accountRoleErrorRenderer renders the account reads and grants of the role
// administration: an unknown or unmanageable account is 404.
var accountRoleErrorRenderer = common.UnwrapRenderer[*identityaccess.AuthenticationError](
	[]common.ErrorRule{
		{Target: identityaccess.ErrAccountNotFound, Render: common.ErrorNotFound},
	},
	common.ErrorInternalServer,
)

// listRoles handles listing roles
func (rs *Resource) listRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := rs.Sessions.ListRoles(r.Context(), identityaccess.RoleFilter{Name: r.URL.Query().Get("name")})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		responses = append(responses, toRoleResponse(role))
	}

	common.Respond(w, r, http.StatusOK, responses, "Roles retrieved successfully")
}

// getAccountRoles handles getting roles for an account
func (rs *Resource) getAccountRoles(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	roles, err := rs.Sessions.GetAccountRoles(r.Context(), int64(accountID))
	if err != nil {
		common.RenderError(w, r, accountRoleErrorRenderer(err))
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		responses = append(responses, toRoleResponse(role))
	}

	common.Respond(w, r, http.StatusOK, responses, "Account roles retrieved successfully")
}

// assignRoleToAccount applies the same school-role assignment policy as
// invitations and registration before creating an account-role mapping. The
// guardian invitation flow remains the only path that may grant guardian
// access.
func (rs *Resource) assignRoleToAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}
	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	requestedRoleID := int64(roleID)
	approvedRoleID, _, abort := rs.authorizeRoleAssignment(w, r, &requestedRoleID)
	if abort {
		return
	}

	if err := rs.Sessions.AssignRoleToAccount(r.Context(), int64(accountID), *approvedRoleID); err != nil {
		rs.renderAccountRoleMutationError(w, r, err)
		return
	}

	common.RespondNoContent(w, r)
}

// replaceAccountRole makes one approved target role the account's only staff
// role at this school; guardian access stays. The role administration
// executes assignment and removals in the request transaction.
func (rs *Resource) replaceAccountRole(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}
	req := &ReplaceAccountRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	requestedRoleID := req.RoleID.Int64()
	approvedRoleID, _, abort := rs.authorizeRoleAssignment(w, r, &requestedRoleID)
	if abort {
		return
	}
	if err := rs.Sessions.ReplaceAccountRole(r.Context(), int64(accountID), *approvedRoleID); err != nil {
		rs.renderAccountRoleMutationError(w, r, err)
		return
	}

	common.RespondNoContent(w, r)
}

func (rs *Resource) renderAccountRoleMutationError(w http.ResponseWriter, r *http.Request, err error) {
	// The mutation may have written before a later identity or removal step
	// fails. The request transaction otherwise commits every non-5xx response.
	tenant.MarkRollback(r.Context())
	for _, policyErr := range []error{
		identityaccess.ErrRoleLehrkraftCaregiverProfile,
		identityaccess.ErrRoleCaregiverNeedsProfile,
		identityaccess.ErrLehrkraftRoleImmutable,
	} {
		if errors.Is(err, policyErr) {
			common.RenderError(w, r, common.ErrorConflict(policyErr))
			return
		}
	}
	if identityaccess.IsSchoolIdentityRequestError(err) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.RenderError(w, r, accountRoleErrorRenderer(err))
}

// The pass-through account and role mutations read the capability at request
// time, so a resource composed without it still builds its router.

func (rs *Resource) removeRoleFromAccount(ctx context.Context, accountID, roleID int64) error {
	return rs.Sessions.RemoveRoleFromAccount(ctx, accountID, roleID)
}

func (rs *Resource) grantPermissionToAccount(ctx context.Context, accountID, permissionID int64) error {
	return rs.Sessions.GrantPermissionToAccount(ctx, accountID, permissionID)
}

func (rs *Resource) denyPermissionToAccount(ctx context.Context, accountID, permissionID int64) error {
	return rs.Sessions.DenyPermissionToAccount(ctx, accountID, permissionID)
}

func (rs *Resource) removePermissionFromAccount(ctx context.Context, accountID, permissionID int64) error {
	return rs.Sessions.RemovePermissionFromAccount(ctx, accountID, permissionID)
}

func (rs *Resource) assignPermissionToRole(ctx context.Context, roleID, permissionID int64) error {
	return rs.Sessions.AssignPermissionToRole(ctx, roleID, permissionID)
}

func (rs *Resource) removePermissionFromRole(ctx context.Context, roleID, permissionID int64) error {
	return rs.Sessions.RemovePermissionFromRole(ctx, roleID, permissionID)
}

// createPermission handles creating a new permission
func (rs *Resource) createPermission(w http.ResponseWriter, r *http.Request) {
	req := &CreatePermissionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var permission identityaccess.Permission
	err := tenant.WithAdminTx(r.Context(), rs.db, func(ctx context.Context, _ bun.Tx) error {
		var createErr error
		permission, createErr = rs.Sessions.CreatePermission(ctx, req.Name, req.Description, req.Resource, req.Action)
		return createErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, toPermissionResponse(permission), "Permission created successfully")
}

// getPermissionByID handles getting a permission by ID
func (rs *Resource) getPermissionByID(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	permission, err := rs.Sessions.GetPermission(r.Context(), int64(id))
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("permission not found")))
		return
	}

	common.Respond(w, r, http.StatusOK, toPermissionResponse(permission), "Permission retrieved successfully")
}

// updatePermission handles updating a permission
func (rs *Resource) updatePermission(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	req := &UpdatePermissionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	permission, err := rs.Sessions.GetPermission(r.Context(), int64(id))
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("permission not found")))
		return
	}

	permission.Name = req.Name
	permission.Description = req.Description
	permission.Resource = req.Resource
	permission.Action = req.Action

	if err := tenant.WithAdminTx(r.Context(), rs.db, func(ctx context.Context, _ bun.Tx) error {
		return rs.Sessions.UpdatePermission(ctx, permission)
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// deletePermission handles deleting a permission
func (rs *Resource) deletePermission(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := tenant.WithAdminTx(r.Context(), rs.db, func(ctx context.Context, _ bun.Tx) error {
		return rs.Sessions.DeletePermission(ctx, int64(id))
	}); err != nil {
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Berechtigung kann nicht gelöscht werden: Berechtigung ist aktuell Rollen oder Konten zugewiesen"))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// listPermissions handles listing permissions
func (rs *Resource) listPermissions(w http.ResponseWriter, r *http.Request) {
	filter := identityaccess.PermissionFilter{
		Resource: r.URL.Query().Get("resource"),
		Action:   r.URL.Query().Get("action"),
	}

	permissions, err := rs.Sessions.ListPermissions(r.Context(), filter)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, toPermissionResponses(permissions), "Permissions retrieved successfully")
}

// getAccountPermissions handles getting permissions for an account
func (rs *Resource) getAccountPermissions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.Sessions.GetAccountPermissions(r.Context(), int64(accountID))
	if err != nil {
		common.RenderError(w, r, accountRoleErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, toPermissionResponses(permissions), "Account permissions retrieved successfully")
}

// getAccountDirectPermissions handles getting only direct permissions for an account (not role-based)
func (rs *Resource) getAccountDirectPermissions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.Sessions.GetAccountDirectPermissions(r.Context(), int64(accountID))
	if err != nil {
		common.RenderError(w, r, accountRoleErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, toPermissionResponses(permissions), "Account direct permissions retrieved successfully")
}

// getRolePermissions handles getting permissions for a role
func (rs *Resource) getRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	permissions, err := rs.Sessions.GetRolePermissions(r.Context(), int64(roleID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, toPermissionResponses(permissions), "Role permissions retrieved successfully")
}

// replaceRolePermissions applies a complete permission selection atomically.
func (rs *Resource) replaceRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}
	req := &ReplaceRolePermissionsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	permissionIDs := make([]int64, len(req.PermissionIDs))
	for i, permissionID := range req.PermissionIDs {
		permissionIDs[i] = permissionID.Int64()
	}
	if err := rs.Sessions.ReplaceRolePermissions(r.Context(), int64(roleID), permissionIDs); err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, renderRoleMutationError(err))
		return
	}

	common.RespondNoContent(w, r)
}
