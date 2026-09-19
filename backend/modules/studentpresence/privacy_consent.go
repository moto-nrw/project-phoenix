package studentpresence

import (
	"context"
	"errors"
	"time"
)

// ErrInvalidPrivacyConsent reports a consent that violates the retention
// bounds or the acceptance/expiry pairing.
var ErrInvalidPrivacyConsent = errors.New("invalid privacy consent")

// Retention bounds and default of the GDPR visit-data retention consent
// (users.privacy_consents.data_retention_days).
const (
	MinPrivacyConsentRetentionDays     = 1
	MaxPrivacyConsentRetentionDays     = 31
	DefaultPrivacyConsentRetentionDays = 30
)

// PrivacyConsent is one recorded retention consent of a student. Student
// Presence owns the row because it bounds how long presence data is kept.
type PrivacyConsent struct {
	ID, TenantID, StudentID int64
	CreatedAt, UpdatedAt    time.Time
	PolicyVersion           string
	Accepted                bool
	AcceptedAt, ExpiresAt   *time.Time
	DurationDays            *int
	RenewalRequired         bool
	DataRetentionDays       int
	// Details is the recorded JSON document, not a query-filter map.
	Details []byte
}

// PrivacyConsentQuery reads the recorded consents of a student.
type PrivacyConsentQuery interface {
	ListAcceptedRetentionSettings(context.Context) ([]StudentRetentionSetting, error)
	ListPrivacyConsents(context.Context, int64) ([]PrivacyConsent, error)
}

// PrivacyConsentCommand appends consents. RecordPrivacyConsent joins the
// caller's tenant transaction and never rewrites an existing row.
type PrivacyConsentCommand interface {
	RevisePrivacyConsent(context.Context, PrivacyConsent) (PrivacyConsent, error)
	RecordPrivacyConsent(context.Context, PrivacyConsent) (PrivacyConsent, error)
}

func (m *Module) ListPrivacyConsents(ctx context.Context, studentID int64) ([]PrivacyConsent, error) {
	return m.engine.ListPrivacyConsents(ctx, studentID)
}

func (m *Module) RecordPrivacyConsent(ctx context.Context, value PrivacyConsent) (PrivacyConsent, error) {
	return m.engine.RecordPrivacyConsent(ctx, value)
}

type StudentRetentionSetting struct {
	StudentID         int64
	DataRetentionDays int
}

func (m *Module) ListAcceptedRetentionSettings(ctx context.Context) ([]StudentRetentionSetting, error) {
	return m.engine.ListAcceptedRetentionSettings(ctx)
}

func (m *Module) RevisePrivacyConsent(ctx context.Context, value PrivacyConsent) (PrivacyConsent, error) {
	return m.engine.RevisePrivacyConsent(ctx, value)
}

// DataRetentionDaysOrDefault is the retention window a child without a
// recorded consent falls back to. override is the tenant's resolved
// gdpr.privacy_consent_retention_days value, or 0 when the tenant has none —
// the settings lookup belongs to the Settings Platform consumer, the fallback
// rule to this owner.
func DataRetentionDaysOrDefault(override int) int {
	if override > 0 {
		return override
	}
	return DefaultPrivacyConsentRetentionDays
}

// AcceptPrivacyConsent marks a consent accepted at now and derives the expiry
// from the configured duration. It returns the changed value; persisting it is
// the caller's job. This is the consent lifecycle the owner decides, not the
// model (issue #586, Rule 12).
func AcceptPrivacyConsent(value PrivacyConsent, now time.Time) PrivacyConsent {
	value.Accepted = true
	value.AcceptedAt = &now
	return DerivePrivacyConsentExpiry(value)
}

// DerivePrivacyConsentExpiry stamps ExpiresAt = AcceptedAt + DurationDays when
// a positive duration is set and no expiry is present yet.
func DerivePrivacyConsentExpiry(value PrivacyConsent) PrivacyConsent {
	if value.DurationDays == nil || *value.DurationDays <= 0 {
		return value
	}
	if value.ExpiresAt != nil || value.AcceptedAt == nil {
		return value
	}
	expiresAt := value.AcceptedAt.AddDate(0, 0, *value.DurationDays)
	value.ExpiresAt = &expiresAt
	return value
}
