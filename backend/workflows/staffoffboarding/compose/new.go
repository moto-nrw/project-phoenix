package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforcecompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/uptrace/bun"
)

// StaffOffboardingPreview is the access snapshot the workflow decides on. It
// contains no credential material: the revision is an exact encoding of the
// access decisions, not an authorization credential.
type StaffOffboardingPreview struct {
	AccountID         int64
	ActiveMembership  bool
	RoleIDs           []int64
	Permissions       int64
	Tokens            int64
	PreserveGuardian  bool
	DeactivateAccount bool
	Revision          string
}

// StaffOffboardingResult is what the revocation actually removed.
type StaffOffboardingResult struct {
	RolesRevoked            int64
	PermissionsRevoked      int64
	TokensRevoked           int64
	GuardianAccessPreserved bool
	AccountDeactivated      bool
}

// Access is the consumer-owned port over the Identity & Access offboarding
// capability: the preview snapshots roles, permissions, sessions and
// membership at the school in context, the execution revokes them against
// that snapshot on the workflow's transaction. The composition root binds it
// (#3364).
type Access interface {
	PreviewStaffOffboarding(context.Context, int64) (StaffOffboardingPreview, error)
	ExecuteStaffOffboarding(context.Context, int64, string) (StaffOffboardingResult, error)
}

type Dependencies struct {
	DB     *bun.DB
	Access Access
	// People is the People Directory the person is unlinked in.
	People staffoffboarding.People
	// Membership, Retirement, Timetable and LockSupervision are the School
	// Membership, Timetable and Student Presence owners the root builds; the
	// composition builds no other owner's composition (#4004).
	Membership      schoolmembership.Query
	Retirement      staffoffboarding.Membership
	Timetable       staffoffboarding.Timetable
	LockSupervision func(context.Context, int64, string) ([]int64, error)
	Authorize       func(context.Context) (staffoffboarding.Actor, error)
	Cleanup         func(context.Context, int64) error
	Broadcaster     realtime.Broadcaster
	Logger          *slog.Logger
	// Audit appends the offboarding's audit records in the caller's
	// transaction, as AbsenceDeletionRecord and StaffDeletionRecord build
	// them; the root binds all three.
	Audit                 AuditLog
	AbsenceDeletionRecord func(workforce.StaffAbsence, int64) (any, error)
	StaffDeletionRecord   func(staffoffboarding.Actor, staffoffboarding.Result) any
}

// AuditLog is the Audit owner's append command.
type AuditLog interface {
	Append(context.Context, any) error
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
	if deps.DB == nil || deps.Access == nil || deps.Membership == nil || deps.Retirement == nil || deps.Timetable == nil || deps.LockSupervision == nil || deps.Authorize == nil || deps.Cleanup == nil ||
		deps.Audit == nil || deps.AbsenceDeletionRecord == nil || deps.StaffDeletionRecord == nil {
		return nil, errors.New("staff offboarding composition: required dependency is missing")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	membership := deps.Membership
	work, err := workforcecompose.NewOffboarding(workforcecompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB: deps.DB, Observe: func(workforcecompose.Observation) {},
		LiveStaffIDs: func(ctx context.Context, membershipIDs []int64) ([]int64, error) {
			rows, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: membershipIDs, MembershipOnly: true})
			if err != nil {
				return nil, err
			}
			ids := make([]int64, 0, len(rows))
			for _, row := range rows {
				ids = append(ids, row.ID)
			}
			return ids, nil
		},
	}, func(ctx context.Context, absence workforce.StaffAbsence, actorID int64) error {
		record, err := deps.AbsenceDeletionRecord(absence, actorID)
		if err != nil {
			return err
		}
		return deps.Audit.Append(ctx, record)
	})
	if err != nil {
		return nil, err
	}
	return staffoffboarding.New(staffoffboarding.Dependencies{
		UnitOfWork: tenant.NewTransactionRunner().RunInTx, Authorize: deps.Authorize,
		Today: func() string { return calendar.TodayDate().String() }, FindStaff: membership.FindStaff,
		Membership: deps.Retirement, Workforce: work, Timetable: deps.Timetable, People: deps.People,
		LockSupervision: deps.LockSupervision,
		PreviewAccess: func(ctx context.Context, id int64) (staffoffboarding.AccessPreview, error) {
			p, err := deps.Access.PreviewStaffOffboarding(ctx, id)
			return staffoffboarding.AccessPreview{Revision: p.Revision, Roles: int64(len(p.RoleIDs)), Permissions: p.Permissions, Tokens: p.Tokens, PreserveGuardian: p.PreserveGuardian, DeactivateAccount: p.DeactivateAccount}, err
		},
		ExecuteAccess: func(ctx context.Context, id int64, revision string) (staffoffboarding.AccessResult, error) {
			r, err := deps.Access.ExecuteStaffOffboarding(ctx, id, revision)
			return staffoffboarding.AccessResult(r), err
		},
		AppendAudit: func(ctx context.Context, actor staffoffboarding.Actor, result staffoffboarding.Result) error {
			return deps.Audit.Append(ctx, deps.StaffDeletionRecord(actor, result))
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
