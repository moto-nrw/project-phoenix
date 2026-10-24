package owners

import (
	"context"
	"log/slog"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// NewEnrollmentRollovers composes Enrollment's rollover over the sources.
func NewEnrollmentRollovers(src EnrollmentRolloverSources) enrollmentOwner.Rollovers {
	deps := enrollmentCompose.RolloverDependencies{
		Bookings: src.Bookings, Phases: src.Phases, Requests: src.Requests, Children: src.Children,
		Catalog: src.Catalog, Notifications: src.Notifications, PhaseEligibility: src.PhaseEligibility,
		Decisions: src.Decisions, Outbox: src.Outbox, ParentsURL: src.ParentsURL, Random: securityruntime.FillRandom, Logger: src.Logger,
	}
	if src.Settings != nil {
		deps.Settings = enrollmentRolloverSettings{settings: src.Settings}
	}
	return enrollmentCompose.NewRollovers(deps)
}

// enrollmentRolloverSettings resolves the grade collection settings of the
// rollover. It never locks the class collection pair: the rollover's
// eligibility check goes through the phase administration.
type enrollmentRolloverSettings struct {
	settings EnrollmentRolloverSettings
}

func (s enrollmentRolloverSettings) CollectGradeLevel(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectGradeLevel)
}

func (s enrollmentRolloverSettings) CollectSchoolClass(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectSchoolClass)
}

func (s enrollmentRolloverSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyEnrollmentGradeLevelMax)
}

func (enrollmentRolloverSettings) LockClassCollectionPair(context.Context) error {
	return nil
}

// NewEnrollmentOfferingCapacity composes the owner's capacity gate the
// submissions share with the restore, over Care Plan's offerings and the
// Enrollment owner's occupancy peaks. Without settings a waitlist-mode
// overflow check fails instead of guessing the tenant's choice.
func NewEnrollmentOfferingCapacity(offerings enrollmentCompose.CapacityOfferings, peaks enrollmentCompose.CapacityPeaks, settings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}) enrollmentOwner.OfferingCapacity {
	var waitlistEnabled func(context.Context) (bool, error)
	if settings != nil {
		waitlistEnabled = func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, configModels.KeyEnrollmentWaitlistEnabled)
		}
	}
	return enrollmentCompose.NewOfferingCapacity(offerings, peaks, waitlistEnabled)
}

// EnrollmentRolloverSettings resolves the grade collection settings the
// rollover reads.
type EnrollmentRolloverSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// EnrollmentRolloverSources are the owners the root binds the rollover to.
type EnrollmentRolloverSources struct {
	Bookings enrollmentOwner.CareBookingCommands
	Phases   enrollmentCompose.RolloverPhases
	Requests enrollmentCompose.RolloverRequests
	Children enrollmentCompose.RolloverChildren
	// Catalog clones the source phase's care-offering catalog (#2249).
	Catalog          enrollmentCompose.RolloverCatalogCloner
	Notifications    enrollmentOwner.Notifications
	PhaseEligibility enrollmentCompose.PhaseEligibilityGuard
	Outbox           enrollmentCompose.MailOutbox
	Settings         EnrollmentRolloverSettings
	// Decisions approves auto-renewed rows on an auto-approve phase.
	Decisions  enrollmentCompose.RolloverDecider
	ParentsURL string
	Logger     *slog.Logger
}
