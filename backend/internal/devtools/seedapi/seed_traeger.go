package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// seedTraegerInvitee is the colleague the seeded Träger-Büro has invited
// (#4650); the address receives no mail outside the dev mail catcher.
const seedTraegerInvitee = "traeger-abrechnung@example.test"

// seedTraegerBueroStep makes the bootstrap school admin the first person of
// the Träger-Büro (ADR 0084, #4649) through the operator route, so every dev
// and demo database can sign in to the Träger-Portal with the admin's
// credentials and sees the Träger's schools. Signed in to the Träger-Portal,
// the admin then invites a colleague with the role Abrechnung (#4650), so the
// page "Personen" shows a person and an open invitation.
type seedTraegerBueroStep struct{}

func (seedTraegerBueroStep) Name() string { return "Seeding the Träger-Büro" }

func (seedTraegerBueroStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.OrganizationID <= 0 || rt.Bootstrap.AdminEmail == "" {
		return fmt.Errorf("the Träger-Büro seed prerequisites are not available")
	}
	previousAuth := rt.Client.auth
	rt.Client.BindAuth(rt.OperatorAuth)
	defer rt.Client.BindAuth(previousAuth)
	path := fmt.Sprintf("/operator/organizations/%d/traeger-members", rt.Bootstrap.OrganizationID)
	if _, err := rt.Client.Post(path, map[string]string{"email": rt.Bootstrap.AdminEmail, "role": "administration"}); err != nil {
		return fmt.Errorf("assign the Träger-Büro: %w", err)
	}
	fmt.Println("  Träger-Büro: school admin assigned with role Verwaltung")
	return seedTraegerInvitation(rt)
}

// seedTraegerInvitation signs the admin in to the Träger-Portal and invites
// the seeded colleague through the Träger route.
func seedTraegerInvitation(rt *Runtime) error {
	raw, err := rt.Client.PostWithAuth(AuthRef{Kind: AuthBearer}, "/traeger/auth/login", map[string]any{
		"email": rt.Bootstrap.AdminEmail, "password": rt.Bootstrap.AdminPassword, "organization_id": rt.Bootstrap.OrganizationID,
	})
	if err != nil {
		return fmt.Errorf("sign in to the Träger-Portal: %w", err)
	}
	var login struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &login); err != nil || login.AccessToken == "" {
		return fmt.Errorf("sign in to the Träger-Portal: no session in %q", raw)
	}
	traeger := AuthRef{Kind: AuthBearer, Token: login.AccessToken, Label: "traeger"}
	if _, err := rt.Client.PostWithAuth(traeger, "/traeger/accounts/invitations", map[string]any{
		"email": seedTraegerInvitee, "roles": []string{"billing"},
	}); err != nil {
		return fmt.Errorf("invite into the Träger-Büro: %w", err)
	}
	fmt.Println("  Träger-Büro: colleague invited with role Abrechnung")
	return nil
}
