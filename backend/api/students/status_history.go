package students

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
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

func newlyReportedAbsenceStatus(student *users.Student, wasSick, wasExcused bool) string {
	if !wasSick && boolPtrValue(student.Sick) {
		return active.StudentStatusDaySick
	}
	if !wasExcused && boolPtrValue(student.Excused) {
		return active.StudentStatusDayExcused
	}
	return ""
}

func (rs *Resource) persistStudentStatusHistory(ctx context.Context, student *users.Student, wasSick, wasExcused bool, now time.Time, sickNote *string) error {
	if rs.StudentStatusDayService == nil {
		return nil
	}
	if student == nil {
		return nil
	}

	today := timezone.DateFromTime(now)
	// Only the sick status carries a free-text reason; excused stays note-less.
	if err := rs.persistSingleStatusHistory(ctx, student.ID, active.StudentStatusDaySick, wasSick, boolPtrValue(student.Sick), statusReportedAt(now, student.SickSince), today, now, sickNote); err != nil {
		return err
	}
	if err := rs.persistSingleStatusHistory(ctx, student.ID, active.StudentStatusDayExcused, wasExcused, boolPtrValue(student.Excused), statusReportedAt(now, student.ExcusedSince), today, now, nil); err != nil {
		return err
	}
	return nil
}

func (rs *Resource) persistSingleStatusHistory(ctx context.Context, studentID int64, status string, wasActive, isActive bool, reportedAt time.Time, date timezone.Date, now time.Time, note *string) error {
	if isActive {
		return rs.StudentStatusDayService.UpsertReported(ctx, &active.StudentStatusDay{
			StudentID:  studentID,
			Date:       date,
			Status:     status,
			ReportedAt: reportedAt,
			Source:     active.StudentStatusSourceManual,
			Note:       note,
		})
	}
	if wasActive {
		if err := rs.StudentStatusDayService.UpsertReported(ctx, &active.StudentStatusDay{
			StudentID:  studentID,
			Date:       date,
			Status:     status,
			ReportedAt: reportedAt,
			Source:     active.StudentStatusSourceManual,
		}); err != nil {
			return err
		}
		return rs.StudentStatusDayService.MarkCleared(ctx, studentID, status, date, now, active.StudentStatusSourceManual)
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

func (rs *Resource) notifyAbsenceReported(tenantID int64, studentIDs []int64, status string, dates []timezone.Date, fromParent bool, actorAccountID int64) {
	if rs.AbsenceNotifier == nil {
		return
	}
	rs.AbsenceNotifier.NotifyAbsenceReported(context.Background(), notificationsService.AbsenceReport{
		TenantID:       tenantID,
		StudentIDs:     studentIDs,
		Status:         status,
		Dates:          dates,
		FromParent:     fromParent,
		ActorAccountID: actorAccountID,
	})
}
