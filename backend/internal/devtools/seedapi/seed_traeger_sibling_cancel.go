package api

import (
	"errors"
	"fmt"
	"slices"
)

// seedHolidaySiblingCancellation lets the second parent register a fourth
// sibling by mistake; the Träger-Büro merges it with the siblings and then
// cancels it (#5662). The cancellation takes it out of the group at once,
// the rest of the group stays, and the history of every registration of
// the group names the cancellation. A demo server that refuses the
// registration keeps the group as it is.
func seedHolidaySiblingCancellation(rt *Runtime, auth AuthRef, token string, periodID int64, sibling map[string]any, siblings int64) error {
	fourth := map[string]any{}
	for key, value := range sibling {
		fourth[key] = value
	}
	child := map[string]any{}
	for key, value := range sibling["children"].([]map[string]any)[0] {
		child[key] = value
	}
	child["first_name"], child["birth_date"] = "Paul", "2019-05-14"
	fourth["children"] = []map[string]any{child}
	raw, err := rt.Client.PostPublic("/traeger/holiday-registrations/"+token, fourth)
	var refused *APIError
	if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
		fmt.Printf("  Ferienanmeldung: fourth sibling refused (%s), no cancellation in the group\n", refused.Code)
		return nil
	}
	if err != nil {
		return fmt.Errorf("register the fourth sibling: %w", err)
	}
	cancelled, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the fourth sibling's registration: %w", err)
	}
	merge := fmt.Sprintf("/traeger/holiday-contributions/%d/sibling-groups", periodID)
	if _, err := rt.Client.PostWithAuth(auth, merge, map[string]any{"registration_ids": []int64{siblings, cancelled}}); err != nil {
		return fmt.Errorf("merge the fourth sibling with the siblings: %w", err)
	}
	cancel := fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d/cancel", periodID, cancelled)
	if _, err := rt.Client.PostWithAuth(auth, cancel, map[string]any{}); err != nil {
		return fmt.Errorf("cancel the fourth sibling: %w", err)
	}
	fmt.Println("  Träger-Büro: the fourth sibling's registration cancelled, the group computed again")
	return nil
}
