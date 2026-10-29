package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestCollectOfferingWeekdayBookingsServiceWindow(t *testing.T) {
	t.Parallel()
	start := calendar.NewDate(2026, 11, 1)
	end := calendar.NewDate(2026, 11, 30)
	before := start.AddDays(-1)
	after := end.AddDays(1)
	insideStart := start.AddDays(5)
	insideEnd := end.AddDays(-5)
	phase := careplan.OfferingPhase{ServiceStart: start, ServiceEnd: end}
	offering := &careplan.CareOffering{DaysOfWeekMode: daysOfWeekModeFixed, AvailableDays: []string{"mon"}}
	for _, tc := range []struct {
		name                string
		from, until         *calendar.Date
		wantFrom, wantUntil calendar.Date
		empty               bool
	}{
		{name: "open link", wantFrom: start, wantUntil: after},
		{name: "wider link", from: &before, until: &after, wantFrom: start, wantUntil: after},
		{name: "narrower link", from: &insideStart, until: &insideEnd, wantFrom: insideStart, wantUntil: insideEnd},
		{name: "ends at service start", until: &start, empty: true},
		{name: "starts after service end", from: &after, empty: true},
		{name: "last service day", from: &end, wantFrom: end, wantUntil: after},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bookings := collectOfferingWeekdayBookings(
				map[int64]*careplan.CareOffering{offering.ID: offering},
				map[int64]careplan.OfferingPhase{phase.ID: phase},
				[]ports.ApprovedOfferingChild{{Link: careplan.BookedOffering{CareOfferingID: offering.ID, ValidFrom: tc.from, ValidUntil: tc.until}}},
				before, nil,
			)
			if tc.empty {
				require.Empty(t, bookings)
				return
			}
			require.Len(t, bookings, 1)
			require.Len(t, bookings[0].Windows, 1)
			assert.Equal(t, &tc.wantFrom, bookings[0].Windows[0].ValidFrom)
			assert.Equal(t, &tc.wantUntil, bookings[0].Windows[0].ValidUntil)
			assert.Equal(t, []int{1}, bookings[0].Windows[0].Weekdays)
		})
	}
}
