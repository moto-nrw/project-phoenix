package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// seedStaffQualificationsStep seeds the Nachweise (#4808): the standard kinds
// of the school, a kind of its own, the Führungszeugnis as mandatory and
// qualifications in every status of the list, so the Nachweise screen and
// the reminders have something to show on every dev machine.
type seedStaffQualificationsStep struct{}

func (seedStaffQualificationsStep) Name() string { return "Seeding staff qualifications" }

type seededQualificationType struct {
	ID             int64  `json:"id"`
	StandardKey    string `json:"standard_key"`
	Name           string `json:"name"`
	ValidityMonths *int   `json:"validity_months"`
}

func (seedStaffQualificationsStep) Run(_ context.Context, rt *Runtime) error {
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) < 6 {
		return fmt.Errorf("staff qualification prerequisites not available")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	kinds, err := seedQualificationKinds(rt)
	if err != nil {
		return err
	}
	firstAid, certificate, hygiene := kinds["first_aid_child"], kinds["extended_police_certificate"], kinds["hygiene"]
	today := todaySeedDate()
	row := func(kind int64, acquired, expires int) map[string]any {
		return map[string]any{"type_id": kind, "name": "", "acquired_on": today.AddDays(acquired).String(), "expires_on": today.AddDays(expires).String()}
	}
	lists := [][]map[string]any{
		{row(firstAid, -700, 30), row(certificate, -1000, 800)},
		{row(firstAid, -760, -30), row(certificate, -400, 1400), row(hygiene, -300, 65)},
		{row(firstAid, -200, 530), row(certificate, -1790, 10)},
		{row(firstAid, -100, 630), {"name": "Rettungsschwimmer Silber", "acquired_on": today.AddDays(-500).String()}},
		{row(certificate, -60, 1760)},
		{row(firstAid, -300, 430)}, // no certificate: shows as missing
	}
	for index, list := range lists {
		path := fmt.Sprintf("/api/staff/%d/stammdaten/qualifikationen", staffIDs[index])
		if _, err := rt.Client.Put(path, map[string]any{"qualifikationen": list, "note": "Nachweise erfasst"}); err != nil {
			return fmt.Errorf("seed staff qualifications: %w", err)
		}
	}
	fmt.Printf("  %d staff qualification lists created\n", len(lists))
	return nil
}

// seedQualificationKinds reads the standard kinds, marks the certificate of
// conduct mandatory and adds the school's own hygiene training.
func seedQualificationKinds(rt *Runtime) (map[string]int64, error) {
	body, err := rt.Client.Get("/api/staff/qualification-types")
	if err != nil {
		return nil, fmt.Errorf("list qualification types: %w", err)
	}
	var listed struct {
		Data []seededQualificationType `json:"data"`
	}
	if err := json.Unmarshal(body, &listed); err != nil {
		return nil, fmt.Errorf("decode qualification types: %w", err)
	}
	kinds := map[string]int64{}
	for _, kind := range listed.Data {
		kinds[kind.StandardKey] = kind.ID
		if kind.StandardKey == "extended_police_certificate" {
			if _, err := rt.Client.Put(fmt.Sprintf("/api/staff/qualification-types/%d", kind.ID), map[string]any{
				"name": kind.Name, "validity_months": kind.ValidityMonths, "reminder_lead_weeks": 8, "mandatory": true,
			}); err != nil {
				return nil, fmt.Errorf("mark certificate mandatory: %w", err)
			}
		}
	}
	created, err := rt.Client.Post("/api/staff/qualification-types", map[string]any{"name": "Hygieneschulung", "validity_months": 12, "reminder_lead_weeks": 4})
	if err != nil {
		return nil, fmt.Errorf("create hygiene training: %w", err)
	}
	var hygiene struct {
		Data seededQualificationType `json:"data"`
	}
	if err := json.Unmarshal(created, &hygiene); err != nil {
		return nil, fmt.Errorf("decode hygiene training: %w", err)
	}
	kinds["hygiene"] = hygiene.Data.ID
	if kinds["first_aid_child"] == 0 || kinds["extended_police_certificate"] == 0 || kinds["hygiene"] == 0 {
		return nil, fmt.Errorf("qualification types incomplete")
	}
	return kinds, nil
}
