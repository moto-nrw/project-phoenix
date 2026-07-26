package users

import (
	"errors"
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

// Validate ensures privacy consent data is valid. It performs pure field
// validation only and never mutates the entity. Acceptance/expiry derivation
// is a business decision owned by the privacy-consent service (issue #586,
// Rule 12: models hold data, not decisions).
func (pc *PrivacyConsent) Validate() error {
	if pc.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if pc.PolicyVersion == "" {
		return errors.New("policy version is required")
	}

	// Validate data retention days
	if pc.DataRetentionDays < MinDataRetentionDays || pc.DataRetentionDays > MaxDataRetentionDays {
		return errors.New("data retention days must be between 1 and 31")
	}

	// Validate expires_at is after acceptance if both set
	if pc.ExpiresAt != nil && pc.AcceptedAt != nil {
		if pc.ExpiresAt.Before(*pc.AcceptedAt) {
			return errors.New("expiration date must be after acceptance date")
		}
	}

	// No need to validate JSONB details - handled by the database
	return nil
}

// NeedsRenewal checks if consent needs renewal based on renewal_required flag
func (pc *PrivacyConsent) NeedsRenewal() bool {
	return pc.RenewalRequired
}

// SetStudent links this privacy consent to a student
func (pc *PrivacyConsent) SetStudent(student *Student) {
	pc.Student = student
	if student != nil {
		pc.StudentID = student.ID
	}
}

// GetDetails returns details map
func (pc *PrivacyConsent) GetDetails() map[string]interface{} {
	if pc.Details == nil {
		pc.Details = make(map[string]interface{})
	}
	return pc.Details
}

// UpdateDetails updates the details map
func (pc *PrivacyConsent) UpdateDetails(details map[string]interface{}) error {
	pc.Details = details
	return nil
}
