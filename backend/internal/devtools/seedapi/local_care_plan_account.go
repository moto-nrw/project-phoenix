package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
)

const localCarePlanAccountKey = "nachbearbeitung-ohne-planung"

// Separate from demoRoleDefinitions: this role is never offered by the public
// demo switch. config:manage permits the offering correction, not planning.
func localCarePlanRoleDefinition() demoRoleDefinition {
	return demoRoleDefinition{
		Name:        localCarePlanAccountKey,
		Description: "Lokales Testkonto für Angebotskorrekturen ohne Planungsrecht.",
		BaseRole:    "user",
		Permissions: []string{
			"activities:list", "activities:read",
			"config:manage", "config:read",
			"groups:list", "groups:read",
			"rooms:list", "rooms:read",
			"schedules:list", "schedules:read",
			"users:list", "users:read",
		},
	}
}

type seedLocalCarePlanAccountStep struct{ seeder *Seeder }

func (seedLocalCarePlanAccountStep) Name() string {
	return "Seeding local care-plan review account without planning rights"
}

func (s seedLocalCarePlanAccountStep) enabled() bool {
	if s.seeder.options.StandingDemo || s.seeder.options.VisitorFirstName != "" {
		return false
	}
	u, err := url.Parse(s.seeder.client.baseURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

func (s seedLocalCarePlanAccountStep) Run(ctx context.Context, rt *Runtime) error {
	if !s.enabled() {
		return nil
	}
	fs := rt.FixedSeeder
	if fs == nil || len(fs.staffCredentials) == 0 {
		return fmt.Errorf("local care-plan account requires seeded staff credentials")
	}
	definition := localCarePlanRoleDefinition()
	if err := fs.seedDemoRoles(ctx, definition); err != nil {
		return err
	}
	// Reuse the existing seed credential, including --staff-password. No new
	// password is embedded; the private seed state is the source for login.
	password := fs.staffCredentials[0].Password
	email := scopedEmail(localCarePlanAccountKey+"@example.test", fs.accountScope)
	username := localCarePlanAccountKey
	if fs.accountScope != "" {
		// Keep the scoped prefix shorter than demo20- of the existing staff.
		username = "np-" + fs.accountScope
	}
	raw, err := rt.Client.Post("/auth/register", map[string]any{
		"email": email, "username": username,
		"password": password, "confirm_password": password,
		"first_name": "Testkonto", "last_name": "Nachbearbeitung",
		"role_id": fs.roleIDs[definition.Name],
	})
	if err != nil {
		return fmt.Errorf("register local care-plan account: %w", err)
	}
	var account struct {
		Data struct {
			ID             int64 `json:"id"`
			SchoolIdentity *struct {
				StaffID int64 `json:"staff_id,string"`
			} `json:"school_identity"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &account); err != nil {
		return fmt.Errorf("decode local care-plan account: %w", err)
	}
	if account.Data.ID <= 0 || account.Data.SchoolIdentity == nil || account.Data.SchoolIdentity.StaffID <= 0 {
		return fmt.Errorf("local care-plan account was created without a staff identity")
	}
	s.seeder.localCarePlanAccount = &AccountCredentials{
		Key: localCarePlanAccountKey, AccountID: account.Data.ID,
		Email: email, Password: password, Name: "Testkonto Nachbearbeitung",
		StaffID: account.Data.SchoolIdentity.StaffID,
	}
	return nil
}
