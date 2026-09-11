package application

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestSickAtCutoffTreatsClearanceAtCutoffAsEffective(t *testing.T) {
	t.Parallel()
	berlin := time.FixedZone("Europe/Berlin", 2*60*60)
	cutoff := time.Date(2026, 9, 7, 9, 0, 0, 0, berlin)
	reportedAt := cutoff.Add(-30 * time.Minute)
	clearedAt := cutoff.UTC()

	assert.False(t, sickAtCutoff(reportedAt, &clearedAt, cutoff))
}

func TestResolveParticipationUsesScheduleForNextEffectiveDate(t *testing.T) {
	t.Parallel()
	berlin := time.FixedZone("Europe/Berlin", 2*60*60)

	tests := []struct {
		name          string
		now           time.Time
		wantEffective domain.Date
		wantWeekdays  []domain.Weekday
	}{
		{
			name:          "before cutoff uses the schedule active today",
			now:           time.Date(2026, 9, 7, 8, 0, 0, 0, berlin),
			wantEffective: domain.Date("2026-09-01"),
			wantWeekdays:  []domain.Weekday{1},
		},
		{
			name:          "after cutoff uses the pending schedule for tomorrow",
			now:           time.Date(2026, 9, 7, 10, 0, 0, 0, berlin),
			wantEffective: domain.Date("2026-09-08"),
			wantWeekdays:  []domain.Weekday{2},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &Service{now: func() time.Time { return test.now }}
			data := domain.ParticipationData{Schedules: []domain.ParticipationSchedule{
				{EffectiveFrom: domain.Date("2026-09-01"), Weekdays: []domain.Weekday{1}},
				{EffectiveFrom: domain.Date("2026-09-08"), Weekdays: []domain.Weekday{2}},
				{EffectiveFrom: domain.Date("2026-09-10"), Weekdays: []domain.Weekday{4}},
			}}

			plan := service.resolveParticipation(
				data,
				domain.Date("2026-09-07"),
				domain.Date("2026-09-11"),
				"09:00",
			)

			assert.Equal(t, test.wantEffective, plan.EffectiveFrom)
			assert.Equal(t, test.wantWeekdays, plan.Weekdays)
			assert.Equal(t, domain.ParticipationRegular, plan.Days[1].Source)
			assert.Equal(t, domain.ParticipationRegular, plan.Days[3].Source)
		})
	}
}
