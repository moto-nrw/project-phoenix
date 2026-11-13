package careplan_test

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/careplantest"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPublicCarePlanContractUpsertsOneObservableStatusDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := careplantest.NewCarePlan(t, db, testpkg.StudentDirectory{})
	student := testpkg.CreateTestStudent(t, db, "Care", "Contract", "1A")
	day := careplan.Date("2026-08-24")
	note := "Arzttermin"
	input := careplan.StudentStatusDay{StudentID: student.ID, Date: day, Status: "sick", Source: "manual", ReportedAt: time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC), Note: &note}
	created, err := module.UpsertStudentStatusDay(testpkg.Ctx(t), input)
	require.NoError(t, err)
	require.Positive(t, created.ID)
	require.Equal(t, testpkg.Tenant(t), created.TenantID)
	updatedNote := "Termin verschoben"
	input.Note = &updatedNote
	updated, err := module.UpsertStudentStatusDay(testpkg.Ctx(t), input)
	require.NoError(t, err)
	require.Equal(t, created.ID, updated.ID)
	rows, err := module.ListStudentStatusDays(testpkg.Ctx(t), careplan.StudentStatusDayFilter{StudentIDs: []int64{student.ID}, Date: day})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, created.ID, rows[0].ID)
	require.Equal(t, "sick", rows[0].Status)
	require.Equal(t, &updatedNote, rows[0].Note)
}

// TestStatusDayReportReplacesTheStandingStatusOfTheDay pins #6958: a child
// has one standing status per day. A new report replaces the one standing,
// and the database refuses a second standing row for the same day.
func TestStatusDayReportReplacesTheStandingStatusOfTheDay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := careplantest.NewCarePlan(t, db, testpkg.StudentDirectory{})
	student := testpkg.CreateTestStudent(t, db, "Kim", "Fahrt", "3b")
	day := careplan.Date("2026-08-25")
	reported := time.Date(2026, 8, 25, 7, 0, 0, 0, time.UTC)

	trip, err := module.UpsertStudentStatusDay(testpkg.Ctx(t), careplan.StudentStatusDay{StudentID: student.ID, Date: day, Status: "class_trip", Source: "manual", ReportedAt: reported})
	require.NoError(t, err)
	sick, err := module.UpsertStudentStatusDay(testpkg.Ctx(t), careplan.StudentStatusDay{StudentID: student.ID, Date: day, Status: "sick", Source: "parent", ReportedAt: reported.Add(time.Hour)})
	require.NoError(t, err)

	standing, err := module.ListStudentStatusDays(testpkg.Ctx(t), careplan.StudentStatusDayFilter{StudentIDs: []int64{student.ID}, Date: day, ActiveOnly: true})
	require.NoError(t, err)
	require.Len(t, standing, 1, "the sick report replaces the class trip")
	require.Equal(t, sick.ID, standing[0].ID)

	replaced, err := module.FindStudentStatusDay(testpkg.Ctx(t), trip.ID, false)
	require.NoError(t, err)
	require.NotNil(t, replaced.ClearedAt, "the class trip no longer stands")
	require.Equal(t, "manual", replaced.Source, "the replaced report keeps its source (#6842)")
	require.NotNil(t, replaced.ClearedSource)
	require.Equal(t, "parent", *replaced.ClearedSource, "the replacing report names who replaced it")

	_, err = db.ExecContext(testpkg.Ctx(t), `
		INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, ?::date, 'excused', NOW(), 'manual')`, testpkg.Tenant(t), student.ID, string(day))
	require.ErrorContains(t, err, "uq_student_status_days_one_standing")
}

// TestRecordingAClearedStatusLeavesTheStandingStatusAlone pins that writing
// a cleared row, as lifting a live flag does to keep its history, does not
// replace another status that stands for the day.
func TestRecordingAClearedStatusLeavesTheStandingStatusAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := careplantest.NewCarePlan(t, db, testpkg.StudentDirectory{})
	student := testpkg.CreateTestStudent(t, db, "Lio", "Fahrt", "3b")
	day := careplan.Date("2026-08-26")
	reported := time.Date(2026, 8, 26, 7, 0, 0, 0, time.UTC)

	trip, err := module.UpsertStudentStatusDay(testpkg.Ctx(t), careplan.StudentStatusDay{StudentID: student.ID, Date: day, Status: "class_trip", Source: "manual", ReportedAt: reported})
	require.NoError(t, err)
	cleared := reported.Add(2 * time.Hour)
	_, err = module.UpsertStudentStatusDay(testpkg.Ctx(t), careplan.StudentStatusDay{StudentID: student.ID, Date: day, Status: "sick", Source: "next_checkin", ReportedAt: reported, ClearedAt: &cleared})
	require.NoError(t, err)

	standing, err := module.ListStudentStatusDays(testpkg.Ctx(t), careplan.StudentStatusDayFilter{StudentIDs: []int64{student.ID}, Date: day, ActiveOnly: true})
	require.NoError(t, err)
	require.Len(t, standing, 1)
	require.Equal(t, trip.ID, standing[0].ID, "the class trip still stands")
	all, err := module.ListStudentStatusDays(testpkg.Ctx(t), careplan.StudentStatusDayFilter{StudentIDs: []int64{student.ID}, Date: day})
	require.NoError(t, err)
	require.Len(t, all, 2, "the cleared sick day is recorded beside it")
}
