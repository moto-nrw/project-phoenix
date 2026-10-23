//go:build testhooks

package owners

import (
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	schedulerCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
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
