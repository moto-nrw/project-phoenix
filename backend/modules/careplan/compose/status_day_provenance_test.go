package compose

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	postgresNative "github.com/moto-ogs/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type statusDayHistoryEvent struct {
	ReportedAt time.Time  `bun:"reported_at"`
	ClearedAt  *time.Time `bun:"cleared_at"`
	Source     *string    `bun:"source"`
	AccountID  *int64     `bun:"account_id"`
}

// A clear and a later report keep who reported the day and when (#6842):
// the row keeps the parent's source next to the clearing, and the history
// keeps every report and clearing with its source and account.
func TestStatusDayClearAndReReportKeepProvenance(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Provenance", "Child", "1a")
	parent := testpkg.CreateTestAccount(t, db, "provenance-parent")
	staff := testpkg.CreateTestAccount(t, db, "provenance-staff")
	date := careplan.Date("2031-03-04")
	parentReport := time.Date(2031, 3, 4, 6, 30, 0, 0, time.UTC)
	staffClear := parentReport.Add(2 * time.Hour)
	staffReport := parentReport.Add(3 * time.Hour)

	reported, err := module.UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
		StudentID: student.ID, Date: date, Status: "sick", ReportedAt: parentReport,
		Source: careplan.StudentStatusSourceParent, GuardianAccountID: &parent.ID, ReportedByAccountID: &parent.ID,
	})
	require.NoError(t, err)
	require.NoError(t, module.ClearStudentStatusDays(ctx, student.ID, "sick", []careplan.Date{date}, staffClear, careplan.StudentStatusSourceManual, &staff.ID))

	cleared, err := module.FindStudentStatusDay(ctx, reported.ID, false)
	require.NoError(t, err)
	assert.Equal(t, careplan.StudentStatusSourceParent, cleared.Source, "clearing must not replace the report's source")
	assert.Equal(t, &parent.ID, cleared.ReportedByAccountID)
	require.NotNil(t, cleared.ClearedSource)
	assert.Equal(t, careplan.StudentStatusSourceManual, *cleared.ClearedSource)
	assert.Equal(t, &staff.ID, cleared.ClearedByAccountID)

	_, err = module.UpsertStudentStatusDay(ctx, careplan.StudentStatusDay{
		StudentID: student.ID, Date: date, Status: "sick", ReportedAt: staffReport,
		Source: careplan.StudentStatusSourceManual, ReportedByAccountID: &staff.ID,
	})
	require.NoError(t, err)
	again, err := module.FindStudentStatusDay(ctx, reported.ID, true)
	require.NoError(t, err)
	assert.Nil(t, again.ClearedSource, "a new report stands again")
	assert.Nil(t, again.ClearedByAccountID)

	events := []statusDayHistoryEvent{}
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		scoped, _, databaseErr := postgresNative.CarePlanDatabase(db)(txCtx)
		if databaseErr != nil {
			return databaseErr
		}
		return scoped.NewSelect().TableExpr("schedule.meal_sickness_status_history").
			Column("reported_at", "cleared_at", "source", "account_id").
			Where("student_id = ? AND date = ? AND status = 'sick'", student.ID, string(date)).
			OrderExpr("id ASC").Scan(txCtx, &events)
	}))
	require.Len(t, events, 3)
	assertStatusDayEvent(t, events[0], parentReport, false, careplan.StudentStatusSourceParent, parent.ID)
	assertStatusDayEvent(t, events[1], parentReport, true, careplan.StudentStatusSourceManual, staff.ID)
	assertStatusDayEvent(t, events[2], staffReport, false, careplan.StudentStatusSourceManual, staff.ID)
}

func assertStatusDayEvent(t *testing.T, event statusDayHistoryEvent, reportedAt time.Time, cleared bool, source string, accountID int64) {
	t.Helper()
	assert.True(t, event.ReportedAt.Equal(reportedAt), "reported_at %s, want %s", event.ReportedAt, reportedAt)
	assert.Equal(t, cleared, event.ClearedAt != nil)
	require.NotNil(t, event.Source)
	assert.Equal(t, source, *event.Source)
	require.NotNil(t, event.AccountID)
	assert.Equal(t, accountID, *event.AccountID)
}
