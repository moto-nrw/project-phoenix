package config

import (
	"errors"
	"strconv"
	"time"
)

// tableActionAuditLog is the schema-qualified table name
const tableActionAuditLog = "config.action_audit_log"

// ActionAuditEntry represents an execution of an action
type ActionAuditEntry struct {
	// ID is the unique identifier
	ID int64 `bun:"id,pk,autoincrement" json:"id"`

	// ActionKey is the action definition key
	ActionKey string `bun:"action_key,notnull" json:"action_key"`

	// ExecutedByAccountID identifies who executed the action
	ExecutedByAccountID *int64 `bun:"executed_by_account_id" json:"executed_by_account_id,omitempty"`

	// ExecutedByName is denormalized for display (survives account deletion)
	ExecutedByName string `bun:"executed_by_name,notnull" json:"executed_by_name"`

	// ExecutedAt is when the action was executed
	ExecutedAt time.Time `bun:"executed_at,notnull,default:current_timestamp" json:"executed_at"`

	// DurationMs is how long the action took in milliseconds
	DurationMs *int64 `bun:"duration_ms" json:"duration_ms,omitempty"`

	// Success indicates if the action completed successfully
	Success bool `bun:"success,notnull" json:"success"`

	// ErrorMessage contains the error if the action failed
	ErrorMessage *string `bun:"error_message" json:"error_message,omitempty"`

	// ResultSummary contains a brief summary of the result
	ResultSummary *string `bun:"result_summary" json:"result_summary,omitempty"`

	// IPAddress is the request origin (optional)
	IPAddress *string `bun:"ip_address" json:"ip_address,omitempty"`

	// UserAgent is the request user agent (optional)
	UserAgent *string `bun:"user_agent" json:"user_agent,omitempty"`
}

// TableName returns the database table name
func (e *ActionAuditEntry) TableName() string {
	return tableActionAuditLog
}

// Validate ensures the audit entry data is valid
func (e *ActionAuditEntry) Validate() error {
	if e.ActionKey == "" {
		return errors.New("action_key is required")
	}
	if e.ExecutedByName == "" {
		return errors.New("executed_by_name is required")
	}
	return nil
}

// ActionAuditEntryDTO is the API response format for action audit entries
type ActionAuditEntryDTO struct {
	ID                  string  `json:"id"`
	ActionKey           string  `json:"actionKey"`
	ExecutedByAccountID *string `json:"executedByAccountId,omitempty"`
	ExecutedByName      string  `json:"executedByName"`
	ExecutedAt          string  `json:"executedAt"`
	DurationMs          *int64  `json:"durationMs,omitempty"`
	Success             bool    `json:"success"`
	ErrorMessage        *string `json:"errorMessage,omitempty"`
	ResultSummary       *string `json:"resultSummary,omitempty"`
	IPAddress           *string `json:"ipAddress,omitempty"`
}

// ToDTO converts an ActionAuditEntry to its DTO representation
func (e *ActionAuditEntry) ToDTO() *ActionAuditEntryDTO {
	dto := &ActionAuditEntryDTO{
		ID:             formatInt64(e.ID),
		ActionKey:      e.ActionKey,
		ExecutedByName: e.ExecutedByName,
		ExecutedAt:     e.ExecutedAt.Format(time.RFC3339),
		DurationMs:     e.DurationMs,
		Success:        e.Success,
		ErrorMessage:   e.ErrorMessage,
		ResultSummary:  e.ResultSummary,
		IPAddress:      e.IPAddress,
	}
	if e.ExecutedByAccountID != nil {
		accountID := formatInt64(*e.ExecutedByAccountID)
		dto.ExecutedByAccountID = &accountID
	}
	return dto
}

// formatInt64 converts int64 to string for JSON serialization
func formatInt64(v int64) string {
	return strconv.FormatInt(v, 10)
}

// ActionAuditContext contains information about who executed an action
type ActionAuditContext struct {
	// AccountID is the ID of the account executing the action
	AccountID int64
	// AccountName is the display name (e.g., "Max Mustermann")
	AccountName string
	// IPAddress is the request IP (optional)
	IPAddress string
	// UserAgent is the request user agent (optional)
	UserAgent string
}

// ToAuditEntry creates an audit entry from this context
func (ac *ActionAuditContext) ToAuditEntry(actionKey string, success bool, durationMs int64, errorMsg, resultSummary string) *ActionAuditEntry {
	var ipAddr, userAgent, errMsgPtr, resultPtr *string
	var accountID *int64

	if ac.IPAddress != "" {
		ipAddr = &ac.IPAddress
	}
	if ac.UserAgent != "" {
		userAgent = &ac.UserAgent
	}
	if errorMsg != "" {
		errMsgPtr = &errorMsg
	}
	if resultSummary != "" {
		resultPtr = &resultSummary
	}
	if ac.AccountID != 0 {
		accountID = &ac.AccountID
	}

	var durationMsPtr *int64
	if durationMs > 0 {
		durationMsPtr = &durationMs
	}

	return &ActionAuditEntry{
		ActionKey:           actionKey,
		ExecutedByAccountID: accountID,
		ExecutedByName:      ac.AccountName,
		ExecutedAt:          time.Now(),
		DurationMs:          durationMsPtr,
		Success:             success,
		ErrorMessage:        errMsgPtr,
		ResultSummary:       resultPtr,
		IPAddress:           ipAddr,
		UserAgent:           userAgent,
	}
}
