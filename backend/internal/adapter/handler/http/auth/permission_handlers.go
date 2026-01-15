package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
)

// createPermission handles creating a new permission
func (rs *Resource) createPermission(w http.ResponseWriter, r *http.Request) {
	req := &CreatePermissionRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	permission, err := rs.AuthService.CreatePermission(r.Context(), req.Name, req.Description, req.Resource, req.Action)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorNotFound(errors.New("permission not found")))
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
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	permission, err := rs.AuthService.GetPermissionByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("permission not found")))
		return
	}

	permission.Name = req.Name
	permission.Description = req.Description
	permission.Resource = req.Resource
	permission.Action = req.Action

	if err := rs.AuthService.UpdatePermission(r.Context(), permission); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorInternalServer(err))
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
		common.RenderError(w, r, ErrorInternalServer(err))
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

// assignPermissionToRole handles assigning a permission to a role
func (rs *Resource) assignPermissionToRole(w http.ResponseWriter, r *http.Request) {
	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	permissionID, ok := common.ParseIntIDWithError(w, r, "permissionId", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.AssignPermissionToRole(r.Context(), roleID, permissionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// removePermissionFromRole handles removing a permission from a role
func (rs *Resource) removePermissionFromRole(w http.ResponseWriter, r *http.Request) {
	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	permissionID, ok := common.ParseIntIDWithError(w, r, "permissionId", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.RemovePermissionFromRole(r.Context(), roleID, permissionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getRolePermissions handles getting permissions for a role
func (rs *Resource) getRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetRolePermissions(r.Context(), roleID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
