package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/observability"
)

// observeCarePlan records the excused-absence workflow as Care Plan
// operations.
func observeCarePlan(observation careplanCompose.Observation) {
	observability.ObserveCarePlanOperation(observation.Operation, observation.Duration, observation.Stats.Queries, observation.Stats.Rows, observation.Stats.Conflicts, observation.Stats.StatementDuration, careplan.ErrorCode(observation.Err), observation.Err)
}

// newEnrollmentFormSchemas builds the form schema administration where the
// root mounts it, over the collection settings (#4012).
func newEnrollmentFormSchemas(settings owners.EnrollmentSettingsReads, logger *slog.Logger) enrollmentOwner.FormSchemaAdministration {
	return enrollmentCompose.NewFormSchemas(owners.NewEnrollmentBookings(), owners.EnrollmentCollectionSettings{Settings: settings}, logger.With("service", "enrollment-form-schema"))
}

// newEnrollmentCaptcha builds the captcha verifier of the public enrollment
// form where the root mounts it, over the platform's Turnstile keys.
func newEnrollmentCaptcha(settings owners.EnrollmentSettingsReads, keys TurnstileKeys, logger *slog.Logger) enrollmentOwner.CaptchaVerifier {
	return enrollmentCompose.NewCaptcha(owners.EnrollmentCaptchaSettings{
		Settings: settings, SiteKey: keys.SiteKey, SecretKey: keys.SecretKey,
	}, "", logger.With("service", "enrollment-captcha"))
}

// newEnrollmentPhaseExpiry builds the phase expiry warnings over the People
// Directory's enrolled students and Care Plan's offerings.
func newEnrollmentPhaseExpiry(students peopledirectory.StudentQuery, carePlan careplan.Query) enrollmentOwner.PhaseExpiryWarnings {
	bookings := owners.NewEnrollmentBookings()
	return enrollmentCompose.NewPhaseExpiryWarnings(enrollmentCompose.NewPhaseExpirySnapshots(
		bookings, phaseExpiryStudents{query: students}, phaseExpiryCarePlanDirectory{query: carePlan}, bookings,
	))
}

// requiredLink trims a configured base URL and refuses an empty one, and in
// production one without https.
func requiredLink(name, raw string, production bool) (string, error) {
	value := strings.TrimRight(raw, "/")
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if production && !strings.HasPrefix(value, "https://") {
		return "", fmt.Errorf("%s must use https:// in production (received %q)", name, raw)
	}
	return value, nil
}

// GroupRoster is the live group roster's People port: the participation
// candidates of a group with their names.
type GroupRoster interface {
	GroupMembers(ctx context.Context, groupID int64) ([]grouplive.RosterStudent, error)
}

// WorkforceMail is the tenant mail the absence requests go out through.
type WorkforceMail struct {
	Dispatcher interface {
		Dispatch(context.Context, email.DeliveryRequest)
	}
	From        email.Email
	ReplyTo     email.ReplyToResolver
	FrontendURL string
}
