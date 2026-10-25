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
	traeger, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
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
// week ago for a month; the weeks follow six weeks from now. A family then
// registers two siblings through the period's public link (#4661): one child
// of the Träger's first school and a holiday guest.
type seedTraegerHolidayPeriodStep struct{}

func (seedTraegerHolidayPeriodStep) Name() string { return "Seeding a Träger holiday period" }

func (seedTraegerHolidayPeriodStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger holiday period seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
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
	published, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/publish", periodID), map[string]any{})
	if err != nil {
		return fmt.Errorf("publish the holiday period: %w", err)
	}
	fmt.Println("  Träger-Büro: holiday period with two weeks published")
	token, registration, err := seedHolidayRegistration(published, schools[0])
	if err != nil {
		return err
	}
	if _, err := rt.Client.PostPublic("/traeger/holiday-registrations/"+token, registration); err != nil {
		return fmt.Errorf("register for the holiday period: %w", err)
	}
	fmt.Println("  Ferienanmeldung: two siblings registered, one of them a holiday guest")
	return nil
}

// seedTurnstileTestToken is the token Cloudflare's Turnstile test keys of
// the dev and CI environments accept (dev.env.example).
const seedTurnstileTestToken = "XXXX.DUMMY.TOKEN.XXXX"

// seedHolidayRegistration builds the public registration of two siblings for
// the published period: a child of homeSchool for the first week and a
// holiday guest for both weeks. All names are invented.
func seedHolidayRegistration(published []byte, homeSchool int64) (string, map[string]any, error) {
	var envelope struct {
		Data struct {
			PublicToken string `json:"public_token"`
			Weeks       []struct {
				ID         int64 `json:"id"`
				TimeModels []struct {
					ID int64 `json:"id"`
				} `json:"time_models"`
			} `json:"weeks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(published, &envelope); err != nil {
		return "", nil, fmt.Errorf("read the published holiday period: %w", err)
	}
	period := envelope.Data
	if period.PublicToken == "" || len(period.Weeks) < 2 || len(period.Weeks[0].TimeModels) < 2 || len(period.Weeks[1].TimeModels) < 1 {
		return "", nil, fmt.Errorf("the published holiday period has no link or no bookable weeks")
	}
	booking := func(week, model int) map[string]any {
		return map[string]any{"week_id": period.Weeks[week].ID, "time_model_id": period.Weeks[week].TimeModels[model].ID}
	}
	return period.PublicToken, map[string]any{
		"guardian": map[string]any{"first_name": "Sabine", "last_name": "Ferienkind", "email": "sabine.ferienkind@example.org", "phone": "0151 0000001"},
		"children": []map[string]any{
			{"first_name": "Lotta", "last_name": "Ferienkind", "birth_date": "2018-04-12", "home_school_id": homeSchool,
				"allergies": "Erdnüsse", "bookings": []map[string]any{booking(0, 1)}},
			{"first_name": "Jonas", "last_name": "Ferienkind", "birth_date": "2016-09-03", "home_school_id": 0,
				"guest_school_name": "Grundschule am See", "bookings": []map[string]any{booking(0, 0), booking(1, 0)}},
		},
		"emergency_contacts": []map[string]any{{"name": "Helga Ferienkind", "phone": "05402 000002", "relationship": "Großmutter"}},
		"pickup_persons":     []map[string]any{{"name": "Peter Ferienkind", "relationship": "Onkel"}},
		"privacy_consent":    true, "health_consent": true, "binding_confirmed": true,
		"captcha_token": seedTurnstileTestToken,
	}, nil
}

// seedTraegerSession is the Träger-Portal session of the bootstrap admin,
// signed in once per runtime and shared by the Träger steps. Every sign-in
// mails a second-factor code (#4651), and Identity & Access mails at most
// three per account in fifteen minutes; one sign-in per step would run into
// that limit.
func seedTraegerSession(rt *Runtime) (AuthRef, error) {
	session := seedTraegerSessionRef{email: rt.Bootstrap.AdminEmail, organizationID: rt.Bootstrap.OrganizationID}
	if cached, ok := rt.Values[seedTraegerSessionKey].(seedTraegerSessionRef); ok && cached.email == session.email && cached.organizationID == session.organizationID {
		return cached.auth, nil
	}
	auth, err := rt.Client.LoginTraeger(rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, rt.Bootstrap.OrganizationID)
	if err != nil {
		return AuthRef{}, fmt.Errorf("sign in to the Träger-Portal: %w", err)
	}
	session.auth = auth
	if rt.Values == nil {
		rt.Values = make(map[string]any)
	}
	rt.Values[seedTraegerSessionKey] = session
	return auth, nil
}

const seedTraegerSessionKey = "traeger.session"

type seedTraegerSessionRef struct {
	email          string
	organizationID int64
	auth           AuthRef
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

// traegerSeedSettingKey is the setting the demo Träger sets for all its
// schools (ADR 0086, #4652): parent news on, as the registry default already
// has it. The value changes no behaviour, but the Träger settings page shows
// it "vom Träger" and the school settings show where it comes from.
const traegerSeedSettingKey = "operations.parent_news_enabled"

// seedTraegerSettingsStep sets one Träger value through the Träger route in
// the Träger-Portal session of the Träger-Büro admin, the way the
// Träger-Büro does it.
type seedTraegerSettingsStep struct{}

func (seedTraegerSettingsStep) Name() string { return "Seeding the Träger settings" }

func (seedTraegerSettingsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger settings seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/settings/values/"+traegerSeedSettingKey, map[string]any{"value": true}); err != nil {
		return fmt.Errorf("set the Träger value of %s: %w", traegerSeedSettingKey, err)
	}
	fmt.Println("  Träger settings: parent news set for all schools of the Träger")
	return nil
}
