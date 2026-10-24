package enrollmenthttp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/settings"
)

// The Settings Platform registry keys and values these suites write. The
// literals pin the registry contract the Enrollment flows resolve;
// TestRegistrySettingsMatchThePlatform pins the keys the platform names in
// its public contract.
const (
	settingKeyEnrollmentEnabled                      = "enrollment.enabled"
	settingKeyAllowSubmissionEdit                    = "enrollment.allow_submission_edit"
	settingKeyBookingsAuthoritative                  = "enrollment.bookings_authoritative"
	settingKeyCareOfferingsEnabled                   = "enrollment.care_offerings_enabled"
	settingKeyChangeRequestEmailNotificationsEnabled = "enrollment.change_request_email_notifications_enabled"
	settingKeyCollectGradeLevel                      = "enrollment.collect_grade_level"
	settingKeyCollectSchoolClass                     = "enrollment.collect_school_class"
	settingKeyDefaultActivationMode                  = "enrollment.default_activation_mode"
	settingKeyDuplicateHandling                      = "enrollment.duplicate_handling"
	settingKeyGradeLevelMax                          = "enrollment.grade_level_max"
	settingKeyLegalAGBText                           = "enrollment.legal_agb_text"
	settingKeyLegalDSGVOEnabled                      = "enrollment.legal_dsgvo_enabled"
	settingKeyLegalDSGVOText                         = "enrollment.legal_dsgvo_text"
	settingKeyLegalEmailContactEnabled               = "enrollment.legal_email_contact_enabled"
	settingKeyLegalEmailContactText                  = "enrollment.legal_email_contact_text"
	settingKeyLegalPhotoEnabled                      = "enrollment.legal_photo_enabled"
	settingKeyLegalPhotoText                         = "enrollment.legal_photo_text"
	settingKeyLegalTermsEnabled                      = "enrollment.legal_terms_enabled"
	settingKeyNotificationEmails                     = "enrollment.notification_emails"
	settingKeyNotifyPerDecision                      = "enrollment.notify_per_decision"
	settingKeyRejectedRetentionDays                  = "enrollment.rejected_retention_days"
	settingKeyStatusTokenTTLDays                     = "enrollment.status_token_ttl_days"
	settingKeyWaitlistEnabled                        = "enrollment.waitlist_enabled"
	settingValueActivationImmediate                  = "immediate"
	settingValueActivationScheduled                  = "scheduled"
	settingValueDuplicateBlock                       = "block"
	settingValueDuplicateIgnore                      = "ignore"
	settingValueDuplicateWarn                        = "warn"
	settingValueNotifyDigest                         = "digest"
	settingValueNotifyImmediate                      = "immediate"
)

// TestRegistrySettingsMatchThePlatform pins the keys above that the Settings
// Platform names publicly, so a renamed key fails here.
func TestRegistrySettingsMatchThePlatform(t *testing.T) {
	t.Parallel()

	assert.Equal(t, settings.KeyEnrollmentBookingsAuthoritative, settingKeyBookingsAuthoritative)
	assert.Equal(t, settings.KeyEnrollmentCareOfferingsEnabled, settingKeyCareOfferingsEnabled)
	assert.Equal(t, settings.KeyEnrollmentCollectGradeLevel, settingKeyCollectGradeLevel)
	assert.Equal(t, settings.KeyEnrollmentDuplicateHandling, settingKeyDuplicateHandling)
	assert.Equal(t, settings.KeyEnrollmentGradeLevelMax, settingKeyGradeLevelMax)
	assert.Equal(t, settings.KeyEnrollmentNotifyPerDecision, settingKeyNotifyPerDecision)
	assert.Equal(t, settings.KeyEnrollmentRejectedRetentionDays, settingKeyRejectedRetentionDays)
	assert.Equal(t, settings.KeyEnrollmentWaitlistEnabled, settingKeyWaitlistEnabled)
}
