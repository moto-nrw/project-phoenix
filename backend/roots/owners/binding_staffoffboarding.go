package owners

import (
	"context"
	"errors"
	"log/slog"

	staffoffboardingcompose "github.com/moto-nrw/project-phoenix/workflows/staffoffboarding/compose"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
)

// Authorize applies the tenant-portal permission boundary before the workflow
// reads its subject. The actor resolver must use that same tenant context.
func AuthorizeStaffOffboarding(ctx context.Context, resolveActor func(context.Context) (int64, error), username string) (staffoffboarding.Actor, error) {
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
func NewStaffOffboarding(deps staffoffboardingcompose.Dependencies, work staffoffboarding.Workforce, groupAccessChanged func(context.Context)) (*staffoffboarding.Workflow, error) {
	if deps.DB == nil || deps.Access == nil || deps.Membership == nil || deps.Retirement == nil || deps.Timetable == nil || deps.LockSupervision == nil || deps.Authorize == nil || deps.Cleanup == nil ||
		deps.Audit == nil || deps.AbsenceDeletionRecord == nil || deps.StaffDeletionRecord == nil {
		return nil, errors.New("staff offboarding composition: required dependency is missing")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	membership := deps.Membership
	return staffoffboardingcompose.New(staffoffboarding.Dependencies{
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
		Cleanup:            deps.Cleanup,
		GroupAccessChanged: groupAccessChanged,
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
