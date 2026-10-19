package owners

import (
	"context"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	timetableProjection "github.com/moto-nrw/project-phoenix/modules/timetableprojection/compose"
)

// enrollmentGuardianDirectory serves the deletion preview's guardians from
// the People Directory.
type enrollmentGuardianDirectory struct{ guardians peopledirectory.GuardianQuery }

func (d enrollmentGuardianDirectory) ListGuardiansByAccount(ctx context.Context, accountIDs []int64) ([]enrollmentCompose.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByAccount(ctx, accountIDs)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) ListGuardiansByID(ctx context.Context, ids []int64) ([]enrollmentCompose.DirectoryGuardian, error) {
	guardians, err := d.guardians.ListGuardiansByID(ctx, ids)
	return toEnrollmentGuardians(guardians), err
}

func (d enrollmentGuardianDirectory) CountGuardianLinks(ctx context.Context, ids []int64) (map[int64]int, error) {
	return d.guardians.CountGuardianLinks(ctx, ids)
}

func toEnrollmentGuardians(guardians []peopledirectory.Guardian) []enrollmentCompose.DirectoryGuardian {
	result := make([]enrollmentCompose.DirectoryGuardian, 0, len(guardians))
	for _, guardian := range guardians {
		result = append(result, enrollmentCompose.DirectoryGuardian{ID: guardian.ID, AccountID: guardian.AccountID})
	}
	return result
}

// enrollmentRejectedCleanup runs Enrollment's retention cleanup of rejected
// enrollments for the scheduler, which reads only the counts.
type enrollmentRejectedCleanup struct {
	cleaner enrollmentOwner.RejectedEnrollmentCleaner
}

// CleanupRejectedEnrollments runs the cleanup for the tenant in context.
func (c enrollmentRejectedCleanup) CleanupRejectedEnrollments(ctx context.Context) (int, int64, int64, error) {
	result, err := c.cleaner.CleanupRejectedEnrollments(ctx)
	return result.DeletedRequests, result.DeletedLateInvites, result.DeletedOutboxRows, err
}

// NewEnrollmentDeletionModule binds the admin deletion and the retention
// cleanup of Enrollment's composition to the Audit Platform's deletion trail
// and the retention setting.
func NewEnrollmentDeletionModule(src EnrollmentDeletionSources) enrollmentCompose.DeletionModule {
	deps := enrollmentCompose.DeletionModuleDependencies{
		Owner: src.Owner, Guardians: src.Guardians,
		CountAuditAdjustments: src.CountAuditAdjustments, CountBookings: src.CountBookings,
		SourceEnrollments: timetableProjection.New().CountSourceEnrollments,
		Delivery:          src.Delivery, Logger: src.Logger,
	}
	if src.Audit != nil {
		deps.Audit = enrollmentDeletionAudit{repo: src.Audit}
	}
	if src.Settings != nil {
		deps.Retention = enrollmentRetentionSettings{settings: src.Settings}
	}
	return enrollmentCompose.NewDeletionModule(deps)
}

// enrollmentRetentionSettings resolves the retention of rejected and of
// withdrawn enrollments.
type enrollmentRetentionSettings struct {
	settings interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
}

func (s enrollmentRetentionSettings) RejectedRetentionDays(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyEnrollmentRejectedRetentionDays)
}

func (s enrollmentRetentionSettings) WithdrawnRetentionDays(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyGDPRWithdrawnEnrollmentRetentionDays)
}

// enrollmentDeletionAudit appends the enrollment deletion audit rows.
type enrollmentDeletionAudit struct {
	repo enrollmentDeletionAppender
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

// EnrollmentDeletionOwner is the Enrollment owner slice the admin deletion
// and the retention cleanup read, lock and delete through.
type EnrollmentDeletionOwner = enrollmentCompose.DeletionOwner

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
	Audit         enrollmentDeletionAppender
	Delivery      enrollmentCompose.EnrollmentDeletionDelivery
	Settings      interface {
		ResolveInt(ctx context.Context, key string) (int, error)
	}
	Logger *slog.Logger
}
