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

// seedExternalInvitation is what happens to the invitation the quick create
// sends: the Personal list shows all three outcomes (#4573).
type seedExternalInvitation int

const (
	externalInvitationAccepted seedExternalInvitation = iota // „Konto“
	externalInvitationOpen                                   // „Einladung offen“
	externalInvitationRevoked                                // „Kein Konto“, the state of the accountless externals of #3823
)

type seedExternalCaregiver struct {
	FirstName    string
	LastName     string
	Organization string
	Invitation   seedExternalInvitation
}

func (e seedExternalCaregiver) name() string { return e.FirstName + " " + e.LastName }

var seedExternalCaregivers = []seedExternalCaregiver{
	{FirstName: "Jonas", LastName: "Becker", Organization: "Musikschule Bergstadt", Invitation: externalInvitationAccepted},
	{FirstName: "Leonie", LastName: "Wagner", Organization: "TuS Sportverein", Invitation: externalInvitationAccepted},
	{FirstName: "Selin", LastName: "Aydın", Invitation: externalInvitationOpen},
	{FirstName: "Dennis", LastName: "Kaya", Organization: "Förderverein", Invitation: externalInvitationRevoked},
}

// seedExternalCaregiversStep records external AG leaders through the quick
// create with invitation (#4573): two accept and sign in with the school
// role „Extern“, one invitation stays open, and one is withdrawn, so the
// Personal list and the pickers show every account state on every dev
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
	email := seedExternalEmail(rt, external)
	body := map[string]string{"first_name": external.FirstName, "last_name": external.LastName, "email": email}
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
	switch external.Invitation {
	case externalInvitationAccepted:
		if resp.Data.InvitationToken == "" {
			return 0, fmt.Errorf("external caregiver response did not include the seed invitation token")
		}
		password := seedExternalPassword(rt)
		if _, err := rt.Client.PostPublic(fmt.Sprintf("/auth/invitations/%s/accept", resp.Data.InvitationToken), map[string]any{
			"password": password, "confirm_password": password,
		}); err != nil {
			return 0, fmt.Errorf("accept invitation: %w", err)
		}
	case externalInvitationRevoked:
		if err := revokeSeedInvitation(rt, email); err != nil {
			return 0, err
		}
	case externalInvitationOpen:
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

// revokeSeedInvitation withdraws the open school invitation sent to email,
// as the Leitung does under „Offene Einladungen“.
func revokeSeedInvitation(rt *Runtime, email string) error {
	raw, err := rt.Client.Get("/auth/invitations")
	if err != nil {
		return fmt.Errorf("list open invitations: %w", err)
	}
	var resp struct {
		Data []struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("decode open invitations: %w", err)
	}
	for _, invitation := range resp.Data {
		if strings.EqualFold(invitation.Email, email) {
			if _, err := rt.Client.Delete(fmt.Sprintf("/auth/invitations/%d", invitation.ID)); err != nil {
				return fmt.Errorf("revoke invitation: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("no open invitation for the seeded address")
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
