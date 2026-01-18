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

// CreatePermissionRequest represents the create permission request payload
type CreatePermissionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
}

// Bind validates the create permission request
func (req *CreatePermissionRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Resource = strings.TrimSpace(strings.ToLower(req.Resource))
	req.Action = strings.TrimSpace(strings.ToLower(req.Action))

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
		validation.Field(&req.Resource, validation.Required, validation.Length(1, 50)),
		validation.Field(&req.Action, validation.Required, validation.Length(1, 50)),
	)
}

// UpdatePermissionRequest represents the update permission request payload
type UpdatePermissionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
}

// Bind validates the update permission request
func (req *UpdatePermissionRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Resource = strings.TrimSpace(strings.ToLower(req.Resource))
	req.Action = strings.TrimSpace(strings.ToLower(req.Action))

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
		validation.Field(&req.Resource, validation.Required, validation.Length(1, 50)),
		validation.Field(&req.Action, validation.Required, validation.Length(1, 50)),
	)
}

// PermissionResponse represents a permission response
type PermissionResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

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

// grantPermissionToAccount handles granting a permission to an account
func (rs *Resource) grantPermissionToAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissionID, ok := common.ParseIntIDWithError(w, r, "permissionId", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.GrantPermissionToAccount(r.Context(), accountID, permissionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// denyPermissionToAccount handles denying a permission to an account
func (rs *Resource) denyPermissionToAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissionID, ok := common.ParseIntIDWithError(w, r, "permissionId", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.DenyPermissionToAccount(r.Context(), accountID, permissionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// removePermissionFromAccount handles removing a permission from an account
func (rs *Resource) removePermissionFromAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissionID, ok := common.ParseIntIDWithError(w, r, "permissionId", common.MsgInvalidPermissionID)
	if !ok {
		return
	}

	if err := rs.AuthService.RemovePermissionFromAccount(r.Context(), accountID, permissionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getAccountPermissions handles getting permissions for an account
func (rs *Resource) getAccountPermissions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	permissions, err := rs.AuthService.GetAccountPermissions(r.Context(), accountID)
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

	common.Respond(w, r, http.StatusOK, responses, "Account direct permissions retrieved successfully")
}
