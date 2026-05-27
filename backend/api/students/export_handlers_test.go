package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/schedule"
)

func TestWeeklyCellUsesExplicitLabels(t *testing.T) {
	tests := []struct {
		name string
		plan weeklySchedule
		want string
	}{
		{
			name: "arrival and pickup",
			plan: weeklySchedule{
				ArrivalByWeekday: map[int]string{schedule.WeekdayMonday: "08:00"},
				PickupByWeekday:  map[int]string{schedule.WeekdayMonday: "16:00"},
			},
			want: "Ankunft: 08:00, Abholung: 16:00",
		},
		{
			name: "pickup only",
			plan: weeklySchedule{
				ArrivalByWeekday: map[int]string{},
				PickupByWeekday:  map[int]string{schedule.WeekdayMonday: "16:00"},
			},
			want: "Abholung: 16:00",
		},
		{
			name: "arrival only",
			plan: weeklySchedule{
				ArrivalByWeekday: map[int]string{schedule.WeekdayMonday: "08:00"},
				PickupByWeekday:  map[int]string{},
			},
			want: "Ankunft: 08:00",
		},
		{
			name: "no plan",
			plan: weeklySchedule{
				ArrivalByWeekday: map[int]string{},
				PickupByWeekday:  map[int]string{},
			},
			want: "nein",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weeklyCell(tt.plan, schedule.WeekdayMonday); got != tt.want {
				t.Fatalf("weeklyCell() = %q, want %q", got, tt.want)
			}
		})
	}
}
