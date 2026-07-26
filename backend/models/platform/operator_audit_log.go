package platform

import (
	"encoding/json"
	"net"
	"time"
)

// Common audit action constants
const (
	ActionCreate               = "create"
	ActionUpdate               = "update"
	ActionDelete               = "delete"
	ActionStatusChange         = "status_change"
	ActionPublish              = "publish"
	ActionLogin                = "login"
	ActionAddComment           = "add_comment"
	ActionDeleteComment        = "delete_comment"
	ActionRotateAPIKey         = "rotate_api_key"
	ActionHidePost             = "hide_post"
	ActionUnhidePost           = "unhide_post"
	ActionDeletePost           = "delete_post"
	ActionSoftDelete           = "soft_delete"
	ActionRestore              = "restore"
	ActionEmailChangeInitiated = "email_change_initiated"
	ActionEmailChangeConfirmed = "email_change_confirmed"
	ActionInvitationCreated    = "invitation_created"
	ActionInvitationAccepted   = "invitation_accepted"
	ActionInvitationRevoked    = "invitation_revoked"
	ActionInvitationResent     = "invitation_resent"

	// MFA action constants (issue #1308 phase 7b). Mirror the auth-side
	// EventTypeMFA* values but live in the operator audit log.
	ActionMFAEmailSent          = "mfa_email_sent"
	ActionMFAVerified           = "mfa_verified"
	ActionMFAFailed             = "mfa_failed"
	ActionMFALocked             = "mfa_locked"
	ActionMFAEnrolled           = "mfa_enrolled"
	ActionMFADisabled           = "mfa_disabled"
	ActionMFATrustedDeviceAdded = "mfa_trusted_device_added"
	// ActionMFAAdminOverride records an operator setting/clearing the
	// account-wide ("Notfall") MFA override. This is a platform-scope
	// action with no single tenant, so it lands here in the operator
	// audit log rather than the tenant-scoped audit.auth_events.
	ActionMFAAdminOverride = "mfa_admin_override"
)

// Common resource type constants
const (
	ResourceAnnouncement = "announcement"
	ResourceSuggestion   = "suggestion"
	ResourceComment      = "operator_comment"
	ResourceOperator     = "operator"
	ResourceOrganization = "organization"
	ResourceSchool       = "school"
	ResourceInvitation   = "invitation"
	ResourceDevice       = "device"
	ResourceAccount      = "account"
	ResourcePerson       = "person"
	ResourceOperatorMFA  = "operator_mfa"
)

// OperatorAuditLog tracks operator actions for auditing
type OperatorAuditLog struct {
	ID           int64           `bun:"id,pk,autoincrement" json:"id"`
	OperatorID   int64           `bun:"operator_id,notnull" json:"operator_id"`
	Action       string          `bun:"action,notnull" json:"action"`
	ResourceType string          `bun:"resource_type,notnull" json:"resource_type"`
	ResourceID   *int64          `bun:"resource_id" json:"resource_id,omitempty"`
	Changes      json.RawMessage `bun:"changes,type:jsonb" json:"changes,omitempty"`
	RequestIP    net.IP          `bun:"request_ip,type:inet,nullzero" json:"request_ip,omitempty"`
	CreatedAt    time.Time       `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`

	// Relations
}

// SetChanges sets the changes field from a map
func (l *OperatorAuditLog) SetChanges(changes map[string]any) error {
	data, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	l.Changes = data
	return nil
}

// GetChanges parses the changes field into a map
func (l *OperatorAuditLog) GetChanges() (map[string]any, error) {
	if l.Changes == nil {
		return nil, nil
	}
	var changes map[string]any
	if err := json.Unmarshal(l.Changes, &changes); err != nil {
		return nil, err
	}
	return changes, nil
}
