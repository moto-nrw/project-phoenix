package students

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
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

func (rs *Resource) persistStudentStatusHistory(ctx context.Context, student *users.Student, wasSick, wasExcused bool, now time.Time) error {
	if rs.StudentStatusDayRepo == nil {
		return nil
	}
	if student == nil {
		return nil
	}

	today := timezone.DateOfUTC(now)
	if err := rs.persistSingleStatusHistory(ctx, student.ID, active.StudentStatusDaySick, wasSick, boolPtrValue(student.Sick), statusReportedAt(now, student.SickSince), today, now); err != nil {
		return err
	}
	if err := rs.persistSingleStatusHistory(ctx, student.ID, active.StudentStatusDayExcused, wasExcused, boolPtrValue(student.Excused), statusReportedAt(now, student.ExcusedSince), today, now); err != nil {
		return err
	}
	return nil
}

func (rs *Resource) persistSingleStatusHistory(ctx context.Context, studentID int64, status string, wasActive, isActive bool, reportedAt, date, now time.Time) error {
	if isActive {
		return rs.StudentStatusDayRepo.UpsertReported(ctx, &active.StudentStatusDay{
			StudentID:  studentID,
			Date:       date,
			Status:     status,
			ReportedAt: reportedAt,
			Source:     active.StudentStatusSourceManual,
		})
	}
	if wasActive {
		if err := rs.StudentStatusDayRepo.UpsertReported(ctx, &active.StudentStatusDay{
			StudentID:  studentID,
			Date:       date,
			Status:     status,
			ReportedAt: reportedAt,
			Source:     active.StudentStatusSourceManual,
		}); err != nil {
			return err
		}
		return rs.StudentStatusDayRepo.MarkCleared(ctx, studentID, status, date, now, active.StudentStatusSourceManual)
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
