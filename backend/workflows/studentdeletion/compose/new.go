// Package compose wires the student deletion workflow over the owner
// capabilities and the shared tenant runtime (#2710).
package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	// Audit overrides the append command of Owners.
	Audit AuditLog
	Today func() string
	Now   func() time.Time
	// Owners are the owners the root builds for the deletion; the composition
	// builds no other owner's composition (#4004, ADR 0059 point 6).
	Owners Owners
}

// Owners are the School Structure, Communication, Enrollment, Student
// Presence, Appointments and Identity & Access capabilities the deletion
// counts in and deletes through.
type Owners struct {
	Structure     studentdeletion.Structure
	Conversations studentdeletion.Conversations
	Enrollment    studentdeletion.Enrollment
	// The counters read the child's Student Presence attendance, visits and
	// consents, appointment recipients and pending guardian invitations.
	Attendance, Visits, Consents      studentdeletion.Counter
	Appointments, GuardianInvitations studentdeletion.Counter
	// Audit appends the deletion's audit records, as DeletionRecords builds
	// them; AuditReferences counts the child's audit rows.
	Audit           AuditLog
	DeletionRecords func(studentdeletion.Actor, studentdeletion.Result) []any
	AuditReferences studentdeletion.Counter
}

// AuditLog is the Audit owner's append command.
type AuditLog interface {
	Append(context.Context, any) error
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

// New constructs only the required owner capabilities, without a factory.
func New(deps Dependencies, enrollments studentdeletion.ActivityEnrollments) (*studentdeletion.Workflow, error) {
	assembled, err := Assemble(deps, enrollments)
	if err != nil {
		return nil, err
	}
	return studentdeletion.New(assembled)
}

// Assemble binds the workflow ports without constructing the workflow, so a
// test can wrap one owner command and still run every other owner for real.
func Assemble(deps Dependencies, enrollments studentdeletion.ActivityEnrollments) (studentdeletion.Dependencies, error) {
	for name, missing := range map[string]bool{
		"database": deps.DB == nil, "activity enrollments": enrollments == nil, "people directory": deps.Directory == nil, "care plan": deps.CarePlan == nil,
		"timetable": deps.Timetable == nil, "feedback": deps.Feedback == nil,
		"staff verification": deps.IsVerifiedStaff == nil, "care booking lock": deps.LockCareBookingWrites == nil,
		"school structure": deps.Owners.Structure == nil, "conversations": deps.Owners.Conversations == nil,
		"enrollment": deps.Owners.Enrollment == nil, "attendance": deps.Owners.Attendance == nil,
		"visits": deps.Owners.Visits == nil, "consents": deps.Owners.Consents == nil,
		"appointments": deps.Owners.Appointments == nil, "guardian invitations": deps.Owners.GuardianInvitations == nil,
		"audit": deps.Audit == nil && deps.Owners.Audit == nil, "deletion records": deps.Owners.DeletionRecords == nil,
		"audit references": deps.Owners.AuditReferences == nil,
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
		today = func() string { return calendar.TodayDate().String() }
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	owners := deps.Owners
	return studentdeletion.Dependencies{
		UnitOfWork: tenant.NewTransactionRunner().RunInTx,
		Authorize: func(ctx context.Context) (studentdeletion.Actor, error) {
			return Authorize(ctx, deps.IsVerifiedStaff)
		},
		Today: today, Now: now,
		Directory: deps.Directory, CarePlan: deps.CarePlan, Timetable: deps.Timetable, Structure: owners.Structure,
		Conversations: owners.Conversations, Enrollment: owners.Enrollment,
		CountAttendance:          owners.Attendance,
		CountVisits:              owners.Visits,
		CountConsents:            owners.Consents,
		CountActivityEnrollments: enrollments.CountStudentEnrollments,
		CountAppointments:        owners.Appointments,
		CountFeedback:            deps.Feedback.CountForStudent,
		CountAuditReferences:     owners.AuditReferences,
		CountGuardianInvitations: owners.GuardianInvitations,
		LockCareBookingWrites:    deps.LockCareBookingWrites,
		AppendAudit:              appendDeletionAudit(deps.Audit, owners),
		PhotoRemoved: func(ctx context.Context, _ studentdeletion.Actor, path string) {
			if deps.UnlinkPhoto != nil {
				deps.UnlinkPhoto(ctx, path)
			}
		},
		CompanionsChanged: func(ctx context.Context, actor studentdeletion.Actor, studentID int64) {
			if deps.Broadcaster == nil {
				return
			}
			// After the OUTER commit: an earlier subscriber would refetch the row.
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

// appendDeletionAudit appends the deletion's audit records through audit,
// or the Owners' command without one.
func appendDeletionAudit(audit AuditLog, owners Owners) func(context.Context, studentdeletion.Actor, studentdeletion.Result) error {
	if audit == nil {
		audit = owners.Audit
	}
	return func(ctx context.Context, actor studentdeletion.Actor, result studentdeletion.Result) error {
		for _, record := range owners.DeletionRecords(actor, result) {
			if err := audit.Append(ctx, record); err != nil {
				return err
			}
		}
		return nil
	}
}

// EnrollmentDatabase supplies the deletion's strict ambient transaction to the root.
func EnrollmentDatabase() func(context.Context) (bun.IDB, int64, error) {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		value, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("student deletion: activity enrollment count requires a tenant transaction")
		}
		tx, ok := value.(bun.Tx)
		if !ok {
			return nil, 0, errors.New("student deletion: activity enrollment count requires a tenant transaction")
		}
		return tx, tenant.FromContext(ctx), nil
	}
}
