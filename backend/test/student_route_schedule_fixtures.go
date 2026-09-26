package test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// Care-plan rows the student route suites (#2731) arrange with exact column
// values. The suites hold no schedule-model import, so the row shapes live on
// this side of the fixture boundary. Every row is stamped with the given
// TenantID, not the test's fixture tenant, because several suites seed rows
// in a guardian chain's tenant.

// StudentArrivalScheduleRow is one weekly arrival row. A zero ExpectedArrival
// stores NULL (the care day inherits the class time).
type StudentArrivalScheduleRow struct {
	TenantID        int64
	StudentID       int64
	Weekday         int
	ExpectedArrival time.Time
	Notes           *string
	CreatedBy       int64
}

// InsertStudentArrivalScheduleRow inserts one schedule.student_arrival_schedules row.
func InsertStudentArrivalScheduleRow(tb testing.TB, db *bun.DB, spec StudentArrivalScheduleRow) *schedule.StudentArrivalSchedule {
	tb.Helper()
	row := &schedule.StudentArrivalSchedule{
		StudentID:       spec.StudentID,
		Weekday:         spec.Weekday,
		ExpectedArrival: spec.ExpectedArrival,
		Notes:           spec.Notes,
		CreatedBy:       spec.CreatedBy,
	}
	row.SetTenantID(spec.TenantID)
	insertStudentRouteRow(tb, db, row, "schedule.student_arrival_schedules")
	return row
}

// StudentPickupScheduleRow is one weekly pickup row. An empty Source stores
// the column default ('staff').
type StudentPickupScheduleRow struct {
	TenantID       int64
	StudentID      int64
	Weekday        int
	PickupTime     time.Time
	Notes          *string
	CreatedBy      int64
	Source         string
	CareOfferingID *int64
}

// InsertStudentPickupScheduleRow inserts one schedule.student_pickup_schedules row.
func InsertStudentPickupScheduleRow(tb testing.TB, db *bun.DB, spec StudentPickupScheduleRow) *schedule.StudentPickupSchedule {
	tb.Helper()
	row := &schedule.StudentPickupSchedule{
		StudentID:      spec.StudentID,
		Weekday:        spec.Weekday,
		PickupTime:     spec.PickupTime,
		Notes:          spec.Notes,
		CreatedBy:      spec.CreatedBy,
		Source:         spec.Source,
		CareOfferingID: spec.CareOfferingID,
	}
	row.SetTenantID(spec.TenantID)
	insertStudentRouteRow(tb, db, row, "schedule.student_pickup_schedules")
	return row
}

// StudentArrivalExceptionRow is one date-specific arrival exception. A nil
// ExpectedArrival marks absence; an empty Source stores the column default
// ('staff'); a zero CreatedBy stores NULL (guardian-authored rows).
type StudentArrivalExceptionRow struct {
	TenantID          int64
	StudentID         int64
	ExceptionDate     CalendarDate
	ExpectedArrival   *time.Time
	Reason            *string
	Source            string
	CreatedBy         int64
	CreatedByGuardian *int64
}

// InsertStudentArrivalExceptionRow inserts one schedule.student_arrival_exceptions row.
func InsertStudentArrivalExceptionRow(tb testing.TB, db *bun.DB, spec StudentArrivalExceptionRow) *schedule.StudentArrivalException {
	tb.Helper()
	row := &schedule.StudentArrivalException{
		StudentID:         spec.StudentID,
		ExceptionDate:     schedule.Date(spec.ExceptionDate.String()),
		ExpectedArrival:   spec.ExpectedArrival,
		Reason:            spec.Reason,
		Source:            spec.Source,
		CreatedBy:         spec.CreatedBy,
		CreatedByGuardian: spec.CreatedByGuardian,
	}
	row.SetTenantID(spec.TenantID)
	insertStudentRouteRow(tb, db, row, "schedule.student_arrival_exceptions")
	return row
}

// StudentPickupExceptionRow is one date-specific pickup exception. A nil
// PickupTime marks absence; an empty Source stores the column default
// ('staff'); a zero CreatedBy stores NULL (guardian-authored rows).
type StudentPickupExceptionRow struct {
	TenantID          int64
	StudentID         int64
	ExceptionDate     CalendarDate
	PickupTime        *time.Time
	Reason            *string
	Source            string
	CreatedBy         int64
	CreatedByGuardian *int64
}

// InsertStudentPickupExceptionRow inserts one schedule.student_pickup_exceptions row.
func InsertStudentPickupExceptionRow(tb testing.TB, db *bun.DB, spec StudentPickupExceptionRow) *schedule.StudentPickupException {
	tb.Helper()
	row := &schedule.StudentPickupException{
		StudentID:         spec.StudentID,
		ExceptionDate:     schedule.Date(spec.ExceptionDate.String()),
		PickupTime:        spec.PickupTime,
		Reason:            spec.Reason,
		Source:            spec.Source,
		CreatedBy:         spec.CreatedBy,
		CreatedByGuardian: spec.CreatedByGuardian,
	}
	row.SetTenantID(spec.TenantID)
	insertStudentRouteRow(tb, db, row, "schedule.student_pickup_exceptions")
	return row
}

// StudentDatedNoteRow is one date-specific arrival or pickup note.
type StudentDatedNoteRow struct {
	TenantID  int64
	StudentID int64
	NoteDate  CalendarDate
	Content   string
	CreatedBy int64
}

// InsertStudentArrivalNoteRow inserts one schedule.student_arrival_notes row.
func InsertStudentArrivalNoteRow(tb testing.TB, db *bun.DB, spec StudentDatedNoteRow) *schedule.StudentArrivalNote {
	tb.Helper()
	row := &schedule.StudentArrivalNote{
		StudentID: spec.StudentID,
		NoteDate:  schedule.Date(spec.NoteDate.String()),
		Content:   spec.Content,
		CreatedBy: spec.CreatedBy,
	}
	row.SetTenantID(spec.TenantID)
	insertStudentRouteRow(tb, db, row, "schedule.student_arrival_notes")
	return row
}

// InsertStudentPickupNoteRow inserts one date-specific schedule.student_pickup_notes row.
func InsertStudentPickupNoteRow(tb testing.TB, db *bun.DB, spec StudentDatedNoteRow) *schedule.StudentPickupNote {
	tb.Helper()
	row := &schedule.StudentPickupNote{
		StudentID: spec.StudentID,
		NoteDate:  schedule.Date(spec.NoteDate.String()),
		Content:   spec.Content,
		CreatedBy: spec.CreatedBy,
	}
	row.SetTenantID(spec.TenantID)
	insertStudentRouteRow(tb, db, row, "schedule.student_pickup_notes")
	return row
}

func insertStudentRouteRow(tb testing.TB, db *bun.DB, row any, table string) {
	tb.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()
	_, err := db.NewInsert().Model(row).ModelTableExpr(table).Returning("id").Exec(ctx)
	require.NoError(tb, err, "Failed to insert %s row", table)
}
