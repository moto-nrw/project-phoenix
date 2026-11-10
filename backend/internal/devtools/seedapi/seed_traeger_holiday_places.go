package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
)

// seedHolidayWaitlist gives the week of the third sibling as many places as
// it has children with a place, so it is full, and lets one more family
// register for it (#4699): its child lands on the waiting list of the week,
// which the Träger-Büro sees under „Plätze“. A demo server that refuses the
// registration keeps the week full without a waiting list.
func seedHolidayWaitlist(rt *Runtime, auth AuthRef, token string, periodID int64, sibling map[string]any) error {
	weekID, ok := seedFirstBookedWeek(sibling)
	if !ok {
		return errors.New("the third sibling books no week")
	}
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/places", periodID))
	if err != nil {
		return fmt.Errorf("read the places of the holiday weeks: %w", err)
	}
	var places struct {
		Data []struct {
			WeekID int64 `json:"week_id"`
			Placed int   `json:"placed"`
		} `json:"data"`
	}
	placed, found := 0, false
	if json.Unmarshal(raw, &places) == nil {
		for _, week := range places.Data {
			if week.WeekID == weekID {
				placed, found = week.Placed, true
			}
		}
	}
	if !found {
		fmt.Println("  Träger-Büro: the places of the holiday week unknown, no waiting list")
		return nil
	}
	capacity := fmt.Sprintf("/traeger/holiday-contributions/%d/weeks/%d/capacity", periodID, weekID)
	if _, err := rt.Client.PutWithAuth(auth, capacity, map[string]any{"capacity": placed}); err != nil {
		return fmt.Errorf("limit the places of the holiday week: %w", err)
	}
	fmt.Printf("  Träger-Büro: the holiday week holds %d places, all of them taken\n", placed)
	late := maps.Clone(sibling)
	child := maps.Clone(sibling["children"].([]map[string]any)[0])
	child["first_name"], child["last_name"], child["birth_date"] = "Jonas", "Spät", "2018-08-30"
	late["children"] = []map[string]any{child}
	late["guardian"] = map[string]any{"first_name": "Sina", "last_name": "Spät", "email": "sina.spaet@example.org", "phone": "0151 0000004"}
	late["declaration_name"] = "Sina Spät"
	_, err = rt.Client.PostPublic("/traeger/holiday-registrations/"+token, late)
	var refused *APIError
	if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
		fmt.Printf("  Ferienanmeldung: late registration refused (%s), no waiting list\n", refused.Code)
		return nil
	}
	if err != nil {
		return fmt.Errorf("register a child for the full holiday week: %w", err)
	}
	fmt.Println("  Ferienanmeldung: one more child on the waiting list of the full week")
	return nil
}

func seedFirstBookedWeek(registration map[string]any) (int64, bool) {
	children, _ := registration["children"].([]map[string]any)
	if len(children) == 0 {
		return 0, false
	}
	bookings, _ := children[0]["bookings"].([]map[string]any)
	if len(bookings) == 0 {
		return 0, false
	}
	switch id := bookings[0]["week_id"].(type) {
	case int64:
		return id, true
	case int:
		return int64(id), true
	case float64:
		return int64(id), true
	}
	return 0, false
}
