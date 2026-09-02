package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// CountEffectiveDashboardAbsences counts the dashboard's effective absence
// buckets for one date. It bridges the legacy live flags on users.students
// (read through the People Directory, #2662) and the newer date-scoped rows
// in active.student_status_days without double counting overlaps. Sick wins
// over excused/class_trip, matching api/students/status_days_response.go.
// Total is every active student, so "at home" stays the remainder of the
// same read.
func (r *StudentStatusDayRepository) CountEffectiveDashboardAbsences(ctx context.Context, date timezone.Date) (*active.StudentStatusCounts, error) {
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	students, err := r.students.ListActiveStudents(ctx)
	if err != nil {
		return nil, err
	}

	type dayRow struct {
		StudentID int64  `bun:"student_id"`
		Status    string `bun:"status"`
	}
	var rows []dayRow
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActiveStudentStatusDaysAsStatusDay).
		ColumnExpr(`"student_status_day".student_id`).
		ColumnExpr(`"student_status_day".status`).
		Where(`"student_status_day".date = ?`, date).
		Where(`"student_status_day".cleared_at IS NULL`)
	query = base.WithTenantFilter(ctx, query, "student_status_day")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count effective dashboard absences",
			Err: base.TranslateNotFound(err),
		}
	}

	type dayFlags struct{ sick, excused, classTrip bool }
	byStudent := make(map[int64]dayFlags, len(rows))
	for _, row := range rows {
		flags := byStudent[row.StudentID]
		switch row.Status {
		case active.StudentStatusDaySick:
			flags.sick = true
		case active.StudentStatusDayExcused:
			flags.excused = true
		case active.StudentStatusDayClassTrip:
			flags.classTrip = true
		}
		byStudent[row.StudentID] = flags
	}

	counts := &active.StudentStatusCounts{Total: len(students)}
	for _, student := range students {
		day := byStudent[student.ID]
		if (student.Sick != nil && *student.Sick) || day.sick {
			counts.Sick++
			continue
		}
		if (student.Excused != nil && *student.Excused) || day.excused || day.classTrip {
			counts.Excused++
		}
	}
	return counts, nil
}
