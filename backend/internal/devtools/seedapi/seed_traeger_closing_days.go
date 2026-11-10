package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerClosingDayStep plans the bridge day after Christi Himmelfahrt as
// a closing day of the Träger for all its schools (#4702), through the
// Träger route in the Träger-Büro admin's session, the way the Träger-Büro
// does it. It runs after the holiday period, whose weeks then stay bookable.
type seedTraegerClosingDayStep struct{}

func (seedTraegerClosingDayStep) Name() string { return "Seeding a Träger closing day" }

func (seedTraegerClosingDayStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger closing day seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	today := calendar.TodayDate()
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/traeger/closing-days/?from=%s&to=%s", today, today.AddDays(365)))
	if err != nil {
		return fmt.Errorf("read the Träger calendar: %w", err)
	}
	bridge, schools, err := seedBridgeDay(raw)
	if err != nil {
		return err
	}
	if len(schools) == 0 || bridge.IsZero() {
		fmt.Println("  Träger-Büro: no school or no Christi Himmelfahrt in the calendar, closing day skipped")
		return nil
	}
	if _, err := rt.Client.PostWithAuth(auth, "/traeger/closing-days/", map[string]any{
		"start_date": bridge.String(), "end_date": bridge.String(), "reason": "Brückentag", "school_ids": schools,
	}); err != nil {
		return fmt.Errorf("plan the Träger closing day: %w", err)
	}
	fmt.Printf("  Träger-Büro: closing day %s for %d schools\n", bridge, len(schools))
	return nil
}

// seedBridgeDay names the Friday after the next Christi Himmelfahrt and every
// school of the Träger calendar.
func seedBridgeDay(raw []byte) (calendar.Date, []int64, error) {
	var envelope struct {
		Data struct {
			Schools []struct {
				ID int64 `json:"id"`
			} `json:"schools"`
			Holidays []struct {
				Holidays []struct {
					Date calendar.Date `json:"date"`
					Name string        `json:"name"`
				} `json:"holidays"`
			} `json:"holidays"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", nil, fmt.Errorf("read the Träger calendar: %w", err)
	}
	schools := make([]int64, 0, len(envelope.Data.Schools))
	for _, school := range envelope.Data.Schools {
		schools = append(schools, school.ID)
	}
	for _, region := range envelope.Data.Holidays {
		for _, holiday := range region.Holidays {
			if holiday.Name == "Christi Himmelfahrt" {
				return holiday.Date.AddDays(1), schools, nil
			}
		}
	}
	return "", schools, nil
}

// seedHolidayPeriodShifts bounds how often the seed moves the two weeks on.
const seedHolidayPeriodShifts = 8

// seedHolidayPeriodOutsideNoCareDays creates the holiday period and, while a
// week holds a statutory holiday or a closing day of the host school (#4702),
// moves both weeks one week on: the seed runs on any day of the year.
func seedHolidayPeriodOutsideNoCareDays(client *Client, auth AuthRef, today calendar.Date, host int64) ([]byte, error) {
	for shift := 0; ; shift++ {
		raw, err := client.PostWithAuth(auth, "/traeger/holiday-periods/", seedHolidayPeriodShifted(today, host, shift))
		var refused *APIError
		if shift+1 < seedHolidayPeriodShifts && errors.As(err, &refused) && refused.Code == seedHolidayWeekNoCareDayCode {
			continue
		}
		return raw, err
	}
}

// seedHolidayWeekNoCareDayCode is the registered code of a week with a day
// without care.
const seedHolidayWeekNoCareDayCode = "enrollment.holiday_week_no_care_day"
