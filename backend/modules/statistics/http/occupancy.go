package statisticshttp

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// Belegung nach Uhrzeit (#4764) on the wire. Slot starts travel as "HH:MM"
// Berlin wall-clock strings.

type occupancySlotResponse struct {
	Start          string   `json:"start"`
	End            string   `json:"end"`
	PlannedAverage *float64 `json:"planned_average"`
	PlannedMax     *int     `json:"planned_max"`
	ActualAverage  *float64 `json:"actual_average"`
	ActualMax      *int     `json:"actual_max"`
}

type occupancyWeekdayResponse struct {
	// Weekday is ISO: 1 = Monday … 7 = Sunday.
	Weekday    int                     `json:"weekday"`
	PlanDays   int                     `json:"plan_days"`
	ActualDays int                     `json:"actual_days"`
	Slots      []occupancySlotResponse `json:"slots"`
}

type occupancyResponse struct {
	SlotMinutes     int                        `json:"slot_minutes"`
	Weekdays        []occupancyWeekdayResponse `json:"weekdays"`
	UntimedPlanDays int                        `json:"untimed_plan_days"`
	ActualDataDays  int                        `json:"actual_data_days"`
	ActualDataFrom  string                     `json:"actual_data_from"`
	PlanDataDays    int                        `json:"plan_data_days"`
	PlanDataFrom    string                     `json:"plan_data_from"`
}

func toOccupancyResponse(occupancy *studentpresence.StatisticsOccupancy) *occupancyResponse {
	if occupancy == nil {
		return nil
	}
	out := &occupancyResponse{
		SlotMinutes:     occupancy.SlotMinutes,
		Weekdays:        make([]occupancyWeekdayResponse, 0, len(occupancy.Weekdays)),
		UntimedPlanDays: occupancy.UntimedPlanDays,
		ActualDataDays:  occupancy.ActualDataDays,
		ActualDataFrom:  occupancy.ActualDataFrom.String(),
		PlanDataDays:    occupancy.PlanDataDays,
		PlanDataFrom:    occupancy.PlanDataFrom.String(),
	}
	for _, weekday := range occupancy.Weekdays {
		row := occupancyWeekdayResponse{
			Weekday: weekday.Weekday, PlanDays: weekday.PlanDays, ActualDays: weekday.ActualDays,
			Slots: make([]occupancySlotResponse, 0, len(weekday.Slots)),
		}
		for _, slot := range weekday.Slots {
			row.Slots = append(row.Slots, occupancySlotResponse{
				Start:          formatSlotStart(slot.StartMinute),
				End:            formatSlotStart(slot.StartMinute + occupancy.SlotMinutes),
				PlannedAverage: slot.PlannedAverage,
				PlannedMax:     slot.PlannedMax,
				ActualAverage:  slot.ActualAverage,
				ActualMax:      slot.ActualMax,
			})
		}
		out.Weekdays = append(out.Weekdays, row)
	}
	return out
}

func formatSlotStart(minute int) string {
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}
