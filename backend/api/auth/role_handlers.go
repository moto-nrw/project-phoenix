package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// CreateRoleRequest represents the create role request payload
type CreateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Bind validates the create role request
func (req *CreateRoleRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
	)
}

// UpdateRoleRequest represents the update role request payload
type UpdateRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Bind validates the update role request
func (req *UpdateRoleRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
	)
}

// RoleResponse represents a role response
type RoleResponse struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Permissions []string `json:"permissions,omitempty"`
}

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

// assignRoleToAccount handles assigning a role to an account
func (rs *Resource) assignRoleToAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	if err := rs.AuthService.AssignRoleToAccount(r.Context(), accountID, roleID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// removeRoleFromAccount handles removing a role from an account
func (rs *Resource) removeRoleFromAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	roleID, ok := common.ParseIntIDWithError(w, r, "roleId", common.MsgInvalidRoleID)
	if !ok {
		return
	}

	if err := rs.AuthService.RemoveRoleFromAccount(r.Context(), accountID, roleID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getAccountRoles handles getting roles for an account
func (rs *Resource) getAccountRoles(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	roles, err := rs.AuthService.GetAccountRoles(r.Context(), accountID)
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

	common.Respond(w, r, http.StatusOK, responses, "Account roles retrieved successfully")
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
