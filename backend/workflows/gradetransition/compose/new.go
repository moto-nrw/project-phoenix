// Package compose wires the grade transition workflow over the owner
// capabilities and the shared tenant runtime (#2711).
package compose

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	auditpostgres "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	structurecompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	presencecompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/uptrace/bun"
)

// Dependencies name the owner capabilities the composition does not build
// itself: the bound People Directory and School Membership of the running
// graph, the Timetable roster reconciliation, the recurrence gate and the
// offering-roster resync, plus the runtime hooks. School Structure and
// Student Presence are composed here over the shared database.
type Dependencies struct {
	DB         *bun.DB
	Directory  gradetransition.Directory
	Membership gradetransition.Membership
	Rosters    gradetransition.Rosters
	// LockRecurrenceWrites is the tenant-wide recurrence gate every timetable
	// writer takes; apply and revert hold it after the class-writes gate.
	LockRecurrenceWrites func(context.Context) error
	// ResyncOfferingRosters re-reconciles the offering-sourced timetable
	// templates from the given calendar day after the class rewrite.
	ResyncOfferingRosters func(context.Context, timezone.Date) error
	Logger                *slog.Logger
	// Audit records the class-list rewrites; nil binds the ambient-transaction
	// audit command.
	Audit auditModels.Command
	Clock func() time.Time
}

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
func Authorize(ctx context.Context, operation string) (gradetransition.Actor, error) {
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

func ambientTransaction(ctx context.Context) (bun.IDB, int64) {
	value, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0
	}
	tx, ok := value.(bun.Tx)
	if !ok {
		return nil, 0
	}
	return tx, tenant.FromContext(ctx)
}

// New constructs only the required owner capabilities. It does not build or
// retain a service/repository factory.
func New(deps Dependencies) (*gradetransition.Workflow, error) {
	assembled, err := Assemble(deps)
	if err != nil {
		return nil, err
	}
	return gradetransition.New(assembled)
}

// Assemble binds the workflow ports without constructing the workflow, so a
// test can wrap one owner command (to inject a failure after it) and still
// run every other owner for real.
func Assemble(deps Dependencies) (gradetransition.Dependencies, error) {
	for name, missing := range map[string]bool{
		"database": deps.DB == nil, "people directory": deps.Directory == nil, "school membership": deps.Membership == nil,
		"roster reconciliation": deps.Rosters == nil, "recurrence lock": deps.LockRecurrenceWrites == nil,
		"offering roster resync": deps.ResyncOfferingRosters == nil,
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
		clock = timezone.Now
	}
	audit := deps.Audit
	if audit == nil {
		command, err := auditSvc.NewCommand(auditpostgres.NewAppender(ambientTransaction), func(event auditSvc.AppendObservation) {
			if event.Err != nil {
				logger.Error("grade transition audit append failed",
					"event_type", event.EventType,
					"error", event.Err,
				)
			}
		})
		if err != nil {
			return gradetransition.Dependencies{}, err
		}
		audit = command
	}
	structure, err := structurecompose.New(structurecompose.Dependencies{DB: deps.DB, Observe: func(structurecompose.Observation) {}})
	if err != nil {
		return gradetransition.Dependencies{}, err
	}
	presence, err := presencecompose.New(presencecompose.Dependencies{DB: deps.DB, Observe: func(presencecompose.Observation) {}})
	if err != nil {
		return gradetransition.Dependencies{}, err
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
		Authorize: Authorize,
		Today:     func() string { return timezone.DateFromTime(clock()).String() },
		Now:       clock,
		Structure: structure, Directory: deps.Directory, Membership: deps.Membership, Presence: presence, Rosters: deps.Rosters,
		LockRecurrenceWrites: deps.LockRecurrenceWrites,
		ResyncOfferingRosters: func(ctx context.Context, effectiveFrom string) error {
			day, err := timezone.ParseDate(effectiveFrom)
			if err != nil {
				return err
			}
			return deps.ResyncOfferingRosters(ctx, day)
		},
		AppendClassListEntryAudit: func(ctx context.Context, actor gradetransition.Actor, change gradetransition.ClassListEntryAudit) error {
			event := &auditModels.ClassListEntryChange{
				EntryID: change.EntryID, Action: change.Action, OldValue: change.OldValue, NewValue: change.NewValue,
				ChangedBy: actor.AccountID,
			}
			event.SetTenantID(actor.TenantID)
			return audit.Append(ctx, event)
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
