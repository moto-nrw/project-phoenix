package audit

import (
	"errors"
	"time"

	"github.com/uptrace/bun"
)

// AuthEvent represents an authentication event for security auditing
type AuthEvent struct {
	ID           int64                  `bun:"id,pk,autoincrement" json:"id"`
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
	EventTypeLogin         = "login"
	EventTypeLogout        = "logout"
	EventTypeTokenRefresh  = "token_refresh"
	EventTypeTokenExpired  = "token_expired"
	EventTypePasswordReset = "password_reset"
	EventTypeAccountLocked = "account_locked"
)

// BeforeAppendModel sets the correct table expression
func (ae *AuthEvent) BeforeAppendModel(query any) error {
	switch q := query.(type) {
	case *bun.SelectQuery:
		q.ModelTableExpr(`audit.auth_events AS "auth_event"`)
	case *bun.UpdateQuery:
		q.ModelTableExpr("audit.auth_events")
	case *bun.DeleteQuery:
		q.ModelTableExpr("audit.auth_events")
	}
	return nil
}

// TableName returns the database table name
func (ae *AuthEvent) TableName() string {
	return "audit.auth_events"
}

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
	case EventTypeLogin, EventTypeLogout, EventTypeTokenRefresh,
		EventTypeTokenExpired, EventTypePasswordReset, EventTypeAccountLocked:
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

// GetID implements the base.Entity interface
func (ae *AuthEvent) GetID() interface{} {
	return ae.ID
}

// GetCreatedAt implements the base.Entity interface
func (ae *AuthEvent) GetCreatedAt() time.Time {
	return ae.CreatedAt
}

// GetUpdatedAt implements the base.Entity interface
func (ae *AuthEvent) GetUpdatedAt() time.Time {
	return ae.CreatedAt
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
