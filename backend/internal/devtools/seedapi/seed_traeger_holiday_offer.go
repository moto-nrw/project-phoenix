package api

import (
	"encoding/json"
	"fmt"
)

// seedHolidayRegistrationOffer reads the start page of the parents app as
// the signed-in seeded parent (#6671): the published period of the link is
// offered there while its registration is open, for at least one child of
// the family. That keeps the card "Ferienbetreuung" filled on every seeded
// machine.
func seedHolidayRegistrationOffer(rt *Runtime, auth AuthRef, token string) error {
	raw, err := rt.Client.GetWithAuth(auth, "/parent/me/holiday-registrations")
	if err != nil {
		return fmt.Errorf("read the holiday periods of the parents app: %w", err)
	}
	var envelope struct {
		Data []struct {
			Token    string `json:"token"`
			Open     bool   `json:"open"`
			Children []struct {
				StudentID string `json:"student_id"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode the holiday periods of the parents app: %w", err)
	}
	for _, offer := range envelope.Data {
		if offer.Token != token {
			continue
		}
		if !offer.Open || len(offer.Children) == 0 {
			return fmt.Errorf("the parents app offers the seeded holiday period without an open registration or without a child")
		}
		fmt.Printf("  Eltern-App: the start page offers the open holiday period for %d child(ren)\n", len(offer.Children))
		return nil
	}
	return fmt.Errorf("the parents app does not offer the seeded holiday period to the seeded parent")
}
