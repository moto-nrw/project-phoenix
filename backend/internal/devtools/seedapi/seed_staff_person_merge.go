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
	staff, personID, err := staffPersonMergeSeedTarget(rt.FixedSeeder)
	if err != nil {
		return err
	}
	rt.Client.BindAuth(rt.TenantAuth)
	alreadySeeded, err := hasSeededStaffPersonMerge(rt, personID)
	if err != nil {
		return err
	}
	if alreadySeeded {
		return nil
	}
	// The quick create invites the person (#4573). The address is the
	// duplicate's own, and the invitation is withdrawn before the employment
	// ends: a retired duplicate must not keep a way into the school.
	email := staffPersonMergeDuplicateEmail(staff, rt.FixedSeeder.accountScope)
	raw, err := rt.Client.Post("/api/staff/externals", map[string]any{
		"first_name": staff.FirstName, "last_name": staff.LastName, "email": email,
		"organization": "Demo: frühere Beschäftigung für die Dublettenprüfung",
	})
	if err != nil {
		return fmt.Errorf("create staff duplicate example: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("decode duplicate employment: %w", err)
	}
	if err := revokeSeedInvitation(rt, email); err != nil {
		return fmt.Errorf("withdraw duplicate invitation: %w", err)
	}
	if _, err := rt.Client.Put(fmt.Sprintf("/api/staff/%d/stammdaten/kontakt", id), map[string]any{"address_city": "Osnabrück", "note": "Demo: Historie bleibt bei der Personen-Zusammenführung erhalten"}); err != nil {
		return fmt.Errorf("record duplicate history: %w", err)
	}
	if _, err := rt.Client.Delete(fmt.Sprintf("/api/staff/%d", id)); err != nil {
		return fmt.Errorf("retire duplicate employment: %w", err)
	}
	return nil
}

// staffPersonMergeDuplicateEmail is the duplicate's own address: the demo
// colleague's account keeps theirs.
func staffPersonMergeDuplicateEmail(staff DemoStaffMember, scope string) string {
	local := emailLocalPart(staff.FirstName, staff.LastName) + ".frueher"
	if scope != "" {
		return scopedEmail(local+"@", scope)
	}
	return local + "@example.test"
}

func staffPersonMergeSeedTarget(seeder *FixedSeeder) (DemoStaffMember, int64, error) {
	if seeder == nil {
		return DemoStaffMember{}, 0, fmt.Errorf("staff duplicate example requires active staff")
	}
	for _, staff := range DemoStaff {
		key := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		if seeder.staffIDs[key] == 0 {
			continue
		}
		personID := seeder.personIDs[key]
		if personID == 0 {
			return DemoStaffMember{}, 0, fmt.Errorf("staff duplicate example has no person ID for %s", key)
		}
		return staff, personID, nil
	}
	return DemoStaffMember{}, 0, fmt.Errorf("staff duplicate example requires active staff")
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
