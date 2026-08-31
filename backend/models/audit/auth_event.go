package audit

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// AuthEvent represents an authentication event for security auditing
type AuthEvent struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	AccountID    int64                  `bun:"account_id,notnull" json:"account_id"`
	EventType    string                 `bun:"event_type,notnull" json:"event_type"`
	Success      bool                   `bun:"success,notnull" json:"success"`
	IPAddress    string                 `bun:"ip_address,notnull" json:"ip_address"`
	UserAgent    string                 `bun:"user_agent" json:"user_agent,omitempty"`
	ErrorMessage string                 `bun:"error_message" json:"error_message,omitempty"`
	Metadata     map[string]interface{} `bun:"metadata,type:jsonb" json:"metadata,omitempty"`
	CreatedAt    time.Time              `bun:"created_at,notnull,default:now()" json:"created_at"`
}

// EventType constants
const (
	EventTypeLogin                       = "login"
	EventTypeLogout                      = "logout"
	EventTypeTokenRefresh                = "token_refresh"
	EventTypeTokenExpired                = "token_expired"
	EventTypeTokenRevoked                = "token_revoked"
	EventTypePasswordReset               = "password_reset"
	EventTypeAccountLocked               = "account_locked"
	EventTypeTenantSwitch                = "tenant_switch"
	EventTypeCaregiverCapabilityEnabled  = "caregiver_capability_enabled"
	EventTypeCaregiverCapabilityDisabled = "caregiver_capability_disabled"

	// MFA event types (Phase 1 of issue #1308)
	EventTypeMFAEmailSent          = "mfa_email_sent"
	EventTypeMFAVerified           = "mfa_verified"
	EventTypeMFAFailed             = "mfa_failed"
	EventTypeMFALocked             = "mfa_locked"
	EventTypeMFARecoveryUsed       = "mfa_recovery_used"
	EventTypeMFADisabled           = "mfa_disabled"
	EventTypeMFATrustedDeviceAdded = "mfa_trusted_device_added"
	EventTypeMFAAdminOverride      = "mfa_admin_override"

	// Cross-tenant access management (issue #1021). Written against the
	// affected school so a tenant admin sees who was granted or revoked
	// access to their own school, even when an operator triggered it.
	EventTypeTenantAccessGranted = "tenant_access_granted"
	EventTypeTenantAccessRevoked = "tenant_access_revoked"
	EventTypeTenantRoleChanged   = "tenant_role_changed"
)

// Validate ensures auth event is valid
func (ae *AuthEvent) Validate() error {
	if ae.AccountID <= 0 {
		return errors.New("account ID is required")
	}

	if ae.EventType == "" {
		return errors.New("event type is required")
	}

	// Validate event type
	switch ae.EventType {
	case EventTypeLogin, EventTypeLogout, EventTypeTokenRefresh, EventTypeTokenRevoked,
		EventTypeTokenExpired, EventTypePasswordReset, EventTypeAccountLocked,
		EventTypeTenantSwitch, EventTypeCaregiverCapabilityEnabled,
		EventTypeCaregiverCapabilityDisabled,
		// MFA event types — were defined as constants but missing from this
		// allowlist, so every Create() call from mfa_service quietly bounced
		// with "invalid event type". The drop was masked by the previous
		// "no tenant context" short-circuit (Item #5 review). Now that the
		// service threads tenant_id through to the recorder, rows actually
		// hit the INSERT and the validation gate has to accept them.
		EventTypeMFAEmailSent, EventTypeMFAVerified, EventTypeMFAFailed,
		EventTypeMFALocked, EventTypeMFARecoveryUsed, EventTypeMFADisabled,
		EventTypeMFATrustedDeviceAdded, EventTypeMFAAdminOverride,
		EventTypeTenantAccessGranted, EventTypeTenantAccessRevoked,
		EventTypeTenantRoleChanged:
		// Valid types
	default:
		return errors.New("invalid event type")
	}

	if ae.IPAddress == "" {
		return errors.New("IP address is required")
	}

	if ae.CreatedAt.IsZero() {
		ae.CreatedAt = time.Now()
	}

	return nil
}

// GetMetadata returns the metadata map
func (ae *AuthEvent) GetMetadata() map[string]interface{} {
	if ae.Metadata == nil {
		ae.Metadata = make(map[string]interface{})
	}
	return ae.Metadata
}

// SetMetadata sets metadata information
func (ae *AuthEvent) SetMetadata(key string, value interface{}) {
	if ae.Metadata == nil {
		ae.Metadata = make(map[string]interface{})
	}
	ae.Metadata[key] = value
}

// NewAuthEvent creates a new auth event
func NewAuthEvent(accountID int64, eventType string, success bool, ipAddress string) *AuthEvent {
	return &AuthEvent{
		AccountID: accountID,
		EventType: eventType,
		Success:   success,
		IPAddress: ipAddress,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
}
