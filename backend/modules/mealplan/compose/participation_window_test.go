package compose

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParticipationLoadsEditableScheduleBeyondDailyWindow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, rhythm := range []string{"daily", "weekly"} {
		for _, previous := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/previous=%t", rhythm, previous), func(t *testing.T) {
				t.Parallel()
				ctx := testpkg.OwnCtx(t)
				student := testpkg.CreateTestStudent(t, db, "Mia", "Plan", "2a")
				account := testpkg.CreateTestAccount(t, db, "meal-window-parent")
				rules := singleDeadlineSettings("09:00")
				rules.OrderRhythm = rhythm
				if previous {
					early := buildModuleWithRules(t, db, rules, time.Date(2026, 9, 7, 8, 0, 0, 0, testBerlin))
					_, err := early.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{
						StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: []mealplan.Weekday{mealplan.Monday},
					})
					require.NoError(t, err)
				}
				module := buildModuleWithRules(t, db, rules, time.Date(2026, 9, 7, 10, 0, 0, 0, testBerlin))
				weekdays := []mealplan.Weekday{mealplan.Tuesday, mealplan.Thursday}
				effective, err := module.ReplaceParticipationSchedule(ctx, mealplan.ReplaceParticipationSchedule{
					StudentID: student.ID, GuardianAccountID: account.ID, Weekdays: weekdays,
				})
				require.NoError(t, err)
				require.True(t, effective > mealplan.Date("2026-09-07"))

				plan, err := module.Participation(ctx, student.ID, "2026-09-07", "2026-09-07")
				require.NoError(t, err)
				assert.Equal(t, effective, plan.EffectiveFrom)
				assert.Equal(t, weekdays, plan.Weekdays)
				require.Len(t, plan.Days, 1)
				assert.Equal(t, mealplan.Date("2026-09-07"), plan.Days[0].Date)
				assert.Equal(t, previous, plan.Days[0].Participating)
			})
		}
	}
}
