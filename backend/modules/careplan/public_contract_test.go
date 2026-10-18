package careplan_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
