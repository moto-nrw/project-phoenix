package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// defaultSeedExternalPassword is the demo password of the external
// caregivers that accept their invitation, unless --staff-password is set.
const defaultSeedExternalPassword = "Extern1234%"

// seedExternalCaregiverIDsKey holds the staff IDs of the seeded external
// caregivers in Runtime.Values, by "first last" name, for later steps.
const seedExternalCaregiverIDsKey = "seed.external_caregiver_ids"

// seedExternalOutcome is the access a seeded external gets and what happens
// to the invitation the quick create sends: the Personal list shows every
// account state and access level (#4573, ADR 0088).
type seedExternalOutcome int

const (
	externalSupervisorAccepted seedExternalOutcome = iota // AG-Leitung, „Konto“
	externalViewerInvited                                 // Gast, „Einladung offen“
	externalPlaceholder                                   // kein Zugang, „Kein Konto“
)

// access is the access level the quick create sends.
func (o seedExternalOutcome) access() string {
	switch o {
	case externalSupervisorAccepted:
		return "supervise"
	case externalViewerInvited:
		return "view"
	default:
		return "none"
	}
}

type seedExternalCaregiver struct {
	FirstName    string
	LastName     string
	Organization string
	Outcome      seedExternalOutcome
}

func (e seedExternalCaregiver) name() string { return e.FirstName + " " + e.LastName }

var seedExternalCaregivers = []seedExternalCaregiver{
	{FirstName: "Jonas", LastName: "Becker", Organization: "Musikschule Bergstadt", Outcome: externalSupervisorAccepted},
	{FirstName: "Leonie", LastName: "Wagner", Organization: "TuS Sportverein", Outcome: externalSupervisorAccepted},
	{FirstName: "Selin", LastName: "Aydın", Outcome: externalViewerInvited},
	{FirstName: "Dennis", LastName: "Kaya", Organization: "Förderverein", Outcome: externalPlaceholder},
}

// seedExternalCaregiversStep records external caregivers through the quick
// create (#4573, ADR 0088): two get the access supervise and sign in with
// the system role „AG-Leitung“, one is invited with the access view („Gast“)
// and keeps the invitation open, and one is a placeholder without access, so
// the Personal list and the pickers show every account state on every dev
// machine.
type seedExternalCaregiversStep struct{}

func (seedExternalCaregiversStep) Name() string { return "Seeding external caregivers" }

func (seedExternalCaregiversStep) Run(_ context.Context, rt *Runtime) error {
	existing, err := seededExternalCaregivers(rt)
	if err != nil {
		return err
	}
	ids := make(map[string]int64, len(seedExternalCaregivers))
	created := 0
	for _, external := range seedExternalCaregivers {
		if id, found := existing[external.name()]; found {
			ids[external.name()] = id
			continue
		}
		id, err := createSeedExternalCaregiver(rt, external)
		if err != nil {
			return fmt.Errorf("create external caregiver %q: %w", external.LastName, err)
		}
		ids[external.name()] = id
		created++
	}
	if rt.Values == nil {
		rt.Values = make(map[string]any)
	}
	rt.Values[seedExternalCaregiverIDsKey] = ids
	fmt.Printf("  %d external caregivers created\n", created)
	return nil
}

// seededExternalCaregiverIDs returns the staff IDs the external caregiver
// step recorded, by "first last" name.
func seededExternalCaregiverIDs(rt *Runtime) map[string]int64 {
	ids, _ := rt.Values[seedExternalCaregiverIDsKey].(map[string]int64)
	return ids
}

func createSeedExternalCaregiver(rt *Runtime, external seedExternalCaregiver) (int64, error) {
	body := map[string]string{"first_name": external.FirstName, "last_name": external.LastName, "access": external.Outcome.access()}
	if external.Outcome != externalPlaceholder {
		body["email"] = seedExternalEmail(rt, external)
	}
	if external.Organization != "" {
		body["organization"] = external.Organization
	}
	raw, err := rt.Client.PostWithHeaders("/api/staff/externals", body, map[string]string{seedTokenHeader: "true"})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Data struct {
			ID              any    `json:"id"`
			InvitationToken string `json:"invitation_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return 0, fmt.Errorf("decode external caregiver: %w", err)
	}
	id, err := parseSeedID(resp.Data.ID)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("external caregiver response has no id")
	}
	if external.Outcome != externalSupervisorAccepted {
		return id, nil
	}
	if resp.Data.InvitationToken == "" {
		return 0, fmt.Errorf("external caregiver response did not include the seed invitation token")
	}
	password := seedExternalPassword(rt)
	if _, err := rt.Client.PostPublic(fmt.Sprintf("/auth/invitations/%s/accept", resp.Data.InvitationToken), map[string]any{
		"password": password, "confirm_password": password,
	}); err != nil {
		return 0, fmt.Errorf("accept invitation: %w", err)
	}
	return id, nil
}

// seedExternalEmail gives the external an ASCII address in the demo domain;
// a scoped seed moves it into the school's own domain like every account.
func seedExternalEmail(rt *Runtime, external seedExternalCaregiver) string {
	local := emailLocalPart(external.FirstName, external.LastName)
	if rt.FixedSeeder != nil && rt.FixedSeeder.accountScope != "" {
		return scopedEmail(local+"@", rt.FixedSeeder.accountScope)
	}
	return local + "@example.test"
}

func seedExternalPassword(rt *Runtime) string {
	if rt.FixedSeeder != nil && strings.TrimSpace(rt.FixedSeeder.staffPassword) != "" {
		return rt.FixedSeeder.staffPassword
	}
	return defaultSeedExternalPassword
}

// seededExternalCaregivers reads the directory once before creating the demo
// entries and returns the staff IDs by name. Internal colleagues with the
// same name deliberately do not match: only the explicit external marker
// identifies a prior seed run.
func seededExternalCaregivers(rt *Runtime) (map[string]int64, error) {
	raw, err := rt.Client.Get("/api/staff/")
	if err != nil {
		return nil, fmt.Errorf("list existing external caregivers: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID         any  `json:"id"`
			IsExternal bool `json:"is_external"`
			Person     *struct {
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
			} `json:"person"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode existing external caregivers: %w", err)
	}
	existing := make(map[string]int64, len(envelope.Data))
	for _, staff := range envelope.Data {
		if !staff.IsExternal || staff.Person == nil {
			continue
		}
		id, err := parseSeedID(staff.ID)
		if err != nil {
			return nil, fmt.Errorf("decode existing external caregiver id: %w", err)
		}
		existing[staff.Person.FirstName+" "+staff.Person.LastName] = id
	}
	return existing, nil
}
