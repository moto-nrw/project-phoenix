//go:build testhooks

package owners

import (
	"context"
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	schedulerCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewSchedulerCleanupTestDependencies binds the same directory, settings,
// presence cleanup, ledger and tenant runtime as the Worker, without unrelated jobs.
func NewSchedulerCleanupTestDependencies(db transactionruntime.Database, unit tenant.UnitOfWork, now func() time.Time) (schedulerPorts.WorkerDependencies, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return schedulerPorts.WorkerDependencies{}, err
	}
	resolver, err := schedulerSettings(settings.Settings)
	if err != nil {
		return schedulerPorts.WorkerDependencies{}, err
	}
	schools, err := NewOrganizationTenancy(db)
	if err != nil {
		return schedulerPorts.WorkerDependencies{}, err
	}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db.Pool(), Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		return schedulerPorts.WorkerDependencies{}, err
	}
	records := ComposePresenceSessionRecords(presenceCompose.SessionRecordDependencies{
		DB: db.Pool(), Now: now,
	})
	_, supervisors := presenceCompose.SessionRepositories(records)
	audit, err := auditCompose.NewTenantPersistence(db.Pool())
	if err != nil {
		return schedulerPorts.WorkerDependencies{}, err
	}
	runtime, err := schedulerCompose.NewTenantRuntime(unit)
	if err != nil {
		return schedulerPorts.WorkerDependencies{}, err
	}
	cleanup := presenceservice.NewPresenceCleanup(presence, supervisors, NewDeletionAudit(audit.DataDeletion),
		settingsCompose.PresenceSettings(settings.Settings), func() calendar.Date { return calendar.DateFromTime(now()) })
	return schedulerPorts.WorkerDependencies{
		Logger: slog.New(slog.DiscardHandler), Now: now,
		SchoolRepo: schedulerTenantDirectory{schools: schools}, Settings: resolver,
		TenantRuntime: runtime, CleanupRuns: schedulerCompose.NewCleanupRunLedger(),
		ActiveCleanup: newSchedulerPresenceCleanup(cleanup),
	}, nil
}

// NewSchedulerFileRetentionTestDependencies composes real retention and file
// recovery over a private upload directory, without starting unrelated jobs.
func NewSchedulerFileRetentionTestDependencies(db transactionruntime.Database, unit tenant.UnitOfWork, now func() time.Time, publicDir string) (schedulerPorts.WorkerDependencies, error) {
	deps, err := NewSchedulerCleanupTestDependencies(db, unit, now)
	if err != nil {
		return deps, err
	}
	audit, _, steps, err := NewRetentionTestOwners(db, unit)
	if err != nil {
		return deps, err
	}
	deps.RetentionSteps = steps
	deps.RetentionLog = func(ctx context.Context, kind string, outcome schedulerPorts.RetentionOutcome) error {
		return audit.LogRetention(ctx, kind, outcome.Deleted, outcome.RetentionDays)
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return deps, err
	}
	// Recovery reads Workforce records and offboarded membership IDs; it does
	// not invoke personnel edits or their audit trail.
	documents, err := workforceCompose.NewStaffAdmin(workforceCompose.StaffAdminDependencies{
		DB: db.Pool(), Subjects: staffAdminSubjects{membership: membership}, Audit: staffAdminAudit{}, Allows: securityruntime.HasPermission,
	})
	if err != nil {
		return deps, err
	}
	queue, err := workforceCompose.NewDocumentCleanup(db.Pool(), nil)
	if err != nil {
		return deps, err
	}
	deps.StaffDocumentCleaner, err = workforceCompose.NewStaffDocumentFileRecovery(documents, queue, cleanupTestUploads{dir: publicDir}, deps.Logger)
	return deps, err
}

type cleanupTestUploads struct{ dir string }

func (u cleanupTestUploads) PublicDir() (string, error) { return u.dir, nil }
