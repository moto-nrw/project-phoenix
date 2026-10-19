package owners

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// Authorize applies the tenant-portal permission boundary: a tenant principal
// of the current tenant holding users:delete who is an administrator or a
// verified staff member. The HTTP adapter additionally checks that the target
// row exists before it reaches the workflow.
func AuthorizeStudentDeletion(ctx context.Context, isVerifiedStaff func(context.Context) (bool, error)) (studentdeletion.Actor, error) {
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
func NewStudentDeletion(deps studentdeletioncompose.Dependencies, enrollments studentdeletion.ActivityEnrollments) (*studentdeletion.Workflow, error) {
	assembled, err := Assemble(deps, enrollments)
	if err != nil {
		return nil, err
	}
	return studentdeletioncompose.New(assembled)
}

// Assemble binds the workflow ports without constructing the workflow, so a
// test can wrap one owner command and still run every other owner for real.
func Assemble(deps studentdeletioncompose.Dependencies, enrollments studentdeletion.ActivityEnrollments) (studentdeletion.Dependencies, error) {
	if err := validateStudentDeletionComposition(deps, enrollments); err != nil {
		return studentdeletion.Dependencies{}, err
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
	return studentdeletion.Dependencies{
		UnitOfWork: tenant.NewTransactionRunner().RunInTx,
		Authorize: func(ctx context.Context) (studentdeletion.Actor, error) {
			return AuthorizeStudentDeletion(ctx, deps.IsVerifiedStaff)
		},
		Today: today, Now: now,
		Directory: deps.Directory, CarePlan: deps.CarePlan, Timetable: deps.Timetable, Structure: deps.Owners.Structure,
		Conversations: deps.Owners.Conversations, Enrollment: deps.Owners.Enrollment,
		CountAttendance:          deps.Owners.Attendance,
		CountVisits:              deps.Owners.Visits,
		CountConsents:            deps.Owners.Consents,
		CountActivityEnrollments: enrollments.CountStudentEnrollments,
		CountAppointments:        deps.Owners.Appointments,
		CountFeedback:            deps.Feedback.CountForStudent,
		CountAuditReferences:     deps.Owners.AuditReferences,
		CountGuardianInvitations: deps.Owners.GuardianInvitations,
		LockCareBookingWrites:    deps.LockCareBookingWrites,
		AppendAudit:              appendDeletionAudit(deps.Audit, deps.Owners),
		PhotoRemoved:             photoRemoval(deps.UnlinkPhoto),
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
		Observe: studentDeletionObserver(logger),
	}, nil
}

// appendDeletionAudit appends the deletion's audit records through audit,
// or the studentdeletioncompose.Owners' command without one.
func appendDeletionAudit(audit studentdeletioncompose.AuditLog, owners studentdeletioncompose.Owners) func(context.Context, studentdeletion.Actor, studentdeletion.Result) error {
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

func validateStudentDeletionComposition(deps studentdeletioncompose.Dependencies, enrollments studentdeletion.ActivityEnrollments) error {
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
			return fmt.Errorf("student deletion composition: %s is required", name)
		}
	}
	return nil
}

func photoRemoval(unlink func(context.Context, string) error) func(context.Context, studentdeletion.Actor, string) error {
	return func(ctx context.Context, _ studentdeletion.Actor, path string) error {
		if unlink != nil {
			return unlink(ctx, path)
		}
		return nil
	}
}

func studentDeletionObserver(logger *slog.Logger) func(studentdeletion.Observation) {
	return func(event studentdeletion.Observation) {
		logger.Debug("student deletion operation",
			"operation", event.Operation,
			"duration", event.Duration,
			"student_id", event.Result.StudentID,
			"error", event.Err,
		)
	}
}
