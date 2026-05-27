package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
)

func TestExportRequestToListParamsPreservesRoomFilter(t *testing.T) {
	params := exportRequestToListParams(studentExportRequest{
		Filters: studentExportFilters{
			Search:  "  mila  ",
			GroupID: "17",
			RoomID:  "42",
		},
	})

	assert.Equal(t, "mila", params.search)
	assert.Equal(t, int64(17), params.groupID)
	assert.Equal(t, int64(42), params.roomID)
	assert.Equal(t, studentExportPageSize, params.pageSize)
	assert.True(t, params.includePickupTimes)
	assert.True(t, params.includeArrivalTimes)
}

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
