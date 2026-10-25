package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// seedTraegerHolidayPeriodStep plans a published holiday period for the
// Träger-Büro (#4659) through the Träger-Portal, signed in as the bootstrap
// school admin the previous step assigned: two weeks at the Träger's schools,
// each with a morning and a full-day time model. Registration is open from a
// week ago for a month; the weeks follow six weeks from now.
type seedTraegerHolidayPeriodStep struct{}

func (seedTraegerHolidayPeriodStep) Name() string { return "Seeding a Träger holiday period" }

func (seedTraegerHolidayPeriodStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger holiday period seed prerequisites are not available")
	}
	auth, err := seedTraegerLogin(rt.Client, rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword)
	if err != nil {
		return err
	}
	schools, err := seedTraegerSchoolIDs(rt.Client, auth)
	if err != nil {
		return err
	}
	raw, err := rt.Client.PostWithAuth(auth, "/traeger/holiday-periods/", seedHolidayPeriod(calendar.TodayDate(), schools))
	if err != nil {
		return fmt.Errorf("create the holiday period: %w", err)
	}
	periodID, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the holiday period: %w", err)
	}
	if _, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/publish", periodID), map[string]any{}); err != nil {
		return fmt.Errorf("publish the holiday period: %w", err)
	}
	fmt.Println("  Träger-Büro: holiday period with two weeks published")
	return nil
}

func seedTraegerLogin(client *Client, email, password string) (AuthRef, error) {
	raw, err := client.PostPublic("/traeger/auth/login", map[string]string{"email": email, "password": password})
	if err != nil {
		return AuthRef{}, fmt.Errorf("sign in to the Träger-Portal: %w", err)
	}
	var answer struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil || answer.AccessToken == "" {
		return AuthRef{}, fmt.Errorf("the Träger-Portal sign-in returned no session")
	}
	return AuthRef{Kind: AuthBearer, Label: "traeger", Token: answer.AccessToken}, nil
}

func seedTraegerSchoolIDs(client *Client, auth AuthRef) ([]int64, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/schools/")
	if err != nil {
		return nil, fmt.Errorf("list the Träger schools: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data) == 0 {
		return nil, fmt.Errorf("the Träger has no school to host a holiday week")
	}
	ids := make([]int64, len(envelope.Data))
	for i, school := range envelope.Data {
		ids[i] = school.ID
	}
	return ids, nil
}

// seedHolidayPeriod is the request body: two Monday-to-Friday weeks, hosted
// in turn by the Träger's schools, with prices in whole cent.
func seedHolidayPeriod(today calendar.Date, schools []int64) map[string]any {
	firstMonday := today.AddDays(42).StartOfISOWeek()
	weeks := make([]map[string]any, 0, 2)
	for i := range 2 {
		monday := firstMonday.AddDays(7 * i)
		weeks = append(weeks, map[string]any{
			"starts_on": monday.String(), "ends_on": monday.AddDays(4).String(), "host_school_id": schools[i%len(schools)],
			"time_models": []map[string]any{
				{"name": "bis 13 Uhr", "starts_at": "08:00", "ends_at": "13:00", "price_cents": 4500},
				{"name": "bis 16 Uhr", "starts_at": "08:00", "ends_at": "16:00", "price_cents": 6500},
			},
		})
	}
	return map[string]any{
		"name": "Ferienbetreuung", "registration_opens_on": today.AddDays(-7).String(),
		"registration_closes_on": today.AddDays(30).String(), "binding": true,
		"notes": "Mittagessen gibt es vor Ort. Bitte geben Sie Ihrem Kind ein Frühstück und etwas zu trinken mit.",
		"weeks": weeks,
	}
}
