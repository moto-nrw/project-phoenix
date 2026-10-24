package grouplive

// PickupTime is today's effective Gehzeit of one child as the OGS group view
// ships it.
type PickupTime struct {
	StudentID   int64     `json:"student_id,string"`
	Date        string    `json:"date"`
	WeekdayName string    `json:"weekday_name"`
	PickupTime  *string   `json:"pickup_time,omitempty"`
	IsException bool      `json:"is_exception"`
	Notes       string    `json:"notes,omitempty"`
	DayNotes    []DayNote `json:"day_notes,omitempty"`

	// DayDepartureMode is the Abholart set for this day only (#3946).
	DayDepartureMode string `json:"day_departure_mode,omitempty"`
}

func pickupTimes(students []Student, pickups map[int64]Pickup) []PickupTime {
	result := make([]PickupTime, 0, len(students))
	for i := range students {
		if !students[i].fullAccess {
			continue
		}
		effective, ok := pickups[students[i].ID]
		if !ok {
			continue
		}
		item := PickupTime{
			StudentID: students[i].ID, Date: string(effective.Date), WeekdayName: effective.WeekdayName,
			IsException: effective.IsException, Notes: effective.Notes,
			DayDepartureMode: effective.DayDepartureMode,
		}
		if effective.PickupTime != nil {
			formatted := effective.PickupTime.Format("15:04")
			item.PickupTime = &formatted
		}
		for _, note := range effective.DayNotes {
			item.DayNotes = append(item.DayNotes, DayNote{ID: note.ID, Content: note.Content})
		}
		result = append(result, item)
	}
	return result
}
