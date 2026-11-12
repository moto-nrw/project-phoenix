package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
)

// seedHolidayWishes lets the period take the holiday care wish of the OGS
// registration into account (#4703): the Träger-Büro switches the rule on,
// a third family registers for the full week without a parent account, so
// its child carries „Ferienwunsch prüfen“, and the office decides that the
// first waiting child has no holiday care wish. That child then waits behind
// the later one. A demo server that refuses the registration keeps the rule
// without the decision.
func seedHolidayWishes(rt *Runtime, auth AuthRef, token string, periodID int64, late map[string]any) error {
	path := fmt.Sprintf("/traeger/holiday-periods/%d", periodID)
	raw, err := rt.Client.GetWithAuth(auth, path)
	if err != nil {
		return fmt.Errorf("read the holiday period: %w", err)
	}
	var period struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &period); err != nil || period.Data == nil {
		return fmt.Errorf("decode the holiday period: %w", err)
	}
	period.Data["holiday_wish_rule"] = true
	if _, err := rt.Client.PutWithAuth(auth, path, period.Data); err != nil {
		return fmt.Errorf("switch on the holiday care wish of the period: %w", err)
	}
	fmt.Println("  Träger-Büro: the holiday period takes the holiday care wish into account")
	later := maps.Clone(late)
	child := maps.Clone(late["children"].([]map[string]any)[0])
	child["first_name"], child["last_name"], child["birth_date"] = "Mila", "Wunsch", "2017-11-12"
	later["children"] = []map[string]any{child}
	later["guardian"] = map[string]any{"first_name": "Nora", "last_name": "Wunsch", "email": "nora.wunsch@example.org", "phone": "0151 0000005"}
	later["declaration_name"] = "Nora Wunsch"
	_, err = rt.Client.PostPublic("/traeger/holiday-registrations/"+token, later)
	var refused *APIError
	if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
		fmt.Printf("  Ferienanmeldung: registration with the wish refused (%s), no decision\n", refused.Code)
		return nil
	}
	if err != nil {
		return fmt.Errorf("register a child whose wish the office checks: %w", err)
	}
	waiting := late["children"].([]map[string]any)[0]
	return seedHolidayWishDecision(rt, auth, periodID, waiting["first_name"], waiting["last_name"])
}

// seedHolidayWishDecision lets the office decide „Ohne Ferienwunsch“ for
// the child of the late registration, found by its name among the children
// to check.
func seedHolidayWishDecision(rt *Runtime, auth AuthRef, periodID int64, firstName, lastName any) error {
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/registrations?wish=check", periodID))
	if err != nil {
		return fmt.Errorf("list the children whose wish the office checks: %w", err)
	}
	var list struct {
		Data struct {
			Registrations []struct {
				ID       int64 `json:"id"`
				Children []struct {
					ID        int64  `json:"id"`
					FirstName string `json:"first_name"`
					LastName  string `json:"last_name"`
					Check     bool   `json:"holiday_wish_check"`
				} `json:"children"`
			} `json:"registrations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("decode the children whose wish the office checks: %w", err)
	}
	for _, registration := range list.Data.Registrations {
		for _, child := range registration.Children {
			if !child.Check || child.FirstName != firstName || child.LastName != lastName {
				continue
			}
			wish := fmt.Sprintf("/traeger/holiday-periods/%d/registrations/%d/children/%d/holiday-wish", periodID, registration.ID, child.ID)
			if _, err := rt.Client.PutWithAuth(auth, wish, map[string]any{"wish": "no"}); err != nil {
				return fmt.Errorf("decide the holiday care wish: %w", err)
			}
			fmt.Println("  Träger-Büro: one waiting child has no holiday care wish and waits behind the others")
			return nil
		}
	}
	return errors.New("the late child carries no holiday care wish to check")
}
