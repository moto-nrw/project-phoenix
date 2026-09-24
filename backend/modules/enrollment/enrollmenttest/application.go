package enrollmenttest

import (
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
)

// Enrollment's application services, composed the way production composes
// them, for behavior tests that drive them against a real database.
type (
	PhaseDependencies          = compose.PhaseDependencies
	Phases                     = compose.Phases
	NotificationDependencies   = compose.NotificationDependencies
	PhaseExpirySnapshots       = compose.PhaseExpirySnapshots
	OfferingStudent            = compose.OfferingStudent
	PhaseRecords               = compose.PhaseRecords
	Mail                       = compose.Mail
	ApprovedOfferingProjection = compose.ApprovedOfferingProjection
)

// NewApprovedOfferingProjection composes the approved-offering projection.
func NewApprovedOfferingProjection(selections compose.ApprovedSelections, students compose.OfferingStudentDirectory) *ApprovedOfferingProjection {
	return compose.NewApprovedOfferingProjection(selections, students)
}

// NewPhases composes the phase administration.
func NewPhases(deps PhaseDependencies) *Phases { return compose.NewPhases(deps) }

// NewFormSchemas composes form-schema publishing over the owner.
func NewFormSchemas(records compose.FormSchemaRecords, settings compose.CollectionSettings, logger *slog.Logger) enrollment.FormSchemaAdministration {
	return compose.NewFormSchemas(records, settings, logger)
}

// NewPhaseExpirySnapshots composes the phase-expiry report inputs.
func NewPhaseExpirySnapshots(owner compose.ExpirySnapshots, students enrollment.PhaseExpiryStudents, carePlan enrollment.PhaseExpiryOfferings, bookings enrollment.PhaseExpiryBookings) *PhaseExpirySnapshots {
	return compose.NewPhaseExpirySnapshots(owner, students, carePlan, bookings)
}

// NewPhaseExpiryWarnings composes the phase-expiry warnings over a snapshot
// source.
func NewPhaseExpiryWarnings(snapshots *PhaseExpirySnapshots) enrollment.PhaseExpiryWarnings {
	return compose.NewPhaseExpiryWarnings(snapshots)
}

// NewCaptcha composes captcha verification against the provider at
// verifyURL.
func NewCaptcha(settings compose.CaptchaSettings, verifyURL string) enrollment.CaptchaVerifier {
	return compose.NewCaptcha(settings, verifyURL, nil)
}

// FormSchemaRecords is the owner slice schema publishing writes through.
type FormSchemaRecords = compose.FormSchemaRecords

// NotificationModePin and NotificationSettings are the notification ports a
// test binds.
type (
	NotificationModePin  = compose.NotificationModePin
	NotificationSettings = compose.NotificationSettings
)

// NewNotifications composes the mail branding and decision notifications.
// Without a fingerprint the idempotency keys carry the hex-encoded state
// itself: stable and state-sensitive, which is all a test observes.
func NewNotifications(deps NotificationDependencies) enrollment.Notifications {
	if deps.Fingerprint == nil {
		deps.Fingerprint = func(content []byte) string { return fmt.Sprintf("%x", content) }
	}
	return compose.NewNotifications(deps)
}
