package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// seedStaffPersonMergeStep leaves an explicit legacy repair example for the
// operator (#3397). The account-bearing person is older than the retired
// duplicate; neither the repair nor its review may infer direction from age.
// All data is created through school APIs; no identity is merged by the seeder.
type seedStaffPersonMergeStep struct{}

func (seedStaffPersonMergeStep) Name() string { return "Seeding a reviewed staff duplicate example" }

func (seedStaffPersonMergeStep) Run(_ context.Context, rt *Runtime) error {
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if len(staffIDs) == 0 {
		return fmt.Errorf("staff duplicate example requires active staff")
	}
	rt.Client.BindAuth(rt.TenantAuth)
	raw, err := rt.Client.Get(fmt.Sprintf("/api/staff/%d", staffIDs[0]))
	if err != nil {
		return fmt.Errorf("read staff duplicate example: %w", err)
	}
	var existing struct {
		Data struct {
			Person struct {
				ID        json.Number `json:"id"`
				FirstName string      `json:"first_name"`
				LastName  string      `json:"last_name"`
			} `json:"person"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &existing); err != nil {
		return fmt.Errorf("decode staff duplicate example: %w", err)
	}
	if existing.Data.Person.FirstName == "" || existing.Data.Person.LastName == "" {
		return fmt.Errorf("staff duplicate example has no person name")
	}
	// The staff detail embeds a numeric person ID; operator previews use
	// decimal strings. Decode either representation without a float64 round trip.
	personID, err := existing.Data.Person.ID.Int64()
	if err != nil {
		return fmt.Errorf("decode staff duplicate person ID: %w", err)
	}
	if personID > 0 {
		alreadySeeded, err := hasSeededStaffPersonMerge(rt, personID)
		if err != nil {
			return err
		}
		if alreadySeeded {
			return nil
		}
	}
	raw, err = rt.Client.Post("/api/staff/externals", map[string]any{
		"first_name": existing.Data.Person.FirstName, "last_name": existing.Data.Person.LastName, "organization": "Demo: frühere Beschäftigung für die Dublettenprüfung",
	})
	if err != nil {
		return fmt.Errorf("create staff duplicate example: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("decode duplicate employment: %w", err)
	}
	if _, err := rt.Client.Put(fmt.Sprintf("/api/staff/%d/stammdaten/kontakt", id), map[string]any{"address_city": "Osnabrück", "note": "Demo: Historie bleibt bei der Personen-Zusammenführung erhalten"}); err != nil {
		return fmt.Errorf("record duplicate history: %w", err)
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/staff/%d", id)); err != nil {
		return fmt.Errorf("retire duplicate employment: %w", err)
	}
	return nil
}

// hasSeededStaffPersonMerge recognizes the example by its account-bearing
// demo person. A completed merge no longer appears in the preview, so a later
// seed can provide a fresh example without accumulating unreviewed duplicates.
func hasSeededStaffPersonMerge(rt *Runtime, personID int64) (bool, error) {
	if rt.Bootstrap == nil || rt.Bootstrap.SchoolID <= 0 {
		return false, fmt.Errorf("staff duplicate example requires a seeded school")
	}
	raw, err := rt.Client.GetWithAuth(rt.OperatorAuth, fmt.Sprintf("/operator/staff-person-merges/%d", rt.Bootstrap.SchoolID))
	if err != nil {
		return false, fmt.Errorf("list staff duplicate examples: %w", err)
	}
	var existing struct {
		Data []struct {
			Keeper struct {
				Person struct {
					ID int64 `json:"id,string"`
				} `json:"person"`
			} `json:"keeper"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &existing); err != nil {
		return false, fmt.Errorf("decode staff duplicate examples: %w", err)
	}
	for _, candidate := range existing.Data {
		if candidate.Keeper.Person.ID == personID {
			return true, nil
		}
	}
	return false, nil
}
