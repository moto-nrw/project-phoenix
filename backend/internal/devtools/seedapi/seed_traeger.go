package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerInvitees are the colleagues the seeded Träger-Büro has invited
// (#4650), one per role it hands out besides the Verwaltung. The Träger-Portal
// shows people by address only; the reserved TLD .example receives no mail.
var seedTraegerInvitees = []struct {
	email string
	role  string
}{
	{"jana.krause@ganztag-nord.example", "billing"},
	{"tobias.engel@ganztag-nord.example", "personnel"},
}

// seedTraegerBueroStep makes the bootstrap school admin the first person of
// the Träger-Büro (ADR 0084, #4649) through the operator route, so every dev
// and demo database can sign in to the Träger-Portal with the admin's
// credentials and sees the Träger's schools. Signed in to the Träger-Portal,
// the admin then invites a colleague for the Abrechnung and one for the
// Personal (#4650), so the page "Personen" shows a person and open
// invitations.
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
// the seeded colleagues through the Träger route.
func seedTraegerInvitation(rt *Runtime) error {
	traeger, err := rt.Client.LoginTraeger(rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, rt.Bootstrap.OrganizationID)
	if err != nil {
		return fmt.Errorf("sign in to the Träger-Portal: %w", err)
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

// seedTraegerHolidayPeriodStep plans a published holiday period for the
// Träger-Büro (#4659) through the Träger-Portal, signed in as the bootstrap
// school admin the previous step assigned: the two Herbstferien weeks the
// school plans around (seedHerbstferien), hosted in turn by the Träger's
// schools, each with a morning and a full-day time model. Published today, it
// opens registration today and closes it a week before the Ferien.
type seedTraegerHolidayPeriodStep struct{}

func (seedTraegerHolidayPeriodStep) Name() string { return "Seeding a Träger holiday period" }

func (seedTraegerHolidayPeriodStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger holiday period seed prerequisites are not available")
	}
	auth, err := rt.Client.LoginTraeger(rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, 0)
	if err != nil {
		return fmt.Errorf("sign in to the Träger-Portal: %w", err)
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

// seedHolidayPeriod is the request body: the two Herbstferien weeks, Monday
// to Friday, hosted in turn by the Träger's schools, with prices in whole
// cent.
func seedHolidayPeriod(today calendar.Date, schools []int64) map[string]any {
	ferienStart, _ := seedHerbstferien(seedDate{Time: today.UTCMidnight()})
	firstMonday := calendar.NewDate(ferienStart.Year(), ferienStart.Month(), ferienStart.Day())
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
		"name":                   fmt.Sprintf("Ferienbetreuung Herbstferien %d", firstMonday.Year()),
		"registration_opens_on":  today.String(),
		"registration_closes_on": firstMonday.AddDays(-7).String(), "binding": true,
		"notes": "Mittagessen gibt es vor Ort. Bitte geben Sie Ihrem Kind ein Frühstück und etwas zu trinken mit.",
		"weeks": weeks,
	}
}
