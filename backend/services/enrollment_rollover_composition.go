package services

import (
	"context"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// The bindings below compose Enrollment's rollover, admin deletion and
// retention cleanup (#3564) over the Enrollment owner, Care Plan's catalog and
// bookings, the Settings Platform, the Delivery outbox and the Audit
// Platform's deletion trail.

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
	Outbox           platformModels.OutboxEnqueuer
	Settings         EnrollmentRolloverSettings
	// Decisions approves auto-renewed rows on an auto-approve phase.
	Decisions  enrollmentCompose.RolloverDecider
	ParentsURL string
	Logger     *slog.Logger
}

// NewEnrollmentRollovers composes Enrollment's rollover over the sources.
func NewEnrollmentRollovers(src EnrollmentRolloverSources) enrollmentOwner.Rollovers {
	deps := enrollmentCompose.RolloverDependencies{
		Bookings: src.Bookings, Phases: src.Phases, Requests: src.Requests, Children: src.Children,
		Catalog: src.Catalog, Notifications: src.Notifications, PhaseEligibility: src.PhaseEligibility,
		Decisions: src.Decisions, ParentsURL: src.ParentsURL, Random: securityruntime.FillRandom, Logger: src.Logger,
	}
	if src.Outbox != nil {
		deps.Outbox = enrollmentMailOutbox{outbox: src.Outbox}
	}
	if src.Settings != nil {
		deps.Settings = enrollmentRolloverSettings{settings: src.Settings}
	}
	return enrollmentCompose.NewRollovers(deps)
}

// enrollmentRolloverSettings resolves the grade collection settings of the
// rollover. It never locks the class collection pair: the rollover's
// eligibility check goes through the phase administration.
type enrollmentRolloverSettings struct{ settings EnrollmentRolloverSettings }

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

// EnrollmentDeletionOwner is the Enrollment owner slice the admin deletion
// and the retention cleanup read, lock and delete through.
type EnrollmentDeletionOwner interface {
	enrollmentCompose.DeletionQueries
	enrollmentCompose.DeletionRequests
	enrollmentCompose.DeletionChildren
	enrollmentCompose.RejectedRequestCleaner
	enrollmentCompose.UsedLateInviteCleaner
}

// EnrollmentDeletionSources are the owners the root binds the admin deletion
// and the retention cleanup to.
type EnrollmentDeletionSources struct {
	Owner     EnrollmentDeletionOwner
	Guardians enrollmentCompose.GuardianDirectory
	// CountAuditAdjustments counts the audited offering adjustments of a
	// request or one of its children.
	CountAuditAdjustments func(context.Context, int64, *int64) (int, error)
	// CountBookings counts Care Plan's bookings of the request children.
	CountBookings func(context.Context, []int64) (int, error)
	Audit         auditModels.EnrollmentDeletionRepository
	Delivery      enrollmentCompose.EnrollmentDeletionDelivery
	Settings      interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
	Logger *slog.Logger
}

// EnrollmentDeletionModule is the admin deletion and the retention cleanup
// over one shared impact preview.
type EnrollmentDeletionModule struct {
	Preview   *enrollmentCompose.DeletionPreview
	Deletions enrollmentOwner.EnrollmentDeletions
	Cleanup   enrollmentOwner.RejectedEnrollmentCleaner
}

// NewEnrollmentDeletionModule composes the admin deletion and the retention
// cleanup over the sources.
func NewEnrollmentDeletionModule(src EnrollmentDeletionSources) EnrollmentDeletionModule {
	preview := enrollmentCompose.NewDeletionPreview(enrollmentCompose.DeletionPreviewDependencies{
		Enrollment: src.Owner, Guardians: src.Guardians,
		CountAuditAdjustments: src.CountAuditAdjustments, CountBookings: src.CountBookings,
	})
	var audit enrollmentCompose.EnrollmentDeletionAudit
	if src.Audit != nil {
		audit = enrollmentDeletionAudit{repo: src.Audit}
	}
	deletions := enrollmentCompose.NewDeletions(enrollmentCompose.DeletionDependencies{
		Requests: src.Owner, Children: src.Owner, Preview: preview, Audit: audit,
		Delivery: src.Delivery, Logger: withService(src.Logger, "enrollment-deletion"),
	})
	var retention enrollmentCompose.RetentionSettings
	if src.Settings != nil {
		retention = enrollmentRetentionSettings{settings: src.Settings}
	}
	// The cleanup audits through the shared preview only with an audit
	// trail bound; without one (focused suites) it runs its unaudited path.
	var cleanupPreview *enrollmentCompose.DeletionPreview
	if audit != nil {
		cleanupPreview = preview
	}
	cleanup := enrollmentCompose.NewRejectedCleanup(enrollmentCompose.RejectedCleanupDependencies{
		Requests: src.Owner, Children: src.Owner, LateInvites: src.Owner, Delivery: src.Delivery,
		Settings: retention, Preview: cleanupPreview, Audit: audit,
		Logger: withService(src.Logger, "enrollment-rejected-cleanup"),
	})
	return EnrollmentDeletionModule{Preview: preview, Deletions: deletions, Cleanup: cleanup}
}

func withService(logger *slog.Logger, service string) *slog.Logger {
	if logger == nil {
		return nil
	}
	return logger.With("service", service)
}

// enrollmentRetentionSettings resolves the retention of rejected
// enrollments.
type enrollmentRetentionSettings struct {
	settings interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
}

func (s enrollmentRetentionSettings) RejectedRetentionDays(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyEnrollmentRejectedRetentionDays)
}

// enrollmentDeletionAudit appends the enrollment deletion audit rows.
type enrollmentDeletionAudit struct {
	repo auditModels.EnrollmentDeletionRepository
}

func (a enrollmentDeletionAudit) RecordEnrollmentDeletion(ctx context.Context, event enrollmentCompose.EnrollmentDeletionEvent) error {
	actorType := auditModels.EnrollmentDeletionActorAdmin
	if event.Actor == enrollmentCompose.DeletionActorSystem {
		actorType = auditModels.EnrollmentDeletionActorSystem
	}
	scope := auditModels.EnrollmentDeletionScopeRequest
	if event.Scope == enrollmentCompose.DeletionScopeChild {
		scope = auditModels.EnrollmentDeletionScopeChild
	}
	counts := event.Counts
	return a.repo.Create(ctx, &auditModels.EnrollmentDeletion{
		RequestID:      event.RequestID,
		ChildID:        event.ChildID,
		ActorAccountID: event.ActorAccountID,
		ActorType:      actorType,
		Scope:          scope,
		Reason:         event.Reason,
		Counts: auditModels.EnrollmentDeletionCounts{
			Requests: counts.Requests, RequestChildren: counts.RequestChildren,
			RequestChildOfferings: counts.RequestChildOfferings, RequestGuardians: counts.RequestGuardians,
			ChangeRequests: counts.ChangeRequests, ChangeRequestMessages: counts.ChangeRequestMessages,
			LateInvites: counts.LateInvites, OfferingAdjustments: counts.OfferingAdjustments,
			EmailOutbox: counts.EmailOutbox, RolloverLinksCleared: counts.RolloverLinksCleared,
			StudentSourceLinksCleared: counts.StudentSourceLinksCleared,
		},
		DeletedAt: event.DeletedAt,
	})
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
