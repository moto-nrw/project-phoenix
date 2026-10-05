package students

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	notificationsService "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
)

func boolPtrValue(v *bool) bool {
	return v != nil && *v
}

func statusReportedAt(now time.Time, existing *time.Time) time.Time {
	if existing != nil {
		return *existing
	}
	return now
}

func newlyReportedAbsenceStatus(student *Student, wasSick, wasExcused bool) string {
	if !wasSick && boolPtrValue(student.Sick) {
		return absencerecords.StudentStatusDaySick
	}
	if !wasExcused && boolPtrValue(student.Excused) {
		return absencerecords.StudentStatusDayExcused
	}
	return ""
}

// clearRequested reports whether the request explicitly lifts a status
// ("sick": false / "excused": false).
func clearRequested(requested *bool) bool {
	return requested != nil && !*requested
}

func (rs *Resource) persistStudentStatusHistory(ctx context.Context, student *Student, req *UpdateStudentRequest, wasSick, wasExcused bool, now time.Time, sickNote *string) error {
	if rs.StudentStatusDayService == nil {
		return nil
	}
	if student == nil {
		return nil
	}

	today := timezone.DateFromTime(now)
	// Only the sick status carries a free-text reason; excused stays note-less.
	if err := rs.persistSingleStatusHistory(ctx, student.ID, absencerecords.StudentStatusDaySick, statusTransition{
		wasActive: wasSick, isActive: boolPtrValue(student.Sick), clearRequested: clearRequested(req.Sick),
	}, statusReportedAt(now, student.SickSince), today, now, sickNote); err != nil {
		return err
	}
	if err := rs.persistSingleStatusHistory(ctx, student.ID, absencerecords.StudentStatusDayExcused, statusTransition{
		wasActive: wasExcused, isActive: boolPtrValue(student.Excused), clearRequested: clearRequested(req.Excused),
	}, statusReportedAt(now, student.ExcusedSince), today, now, nil); err != nil {
		return err
	}
	return nil
}

// statusTransition describes one status flag across an update. wasActive and
// isActive are the stored live flag before and after; clearRequested is set
// when the request explicitly lifts the status.
type statusTransition struct {
	wasActive      bool
	isActive       bool
	clearRequested bool
}

func (rs *Resource) persistSingleStatusHistory(ctx context.Context, studentID int64, status string, transition statusTransition, reportedAt time.Time, date timezone.Date, now time.Time, note *string) error {
	if transition.isActive {
		return rs.StudentStatusDayService.UpsertReported(ctx, &absencerecords.StudentStatusDay{
			StudentID:  studentID,
			Date:       date,
			Status:     status,
			ReportedAt: reportedAt,
			Source:     absencerecords.StudentStatusSourceManual,
			Note:       note,
		})
	}
	if transition.wasActive {
		if err := rs.StudentStatusDayService.UpsertReported(ctx, &absencerecords.StudentStatusDay{
			StudentID:  studentID,
			Date:       date,
			Status:     status,
			ReportedAt: reportedAt,
			Source:     absencerecords.StudentStatusSourceManual,
		}); err != nil {
			return err
		}
		return rs.StudentStatusDayService.MarkCleared(ctx, studentID, status, date, now, absencerecords.StudentStatusSourceManual)
	}
	if transition.clearRequested {
		// Today's row can be active without the live flag: a parent's
		// Abmeldung (#1735) and a day planned in advance never set it. The
		// profile shows those rows as the same status, so lifting it must clear
		// them too, whoever entered them (#3854). Without an active row this is
		// a no-op.
		return rs.StudentStatusDayService.MarkCleared(ctx, studentID, status, date, now, absencerecords.StudentStatusSourceManual)
	}
	return nil
}

func (rs *Resource) logStatusHistoryError(studentID int64, err error) {
	if rs.Logger == nil || err == nil {
		return
	}
	rs.Logger.Error("student status history write failed",
		slog.Int64("student_id", studentID),
		slog.String("error", err.Error()),
	)
}

func (rs *Resource) notifyAbsenceReported(ctx context.Context, tenantID int64, studentIDs []int64, status string, dates []timezone.Date, fromParent bool, actorAccountID int64) error {
	if rs.AbsenceNotifier == nil {
		return nil
	}
	return rs.AbsenceNotifier.NotifyAbsenceReported(ctx, notificationsService.AbsenceReport{
		TenantID:       tenantID,
		StudentIDs:     studentIDs,
		Status:         status,
		Dates:          dates,
		FromParent:     fromParent,
		ActorAccountID: actorAccountID,
	})
}
