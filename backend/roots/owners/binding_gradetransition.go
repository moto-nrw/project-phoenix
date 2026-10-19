package owners

import (
	"context"
	"fmt"
	"log/slog"

	gradetransitioncompose "github.com/moto-nrw/project-phoenix/workflows/gradetransition/compose"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
)

// operationPermissions maps every workflow operation onto the tenant
// permission the HTTP surface enforces for the same route.
var operationPermissions = map[string]string{
	gradetransition.OperationRead:   permissions.GradeTransitionsRead,
	gradetransition.OperationCreate: permissions.GradeTransitionsCreate,
	gradetransition.OperationUpdate: permissions.GradeTransitionsUpdate,
	gradetransition.OperationDelete: permissions.GradeTransitionsDelete,
	gradetransition.OperationApply:  permissions.GradeTransitionsApply,
}

// Authorize applies the tenant-portal permission boundary: a tenant principal
// of the current tenant holding the operation's grade-transition permission.
func AuthorizeGradeTransition(ctx context.Context, operation string) (gradetransition.Actor, error) {
	required, known := operationPermissions[operation]
	if !known {
		return gradetransition.Actor{}, gradetransition.ErrUnauthorized
	}
	principal, err := permissions.PrincipalFromContext(ctx)
	if err != nil || principal.Scope() != permissions.ScopeTenant ||
		principal.TenantID() != tenant.FromContext(ctx) || !principal.HasPermission(required) {
		return gradetransition.Actor{}, gradetransition.ErrUnauthorized
	}
	return gradetransition.Actor{TenantID: principal.TenantID(), AccountID: principal.AccountID()}, nil
}

// New constructs only the required owner capabilities. It does not build or
// retain a service/repository factory.
func NewGradeTransition(deps gradetransitioncompose.Dependencies) (*gradetransition.Workflow, error) {
	assembled, err := AssembleGradeTransition(deps)
	if err != nil {
		return nil, err
	}
	return gradetransitioncompose.New(assembled)
}

// Assemble binds the workflow ports without constructing the workflow, so a
// test can wrap one owner command (to inject a failure after it) and still
// run every other owner for real.
func AssembleGradeTransition(deps gradetransitioncompose.Dependencies) (gradetransition.Dependencies, error) {
	for name, missing := range map[string]bool{
		"database": deps.DB == nil, "school structure": deps.Structure == nil, "student presence": deps.Presence == nil,
		"people directory": deps.Directory == nil, "school membership": deps.Membership == nil,
		"roster reconciliation": deps.Rosters == nil, "recurrence lock": deps.LockRecurrenceWrites == nil,
		"offering roster resync": deps.ResyncOfferingRosters == nil, "audit": deps.Audit == nil,
		"class list entry record": deps.ClassListEntryRecord == nil,
	} {
		if missing {
			return gradetransition.Dependencies{}, fmt.Errorf("grade transition composition: %s is required", name)
		}
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	clock := deps.Clock
	if clock == nil {
		clock = calendar.Now
	}
	runner := tenant.NewTransactionRunner()
	return gradetransition.Dependencies{
		// Joining an ambient request transaction hands the commit to the route
		// middleware, which commits on every non-5xx response. A refused
		// command after partial owner writes (a stale preview seen late, a
		// status conflict) therefore requests the rollback here, so the
		// guarantee does not depend on the HTTP adapter remembering to.
		UnitOfWork: func(ctx context.Context, fn func(context.Context) error) error {
			err := runner.RunInTx(ctx, fn)
			if err != nil {
				tenant.MarkRollback(ctx)
			}
			return err
		},
		Authorize: AuthorizeGradeTransition,
		Today:     func() string { return calendar.DateFromTime(clock()).String() },
		Now:       clock,
		Structure: deps.Structure, Directory: deps.Directory, Membership: deps.Membership, Presence: deps.Presence, Rosters: deps.Rosters,
		LockRecurrenceWrites: deps.LockRecurrenceWrites,
		ResyncOfferingRosters: func(ctx context.Context, effectiveFrom string) error {
			day, err := calendar.ParseDate(effectiveFrom)
			if err != nil {
				return err
			}
			return deps.ResyncOfferingRosters(ctx, day)
		},
		AppendClassListEntryAudit: func(ctx context.Context, actor gradetransition.Actor, change gradetransition.ClassListEntryAudit) error {
			return deps.Audit.Append(ctx, deps.ClassListEntryRecord(actor, change))
		},
		Observe: func(event gradetransition.Observation) {
			logger.Debug("grade transition operation",
				"operation", event.Operation,
				"duration", event.Duration,
				"transition_id", event.TransitionID,
				"error", event.Err,
			)
		},
	}, nil
}
