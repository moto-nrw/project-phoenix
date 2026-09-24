package repositories

import (
	"context"
	"fmt"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	"github.com/uptrace/bun"
)

func auditRootRuntime(db *bun.DB) auditRepo.Runtime {
	return func(ctx context.Context) (bun.IDB, int64) {
		tenantID := auditModels.TenantIDFromContext(ctx)
		if raw, ok := auditModels.TransactionFromContext(ctx); ok {
			switch tx := raw.(type) {
			case bun.Tx:
				return tx, tenantID
			case *bun.Tx:
				if tx != nil {
					return tx, tenantID
				}
			}
		}
		return db, tenantID
	}
}

// AuthCleanupRepositories are the retained repositories the auth cleanup
// CLI composes. The expired session sweep runs through Identity & Access,
// which the composition binds to the school and person lookups here (#3251).
type AuthCleanupRepositories struct {
	School           organizationtenancy.Capability
	Person           userModels.PersonRepository
	AuthEvent        auditModels.AuthEventRepository
	PushSubscription deliveryModels.PushSubscriptionRepository
}

func NewAuthCleanupRepositories(db *bun.DB, command auditModels.Command) AuthCleanupRepositories {
	authEvents := auditRepo.NewAuthEventRepository(auditRootRuntime(db))
	return AuthCleanupRepositories{
		School: mustNewOrganizationTenancy(db), Person: NewPersonRepository(db),
		AuthEvent: RouteAuthEventWrites(authEvents, command), PushSubscription: deliveryCompose.NewPushSubscriptionRepository(db),
	}
}

type SessionCleanupRepositories struct {
	// Sessions are the Student Presence session and supervision records the
	// session cleanup runs on.
	Sessions        studentpresence.SessionRecords
	Device          iotModels.DeviceRepository
	TimetableBridge scheduleModels.ActivityInstanceRepository
}

func NewSessionCleanupRepositories(db *bun.DB, timetableCapability timetable.Capability) SessionCleanupRepositories {
	if timetableCapability == nil {
		panic("session cleanup repositories: timetable capability is required")
	}
	fleet, err := NewDeviceFleet(db)
	if err != nil {
		panic(fmt.Sprintf("session cleanup repositories: compose device fleet: %v", err))
	}
	device := devicefleetRepositoryAdapter.NewDeviceRepository(fleet)
	rooms, err := NewFacilities(db)
	if err != nil {
		panic(fmt.Sprintf("session cleanup repositories: compose facilities: %v", err))
	}
	sessions := newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{
		DB: db, Devices: activeDeviceDirectory{devices: fleet}, Rooms: &activeRoomDirectory{rooms: rooms},
		Activities: NewSessionActivities(timetableActivityGroupRepository{timetable: timetableCapability}),
	})
	return SessionCleanupRepositories{
		Sessions: sessions, Device: device,
		TimetableBridge: newTimetableActivityInstanceRepository(db, timetableCapability, newStudentPresence(db)),
	}
}

type RetentionCleanupRepositories struct {
	// Sessions are the Student Presence session and supervision records the
	// stale-supervisor cleanup runs on.
	Sessions studentpresence.SessionRecords
	Deletion auditModels.DataDeletionRepository
}

func NewRetentionCleanupRepositories(db *bun.DB, command auditModels.Command) RetentionCleanupRepositories {
	deletions := auditRepo.NewDataDeletionRepository(auditRootRuntime(db))
	return RetentionCleanupRepositories{
		Sessions: NewPresenceSessionRecords(db),
		Deletion: RouteDataDeletionWrites(deletions, command),
	}
}

// TimetableCleanupRepositories are the Audit Platform stores the Timetable
// retention writes through: the per-child deletion records and the
// Änderungsprotokoll it deletes in lockstep (#3551).
type TimetableCleanupRepositories struct {
	Deletion  auditModels.DataDeletionRepository
	Deviation auditModels.DeviationEventRepository
}

func NewTimetableCleanupRepositories(db *bun.DB, command auditModels.Command) TimetableCleanupRepositories {
	deletions := auditRepo.NewDataDeletionRepository(auditRootRuntime(db))
	return TimetableCleanupRepositories{
		Deletion:  RouteDataDeletionWrites(deletions, command),
		Deviation: auditRepo.NewDeviationEventRepository(auditRootRuntime(db)),
	}
}

type TimeTrackingCleanupRepositories struct {
	Session  timerecords.WorkSessionRepository
	Absence  timerecords.StaffAbsenceRepository
	Deletion auditModels.DataDeletionRepository
}

func NewTimeTrackingCleanupRepositories(db *bun.DB, command auditModels.Command) TimeTrackingCleanupRepositories {
	deletions := auditRepo.NewDataDeletionRepository(auditRootRuntime(db))
	membership, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup repositories: compose school membership: %v", err))
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		panic(fmt.Sprintf("time tracking cleanup repositories: compose workforce: %v", err))
	}
	return TimeTrackingCleanupRepositories{
		Session: workforceLegacy.NewWorkSessionRepository(workTime), Absence: workforceLegacy.NewStaffAbsenceRepository(workTime),
		Deletion: RouteDataDeletionWrites(deletions, command),
	}
}

type CleanupSettingsRepositories struct {
	Value configModels.SettingValueRepository
	Audit configModels.SettingAuditRepository
}

func NewCleanupSettingsRepositories(db *bun.DB, runtime configRepo.Runtime) CleanupSettingsRepositories {
	return CleanupSettingsRepositories{
		Value: configRepo.NewSettingValueRepository(runtime), Audit: configRepo.NewSettingAuditRepository(runtime),
	}
}

func NewSettingsCommandRepository(db *bun.DB) configModels.SettingValueRepository {
	return configRepo.NewSettingValueRepository(configRepo.NewRuntime(db))
}
