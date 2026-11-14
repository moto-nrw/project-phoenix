package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// seedTraegerStaffStep seeds the Personalstamm of the demo Träger (#4705)
// through the routes the people use: the moto team grants the release
// „Personal beim Träger“ in the Operator portal, then the Personalverwaltung
// confirms the proposals of the overview, choosing the first value where the
// schools disagree, and keeps an invented IBAN for the first record. Several
// demo schools seed the same Träger; a release or link another school's run
// made is no error.
type seedTraegerStaffStep struct{}

func (seedTraegerStaffStep) Name() string { return "Seeding the staff records of the Träger" }

// seedTraegerStaffLinks bounds the proposals the seed confirms, so the
// overview keeps open proposals to review.
const seedTraegerStaffLinks = 6

// seedTraegerStaffIBAN is an invented IBAN with a valid checksum.
const seedTraegerStaffIBAN = "DE02120300000000202051"

func (seedTraegerStaffStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.OrganizationID <= 0 {
		return fmt.Errorf("the Träger staff seed prerequisites are not available")
	}
	if err := seedTraegerStaffRelease(rt); err != nil {
		return err
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	suggestions, err := seedTraegerStaffSuggestions(rt.Client, auth)
	if err != nil {
		return err
	}
	linked, first, err := seedTraegerStaffLinkAll(rt.Client, auth, suggestions)
	if err != nil {
		return err
	}
	if first > 0 {
		if _, err := rt.Client.PutWithAuth(auth, fmt.Sprintf("/traeger/staff-records/%d/financial", first), map[string]any{"iban": seedTraegerStaffIBAN}); err != nil {
			return fmt.Errorf("keep the bank values of a Träger staff record: %w", err)
		}
	}
	fmt.Printf("  Träger staff records: %d linked\n", linked)
	return nil
}

// seedTraegerStaffLinkAll confirms up to seedTraegerStaffLinks proposals and
// returns how many it linked and the first record it created.
func seedTraegerStaffLinkAll(client *Client, auth AuthRef, suggestions []seedTraegerStaffSuggestion) (int, int64, error) {
	linked := 0
	first := int64(0)
	for _, suggestion := range suggestions {
		if linked == seedTraegerStaffLinks {
			break
		}
		id, err := seedTraegerStaffLink(client, auth, suggestion)
		if err != nil {
			return 0, 0, err
		}
		if id > 0 {
			linked++
		}
		if first == 0 {
			first = id
		}
	}
	return linked, first, nil
}

// seedTraegerStaffRelease grants the release with the statement of an
// agreement with the employee representation.
func seedTraegerStaffRelease(rt *Runtime) error {
	path := fmt.Sprintf("/operator/organizations/%d/staff-release", rt.Bootstrap.OrganizationID)
	_, err := rt.Client.PostWithAuth(rt.OperatorAuth, path, map[string]any{"representation": "agreement"})
	var refused *APIError
	if errors.As(err, &refused) && refused.Code == "workforce.traeger_staff_release_exists" {
		return nil
	}
	if err != nil {
		return fmt.Errorf("grant the release Personal beim Träger: %w", err)
	}
	return nil
}

type seedTraegerStaffSuggestion struct {
	Reason   string `json:"reason"`
	RecordID *int64 `json:"staff_record_id"`
	Members  []struct {
		SchoolID int64 `json:"school_id"`
		StaffID  int64 `json:"staff_id"`
	} `json:"members"`
}

func seedTraegerStaffSuggestions(client *Client, auth AuthRef) ([]seedTraegerStaffSuggestion, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/staff-records/")
	if err != nil {
		return nil, fmt.Errorf("read the Träger staff records: %w", err)
	}
	var envelope struct {
		Data struct {
			Suggestions []seedTraegerStaffSuggestion `json:"suggestions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode the Träger staff records: %w", err)
	}
	return envelope.Data.Suggestions, nil
}

// seedTraegerStaffLink confirms one proposal with the first value of every
// field the schools disagree on; it returns 0 when another run linked it.
func seedTraegerStaffLink(client *Client, auth AuthRef, suggestion seedTraegerStaffSuggestion) (int64, error) {
	members := make([]map[string]int64, 0, len(suggestion.Members))
	for _, member := range suggestion.Members {
		members = append(members, map[string]int64{"school_id": member.SchoolID, "staff_id": member.StaffID})
	}
	body := map[string]any{"members": members}
	if suggestion.RecordID != nil {
		body["staff_record_id"] = *suggestion.RecordID
	}
	raw, err := client.PostWithAuth(auth, "/traeger/staff-records/link-preview", body)
	if seedTraegerStaffAlreadyLinked(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("preview a Träger staff link: %w", err)
	}
	var preview struct {
		Data struct {
			Fields []struct {
				Field    string `json:"field"`
				Conflict bool   `json:"conflict"`
				Options  []struct {
					Source string `json:"source"`
				} `json:"options"`
			} `json:"fields"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &preview); err != nil {
		return 0, fmt.Errorf("decode a Träger staff link preview: %w", err)
	}
	choices := map[string]string{}
	for _, field := range preview.Data.Fields {
		if field.Conflict && len(field.Options) > 0 {
			choices[field.Field] = field.Options[0].Source
		}
	}
	body["choices"] = choices
	raw, err = client.PostWithAuth(auth, "/traeger/staff-records/", body)
	if seedTraegerStaffAlreadyLinked(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("link a Träger staff record: %w", err)
	}
	var record struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &record); err != nil || record.Data.ID == 0 {
		return 0, fmt.Errorf("decode a Träger staff record: %w", errors.Join(err, errors.New("no record id")))
	}
	return record.Data.ID, nil
}

// seedTraegerStaffAlreadyLinked recognizes a proposal another school's run
// confirmed meanwhile, or a number another record holds.
func seedTraegerStaffAlreadyLinked(err error) bool {
	var refused *APIError
	return errors.As(err, &refused) && (refused.Code == "workforce.traeger_staff_already_linked" || refused.Code == "workforce.personnel_number_taken")
}
