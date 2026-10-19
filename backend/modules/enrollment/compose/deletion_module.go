package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
)

// DeletionOwner is the Enrollment owner slice the admin deletion and the
// retention cleanup read, lock and delete through.
type DeletionOwner interface {
	DeletionQueries
	DeletionRequests
	DeletionChildren
	RejectedRequestCleaner
	UsedLateInviteCleaner
}

// DeletionModuleDependencies are the owner and the ports the admin deletion
// and the retention cleanup share. A nil Audit leaves both unaudited; a nil
// Retention fails the cleanup's retention lookup.
type DeletionModuleDependencies struct {
	Owner     DeletionOwner
	Guardians GuardianDirectory
	// CountAuditAdjustments counts the audited offering adjustments of a
	// request or one of its children.
	CountAuditAdjustments func(context.Context, int64, *int64) (int, error)
	// CountBookings counts the bookings of the request children.
	CountBookings func(context.Context, []int64) (int, error)
	// SourceEnrollments counts the timetable enrollments a request sourced.
	SourceEnrollments postgres.SourceEnrollmentCount
	Audit             EnrollmentDeletionAudit
	Delivery          EnrollmentDeletionDelivery
	Retention         RetentionSettings
	Logger            *slog.Logger
}

// DeletionModule is the admin deletion and the retention cleanup over one
// shared impact preview.
type DeletionModule struct {
	Preview   *DeletionPreview
	Deletions enrollment.EnrollmentDeletions
	Cleanup   enrollment.RejectedEnrollmentCleaner
}

// NewDeletionModule composes the admin deletion and the retention cleanup.
func NewDeletionModule(deps DeletionModuleDependencies) DeletionModule {
	preview := NewDeletionPreview(DeletionPreviewDependencies{
		Enrollment: deps.Owner, Guardians: deps.Guardians,
		CountAuditAdjustments: deps.CountAuditAdjustments, CountBookings: deps.CountBookings,
		CountSourceEnrollments: newStore().SourceEnrollments(deps.SourceEnrollments),
	})
	deletions := NewDeletions(DeletionDependencies{
		Requests: deps.Owner, Children: deps.Owner, Preview: preview, Audit: deps.Audit,
		Delivery: deps.Delivery, Logger: withService(deps.Logger, "enrollment-deletion"),
	})
	// The cleanup audits through the shared preview only with an audit trail
	// bound; without one (focused suites) it runs its unaudited path.
	var cleanupPreview *DeletionPreview
	if deps.Audit != nil {
		cleanupPreview = preview
	}
	cleanup := NewRejectedCleanup(RejectedCleanupDependencies{
		Requests: deps.Owner, Children: deps.Owner, LateInvites: deps.Owner, Delivery: deps.Delivery,
		Settings: deps.Retention, Preview: cleanupPreview, Audit: deps.Audit,
		Logger: withService(deps.Logger, "enrollment-rejected-cleanup"),
	})
	return DeletionModule{Preview: preview, Deletions: deletions, Cleanup: cleanup}
}

func withService(logger *slog.Logger, service string) *slog.Logger {
	if logger == nil {
		return nil
	}
	return logger.With("service", service)
}
