package auth

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// toRoleResponse maps a domain Role to its API response representation.
func toRoleResponse(role *authModel.Role) *RoleResponse {
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

// Permission Management Request/Response Types

// createRole handles creating a new role
func (rs *Resource) createRole(w http.ResponseWriter, r *http.Request) {
	req := &CreateRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	role, err := rs.AuthService.CreateRole(r.Context(), req.Name, req.Description, req.BaseRole)
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

	role, err := rs.AuthService.GetRoleByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("role not found")))
		return
	}

	// Get permissions for the role
	permissions, _ := rs.AuthService.GetRolePermissions(r.Context(), id)
	permissionNames := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionNames = append(permissionNames, perm.GetFullName())
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

	role, err := rs.AuthService.GetRoleByID(r.Context(), id)
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

	if err := rs.AuthService.UpdateRole(r.Context(), role); err != nil {
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

	if err := rs.AuthService.DeleteRole(r.Context(), id); err != nil {
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Rolle kann nicht gelöscht werden: Rolle ist aktuell Konten zugewiesen"))
			return
		}
		common.RenderError(w, r, renderRoleMutationError(err))
		return
	}

	common.RespondNoContent(w, r)
}

// renderRoleMutationError maps service-layer role errors to appropriate HTTP responses.
func renderRoleMutationError(err error) render.Renderer {
	var authErr *authService.AuthError
	if errors.As(err, &authErr) {
		if errors.Is(authErr.Err, authService.ErrSystemRoleImmutable) {
			return common.ErrorForbidden(authErr.Err)
		}
		if errors.Is(authErr.Err, authService.ErrRoleNotFound) {
			return common.ErrorNotFound(authErr.Err)
		}
	}
	// FindByID failures (sql.ErrNoRows wrapped in DatabaseError) → 404
	if errors.Is(err, sql.ErrNoRows) {
		return common.ErrorNotFound(errors.New("role not found"))
	}
	return common.ErrorInternalServer(err)
}

// listRoles handles listing roles
func (rs *Resource) listRoles(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if name := r.URL.Query().Get("name"); name != "" {
		filters["name"] = name
	}

	roles, err := rs.AuthService.ListRoles(r.Context(), filters)
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

	roles, err := rs.AuthService.GetAccountRoles(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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

	if err := rs.AuthService.AssignRoleToAccount(r.Context(), accountID, int(*approvedRoleID)); err != nil {
		// Caller mistake, not a server fault: the caregiver profile and the
		// Lehrkraft role exclude each other in both directions (#1772) —
		// surface the German policy message instead of a 500.
		for _, policyErr := range []error{
			authService.ErrRoleLehrkraftCaregiverProfile,
			authService.ErrRoleCaregiverNeedsProfile,
		} {
			if errors.Is(err, policyErr) {
				common.RenderError(w, r, common.ErrorConflict(policyErr))
				return
			}
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// createPermission handles creating a new permission
func (rs *Resource) createPermission(w http.ResponseWriter, r *http.Request) {
	req := &CreatePermissionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	permission, err := rs.AuthService.CreatePermission(r.Context(), req.Name, req.Description, req.Resource, req.Action)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	resp := &PermissionResponse{
		ID:          permission.ID,
		Name:        permission.Name,
		Description: permission.Description,
		Resource:    permission.Resource,
		Action:      permission.Action,
		CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
	}

	common.Respond(w, r, http.StatusCreated, resp, "Permission created successfully")
}

// getPermissionByID handles getting a permission by ID
func (rs *Resource) getPermissionByID(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	permission, err := rs.AuthService.GetPermissionByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("permission not found")))
		return
	}

	resp := &PermissionResponse{
		ID:          permission.ID,
		Name:        permission.Name,
		Description: permission.Description,
		Resource:    permission.Resource,
		Action:      permission.Action,
		CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
	}

	common.Respond(w, r, http.StatusOK, resp, "Permission retrieved successfully")
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

	permission, err := rs.AuthService.GetPermissionByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("permission not found")))
		return
	}

	permission.Name = req.Name
	permission.Description = req.Description
	permission.Resource = req.Resource
	permission.Action = req.Action

	if err := rs.AuthService.UpdatePermission(r.Context(), permission); err != nil {
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

	if err := rs.AuthService.DeletePermission(r.Context(), id); err != nil {
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
	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if resource := r.URL.Query().Get("resource"); resource != "" {
		filters["resource"] = resource
	}

	if action := r.URL.Query().Get("action"); action != "" {
		filters["action"] = action
	}

	permissions, err := rs.AuthService.ListPermissions(r.Context(), filters)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Permissions retrieved successfully")
}

// getAccountPermissions handles getting permissions for an account
func (rs *Resource) getAccountPermissions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetAccountPermissions(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Account permissions retrieved successfully")
}

// getAccountDirectPermissions handles getting only direct permissions for an account (not role-based)
func (rs *Resource) getAccountDirectPermissions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetAccountDirectPermissions(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Account direct permissions retrieved successfully")
}

// getRolePermissions handles getting permissions for a role
func (rs *Resource) getRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetRolePermissions(r.Context(), roleID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	responses := make([]*PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		resp := &PermissionResponse{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
			Resource:    permission.Resource,
			Action:      permission.Action,
			CreatedAt:   permission.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   permission.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Role permissions retrieved successfully")
}

// Account Management Extension Endpoints
