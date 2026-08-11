package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
)

const errPasswordsNotMatch = "passwords do not match"

// TenantResolveResponse represents the public tenant info returned by resolve
type TenantResolveResponse struct {
	TenantID         int64           `json:"tenant_id"`
	Slug             string          `json:"slug"`
	Name             string          `json:"name"`
	Subdomain        string          `json:"subdomain"`
	OrganizationID   int64           `json:"organization_id"`
	OrganizationName string          `json:"organization_name"`
	Hidden           bool            `json:"hidden"`
	Settings         json.RawMessage `json:"settings"`
	// PresenceMode is the tenant's resolved operations.presence_mode setting
	// ("detailed" | "binary"). Included so the frontend can render the right
	// badge component (PresenceBadge vs LocationBadge) without a second call.
	// Defaults to "detailed" if SettingsService is unavailable.
	PresenceMode string `json:"presence_mode"`
	// StudentPhotosEnabled is the tenant's resolved
	// operations.student_photos_enabled setting. Surfaced here (rather than
	// via /api/settings/schema) because every authenticated user — including
	// non-admin Betreuer who don't carry config:read — needs the flag to
	// decide whether to render avatars on student cards. Defaults to false
	// when the setting is missing or unresolvable.
	StudentPhotosEnabled bool `json:"student_photos_enabled"`
	// NFCEnabled is the tenant's resolved attendance.nfc_enabled setting.
	// It is public tenant-shell metadata because non-admin staff also need to
	// know whether NFC-only areas like classic activities should appear.
	NFCEnabled bool `json:"nfc_enabled"`
	// ParentMessagingEnabled is the tenant's resolved operations.parent_notes_enabled
	// setting. Surfaced here (like the photo flag) so non-admin staff — who don't
	// carry config:read — can hide the "Neue Nachricht" compose entry points when
	// the school has parent-OGS messaging turned off, instead of composing into a
	// 403 dead-end. Defaults to false when the setting is missing/unresolvable.
	ParentMessagingEnabled bool `json:"parent_messaging_enabled"`
	// DisplayEnabled is the tenant's resolved display.enabled setting. The
	// Info-Point Dashboard is opt-in and defaults off, so the frontend needs
	// this to hide the sidebar entry / admin page for schools that haven't
	// turned it on. Defaults to false when the setting is missing/unresolvable.
	DisplayEnabled bool `json:"display_enabled"`
	// GradeLevelMax is the tenant's enrollment.grade_level_max setting.
	// Timetable users need it even without config:read, so it belongs to the
	// same public tenant-shell metadata contract as the feature flags above.
	// Unlike optional display flags, this constraint fails closed: omitting or
	// inventing it would make enrollment forms disagree with server validation.
	GradeLevelMax int `json:"grade_level_max"`
	// CareOfferingsEnabled is shell metadata for approved-child offering
	// corrections. Missing metadata stays enabled for compatibility with older
	// backends; an explicit settings-resolution error fails closed so the UI
	// cannot offer a mutation that the authoritative service will reject.
	CareOfferingsEnabled bool `json:"care_offerings_enabled"`
	AttendanceWebEnabled bool `json:"attendance_web_enabled"`
	// AttendanceLogEnabled is the tenant's resolved gdpr.attendance_log_enabled
	// setting. Shell metadata like the flags above: non-admin staff (no
	// config:read) need it to know whether the Tagesauswertung / attendance
	// history areas exist at this school. Defaults to false (opt-in feature).
	AttendanceLogEnabled bool   `json:"attendance_log_enabled"`
	GroupMode            string `json:"group_mode"`
	ShowTimetableCounts  bool   `json:"show_timetable_counts"`
	WaitlistEnabled      bool   `json:"waitlist_enabled"`
}

type tenantShellSettings struct {
	presenceMode           string
	studentPhotosEnabled   bool
	nfcEnabled             bool
	parentMessagingEnabled bool
	displayEnabled         bool
	careOfferingsEnabled   bool
	attendanceWebEnabled   bool
	attendanceLogEnabled   bool
	groupMode              string
	showTimetableCounts    bool
	waitlistEnabled        bool
}

// SwitchTenantRequest represents the switch-tenant request payload
type SwitchTenantRequest struct {
	TenantSlug string `json:"tenant_slug"`
}

// Bind validates the switch-tenant request
func (req *SwitchTenantRequest) Bind(_ *http.Request) error {
	req.TenantSlug = strings.TrimSpace(req.TenantSlug)

	return validation.ValidateStruct(req,
		validation.Field(&req.TenantSlug, validation.Required),
	)
}

// AccountTenantResponse represents a tenant available to the authenticated user
type AccountTenantResponse struct {
	TenantID         int64  `json:"tenant_id"`
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Subdomain        string `json:"subdomain"`
	OrganizationID   int64  `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
}

// PublicTenantResponse is the public-facing tenant info returned by the
// unauthenticated /auth/tenants endpoint. It intentionally omits internal
// database IDs (tenant_id, organization_id) to prevent enumeration.
type PublicTenantResponse struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	Subdomain        string `json:"subdomain"`
	OrganizationName string `json:"organization_name"`
}

// LoginRequest represents the login request payload
type LoginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TenantSlug string `json:"tenant_slug,omitempty"`
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

// LoginResponse is the discriminated response shape /auth/login returns now
// that MFA can short-circuit the token pair. The `status` field tells the
// client which fields to read:
//
//   - status == "authenticated"           → access_token + refresh_token
//     populated; the frontend stores them as before.
//   - status == "mfa_required"             → challenge_token + masked_email
//     populated; the frontend renders the code-entry UI and POSTs to
//     /auth/mfa/verify.
//   - status == "mfa_enrollment_required"  → access_token carries a
//     short-lived enrollment-scoped JWT (no refresh_token). The frontend
//     uses it as the Authorization header for /auth/mfa/enroll/* and
//     replaces it with the real token pair returned by
//     /auth/mfa/enroll/confirm. The token is REJECTED by the regular
//     Authenticator middleware, so it cannot be used to bypass MFA.
//
// mfa_enrollment_required is retained as a legacy hint for clients that key
// off the boolean rather than the status discriminator; new code should
// branch on `status`.
type LoginResponse struct {
	Status                string `json:"status"`
	AccessToken           string `json:"access_token,omitempty"`
	RefreshToken          string `json:"refresh_token,omitempty"`
	ChallengeToken        string `json:"challenge_token,omitempty"`
	MaskedEmail           string `json:"masked_email,omitempty"`
	MFAEnrollmentRequired bool   `json:"mfa_enrollment_required,omitempty"`
	// TrustedDeviceEnabled is sent on the mfa_required branch so the
	// frontend can hide the "remember this device" checkbox when the
	// tenant admin has disabled the feature. A missing field defaults to
	// "enabled" on the client for backwards compatibility.
	TrustedDeviceEnabled *bool `json:"trusted_device_enabled,omitempty"`
	// TrustedDeviceDays mirrors security.mfa_trusted_device_days for the
	// tenant. The frontend uses it to render the dynamic "Auf diesem
	// Gerät N Tage merken" label so the checkbox always reflects the
	// cookie lifetime the backend will actually issue.
	TrustedDeviceDays *int `json:"trusted_device_days,omitempty"`
}

// RegisterRequest represents the register request payload
type RegisterRequest struct {
	Email           string `json:"email"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	RoleID          *int64 `json:"role_id,omitempty"` // Optional role assignment

	// Identity fields. Supplying them makes the account staff at the school in
	// the same transaction (#2222) instead of leaving that to follow-up
	// requests that may never arrive.
	FirstName string  `json:"first_name,omitempty"`
	LastName  string  `json:"last_name,omitempty"`
	TagID     *string `json:"tag_id,omitempty"`
}

// Bind validates the register request
func (req *RegisterRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	return validateAccountCredentialFields(req, &req.Email, &req.Username, &req.Password, &req.ConfirmPassword)
}

// validateAccountCredentialFields normalizes and validates the shared
// email/username/password(+confirmation) fields of the account-creation
// payloads (staff registration and parent accounts).
func validateAccountCredentialFields(structPtr any, email, username, password, confirmPassword *string) error {
	*email = strings.TrimSpace(strings.ToLower(*email))
	*username = strings.TrimSpace(*username)

	return validation.ValidateStruct(structPtr,
		validation.Field(email, validation.Required, is.Email),
		validation.Field(username, validation.Required, validation.Length(3, 30)),
		validation.Field(password, validation.Required, validation.Length(8, 0)),
		validation.Field(confirmPassword, validation.Required, validation.By(func(value interface{}) error {
			if *password != *confirmPassword {
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

	// SchoolIdentity reports what was provisioned alongside the account, so the
	// caller can attach staff details without looking the rows up again. Absent
	// when the request carried no identity fields.
	SchoolIdentity *SchoolIdentityResponse `json:"school_identity,omitempty"`
}

// SchoolIdentityResponse carries the ids of the person/staff chain provisioned
// for an account at the caller's school.
type SchoolIdentityResponse struct {
	PersonID  int64 `json:"person_id"`
	StaffID   int64 `json:"staff_id"`
	TeacherID int64 `json:"teacher_id,omitempty"`
}

// LinkToTenantRequest represents a request to link an existing account to the current tenant.
type LinkToTenantRequest struct {
	Email  string `json:"email"`
	RoleID *int64 `json:"role_id,omitempty"`

	// Identity fields, same meaning as on RegisterRequest (#2222).
	FirstName string  `json:"first_name,omitempty"`
	LastName  string  `json:"last_name,omitempty"`
	TagID     *string `json:"tag_id,omitempty"`
}

func (req *LinkToTenantRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
	)
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
	Name        string  `json:"name"`
	Description string  `json:"description"`
	BaseRole    *string `json:"base_role,omitempty"`
}

// Bind validates the create role request
func (req *CreateRoleRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	// Empty/blank base_role trims to nil, treated as "field not sent" (preserve
	// existing value): once set, a base_role cannot be cleared back to NULL via the API.
	req.BaseRole = strutil.TrimPtrToNil(req.BaseRole)

	if err := validateBaseRole(req.BaseRole); err != nil {
		return err
	}

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
	)
}

// UpdateRoleRequest represents the update role request payload
type UpdateRoleRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	BaseRole    *string `json:"base_role,omitempty"`
}

// Bind validates the update role request.
// BaseRole is optional on update — if omitted, the handler preserves the existing value.
func (req *UpdateRoleRequest) Bind(_ *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.BaseRole = strutil.TrimPtrToNil(req.BaseRole)

	// Only validate base_role if the caller explicitly sent one.
	if req.BaseRole != nil {
		if err := validateBaseRoleValue(*req.BaseRole); err != nil {
			return err
		}
	}

	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&req.Description, validation.Length(0, 500)),
	)
}

// validateBaseRoleValue checks that a non-nil base_role value is one of the allowed system roles.
func validateBaseRoleValue(value string) error {
	if !slices.Contains(authModel.ValidBaseRoles(), value) {
		return fmt.Errorf("base_role must be one of: %v", authModel.ValidBaseRoles())
	}
	return nil
}

// validateBaseRole checks that base_role is provided and is one of the allowed system roles.
// This runs at the request layer to return 400 (not 500) for invalid payloads.
func validateBaseRole(br *string) error {
	if br == nil {
		return fmt.Errorf("base_role is required; must be one of: %v", authModel.ValidBaseRoles())
	}
	return validateBaseRoleValue(*br)
}

// RoleResponse represents a role response
type RoleResponse struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsSystem    bool     `json:"is_system"`
	BaseRole    *string  `json:"base_role,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Permissions []string `json:"permissions,omitempty"`
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

// UpdatePermissionRequest represents the update permission request payload.
// Field set, JSON tags, and Bind validation are identical to the create
// payload, so it is a type alias.
type UpdatePermissionRequest = CreatePermissionRequest

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

// UpdateAccountRequest represents the update account request payload
type UpdateAccountRequest struct {
	Email    string `json:"email"`
	Username string `json:"username,omitempty"`
}

type EnableCaregiverCapabilityRequest struct {
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	Position  string `json:"position,omitempty"`
}

// Bind validates the update account request
func (req *UpdateAccountRequest) Bind(_ *http.Request) error {
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(req.Username)

	return validation.ValidateStruct(req,
		validation.Field(&req.Email, validation.Required, is.Email),
		validation.Field(&req.Username, validation.Length(3, 30)),
	)
}

func (req *EnableCaregiverCapabilityRequest) Bind(_ *http.Request) error {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Position = strings.TrimSpace(req.Position)
	return nil
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

// CreateParentAccountRequest represents the create parent account request payload
type CreateParentAccountRequest struct {
	Email           string `json:"email"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// Bind validates the create parent account request
func (req *CreateParentAccountRequest) Bind(_ *http.Request) error {
	return validateAccountCredentialFields(req, &req.Email, &req.Username, &req.Password, &req.ConfirmPassword)
}

// ParentAccountResponse represents a parent account response
type ParentAccountResponse struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username,omitempty"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Role Management Endpoints
