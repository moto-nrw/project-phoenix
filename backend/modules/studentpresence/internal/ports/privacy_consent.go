package ports

import (
	"context"
	"errors"
	"time"
)

var ErrInvalidPrivacyConsent = errors.New("invalid privacy consent")

// PrivacyConsent is one users.privacy_consents row: the GDPR visit-data
// retention consent of a student. DataRetentionDays bounds how long presence
// rows are kept.
type PrivacyConsent struct {
	ID, TenantID, StudentID int64
	CreatedAt, UpdatedAt    time.Time
	PolicyVersion           string
	Accepted                bool
	AcceptedAt, ExpiresAt   *time.Time
	DurationDays            *int
	RenewalRequired         bool
	DataRetentionDays       int
	Details                 []byte
}

type PrivacyConsentStore interface {
	RevisePrivacyConsent(context.Context, *PrivacyConsent) (Stats, error)
	ListAcceptedRetentionSettings(context.Context) ([]StudentRetentionSetting, Stats, error)
	// ListPrivacyConsents returns the consents of one student, oldest first.
	ListPrivacyConsents(context.Context, int64) ([]PrivacyConsent, Stats, error)
	// RecordPrivacyConsent inserts one consent row and fills the generated
	// columns on the passed value.
	RecordPrivacyConsent(context.Context, *PrivacyConsent) (Stats, error)
}

type StudentRetentionSetting struct {
	StudentID         int64
	DataRetentionDays int
}
