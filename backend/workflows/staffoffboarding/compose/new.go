package compose

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	auditpostgres "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	peoplecompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	membershipcompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	presencecompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	timetablecompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforcecompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/uptrace/bun"
)

type Access interface {
	PreviewStaffOffboarding(context.Context, int64) (authSvc.StaffOffboardingPreview, error)
	ExecuteStaffOffboarding(context.Context, int64, string) (authSvc.StaffOffboardingResult, error)
}

type Dependencies struct {
	DB          *bun.DB
	Access      Access
	Authorize   func(context.Context) (staffoffboarding.Actor, error)
	Cleanup     func(context.Context, int64) error
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	Audit       auditModels.Command
}

// Authorize applies the tenant-portal permission boundary before the workflow
// reads its subject. The actor resolver must use that same tenant context.
func Authorize(ctx context.Context, resolveActor func(context.Context) (int64, error), username string) (staffoffboarding.Actor, error) {
	principal, err := permissions.PrincipalFromContext(ctx)
	if err != nil || principal.Scope() != permissions.ScopeTenant ||
		principal.TenantID() != tenant.FromContext(ctx) || !principal.HasPermission(permissions.UsersDelete) {
		return staffoffboarding.Actor{}, staffoffboarding.ErrUnauthorized
	}
	staffID, err := resolveActor(ctx)
	if err != nil {
		return staffoffboarding.Actor{}, err
	}
	return staffoffboarding.Actor{TenantID: principal.TenantID(), AccountID: principal.AccountID(), StaffID: staffID, Username: username}, nil
}

// New constructs only the required owner capabilities.
// It does not build or retain a service/repository factory.
func New(deps Dependencies) (*staffoffboarding.Workflow, error) {
	if deps.DB == nil || deps.Access == nil || deps.Authorize == nil || deps.Cleanup == nil {
		return nil, errors.New("staff offboarding composition: required dependency is missing")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	audit := deps.Audit
	if audit == nil {
		command, err := auditSvc.NewCommand(auditpostgres.NewAppender(func(ctx context.Context) (bun.IDB, int64) {
			value, ok := tenant.TransactionFromContext(ctx)
			if !ok {
				return nil, 0
			}
			tx, ok := value.(bun.Tx)
			if !ok {
				return nil, 0
			}
			return tx, tenant.FromContext(ctx)
		}), func(event auditSvc.AppendObservation) {
			if event.Err != nil {
				logger.Error("offboarding audit append failed",
					"event_type", event.EventType,
					"error", event.Err,
				)
			}
		})
		if err != nil {
			return nil, err
		}
		audit = command
	}
	memberDeps := membershipcompose.Dependencies{DB: deps.DB, Observe: func(membershipcompose.Observation) {}}
	membership, err := membershipcompose.New(memberDeps)
	if err != nil {
		return nil, err
	}
	retirement, err := membershipcompose.NewOffboarding(memberDeps)
	if err != nil {
		return nil, err
	}
	people, err := peoplecompose.New(peoplecompose.Dependencies{DB: deps.DB, Observe: func(peoplecompose.Observation) {}})
	if err != nil {
		return nil, err
	}
	presence, err := presencecompose.New(presencecompose.Dependencies{DB: deps.DB, Observe: func(presencecompose.Observation) {}})
	if err != nil {
		return nil, err
	}
	timetable, err := timetablecompose.NewOffboarding(timetablecompose.OffboardingDependencies{DB: deps.DB, Observe: func(timetablecompose.Observation) {}})
	if err != nil {
		return nil, err
	}
	work, err := workforcecompose.NewOffboarding(workforcecompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB: deps.DB, Observe: func(workforcecompose.Observation) {},
		AssignedStaffIDs: func(ctx context.Context, modelID int64) ([]int64, error) {
			rows, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{WorkTimeModelID: &modelID})
			if err != nil {
				return nil, err
			}
			ids := make([]int64, 0, len(rows))
			for _, row := range rows {
				ids = append(ids, row.ID)
			}
			return ids, nil
		},
		RebaseStaffAnchor: membership.RebaseWorkTimeModelAnchor,
	}, func(ctx context.Context, absence workforce.StaffAbsence, actorID int64) error {
		payload, err := json.Marshal(absence)
		if err != nil {
			return err
		}
		return audit.Append(ctx, &auditModels.TimeTrackingDeletion{StaffID: absence.StaffID, Source: auditModels.TimeTrackingDeletionSourceAbsence, SourceID: absence.ID, DeletedBy: actorID, Payload: payload, Note: "Personal-Offboarding"})
	})
	if err != nil {
		return nil, err
	}
	return staffoffboarding.New(staffoffboarding.Dependencies{
		UnitOfWork: tenant.NewTransactionRunner().RunInTx, Authorize: deps.Authorize,
		Today: func() string { return timezone.TodayDate().String() }, FindStaff: membership.FindStaff,
		Membership: retirement, Workforce: work, Timetable: timetable, People: people,
		LockSupervision: presence.LockStaffSupervision,
		PreviewAccess: func(ctx context.Context, id int64) (staffoffboarding.AccessPreview, error) {
			p, err := deps.Access.PreviewStaffOffboarding(ctx, id)
			return staffoffboarding.AccessPreview{Revision: p.Revision, Roles: int64(len(p.RoleIDs)), Permissions: p.Permissions, Tokens: p.Tokens, PreserveGuardian: p.PreserveGuardian, DeactivateAccount: p.DeactivateAccount}, err
		},
		ExecuteAccess: func(ctx context.Context, id int64, revision string) (staffoffboarding.AccessResult, error) {
			r, err := deps.Access.ExecuteStaffOffboarding(ctx, id, revision)
			return staffoffboarding.AccessResult(r), err
		},
		AppendAudit: func(ctx context.Context, actor staffoffboarding.Actor, result staffoffboarding.Result) error {
			counts := map[string]int64{
				"group_teacher": result.Membership.GroupAssignments, "class_teachers": result.Membership.ClassAssignments,
				"activity_supervisors": result.Timetable.PlannedSupervisors, "timetable_instance_staff": result.Timetable.InstanceAssignments,
				"group_substitutions": result.Workforce.Substitutions, "staff_shifts": result.Workforce.Shifts,
				"staff_shift_series": result.Workforce.Series, "staff_absences": result.Workforce.Absences,
				"account_permissions": result.Access.PermissionsRevoked,
			}
			records := 1
			for _, count := range counts {
				records += int(count)
			}
			deletedBy := actor.Username
			if deletedBy == "" {
				// Preserve the legacy provider's audit value for authenticated
				// sessions without a display name, including the API seeder.
				deletedBy = "system"
			}
			event := auditModels.NewStaffDataDeletion(result.StaffID, auditModels.DeletionTypeManual, records, deletedBy)
			event.DeletionReason = "staff offboarding"
			event.SetTenantID(actor.TenantID)
			for key, count := range counts {
				event.Metadata[key] = count
			}
			return audit.Append(ctx, event)
		},
		Cleanup: deps.Cleanup,
		GroupAccessChanged: func(ctx context.Context) {
			realtimeevents.QueueGroupAccessChanged(ctx, deps.Broadcaster, logger, "staff_offboarding")
		},
		Observe: func(event staffoffboarding.Observation) {
			logger.Debug("staff offboarding operation",
				"operation", event.Operation,
				"duration", event.Duration,
				"staff_id", event.Result.StaffID,
				"error", event.Err,
			)
		},
	})
}
