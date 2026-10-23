package compose

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnualRegistrationWithoutDeadlineRecordsLateAddition(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Mia", "Jahresanmeldung", "2a")
	parent := testpkg.CreateTestAccount(t, db, "meal-annual-open")
	settings := singleDeadlineSettings("09:00")
	settings.Model, settings.AnnualDeadline = "annual", ""
	settings.CancellationKind, settings.CancellationTime = "days_before", "09:00"
	module := buildModuleWithRules(t, db, settings, time.Date(2026, 9, 17, 12, 0, 0, 0, testBerlin))

	week, err := module.Orders(ctx, "2026-09-17", "2026-09-17")
	require.NoError(t, err)
	assert.Equal(t, mealplan.OrderOrdered, week.Days[0].Status)
	assert.Zero(t, week.Days[0].Total)
	plan, err := module.Participation(ctx, student.ID, "2026-09-17", "2026-09-17")
	require.NoError(t, err)
	assert.True(t, plan.Days[0].CanRegister)
	assert.False(t, plan.Days[0].CanCancel)
	require.NoError(t, module.SetParticipationForDay(ctx, mealplan.SetParticipationDay{StudentID: student.ID, GuardianAccountID: parent.ID, Date: "2026-09-17", Participating: true}))
	week, err = module.Orders(ctx, "2026-09-17", "2026-09-17")
	require.NoError(t, err)
	assert.Zero(t, week.Days[0].Total, "the frozen base count stays unchanged")
	require.Len(t, week.Days[0].Changes, 1)
	assert.Equal(t, 1, week.Days[0].Changes[0].Delta)
}
