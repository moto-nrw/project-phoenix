package api

import (
	"context"
	"errors"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/studentpresence/http"
	"github.com/moto-nrw/project-phoenix/tenant"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below connect the Schulverwaltung's absence reports of moto
// schule (#4791) to Student Presence's status-day write, the child's change
// history in the Audit Platform and Delivery's notice. The notice reaches the
// Schulverwaltung's open moto schule tabs over the notification stream as
// well, which wakes its list; a report or withdrawal wakes the class views
// through the status-day write's student_updated (class_day_changed).

// classDayAbsenceReportRecorder is Student Presence's status-day write for a
// report the Schulverwaltung enters.
type classDayAbsenceReportRecorder interface {
	RecordSchoolAbsenceReport(ctx context.Context, studentID int64, status string, date timezone.Date, actorAccountID int64, inClass func(schoolClass string) bool) error
	WithdrawSchoolAbsenceReport(ctx context.Context, studentID int64, date timezone.Date, inClass func(schoolClass string) bool) (string, error)
}

// classDayFieldEdits is the Audit Platform's change history of a child.
type classDayFieldEdits interface {
	CreateBatch(ctx context.Context, edits []*auditModels.StudentFieldEdit) error
}

// withClassDayAbsenceReports binds the absence-report seam into the morning
// attendance's dependencies. Without the recorder or the change history the
// writes answer "not configured".
func withClassDayAbsenceReports(deps classdayCompose.MorningAttendanceDependencies, recorder classDayAbsenceReportRecorder, edits classDayFieldEdits, notifier deliveryContract.UnreportedAbsenceNotifier) classdayCompose.MorningAttendanceDependencies {
	if recorder != nil && edits != nil {
		deps.ReportStatusDays = classDayAbsenceReportStatusDays{recorder: recorder}
		deps.ReportLog = classDayAbsenceReportLog{edits: edits}
	}
	if notifier != nil {
		deps.UnreportedNotifier = classDayUnreportedNotifier{notifier: notifier}
	}
	return deps
}

// classDayAbsenceReportStatusDays compares the class on the locked child the
// way the caller's class reach does, and maps the write's refusals onto the
// projection's sentinels.
type classDayAbsenceReportStatusDays struct{ recorder classDayAbsenceReportRecorder }

func (b classDayAbsenceReportStatusDays) RecordSchoolAbsenceReport(ctx context.Context, schoolClass string, studentID int64, status string, date timezone.Date, actorAccountID int64) error {
	return absenceReportError(b.recorder.RecordSchoolAbsenceReport(ctx, studentID, status, date, actorAccountID, sameClass(schoolClass)))
}

func (b classDayAbsenceReportStatusDays) WithdrawSchoolAbsenceReport(ctx context.Context, schoolClass string, studentID int64, date timezone.Date) (string, error) {
	status, err := b.recorder.WithdrawSchoolAbsenceReport(ctx, studentID, date, sameClass(schoolClass))
	return status, absenceReportError(err)
}

func sameClass(schoolClass string) func(string) bool {
	want := schoolstructure.NormalizeClass(schoolClass)
	return func(class string) bool { return schoolstructure.NormalizeClass(class) == want }
}

func absenceReportError(err error) error {
	var conflict *studentpresence.StudentStatusDayConflictError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &conflict), errors.Is(err, studentpresence.ErrStudentStatusDayPartialAbsenceConflict):
		return fmt.Errorf("%w: %w", classday.ErrAbsenceReportConflict, err)
	case errors.Is(err, studentpresence.ErrStudentStatusDayReassigned):
		return fmt.Errorf("%w: %w", classday.ErrAbsenceReportUnknownChild, err)
	case errors.Is(err, presenceStudentsHTTP.ErrSchoolAbsenceReportNotFound):
		return fmt.Errorf("%w: %w", classday.ErrAbsenceReportNotFound, err)
	}
	return err
}

// classDayAbsenceReportLog writes a report or withdrawal into the child's
// change history, which the OGS reads on the child's page.
type classDayAbsenceReportLog struct{ edits classDayFieldEdits }

func (l classDayAbsenceReportLog) RecordAbsenceReportChange(ctx context.Context, entry classday.AbsenceReportLogEntry) error {
	edit := &auditModels.StudentFieldEdit{
		StudentID: entry.StudentID, EditedBy: entry.Actor.AccountID, EditedByName: entry.Actor.Name,
		FieldName: auditModels.StudentFieldSchoolAbsenceReport,
	}
	if entry.OldValue != "" {
		edit.OldValue = &entry.OldValue
	}
	if entry.NewValue != "" {
		edit.NewValue = &entry.NewValue
	}
	return l.edits.CreateBatch(ctx, []*auditModels.StudentFieldEdit{edit})
}

// classDayUnreportedNotifier hands the children missing without a report to
// Delivery for the request's school.
type classDayUnreportedNotifier struct {
	notifier deliveryContract.UnreportedAbsenceNotifier
}

func (n classDayUnreportedNotifier) NotifyUnreportedAbsences(ctx context.Context, notice classday.UnreportedAbsenceNotice) error {
	return n.notifier.NotifyUnreportedAbsences(ctx, deliveryContract.UnreportedAbsenceReport{
		TenantID: tenant.FromContext(ctx), Date: notice.Date, StudentIDs: notice.StudentIDs, ActorAccountID: notice.ActorAccountID,
	})
}
