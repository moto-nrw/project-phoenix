package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type PrivacyConsentStore interface {
	RevisePrivacyConsent(context.Context, *studentpresence.PrivacyConsent) (Stats, error)
	ListAcceptedRetentionSettings(context.Context) ([]studentpresence.StudentRetentionSetting, Stats, error)
	// ListPrivacyConsents returns the consents of one student, oldest first.
	ListPrivacyConsents(context.Context, int64) ([]studentpresence.PrivacyConsent, Stats, error)
	// RecordPrivacyConsent inserts one consent row and fills the generated
	// columns on the passed value.
	RecordPrivacyConsent(context.Context, *studentpresence.PrivacyConsent) (Stats, error)
}
