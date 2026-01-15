package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
)

// Role Management Endpoints

// createRole handles creating a new role
func (rs *Resource) createRole(w http.ResponseWriter, r *http.Request) {
	req := &CreateRoleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	role, err := rs.AuthService.CreateRole(r.Context(), req.Name, req.Description)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	resp := &RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		CreatedAt:   role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
	}

	common.Respond(w, r, http.StatusCreated, resp, "Role created successfully")
}

// getRoleByID handles getting a role by ID
func (rs *Resource) getRoleByID(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseIntIDWithError(w, r, "id", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	role, err := rs.AuthService.GetRoleByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("role not found")))
		return
	}

	// Get permissions for the role
	permissions, _ := rs.AuthService.GetRolePermissions(r.Context(), id)
	permissionNames := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionNames = append(permissionNames, perm.GetFullName())
	}

	resp := &RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		CreatedAt:   role.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
		Permissions: permissionNames,
	}

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
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	role, err := rs.AuthService.GetRoleByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("role not found")))
		return
	}

	role.Name = req.Name
	role.Description = req.Description

	if err := rs.AuthService.UpdateRole(r.Context(), role); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
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
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		resp := &RoleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			CreatedAt:   role.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   role.UpdatedAt.Format(time.RFC3339),
		}
		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Roles retrieved successfully")
}
