package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

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

// seedTraegerOperatorInvitee is the person the moto team invites into the
// Träger-Büro from the operator portal (#5144), with two roles in one step.
var seedTraegerOperatorInvitee = struct {
	email string
	roles []string
}{"petra.hoffmann@ganztag-nord.example", []string{"administration", "billing"}}

// seedTraegerBueroStep makes the bootstrap school admin the first person of
// the Träger-Büro (ADR 0084, #4649) through the operator route, so every dev
// and demo database can sign in to the Träger-Portal with the admin's
// credentials and sees the Träger's schools. The admin also holds the role
// Abrechnung, so the dev database shows the contribution rules (#4664); both
// roles go in one assignment (#5144). The operator also invites a person
// without a moto account (#5144). Signed in to the Träger-Portal,
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
	if _, err := rt.Client.Post(path, map[string]any{"email": rt.Bootstrap.AdminEmail, "roles": []string{"administration", "billing"}}); err != nil {
		return fmt.Errorf("assign the Träger-Büro: %w", err)
	}
	fmt.Println("  Träger-Büro: school admin assigned with roles Verwaltung and Abrechnung")
	if _, err := rt.Client.Post(path+"/invitations", map[string]any{"email": seedTraegerOperatorInvitee.email, "roles": seedTraegerOperatorInvitee.roles}); err != nil {
		return fmt.Errorf("invite into the Träger-Büro from the operator portal: %w", err)
	}
	fmt.Println("  Träger-Büro: operator invited a person with roles Verwaltung and Abrechnung")
	return seedTraegerInvitation(rt)
}

// seedTraegerInvitation signs the admin in to the Träger-Portal and invites
// the seeded colleagues through the Träger route.
func seedTraegerInvitation(rt *Runtime) error {
	traeger, err := seedTraegerSession(rt)
	if err != nil {
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

// seedTraegerHolidayPeriodStep plans a published holiday period for the
// Träger-Büro (#4659) through the Träger-Portal, signed in as the bootstrap
// school admin the previous step assigned: the two Herbstferien weeks the
// school plans around (seedHerbstferien), both hosted by the profile's own
// school, each with a morning and a full-day time model. The Träger's other
// school is the manual test school, which no family would be sent to. Published today, it
// opens registration today and closes it a week before the Ferien.
// A family then registers two siblings through the period's public link
// (#4661): one child of the Träger's first school and a holiday guest; the
// Träger-Büro reviews the registrations (#4663). The Abrechnung keeps a
// sibling rule and an exemption, the form shows the contribution, and the
// confirmation freezes it (#4664). The Abrechnung bills the period in a run
// and releases its invoice (#4668). Last, the first seeded parent reads the
// link's prefill of known children (#4662).
type seedTraegerHolidayPeriodStep struct{}

func (seedTraegerHolidayPeriodStep) Name() string { return "Seeding a Träger holiday period" }

func (seedTraegerHolidayPeriodStep) Run(ctx context.Context, rt *Runtime) error {
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
	if !slices.Contains(schools, rt.Bootstrap.SchoolID) {
		return fmt.Errorf("the Träger does not list school %d to host the holiday weeks", rt.Bootstrap.SchoolID)
	}
	raw, err := rt.Client.PostWithAuth(auth, "/traeger/holiday-periods/", seedHolidayPeriod(calendar.TodayDate(), rt.Bootstrap.SchoolID))
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
	if err := seedContributionRules(rt.Client, auth, calendar.TodayDate()); err != nil {
		return err
	}
	token, err := seedHolidayRegistrations(rt, auth, periodID, published, schools[0])
	if errors.Is(err, errSeedHolidayRegistrationRefused) {
		fmt.Printf("  WARNING: %v; the holiday period stays without registrations\n", err)
		rt.Values[seedHolidayRegistrationsSkippedKey] = true
		return nil
	}
	if err != nil {
		return err
	}
	runID, err := seedHolidayBillingRun(rt.Client, auth, periodID)
	if err != nil {
		return err
	}
	rt.Values[seedBillingRunKey] = runID
	if err := seedHolidayRegistrationPrefill(ctx, rt, token); err != nil {
		return err
	}
	weekID, err := seedHolidayParticipantList(rt)
	if err != nil || weekID == 0 {
		return err
	}
	return seedHolidayGuests(rt, weekID)
}

// seedHolidayParticipantList reads the participant list of the first hosted
// week as the hosting school's admin (#4665): the confirmed siblings, counted
// per time model and home school, and the read in the school's access log. A
// run without a school session, like the step's own tests, reads nothing.
// It returns the week it read.
func seedHolidayParticipantList(rt *Runtime) (int64, error) {
	if rt.TenantAuth.Token == "" {
		return 0, nil
	}
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, "/api/holiday-weeks/")
	if err != nil {
		return 0, fmt.Errorf("list the hosted holiday weeks: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID           int64 `json:"id"`
			Participants int   `json:"participants"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("decode the hosted holiday weeks: %w", err)
	}
	if len(envelope.Data) == 0 || envelope.Data[0].Participants == 0 {
		return 0, fmt.Errorf("the hosting school sees no confirmed child in its first holiday week")
	}
	if _, err := rt.Client.GetWithAuth(rt.TenantAuth, fmt.Sprintf("/api/holiday-weeks/%d/participants", envelope.Data[0].ID)); err != nil {
		return 0, fmt.Errorf("read the holiday participant list: %w", err)
	}
	fmt.Printf("  Ausrichtende OGS: participant list with %d confirmed child(ren) read\n", envelope.Data[0].Participants)
	return envelope.Data[0].ID, nil
}

// seedHolidayGuests creates the group "Feriengruppe" at the hosting school
// and picks it for the first hosted week (#4666): the confirmed holiday
// guest of the siblings' registration joins it with its booked care, its
// emergency contact and its pickup person. The sibling of the hosting school
// stays in its own group. The step fails when no guest joins.
func seedHolidayGuests(rt *Runtime, weekID int64) error {
	raw, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/groups", map[string]any{"name": "Feriengruppe", "teacher_ids": []int64{}})
	if err != nil {
		return fmt.Errorf("create the holiday group: %w", err)
	}
	groupID, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the holiday group: %w", err)
	}
	raw, err = rt.Client.PutWithAuth(rt.TenantAuth, fmt.Sprintf("/api/holiday-weeks/%d/guest-group", weekID),
		map[string]any{"group_id": fmt.Sprint(groupID)})
	if err != nil {
		return fmt.Errorf("take the holiday guests into the group: %w", err)
	}
	var envelope struct {
		Data struct {
			Guests int `json:"guests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode the holiday guest transfer: %w", err)
	}
	if envelope.Data.Guests == 0 {
		return fmt.Errorf("no holiday guest joined the holiday group")
	}
	fmt.Printf("  Ausrichtende OGS: %d holiday guest(s) in the Feriengruppe\n", envelope.Data.Guests)
	return nil
}

// seedHolidayRegistrations registers two siblings through the public link
// (#4661), then a second parent registers the first sibling once more for
// the second week, a possible duplicate. The Träger-Büro reviews the list
// and confirms the siblings' registration (#4663), which freezes its
// contribution (#4664); the duplicate stays received. The form shows the
// siblings' contribution before it is sent.
func seedHolidayRegistrations(rt *Runtime, auth AuthRef, periodID int64, published []byte, homeSchool int64) (string, error) {
	token, registration, err := seedHolidayRegistration(published, homeSchool)
	if err != nil {
		return "", err
	}
	seedHolidayRegistrationByParent(rt, registration)
	if _, err := rt.Client.PostPublic("/traeger/holiday-quotes/"+token, seedHolidayQuote(registration)); err != nil {
		return "", fmt.Errorf("show the holiday contribution: %w", err)
	}
	raw, err := rt.Client.PostPublic("/traeger/holiday-registrations/"+token, registration)
	var refused *APIError
	if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
		return "", fmt.Errorf("%w: %s", errSeedHolidayRegistrationRefused, refused.Code)
	}
	if err != nil {
		return "", fmt.Errorf("register for the holiday period: %w", err)
	}
	siblings, err := decodeSeedEntityID(raw)
	if err != nil {
		return "", fmt.Errorf("read the holiday registration: %w", err)
	}
	fmt.Println("  Ferienanmeldung: two siblings registered, one of them a holiday guest")
	if _, err := rt.Client.PostPublic("/traeger/holiday-registrations/"+token, seedHolidayDuplicate(registration)); err != nil {
		return "", fmt.Errorf("register the possible duplicate: %w", err)
	}
	registrations := fmt.Sprintf("/traeger/holiday-periods/%d/registrations", periodID)
	if _, err := rt.Client.GetWithAuth(auth, registrations); err != nil {
		return "", fmt.Errorf("list the holiday registrations: %w", err)
	}
	confirm := fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d/confirm", periodID, siblings)
	if _, err := rt.Client.PostWithAuth(auth, confirm, map[string]any{"benefits_exemption": false}); err != nil {
		return "", fmt.Errorf("confirm the holiday registration: %w", err)
	}
	fmt.Println("  Träger-Büro: siblings confirmed with their contribution, a possible duplicate waits for review")
	return token, nil
}

// seedHolidayRegistrationPrefill reads the prefill of the holiday link as the
// first seeded parent (#4662): the parents portal offers that family's
// children of the Träger's schools, and the school's access log records the
// read. A run without seeded parents, like the step's own tests, has no
// family to read.
func seedHolidayRegistrationPrefill(ctx context.Context, rt *Runtime, token string) error {
	if len(rt.Parents) == 0 {
		return nil
	}
	parent := rt.Parents[0]
	auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
	if err != nil {
		return fmt.Errorf("sign in the parent for the holiday prefill: %w", err)
	}
	raw, err := rt.Client.GetWithAuth(auth, "/parent/holiday-registrations/"+token+"/family")
	if err != nil {
		return fmt.Errorf("read the holiday prefill: %w", err)
	}
	var envelope struct {
		Data struct {
			Children []struct {
				StudentID string `json:"student_id"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode the holiday prefill: %w", err)
	}
	if len(envelope.Data.Children) == 0 {
		return fmt.Errorf("the seeded parent has no child to prefill the holiday registration with")
	}
	fmt.Printf("  Ferienanmeldung: prefill offers %d known child(ren) of a parent account\n", len(envelope.Data.Children))
	return nil
}

// seedContributionRules keeps the contribution rules of the Abrechnung
// (#4664) from today on: half the base price from the second child on, and
// no contribution for families receiving benefits. A Träger keeps one rule
// of each kind at a time; when another demo school of the same Träger
// already kept one, the backend refuses the second as an overlap and the
// Träger keeps the first.
func seedContributionRules(client *Client, auth AuthRef, today calendar.Date) error {
	for _, rule := range []map[string]any{
		{"kind": "sibling", "valid_from": today.String(), "sibling_from_child": 2, "discount_basis_points": 5000},
		{"kind": "benefits_exemption", "valid_from": today.String(), "note": "Die Familie bezieht Leistungen für Bildung und Teilhabe."},
	} {
		_, err := client.PostWithAuth(auth, "/traeger/contribution-rules/", rule)
		var refused *APIError
		if errors.As(err, &refused) && refused.Code == seedContributionRuleOverlap {
			continue
		}
		if err != nil {
			return fmt.Errorf("keep the contribution rules: %w", err)
		}
	}
	fmt.Println("  Abrechnung: sibling rule and exemption for families receiving benefits")
	return nil
}

// seedContributionRuleOverlap is the registered code of a second rule of one
// kind valid at the same time.
const seedContributionRuleOverlap = "parent_billing.contribution_rule_overlap"

// seedHolidayQuote is the body of the form's contribution: the bookings of
// every child, without personal data.
func seedHolidayQuote(registration map[string]any) map[string]any {
	children := registration["children"].([]map[string]any)
	quoted := make([]map[string]any, len(children))
	for i, child := range children {
		quoted[i] = map[string]any{"bookings": child["bookings"]}
	}
	return map[string]any{"children": quoted, "benefits_declared": false}
}

// seedHolidayDuplicate is a second parent's registration of the first
// sibling of registration for the last booked week of the guest sibling.
func seedHolidayDuplicate(registration map[string]any) map[string]any {
	children := registration["children"].([]map[string]any)
	child := map[string]any{}
	for key, value := range children[0] {
		child[key] = value
	}
	guestBookings := children[1]["bookings"].([]map[string]any)
	child["allergies"] = ""
	child["bookings"] = guestBookings[len(guestBookings)-1:]
	duplicate := map[string]any{}
	for key, value := range registration {
		duplicate[key] = value
	}
	duplicate["guardian"] = map[string]any{"first_name": "Markus", "last_name": "Ferienkind", "email": "markus.ferienkind@example.org", "phone": "0151 0000003"}
	duplicate["children"] = []map[string]any{child}
	return duplicate
}

// seedTurnstileTestToken is the token Cloudflare's Turnstile test keys of
// the dev and CI environments accept (dev.env.example).
const seedTurnstileTestToken = "XXXX.DUMMY.TOKEN.XXXX"

// The public holiday registration always requires Turnstile (ADR 0110). The
// demo server holds a real Turnstile key, which refuses the test token. It
// also counts every attempt per Träger, sender e-mail and IP address, and
// all demo schools share one Träger and the seeded parent's address: after
// five orders a day the server refuses the attempt before the captcha. On
// either refusal the holiday step publishes the period without
// registrations and skips what builds on them, the billing run and the
// confirmation letters, instead of failing every demo school order.
var seedHolidayRefusals = []string{seedCaptchaFailed, seedSubmissionRateLimited}

var errSeedHolidayRegistrationRefused = errors.New("the server refuses the seeded holiday registration")

// seedHolidayRegistrationsSkippedKey marks a run whose holiday period has no
// registrations, so the confirmation letters step has no run to write for.
const seedHolidayRegistrationsSkippedKey = "traeger.holiday_registrations_skipped"

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
		"second_guardian": map[string]any{"first_name": "Thomas", "last_name": "Ferienkind", "email": "thomas.ferienkind@example.org", "phone": "0151 0000002"},
		"guardian":        map[string]any{"first_name": "Sabine", "last_name": "Ferienkind", "email": "sabine.ferienkind@example.org", "phone": "0151 0000001"},
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

// seedHolidayRegistrationByParent lets the first seeded parent register the
// siblings with the address of the parent's account, as a family with the
// Eltern-App does after the prefill (#4662). Their confirmation letter then
// lands in that parent's document inbox (#4671). A run without seeded
// parents keeps the invented family.
func seedHolidayRegistrationByParent(rt *Runtime, registration map[string]any) {
	if len(rt.Parents) == 0 || rt.Parents[0].Email == "" {
		return
	}
	guardian, _ := registration["guardian"].(map[string]any)
	if guardian == nil {
		return
	}
	guardian["email"] = rt.Parents[0].Email
	if first, last, ok := strings.Cut(strings.TrimSpace(rt.Parents[0].Name), " "); ok && first != "" && last != "" {
		guardian["first_name"], guardian["last_name"] = first, last
	}
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

// seedHolidayPeriod is the request body: the two Herbstferien weeks, Monday
// to Friday, both hosted by host, with prices in whole cent.
func seedHolidayPeriod(today calendar.Date, host int64) map[string]any {
	ferienStart, _ := seedHerbstferien(seedDate{Time: today.UTCMidnight()})
	firstMonday := calendar.NewDate(ferienStart.Year(), ferienStart.Month(), ferienStart.Day())
	weeks := make([]map[string]any, 0, 2)
	for i := range 2 {
		monday := firstMonday.AddDays(7 * i)
		weeks = append(weeks, map[string]any{
			"starts_on": monday.String(), "ends_on": monday.AddDays(4).String(), "host_school_id": host,
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
