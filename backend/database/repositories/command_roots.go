package repositories

import (
	"context"
	"fmt"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
	School                 platformModels.SchoolRepository
	Person                 userModels.PersonRepository
	PasswordResetRateLimit authModels.PasswordResetRateLimitRepository
	AuthEvent              auditModels.AuthEventRepository
	PushSubscription       deliveryModels.PushSubscriptionRepository
}

func NewAuthCleanupRepositories(db *bun.DB, command auditModels.Command) AuthCleanupRepositories {
	authEvents := auditRepo.NewAuthEventRepository(auditRootRuntime(db))
	return AuthCleanupRepositories{
		School: NewSchoolRepository(db), Person: NewPersonRepository(db),
		PasswordResetRateLimit: authRepo.NewPasswordResetRateLimitRepository(db),
		AuthEvent:              RouteAuthEventWrites(authEvents, command), PushSubscription: deliveryCompose.NewPushSubscriptionRepository(db),
	}
}

func NewInvitationCleanupRepository(db *bun.DB) authModels.InvitationTokenRepository {
	return authRepo.NewInvitationTokenRepository(db)
}

type SessionCleanupRepositories struct {
	Group           activeModels.GroupRepository
	Supervisor      activeModels.GroupSupervisorRepository
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
	group := presenceCompose.NewLegacyGroupRepository(activeDeviceDirectory{devices: fleet}, NewPresenceGroupRecords(db), NewSessionActivities(timetableActivityGroupRepository{timetable: timetableCapability}),
		presenceCompose.WithLegacyRoomDirectory(&activeRoomDirectory{rooms: rooms}))
	return SessionCleanupRepositories{
		Group: group, Supervisor: presenceCompose.NewLegacyGroupSupervisorRepository(NewPresenceSupervisionRecords(db)), Device: device,
		TimetableBridge: timetableActivityInstanceRepository{timetable: timetableCapability},
	}
}

type RetentionCleanupRepositories struct {
	Supervisor activeModels.GroupSupervisorRepository
	Deletion   auditModels.DataDeletionRepository
}

func NewRetentionCleanupRepositories(db *bun.DB, command auditModels.Command) RetentionCleanupRepositories {
	deletions := auditRepo.NewDataDeletionRepository(auditRootRuntime(db))
	return RetentionCleanupRepositories{
		Supervisor: presenceCompose.NewLegacyGroupSupervisorRepository(NewPresenceSupervisionRecords(db)),
		Deletion:   RouteDataDeletionWrites(deletions, command),
	}
}

type TimetableCleanupRepositories struct {
	Instance  scheduleModels.ActivityInstanceRepository
	Exception scheduleModels.ActivityExceptionRepository
	Student   scheduleModels.InstanceStudentRepository
	Deletion  auditModels.DataDeletionRepository
	Deviation auditModels.DeviationEventRepository
}

func NewTimetableCleanupRepositories(db *bun.DB, command auditModels.Command, timetableCapability timetable.Capability) TimetableCleanupRepositories {
	if timetableCapability == nil {
		panic("timetable cleanup repositories: timetable capability is required")
	}
	deletions := auditRepo.NewDataDeletionRepository(auditRootRuntime(db))
	return TimetableCleanupRepositories{
		Instance:  timetableActivityInstanceRepository{timetable: timetableCapability},
		Exception: timetableActivityExceptionRepository{timetable: timetableCapability},
		Student:   timetableInstanceStudentRepository{timetable: timetableCapability}, Deletion: RouteDataDeletionWrites(deletions, command),
		Deviation: auditRepo.NewDeviationEventRepository(auditRootRuntime(db)),
	}
}

type TimeTrackingCleanupRepositories struct {
	Session  activeModels.WorkSessionRepository
	Absence  activeModels.StaffAbsenceRepository
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
