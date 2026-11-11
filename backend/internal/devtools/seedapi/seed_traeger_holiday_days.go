package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerHolidayDaysStep plans a second holiday period of the demo Träger
// that is booked by days (#4701): the Winterferien week offers single days at
// 15 € a day with one place on each day, and a child may come on at most
// three days. Three families register: the first takes Monday and Tuesday,
// the second wants Tuesday and Wednesday and waits on the waiting list, the
// third takes Wednesday and passes it. The Träger-Büro confirms the first,
// so its contribution prices two days. The page „Plätze“ then shows the
// places of each day and the waiting child with its days.
type seedTraegerHolidayDaysStep struct{}

func (seedTraegerHolidayDaysStep) Name() string {
	return "Seeding a Träger holiday period booked by days"
}

func (seedTraegerHolidayDaysStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger holiday day seed prerequisites are not available")
	}
	if skipped, _ := rt.Values[seedHolidayRegistrationsSkippedKey].(bool); skipped {
		fmt.Println("  Träger-Büro: holiday registrations refused before, no period booked by days")
		return nil
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	today := calendar.TodayDate()
	monday := seedNextWinterMonday(today)
	raw, err := rt.Client.PostWithAuth(auth, "/traeger/holiday-periods/", seedHolidayDayPeriod(today, monday, rt.Bootstrap.SchoolID))
	if err != nil {
		return fmt.Errorf("create the holiday period booked by days: %w", err)
	}
	periodID, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the holiday period booked by days: %w", err)
	}
	published, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/publish", periodID), map[string]any{})
	if err != nil {
		return fmt.Errorf("publish the holiday period booked by days: %w", err)
	}
	var envelope struct {
		Data struct {
			PublicToken string `json:"public_token"`
			Weeks       []struct {
				ID         int64 `json:"id"`
				TimeModels []struct {
					ID int64 `json:"id"`
				} `json:"time_models"`
			} `json:"weeks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(published, &envelope); err != nil || envelope.Data.PublicToken == "" ||
		len(envelope.Data.Weeks) == 0 || len(envelope.Data.Weeks[0].TimeModels) == 0 {
		return fmt.Errorf("the holiday period booked by days has no link or no bookable week")
	}
	week := envelope.Data.Weeks[0]
	capacity := fmt.Sprintf("/traeger/holiday-contributions/%d/weeks/%d/capacity", periodID, week.ID)
	if _, err := rt.Client.PutWithAuth(auth, capacity, map[string]any{"capacity": 1}); err != nil {
		return fmt.Errorf("give the days one place each: %w", err)
	}
	fmt.Println("  Träger-Büro: Winterferien booked by days, one place a day, at most 3 days per child")
	return seedHolidayDayRegistrations(rt, auth, envelope.Data.PublicToken, periodID, week.ID, week.TimeModels[0].ID, monday)
}

// seedHolidayDayRegistrations registers the three families and confirms the
// first.
func seedHolidayDayRegistrations(rt *Runtime, auth AuthRef, token string, periodID, weekID, modelID int64, monday calendar.Date) error {
	families := []struct {
		first, last, child, email string
		days                      []calendar.Date
	}{
		{"Mira", "Tageskind", "Emil", "mira.tageskind@example.org", []calendar.Date{monday, monday.AddDays(1)}},
		{"Nora", "Wartetag", "Ida", "nora.wartetag@example.org", []calendar.Date{monday.AddDays(1), monday.AddDays(2)}},
		{"Ole", "Mittwoch", "Paul", "ole.mittwoch@example.org", []calendar.Date{monday.AddDays(2)}},
	}
	var first int64
	for i, family := range families {
		days := make([]string, len(family.days))
		for j, day := range family.days {
			days[j] = day.String()
		}
		raw, err := rt.Client.PostPublic("/traeger/holiday-registrations/"+token, map[string]any{
			"guardian": map[string]any{"first_name": family.first, "last_name": family.last, "email": family.email, "phone": fmt.Sprintf("0151 000010%d", i)},
			"children": []map[string]any{{
				"first_name": family.child, "last_name": family.last, "birth_date": "2018-02-14", "home_school_id": rt.Bootstrap.SchoolID,
				"bookings": []map[string]any{{"week_id": weekID, "time_model_id": modelID, "days": days}},
			}},
			"emergency_contacts": []map[string]any{{"name": "Oma " + family.last, "phone": "05402 000010", "relationship": "Großmutter"}},
			"privacy_consent":    true, "binding_confirmed": true, "custody_confirmed": true,
			"declaration_name": family.first + " " + family.last, "consent_version": seedHolidayConsentVersion, "retention_days": seedHolidayRetentionDays, "captcha_token": seedTurnstileTestToken,
		})
		var refused *APIError
		if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
			fmt.Printf("  Ferienanmeldung: registration by days refused (%s)\n", refused.Code)
			return nil
		}
		if err != nil {
			return fmt.Errorf("register a child for single days: %w", err)
		}
		if i == 0 {
			if first, err = decodeSeedEntityID(raw); err != nil {
				return fmt.Errorf("read the registration by days: %w", err)
			}
		}
	}
	fmt.Println("  Ferienanmeldung: three children booked single days, one of them waits")
	confirm := fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d/confirm", periodID, first)
	if _, err := rt.Client.PostWithAuth(auth, confirm, map[string]any{"benefits_exemption": false, "debtor_number": "D-4701"}); err != nil {
		return fmt.Errorf("confirm the registration by days: %w", err)
	}
	fmt.Println("  Träger-Büro: registration by days confirmed with two days at the day price")
	return nil
}

// seedHolidayDayPeriod is the Winterferien period of one week booked by days.
func seedHolidayDayPeriod(today, monday calendar.Date, host int64) map[string]any {
	return map[string]any{
		"name":                  fmt.Sprintf("Ferienbetreuung Winterferien %d", monday.Year()),
		"registration_opens_on": today.String(), "registration_closes_on": monday.AddDays(-3).String(),
		"binding": true, "max_days_per_child": 3,
		"notes": "Sie wählen einzelne Tage. Ein Kind kann höchstens drei Tage kommen.",
		"weeks": []map[string]any{{
			"starts_on": monday.String(), "ends_on": monday.AddDays(4).String(), "host_school_id": host, "booking_unit": "day",
			"time_models": []map[string]any{{"name": "bis 15 Uhr", "starts_at": "08:00", "ends_at": "15:00", "price_cents": 1500}},
		}},
	}
}

// seedNextWinterMonday is the first Monday of February whose registration
// window still has more than a week left.
func seedNextWinterMonday(today calendar.Date) calendar.Date {
	for year := today.Year(); ; year++ {
		day := calendar.NewDate(year, time.February, 1)
		for day.Weekday() != time.Monday {
			day = day.AddDays(1)
		}
		if day.AddDays(-3).After(today.AddDays(7)) {
			return day
		}
	}
}
