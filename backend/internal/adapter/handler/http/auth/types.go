package auth

import (
	"errors"
	"net/http"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
)

// errPasswordsNotMatch is the error message for password mismatch
const errPasswordsNotMatch = "passwords do not match"

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Bind validates the login request
func (req *LoginRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Password, validation.Required),
	)
}

// TokenResponse represents the token response payload
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RegisterRequest represents the register request payload
type RegisterRequest struct {
	Email           string `json:"email"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	RoleID          *int64 `json:"role_id,omitempty"` // Optional role assignment
}

// Bind validates the register request
func (req *RegisterRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Username, validation.Required, validation.Length(3, 30)),
		validation.Field(&req.Password, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.Password != req.ConfirmPassword {
				return errors.New(errPasswordsNotMatch)
			}
			return nil
		})),
	)
}

// AccountResponse represents the account response payload
type AccountResponse struct {
	ID          int64    `json:"id"`
	Email       string   `json:"email"`
	Username    string   `json:"username,omitempty"`
	Active      bool     `json:"active"`
	Roles       []string `json:"roles,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

// ChangePasswordRequest represents the change password request payload
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the change password request
func (req *ChangePasswordRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.CurrentPassword, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New(errPasswordsNotMatch)
			}
			return nil
		})),
	)
}

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
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	PermissionIDs []int64 `json:"permission_ids,omitempty"`
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

// RoleResponse represents the role response payload
type RoleResponse struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

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

// PasswordResetRequest represents the password reset request payload
type PasswordResetRequest struct {
	Email string `json:"email"`
}

// Bind validates the password reset request
func (req *PasswordResetRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
	)
}

// PasswordResetConfirmRequest represents the password reset confirm request payload
type PasswordResetConfirmRequest struct {
	Token           string `json:"token"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the password reset confirm request
func (req *PasswordResetConfirmRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Token, validation.Required),
		validation.Field(&req.NewPassword, validation.Required, validation.Length(8, 0)),
		validation.Field(&req.ConfirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if req.NewPassword != req.ConfirmPassword {
				return errors.New(errPasswordsNotMatch)
			}
			return nil
		})),
	)
}
