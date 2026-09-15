// Package compose wires the student deletion workflow over the owner
// capabilities and the shared tenant runtime (#2710).
package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	auditpostgres "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	appointmentscompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	enrollmentcompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	identitycompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	structurecompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	presencecompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetableprojection"
	"github.com/moto-nrw/project-phoenix/realtime"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
	"github.com/uptrace/bun"
)

// Feedback is the Feedback owner's deletion count.
type Feedback interface {
	CountForStudent(context.Context, int64) (int, error)
}

// Dependencies name the owner capabilities the composition does not build
// itself: the bound People Directory, Care Plan and Timetable of the running
// graph, the Feedback module, and the runtime hooks.
type Dependencies struct {
	DB        *bun.DB
	Directory studentdeletion.Directory
	CarePlan  studentdeletion.CarePlan
	Timetable studentdeletion.Timetable
	Feedback  Feedback
	// IsVerifiedStaff reports whether the caller has a staff record in the
	// current tenant; guests and guardians authenticate against the same
	// portal, so a permission alone never authorizes a deletion.
	IsVerifiedStaff       func(context.Context) (bool, error)
	LockCareBookingWrites func(context.Context) error
	// UnlinkPhoto schedules the removal of a stored photo after the outer
	// transaction commits. Optional: nil keeps the bytes (bare compositions).
	UnlinkPhoto func(context.Context, string)
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
	Audit       auditModels.Command
	Today       func() string
	Now         func() time.Time
}

// Authorize applies the tenant-portal permission boundary: a tenant principal
// of the current tenant holding users:delete who is an administrator or a
// verified staff member. The HTTP adapter additionally checks that the target
// row exists before it reaches the workflow.
func Authorize(ctx context.Context, isVerifiedStaff func(context.Context) (bool, error)) (studentdeletion.Actor, error) {
	principal, err := permissions.PrincipalFromContext(ctx)
	if err != nil || principal.Scope() != permissions.ScopeTenant ||
		principal.TenantID() != tenant.FromContext(ctx) || !principal.HasPermission(permissions.UsersDelete) {
		return studentdeletion.Actor{}, studentdeletion.ErrUnauthorized
	}
	if !principal.HasAdminScope() {
		staff, err := isVerifiedStaff(ctx)
		if err != nil || !staff {
			return studentdeletion.Actor{}, studentdeletion.ErrUnauthorized
		}
	}
	return studentdeletion.Actor{TenantID: principal.TenantID(), AccountID: principal.AccountID()}, nil
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
func New(deps Dependencies) (*studentdeletion.Workflow, error) {
	assembled, err := Assemble(deps)
	if err != nil {
		return nil, err
	}
	return studentdeletion.New(assembled)
}

// Assemble binds the workflow ports without constructing the workflow, so a
// test can wrap one owner command (to inject a failure after it) and still
// run every other owner for real.
func Assemble(deps Dependencies) (studentdeletion.Dependencies, error) {
	for name, missing := range map[string]bool{
		"database": deps.DB == nil, "people directory": deps.Directory == nil, "care plan": deps.CarePlan == nil,
		"timetable": deps.Timetable == nil, "feedback": deps.Feedback == nil,
		"staff verification": deps.IsVerifiedStaff == nil, "care booking lock": deps.LockCareBookingWrites == nil,
	} {
		if missing {
			return studentdeletion.Dependencies{}, fmt.Errorf("student deletion composition: %s is required", name)
		}
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	today := deps.Today
	if today == nil {
		today = func() string { return timezone.TodayDate().String() }
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	audit := deps.Audit
	if audit == nil {
		command, err := auditSvc.NewCommand(auditpostgres.NewAppender(ambientTransaction), func(event auditSvc.AppendObservation) {
			if event.Err != nil {
				logger.Error("student deletion audit append failed",
					"event_type", event.EventType,
					"error", event.Err,
				)
			}
		})
		if err != nil {
			return studentdeletion.Dependencies{}, err
		}
		audit = command
	}
	presence, err := presencecompose.New(presencecompose.Dependencies{DB: deps.DB, Observe: func(presencecompose.Observation) {}})
	if err != nil {
		return studentdeletion.Dependencies{}, err
	}
	appointments, err := appointmentscompose.New(appointmentscompose.Dependencies{DB: deps.DB, Observe: func(appointmentscompose.Observation) {}})
	if err != nil {
		return studentdeletion.Dependencies{}, err
	}
	structure, err := structurecompose.New(structurecompose.Dependencies{DB: deps.DB, Observe: func(structurecompose.Observation) {}})
	if err != nil {
		return studentdeletion.Dependencies{}, err
	}
	identity, err := identitycompose.New(identitycompose.Dependencies{DB: deps.DB, Observe: func(identitycompose.Observation) {}})
	if err != nil {
		return studentdeletion.Dependencies{}, err
	}
	enrollment := enrollmentcompose.New()
	auditReferences := auditpostgres.NewStudentDeletionRepository(ambientTransaction)
	return studentdeletion.Dependencies{
		UnitOfWork: tenant.NewTransactionRunner().RunInTx,
		Authorize: func(ctx context.Context) (studentdeletion.Actor, error) {
			return Authorize(ctx, deps.IsVerifiedStaff)
		},
		Today: today, Now: now,
		Directory: deps.Directory, CarePlan: deps.CarePlan, Timetable: deps.Timetable, Structure: structure,
		Conversations:   parentstore.NewStudentConversations(deps.DB),
		CountAttendance: presence.CountAttendanceRecords,
		CountVisits:     presence.CountStudentVisitsForDeletion,
		CountConsents: func(ctx context.Context, studentID int64) (int, error) {
			consents, err := presence.ListPrivacyConsents(ctx, studentID)
			return len(consents), err
		},
		CountEnrollmentReferences: enrollment.CountStudentReferences,
		CountActivityEnrollments: func(ctx context.Context, studentID int64) (int, error) {
			db, tenantID := ambientTransaction(ctx)
			if db == nil {
				return 0, errors.New("student deletion: activity enrollment count requires a tenant transaction")
			}
			return timetableprojection.CountStudentEnrollments(ctx, db, tenantID, studentID)
		},
		CountAppointments:        appointments.CountAppointmentRecipientStudents,
		CountFeedback:            deps.Feedback.CountForStudent,
		CountAuditReferences:     auditReferences.CountStudentReferences,
		CountGuardianInvitations: identity.CountStudentGuardianInvitations,
		LockCareBookingWrites:    deps.LockCareBookingWrites,
		AppendAudit: func(ctx context.Context, actor studentdeletion.Actor, result studentdeletion.Result) error {
			deletedBy := "account:" + strconv.FormatInt(actor.AccountID, 10)
			dataDeletion := auditModels.NewDataDeletion(result.StudentID, auditModels.DeletionTypeManual, result.Counts.Total()+result.PrimaryRowsDeleted, deletedBy)
			dataDeletion.DeletionReason = result.Reason
			dataDeletion.SetTenantID(actor.TenantID)
			dataDeletion.SetMetadata("student_deletion", true)
			dataDeletion.SetMetadata("counts", result.Counts)
			if err := audit.Append(ctx, dataDeletion); err != nil {
				return err
			}
			tombstone := &auditModels.StudentDeletion{
				StudentID: result.StudentID, ActorAccountID: actor.AccountID, Reason: result.Reason,
				Counts: auditModels.StudentDeletionCounts(result.Counts),
			}
			tombstone.SetTenantID(actor.TenantID)
			return audit.Append(ctx, tombstone)
		},
		PhotoRemoved: func(ctx context.Context, _ studentdeletion.Actor, path string) {
			if deps.UnlinkPhoto != nil {
				deps.UnlinkPhoto(ctx, path)
			}
		},
		CompanionsChanged: func(ctx context.Context, actor studentdeletion.Actor, studentID int64) {
			if deps.Broadcaster == nil {
				return
			}
			// After the OUTER transaction commits, like every other companion
			// broadcast: a subscriber woken earlier would refetch the
			// still-present row.
			tenant.RegisterAfterCommit(ctx, func() {
				source := "manual"
				event := realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{Source: &source})
				if err := deps.Broadcaster.BroadcastToTenant(actor.TenantID, event); err != nil {
					logger.Warn("failed to broadcast student companions change",
						"tenant_id", actor.TenantID,
						"student_id", studentID,
						"error", err.Error(),
					)
				}
			})
		},
		Observe: func(event studentdeletion.Observation) {
			logger.Debug("student deletion operation",
				"operation", event.Operation,
				"duration", event.Duration,
				"student_id", event.Result.StudentID,
				"error", event.Err,
			)
		},
	}, nil
}
