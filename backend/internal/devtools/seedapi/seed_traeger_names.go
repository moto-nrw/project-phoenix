package api

import "fmt"

// seedTraegerAdminName is the name the bootstrap school admin carries in the
// Träger-Büro (#4693), so the export protocol and the history of an invoice
// name a person instead of showing an address. An invented name.
const seedTraegerAdminName = "Sabine Hartmann"

// seedTraegerOwnName names the signed-in admin through the Träger-Portal's
// own profile route, the way every person of the Träger-Büro names
// themselves.
func seedTraegerOwnName(rt *Runtime, traeger AuthRef) error {
	if _, err := rt.Client.PutWithAuth(traeger, "/traeger/profile/name", map[string]any{"name": seedTraegerAdminName}); err != nil {
		return fmt.Errorf("name the Träger-Büro admin: %w", err)
	}
	fmt.Printf("  Träger-Büro: school admin named %s\n", seedTraegerAdminName)
	return nil
}

// seedTraegerInvitation signs the admin in to the Träger-Portal, names the
// admin and invites the seeded colleagues through the Träger routes.
func seedTraegerInvitation(rt *Runtime) error {
	traeger, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	if err := seedTraegerOwnName(rt, traeger); err != nil {
		return err
	}
	for _, invitee := range seedTraegerInvitees {
		if _, err := rt.Client.PostWithAuth(traeger, "/traeger/accounts/invitations", map[string]any{
			"email": invitee.email, "roles": []string{invitee.role},
		}); err != nil {
			return fmt.Errorf("invite into the Träger-Büro: %w", err)
		}
	}
	fmt.Printf("  Träger-Büro: %d colleagues invited\n", len(seedTraegerInvitees))
	return nil
}
