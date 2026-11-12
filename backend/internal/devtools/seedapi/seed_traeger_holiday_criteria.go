package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerHolidayCriteriaStep plans a holiday period of the demo Träger
// that gives its places by criteria (#4700): the Träger names three criteria
// with their priority and leaves a fourth switched off, the second
// Winterferien week has two places, and four families register with their
// answers. Nobody gets a place when sending. The Träger-Büro gives the
// first place of the proposal, so the page „Vergabe“ shows one decision and
// three pending children: one more proposed for the free place, two for the
// waiting list.
type seedTraegerHolidayCriteriaStep struct{}

func (seedTraegerHolidayCriteriaStep) Name() string {
	return "Seeding a Träger holiday period by criteria"
}

func (seedTraegerHolidayCriteriaStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger holiday criteria seed prerequisites are not available")
	}
	if skipped, _ := rt.Values[seedHolidayRegistrationsSkippedKey].(bool); skipped {
		fmt.Println("  Träger-Büro: holiday registrations refused before, no period by criteria")
		return nil
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	criteria, err := seedHolidayCriteria(rt, auth)
	if err != nil {
		return err
	}
	today := calendar.TodayDate()
	monday := seedNextWinterMonday(today).AddDays(7)
	raw, err := rt.Client.PostWithAuth(auth, "/traeger/holiday-periods/", seedHolidayCriteriaPeriod(today, monday, rt.Bootstrap.SchoolID))
	if err != nil {
		return fmt.Errorf("create the holiday period by criteria: %w", err)
	}
	periodID, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the holiday period by criteria: %w", err)
	}
	link, err := seedPublishedHolidayLink(rt, auth, periodID)
	if err != nil {
		return err
	}
	capacity := fmt.Sprintf("/traeger/holiday-contributions/%d/weeks/%d/capacity", periodID, link.weekID)
	if _, err := rt.Client.PutWithAuth(auth, capacity, map[string]any{"capacity": 2}); err != nil {
		return fmt.Errorf("give the week by criteria two places: %w", err)
	}
	fmt.Println("  Träger-Büro: Winterferien by criteria, two places, three criteria in their priority")
	registered, err := seedHolidayCriteriaRegistrations(rt, link, criteria)
	if err != nil || !registered {
		return err
	}
	return seedHolidayFirstAllocation(rt, auth, periodID)
}

// seedHolidayCriteria names the criteria of the demo Träger: working
// parents first, single parents second, a sibling in the OGS third, and a
// hardship case switched off. It returns their IDs in this order.
func seedHolidayCriteria(rt *Runtime, auth AuthRef) ([]int64, error) {
	raw, err := rt.Client.PutWithAuth(auth, "/traeger/holiday-periods/criteria", map[string]any{"criteria": []map[string]any{
		{"name": "Beide Eltern berufstätig", "question": "Sind beide Eltern berufstätig?", "scope": "family", "active": true},
		{"name": "Alleinerziehend und berufstätig", "question": "Sind Sie alleinerziehend und berufstätig?", "scope": "family", "active": true},
		{"name": "Geschwisterkind in der OGS", "question": "Geht ein Geschwisterkind schon in die OGS?", "scope": "child", "active": true},
		{"name": "Besonderer Härtefall", "question": "Gibt es einen besonderen Grund, warum Ihr Kind einen Platz braucht?", "scope": "family", "active": false},
	}})
	if err != nil {
		return nil, fmt.Errorf("name the holiday criteria: %w", err)
	}
	var envelope struct {
		Data struct {
			Criteria []struct {
				ID int64 `json:"id"`
			} `json:"criteria"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data.Criteria) < 3 {
		return nil, fmt.Errorf("the holiday criteria were not stored")
	}
	ids := make([]int64, len(envelope.Data.Criteria))
	for i, criterion := range envelope.Data.Criteria {
		ids[i] = criterion.ID
	}
	return ids, nil
}

// seedHolidayLink is the public link and the bookable week of a published
// period.
type seedHolidayLink struct {
	token           string
	weekID, modelID int64
}

func seedPublishedHolidayLink(rt *Runtime, auth AuthRef, periodID int64) (seedHolidayLink, error) {
	published, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/publish", periodID), map[string]any{})
	if err != nil {
		return seedHolidayLink{}, fmt.Errorf("publish the holiday period by criteria: %w", err)
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
		return seedHolidayLink{}, fmt.Errorf("the holiday period by criteria has no link or no bookable week")
	}
	week := envelope.Data.Weeks[0]
	return seedHolidayLink{token: envelope.Data.PublicToken, weekID: week.ID, modelID: week.TimeModels[0].ID}, nil
}

// seedHolidayCriteriaRegistrations registers four families with their
// answers, the earliest without any criterion met. false reports a refused
// registration, which ends the step quietly.
func seedHolidayCriteriaRegistrations(rt *Runtime, link seedHolidayLink, criteria []int64) (bool, error) {
	families := []struct {
		first, last, child, email string
		working, single, sibling  bool
	}{
		{"Lena", "Frühauf", "Jonas", "lena.fruehauf@example.org", false, false, false},
		{"Sami", "Arbeit", "Mia", "sami.arbeit@example.org", true, false, false},
		{"Tina", "Allein", "Noah", "tina.allein@example.org", false, true, true},
		{"Kai", "Geschwister", "Ella", "kai.geschwister@example.org", false, false, true},
	}
	for i, family := range families {
		// Each family sends from its own address, as real families do, so the
		// four registrations leave the submission limit of one address to the
		// steps after this one.
		_, err := rt.Client.PostPublicWithHeaders("/traeger/holiday-registrations/"+link.token, map[string]any{
			"guardian": map[string]any{"first_name": family.first, "last_name": family.last, "email": family.email, "phone": fmt.Sprintf("0151 000020%d", i)},
			"criteria": map[string]bool{fmt.Sprint(criteria[0]): family.working, fmt.Sprint(criteria[1]): family.single},
			"children": []map[string]any{{
				"first_name": family.child, "last_name": family.last, "birth_date": "2018-04-21", "home_school_id": rt.Bootstrap.SchoolID,
				"bookings": []map[string]any{{"week_id": link.weekID, "time_model_id": link.modelID}},
				"criteria": map[string]bool{fmt.Sprint(criteria[2]): family.sibling},
			}},
			"emergency_contacts": []map[string]any{{"name": "Oma " + family.last, "phone": "05402 000020", "relationship": "Großmutter"}},
			"privacy_consent":    true, "binding_confirmed": true, "custody_confirmed": true,
			"declaration_name": family.first + " " + family.last, "consent_version": seedHolidayConsentVersion, "retention_days": seedHolidayRetentionDays, "captcha_token": seedTurnstileTestToken,
		}, publicEnrollmentSeedHeaders(120+i))
		var refused *APIError
		if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
			fmt.Printf("  Ferienanmeldung: registration by criteria refused (%s)\n", refused.Code)
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("register a child by criteria: %w", err)
		}
	}
	fmt.Println("  Ferienanmeldung: four children wait for the decision of the Träger-Büro")
	return true, nil
}

// seedHolidayFirstAllocation takes the proposal for its first place: the
// family that meets the first criterion gets it.
func seedHolidayFirstAllocation(rt *Runtime, auth AuthRef, periodID int64) error {
	raw, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/allocation", periodID))
	if err != nil {
		return fmt.Errorf("read the holiday allocation: %w", err)
	}
	var envelope struct {
		Data struct {
			Weeks []struct {
				Pending []struct {
					BookingID int64  `json:"booking_id"`
					Proposal  string `json:"proposal"`
				} `json:"pending"`
			} `json:"weeks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data.Weeks) == 0 || len(envelope.Data.Weeks[0].Pending) == 0 {
		return fmt.Errorf("the holiday allocation lists no pending child")
	}
	first := envelope.Data.Weeks[0].Pending[0]
	if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/allocation/decisions", periodID), map[string]any{
		"decisions": []map[string]any{{"booking_id": first.BookingID, "placement": first.Proposal, "proposal": first.Proposal}},
	}); err != nil {
		return fmt.Errorf("allocate the first holiday place: %w", err)
	}
	fmt.Println("  Träger-Büro: first place of the proposal given, three children still pending")
	return nil
}

// seedHolidayCriteriaPeriod is the Winterferien period by criteria of one
// week.
func seedHolidayCriteriaPeriod(today, monday calendar.Date, host int64) map[string]any {
	return map[string]any{
		"name":                  fmt.Sprintf("Ferienbetreuung Winterferien %d – Vergabe nach Kriterien", monday.Year()),
		"registration_opens_on": today.String(), "registration_closes_on": monday.AddDays(-10).String(),
		"binding": true, "allocation_mode": "criteria",
		"notes": "Die Plätze vergibt das Büro nach den Vergabekriterien des Trägers.",
		"weeks": []map[string]any{{
			"starts_on": monday.String(), "ends_on": monday.AddDays(4).String(), "host_school_id": host,
			"time_models": []map[string]any{{"name": "bis 16 Uhr", "starts_at": "08:00", "ends_at": "16:00", "price_cents": 6500}},
		}},
	}
}
