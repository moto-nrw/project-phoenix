package postgres

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-ogs/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// ArchiveStudentStatusFlags closes or carries the status days the live
// status flags stand for (see careplan.StatusFlagArchive). An archived day
// keeps the row the report wrote: its status, source and visibility stay as
// they were (#6291), and a day someone cleared stays cleared. Only a day
// without any row gets an end-of-day row.
func (s *statusDayStore) ArchiveStudentStatusFlags(ctx context.Context, value careplan.StatusFlagArchive) (int64, carePlanCompose.RequestStoreStats, error) {
	if !statusFlagColumnsMatch(value) {
		return 0, carePlanCompose.RequestStoreStats{}, fmt.Errorf("status flag columns do not match status %q", value.Status)
	}
	students, err := s.lockedStatusFlagStudents(ctx, value.Status)
	if err != nil {
		return 0, carePlanCompose.RequestStoreStats{}, err
	}
	students = statusFlagsReportedBefore(value, students)
	if len(students) == 0 {
		return 0, carePlanCompose.RequestStoreStats{}, nil
	}
	db, tenantID, err := s.databaseForWrite(ctx, "archive student status flags")
	if err != nil {
		return 0, carePlanCompose.RequestStoreStats{}, err
	}
	if value.Carry {
		return s.carryStatusFlags(ctx, db, tenantID, value, students)
	}
	rows, ids := statusFlagArchiveRows(value, students)
	started := time.Now()
	_, err = db.NewInsert().Model(&rows).ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO NOTHING").Exec(ctx)
	stats := requestStats(started, int64(len(rows)))
	if err != nil {
		return 0, stats, requestDBError("archive status flag", err)
	}
	affected, err := s.students.ClearStudentStatusFlags(ctx, ids, value.Status)
	return affected, stats, err
}

// carryStatusFlags gives every flagged student an active row for the carried
// day. The row is the OGS's own (manual): the status stands because of the
// school's clear mode, not because of a new report, so a check-in that clears
// reported days (planned, parent, school) leaves it alone and the class day
// does not show it as entered by the school. A day that already has a row
// keeps it; a new row books the day's slots like any status day.
func (s *statusDayStore) carryStatusFlags(ctx context.Context, db bun.IDB, tenantID int64, value careplan.StatusFlagArchive, students []StatusStudent) (int64, carePlanCompose.RequestStoreStats, error) {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	slices.Sort(ids)
	var stats carePlanCompose.RequestStoreStats
	for _, studentID := range slices.Compact(ids) {
		if err := LockExceptionDay(ctx, db, tenantID, studentID, string(value.Date)); err != nil {
			return 0, stats, fmt.Errorf("lock carried student status day: %w", err)
		}
	}
	rows := make([]studentStatusDayRow, 0, len(students))
	for _, student := range students {
		rows = append(rows, studentStatusDayRow{
			TenantID: student.TenantID, StudentID: student.ID, Date: calendarDate(value.Date), Status: value.Status,
			ReportedAt: statusFlagReportedAt(value, student), Source: value.Source,
		})
	}
	var inserted []struct {
		ID        int64 `bun:"id"`
		StudentID int64 `bun:"student_id"`
	}
	started := time.Now()
	err := db.NewInsert().Model(&rows).ModelTableExpr("active.student_status_days").
		On("CONFLICT (tenant_id, student_id, date, status) DO NOTHING").Returning("id, student_id").Scan(ctx, &inserted)
	writeStats := requestStats(started, int64(len(inserted)))
	stats.Queries += writeStats.Queries
	stats.Rows += writeStats.Rows
	stats.StatementDuration += writeStats.StatementDuration
	if err != nil {
		return 0, stats, requestDBError("carry status flag", err)
	}
	for _, row := range inserted {
		if _, err := s.slots.ApplyStatusDay(ctx, row.StudentID, value.Date, row.ID, attendanceSubstatus(value.Status)); err != nil {
			return 0, stats, err
		}
	}
	return int64(len(inserted)), stats, nil
}

// statusFlagColumnsMatch rejects an archive whose flag and since columns do
// not belong to its status.
func statusFlagColumnsMatch(value careplan.StatusFlagArchive) bool {
	switch value.Status {
	case statusSick:
		return value.FlagColumn == "sick" && value.SinceColumn == "sick_since"
	case statusExcused:
		return value.FlagColumn == "excused" && value.SinceColumn == "excused_since"
	}
	return true
}

// lockedStatusFlagStudents locks every student carrying the status flag in ID
// order, then re-reads the flagged students and keeps only those it locked.
// It returns nil without locking when no student carries the flag.
func (s *statusDayStore) lockedStatusFlagStudents(ctx context.Context, status string) ([]StatusStudent, error) {
	candidates, err := s.students.ListStudentsWithStatusFlag(ctx, status)
	if err != nil || len(candidates) == 0 {
		return nil, err
	}
	ids := make([]int64, 0, len(candidates))
	locked := make(map[int64]struct{}, len(candidates))
	for _, student := range candidates {
		ids = append(ids, student.ID)
		locked[student.ID] = struct{}{}
	}
	slices.Sort(ids)
	for _, studentID := range slices.Compact(ids) {
		if err := s.students.LockStudent(ctx, studentID); err != nil {
			return nil, err
		}
	}
	students, err := s.students.ListStudentsWithStatusFlag(ctx, status)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(students, func(student StatusStudent) bool { _, ok := locked[student.ID]; return !ok }), nil
}

// statusFlagsReportedBefore keeps the flags reported before the archive's
// bound. A flag without a since timestamp has no day of its own: a carry
// keeps it, an archive takes it only when the run is on the archived day
// itself (after its end), so it is never dated onto an earlier day.
func statusFlagsReportedBefore(value careplan.StatusFlagArchive, students []StatusStudent) []StatusStudent {
	if value.Before.IsZero() {
		return students
	}
	return slices.DeleteFunc(students, func(student StatusStudent) bool {
		since := statusFlagSince(value, student)
		if since == nil {
			return !value.Carry && calendar.DateFromTime(value.ReportedFallback) != calendar.Date(value.Date)
		}
		return !since.Before(value.Before)
	})
}

// statusFlagArchiveRows builds the archived status day of every flagged
// student, dated on the flag's own status day, and returns the student IDs
// whose flags the archive clears.
func statusFlagArchiveRows(value careplan.StatusFlagArchive, students []StatusStudent) ([]studentStatusDayRow, []int64) {
	rows := make([]studentStatusDayRow, 0, len(students))
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		date := calendarDate(value.Date)
		if since := statusFlagSince(value, student); since != nil {
			date = calendarDate(calendar.DateFromTime(*since))
		}
		row := studentStatusDayRow{TenantID: student.TenantID, StudentID: student.ID, Date: date, Status: value.Status, ReportedAt: statusFlagReportedAt(value, student), ClearedAt: &value.ReportedFallback, Source: value.Source}
		rows = append(rows, row)
		ids = append(ids, student.ID)
	}
	return rows, ids
}

func statusFlagSince(value careplan.StatusFlagArchive, student StatusStudent) *time.Time {
	switch value.Status {
	case statusSick:
		return student.SickSince
	case statusExcused:
		return student.ExcusedSince
	}
	return nil
}

func statusFlagReportedAt(value careplan.StatusFlagArchive, student StatusStudent) time.Time {
	if since := statusFlagSince(value, student); since != nil {
		return *since
	}
	return value.ReportedFallback
}
