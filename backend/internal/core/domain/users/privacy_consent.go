package users

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

// PrivacyConsent represents a privacy consent record for a student
type PrivacyConsent struct {
	base.Model        `bun:"schema:users,table:privacy_consents"`
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

func (pc *PrivacyConsent) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`users.privacy_consents AS "privacy_consent"`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`users.privacy_consents AS "privacy_consent"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`users.privacy_consents AS "privacy_consent"`)
	}
	return nil
}

// TableName returns the database table name
func (pc *PrivacyConsent) TableName() string {
	return "users.privacy_consents"
}

// Validate ensures privacy consent data is valid
func (pc *PrivacyConsent) Validate() error {
	if pc.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if pc.PolicyVersion == "" {
		return errors.New("policy version is required")
	}

	// Validate data retention days
	if pc.DataRetentionDays < 1 || pc.DataRetentionDays > 31 {
		return errors.New("data retention days must be between 1 and 31")
	}

	// If consent is accepted, accepted_at must be set
	if pc.Accepted && pc.AcceptedAt == nil {
		now := time.Now()
		pc.AcceptedAt = &now
	}

	// If duration days is set but expires_at is not, calculate expires_at
	if pc.DurationDays != nil && *pc.DurationDays > 0 && pc.ExpiresAt == nil && pc.AcceptedAt != nil {
		expiresAt := pc.AcceptedAt.AddDate(0, 0, *pc.DurationDays)
		pc.ExpiresAt = &expiresAt
	}

	// Validate expires_at is in the future if set
	if pc.ExpiresAt != nil && pc.AcceptedAt != nil {
		if pc.ExpiresAt.Before(*pc.AcceptedAt) {
			return errors.New("expiration date must be after acceptance date")
		}
	}

	// No need to validate JSONB details - handled by the database
	return nil
}

// IsValid checks if the consent is currently valid (accepted and not expired)
func (pc *PrivacyConsent) IsValid() bool {
	if !pc.Accepted {
		return false
	}

	if pc.ExpiresAt != nil && time.Now().After(*pc.ExpiresAt) {
		return false
	}

	return true
}

// IsExpired checks if the consent has expired
func (pc *PrivacyConsent) IsExpired() bool {
	if pc.ExpiresAt == nil {
		return false
	}

	return time.Now().After(*pc.ExpiresAt)
}

// NeedsRenewal checks if consent needs renewal based on renewal_required flag
func (pc *PrivacyConsent) NeedsRenewal() bool {
	return pc.RenewalRequired
}

// GetTimeToExpiry returns the duration until expiry or nil if no expiry
func (pc *PrivacyConsent) GetTimeToExpiry() *time.Duration {
	if pc.ExpiresAt == nil {
		return nil
	}

	if time.Now().After(*pc.ExpiresAt) {
		// Already expired
		duration := time.Duration(0)
		return &duration
	}

	duration := time.Until(*pc.ExpiresAt)
	return &duration
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

// Accept marks the consent as accepted with current timestamp
func (pc *PrivacyConsent) Accept() {
	pc.Accepted = true
	now := time.Now()
	pc.AcceptedAt = &now

	// If duration days is set, calculate expires_at
	if pc.DurationDays != nil && *pc.DurationDays > 0 {
		expiresAt := now.AddDate(0, 0, *pc.DurationDays)
		pc.ExpiresAt = &expiresAt
	}
}

// Revoke revokes the consent
func (pc *PrivacyConsent) Revoke() {
	pc.Accepted = false
}

// GetID implements the base.Entity interface
func (pc *PrivacyConsent) GetID() interface{} {
	return pc.ID
}

// GetCreatedAt implements the base.Entity interface
func (pc *PrivacyConsent) GetCreatedAt() time.Time {
	return pc.CreatedAt
}

// GetUpdatedAt implements the base.Entity interface
func (pc *PrivacyConsent) GetUpdatedAt() time.Time {
	return pc.UpdatedAt
}

// GetDataRetentionDays returns the number of days to retain visit data
func (pc *PrivacyConsent) GetDataRetentionDays() int {
	// If DataRetentionDays is not set (0), default to 30 days
	if pc.DataRetentionDays == 0 {
		return 30
	}
	return pc.DataRetentionDays
}

// SetDataRetentionDays sets the data retention days with validation
func (pc *PrivacyConsent) SetDataRetentionDays(days int) error {
	if days < 1 || days > 31 {
		return errors.New("data retention days must be between 1 and 31")
	}
	pc.DataRetentionDays = days
	return nil
}
