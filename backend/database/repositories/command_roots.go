package repositories

import (
	"context"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	iotRepo "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
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

type AuthCleanupRepositories struct {
	Account                authModels.AccountRepository
	Token                  authModels.TokenRepository
	PasswordResetRateLimit authModels.PasswordResetRateLimitRepository
	AuthEvent              auditModels.AuthEventRepository
	PushSubscription       deliveryModels.PushSubscriptionRepository
}

func NewAuthCleanupRepositories(db *bun.DB, command auditModels.Command) AuthCleanupRepositories {
	authEvents := auditRepo.NewAuthEventRepository(auditRootRuntime(db))
	return AuthCleanupRepositories{
		Account: authRepo.NewAccountRepository(db), Token: authRepo.NewTokenRepository(db),
		PasswordResetRateLimit: authRepo.NewPasswordResetRateLimitRepository(db),
		AuthEvent:              RouteAuthEventWrites(authEvents, command), PushSubscription: deliveryCompose.NewPushSubscriptionRepository(db),
	}
}

func NewInvitationCleanupRepository(db *bun.DB) authModels.InvitationTokenRepository {
	return authRepo.NewInvitationTokenRepository(db)
}

type SessionCleanupRepositories struct {
	Group           activeModels.GroupRepository
	Visit           activeModels.VisitRepository
	Supervisor      activeModels.GroupSupervisorRepository
	Device          iotModels.DeviceRepository
	TimetableBridge *scheduleRepo.ActivityInstanceRepository
}

func NewSessionCleanupRepositories(db *bun.DB) SessionCleanupRepositories {
	return SessionCleanupRepositories{
		Group: activeRepo.NewGroupRepository(db), Visit: activeRepo.NewVisitRepository(db),
		Supervisor: activeRepo.NewGroupSupervisorRepository(db), Device: iotRepo.NewDeviceRepository(db),
		TimetableBridge: scheduleRepo.NewActivityInstanceRepository(db),
	}
}

type RetentionCleanupRepositories struct {
	Visit      activeModels.VisitRepository
	Attendance activeModels.AttendanceRepository
	Supervisor activeModels.GroupSupervisorRepository
	Consent    usersModels.PrivacyConsentRepository
	Deletion   auditModels.DataDeletionRepository
}

func NewRetentionCleanupRepositories(db *bun.DB, command auditModels.Command) RetentionCleanupRepositories {
	deletions := auditRepo.NewDataDeletionRepository(auditRootRuntime(db))
	return RetentionCleanupRepositories{
		Visit: activeRepo.NewVisitRepository(db), Attendance: activeRepo.NewAttendanceRepository(db),
		Supervisor: activeRepo.NewGroupSupervisorRepository(db), Consent: usersRepo.NewPrivacyConsentRepository(db),
		Deletion: RouteDataDeletionWrites(deletions, command),
	}
}

type TimetableCleanupRepositories struct {
	Instance  scheduleModels.ActivityInstanceRepository
	Exception scheduleModels.ActivityExceptionRepository
	Student   scheduleModels.InstanceStudentRepository
	Deletion  auditModels.DataDeletionRepository
	Deviation auditModels.DeviationEventRepository
}

func NewTimetableCleanupRepositories(db *bun.DB, command auditModels.Command) TimetableCleanupRepositories {
	deletions := auditRepo.NewDataDeletionRepository(auditRootRuntime(db))
	return TimetableCleanupRepositories{
		Instance: scheduleRepo.NewActivityInstanceRepository(db), Exception: scheduleRepo.NewActivityExceptionRepository(db),
		Student: scheduleRepo.NewInstanceStudentRepository(db), Deletion: RouteDataDeletionWrites(deletions, command),
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
	return TimeTrackingCleanupRepositories{
		Session: activeRepo.NewWorkSessionRepository(db), Absence: activeRepo.NewStaffAbsenceRepository(db),
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
