package repositories

import (
	"github.com/uptrace/bun"
)

// NewStudentPrivacyConsentStore builds the consent capability from a database
// handle. Production composes it from the presence module it already holds;
// the student-route and parent fixtures take this shorter path.
func NewStudentPrivacyConsentStore(db *bun.DB) *studentPrivacyConsents {
	return StudentPrivacyConsentCapability(newStudentPresence(db))
}
