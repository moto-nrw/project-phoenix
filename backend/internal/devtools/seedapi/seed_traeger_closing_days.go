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
	if _, err := seedPlanTraegerClosingDay(rt.Client, auth, bridge, bridge, "Brückentag", schools); err != nil {
		return fmt.Errorf("plan the Träger closing day: %w", err)
	}
	fmt.Printf("  Träger-Büro: closing day %s for %d schools\n", bridge, len(schools))
	return nil
}

// seedHolidayClosuresStep puts closing days into a booked holiday week
// (#6434). It runs after every step that books the holiday weeks: a closing
// day refuses each new booking of its week.
type seedHolidayClosuresStep struct{}

func (seedHolidayClosuresStep) Name() string { return "Seeding closing days in a booked holiday week" }

func (seedHolidayClosuresStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the holiday closure seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	return seedHolidayClosures(rt, auth)
}

// seedPlanTraegerClosingDay saves a closing day of the Träger the way the
// Büro does (#6434): it previews the holiday bookings the day affects and
// confirms that preview with the save, which tells their parents. It
// returns how many families the save told.
func seedPlanTraegerClosingDay(client *Client, auth AuthRef, from, to calendar.Date, reason string, schools []int64) (int, error) {
	body := map[string]any{"start_date": from.String(), "end_date": to.String(), "reason": reason, "school_ids": schools}
	raw, err := client.PostWithAuth(auth, "/traeger/holiday-closures/preview", body)
	if err != nil {
		return 0, fmt.Errorf("preview the holiday bookings of the closing day: %w", err)
	}
	var preview struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &preview); err != nil {
		return 0, fmt.Errorf("read the holiday bookings of the closing day: %w", err)
	}
	body["holiday_bookings_token"] = preview.Data.Token
	raw, err = client.PostWithAuth(auth, "/traeger/closing-days/", body)
	if err != nil {
		return 0, err
	}
	var saved struct {
		Data struct {
			NotifiedFamilies int `json:"notified_families"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &saved); err != nil {
		return 0, fmt.Errorf("read the saved closing day: %w", err)
	}
	return saved.Data.NotifiedFamilies, nil
}

// seedHolidayClosures puts closing days into the booked autumn week of the
// seeded holiday period (#6434): the Träger's staff outing on its Wednesday,
// whose save tells the parents, and the host school's own pedagogical day on
// its Thursday, which the Träger-Büro then finds among the parents not told
// yet. A run without registrations or without a school session plans
// neither.
func seedHolidayClosures(rt *Runtime, auth AuthRef) error {
	published, ok := rt.Values[seedHolidayLegalPublishedKey].([]byte)
	if !ok || rt.Values[seedHolidayRegistrationsSkippedKey] == true {
		return nil
	}
	var period struct {
		Data struct {
			Weeks []struct {
				StartsOn     calendar.Date `json:"starts_on"`
				HostSchoolID int64         `json:"host_school_id"`
			} `json:"weeks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(published, &period); err != nil {
		return fmt.Errorf("read the holiday weeks: %w", err)
	}
	if len(period.Data.Weeks) == 0 {
		return nil
	}
	week := period.Data.Weeks[0]
	outing := week.StartsOn.AddDays(2)
	families, err := seedPlanTraegerClosingDay(rt.Client, auth, outing, outing, "Betriebsausflug", []int64{week.HostSchoolID})
	if err != nil {
		return fmt.Errorf("plan the closing day in the booked holiday week: %w", err)
	}
	fmt.Printf("  Träger-Büro: closing day %s in a booked holiday week, %d famil(ies) told\n", outing, families)
	if rt.TenantAuth.Token == "" {
		return nil
	}
	pedagogical := week.StartsOn.AddDays(3)
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/timetable/closing-days", map[string]any{
		"start_date": pedagogical.String(), "end_date": pedagogical.String(), "reason": "Pädagogischer Tag",
	}); err != nil {
		return fmt.Errorf("plan the school's closing day in the booked holiday week: %w", err)
	}
	notified, err := seedNotifySchoolHolidayClosures(rt)
	if err != nil {
		return err
	}
	fmt.Printf("  Ausrichtende OGS: closing day %s in a booked holiday week, %d notice(s) sent by one click\n", pedagogical, notified)
	// A second closing day of the school stays a proposal: the school and
	// the Träger-Büro both see it waiting for the click (ADR 0196).
	training := week.StartsOn.AddDays(4)
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/timetable/closing-days", map[string]any{
		"start_date": training.String(), "end_date": training.String(), "reason": "Teamfortbildung",
	}); err != nil {
		return fmt.Errorf("plan the school's second closing day in the booked holiday week: %w", err)
	}
	fmt.Printf("  Ausrichtende OGS: closing day %s in a booked holiday week, parents not told yet\n", training)
	return nil
}

// seedNotifySchoolHolidayClosures clicks "Eltern informieren" on every
// notice of the school that waits for it, as the school admin does
// (ADR 0196), and returns how many it confirmed.
func seedNotifySchoolHolidayClosures(rt *Runtime) (int, error) {
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, "/api/holiday-closure-notices")
	if err != nil {
		return 0, fmt.Errorf("read the school's holiday closure notices: %w", err)
	}
	var notices struct {
		Data []struct {
			ID        int64 `json:"id"`
			Revision  int64 `json:"revision"`
			CanNotify bool  `json:"can_notify"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &notices); err != nil {
		return 0, fmt.Errorf("read the school's holiday closure notices: %w", err)
	}
	confirmed := 0
	for _, notice := range notices.Data {
		if !notice.CanNotify {
			continue
		}
		if _, err := rt.Client.PostWithAuth(rt.TenantAuth, fmt.Sprintf("/api/holiday-closure-notices/%d/notify", notice.ID),
			map[string]any{"revision": notice.Revision}); err != nil {
			return confirmed, fmt.Errorf("tell the parents about the school's closing day: %w", err)
		}
		confirmed++
	}
	return confirmed, nil
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
