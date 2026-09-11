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
}

// PrivacyConsentQuery reads the recorded consents of a student.
type PrivacyConsentQuery interface {
	ListPrivacyConsents(context.Context, int64) ([]PrivacyConsent, error)
}

// PrivacyConsentCommand appends consents. RecordPrivacyConsent joins the
// caller's tenant transaction and never rewrites an existing row.
type PrivacyConsentCommand interface {
	RecordPrivacyConsent(context.Context, PrivacyConsent) (PrivacyConsent, error)
}

func (m *Module) ListPrivacyConsents(ctx context.Context, studentID int64) ([]PrivacyConsent, error) {
	return m.engine.ListPrivacyConsents(ctx, studentID)
}

func (m *Module) RecordPrivacyConsent(ctx context.Context, value PrivacyConsent) (PrivacyConsent, error) {
	return m.engine.RecordPrivacyConsent(ctx, value)
}
