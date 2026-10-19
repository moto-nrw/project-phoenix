package owners

import (
	"context"
	"log/slog"

	appointmentsCompose "github.com/moto-nrw/project-phoenix/modules/appointments/compose"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	studentdeletioncompose "github.com/moto-nrw/project-phoenix/workflows/studentdeletion/compose"
)

// NewStudentDeletionOwners builds the owners the student deletion workflow
// counts in and deletes through, over db and without observation, as the
// workflow composition did before (#4004, ADR 0059 point 6), with the
// ambient-transaction audit command and the Audit owner's deletion records.
func NewStudentDeletionOwners(db transactionruntime.Database) (studentdeletioncompose.Owners, error) {
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: db.Pool(), Observe: func(presenceCompose.Observation) {}})
	if err != nil {
		return studentdeletioncompose.Owners{}, err
	}
	appointments, err := appointmentsCompose.New(appointmentsCompose.Dependencies{DB: db.Pool(), Observe: func(appointmentsCompose.Observation) {}})
	if err != nil {
		return studentdeletioncompose.Owners{}, err
	}
	structure, err := schoolStructureCompose.New(schoolStructureCompose.Dependencies{DB: db.Pool(), Observe: func(schoolStructureCompose.Observation) {}})
	if err != nil {
		return studentdeletioncompose.Owners{}, err
	}
	identity, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		return studentdeletioncompose.Owners{}, err
	}
	audit, err := NewWorkflowAuditCommand(db, slog.Default(), "student deletion audit append failed")
	if err != nil {
		return studentdeletioncompose.Owners{}, err
	}
	audits, err := auditCompose.NewTenantPersistence(db.Pool(), false)
	if err != nil {
		return studentdeletioncompose.Owners{}, err
	}
	return studentdeletioncompose.Owners{
		Structure: structure, Conversations: parentstore.NewStudentConversations(db.Pool()), Enrollment: enrollmentCompose.New(),
		Attendance: presence.CountAttendanceRecords, Visits: presence.CountStudentVisitsForDeletion,
		Consents: func(ctx context.Context, studentID int64) (int, error) {
			consents, err := presence.ListPrivacyConsents(ctx, studentID)
			return len(consents), err
		},
		Appointments:        appointments.CountAppointmentRecipientStudents,
		GuardianInvitations: identity.CountStudentGuardianInvitations,
		Audit:               audit, DeletionRecords: studentDeletionRecords,
		AuditReferences: audits.StudentDeletionAudit.CountStudentReferences,
	}, nil
}
