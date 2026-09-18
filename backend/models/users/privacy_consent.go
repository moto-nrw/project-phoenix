package users

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Data-retention bounds and default for privacy consents (GDPR visit-data
// retention window). The default is also registered as the per-tenant setting
// gdpr.privacy_consent_retention_days; these consts are the single in-package
// source for the bounds and the fallback default. Issue #586 (Rule 12): the
// numeric policy no longer lives as bare literals inside model methods.
const (
	MinDataRetentionDays     = 1
	MaxDataRetentionDays     = 31
	DefaultDataRetentionDays = 30
)

// The consent lifecycle — what a valid window is, when one needs renewing, how
// its details are read — moved to Student Presence with the table it belongs to
// (#3349). What stays here is the row shape the retained fixtures insert.

// PrivacyConsent represents a privacy consent record for a student
type PrivacyConsent struct {
	base.Model `bun:"schema:users,table:privacy_consents"`
	base.TenantModel
	StudentID         int64                  `bun:"student_id,notnull" json:"student_id"`
	PolicyVersion     string                 `bun:"policy_version,notnull" json:"policy_version"`
	Accepted          bool                   `bun:"accepted,notnull" json:"accepted"`
	AcceptedAt        *time.Time             `bun:"accepted_at" json:"accepted_at,omitempty"`
	ExpiresAt         *time.Time             `bun:"expires_at" json:"expires_at,omitempty"`
	DurationDays      *int                   `bun:"duration_days" json:"duration_days,omitempty"`
	RenewalRequired   bool                   `bun:"renewal_required,notnull" json:"renewal_required"`
	DataRetentionDays int                    `bun:"data_retention_days,notnull" json:"data_retention_days"`
	Details           map[string]interface{} `bun:"details,type:jsonb" json:"details,omitempty"`

	// Relations not stored in the database
	Student *Student `bun:"-" json:"student,omitempty"`
}
