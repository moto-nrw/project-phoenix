package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerInvitees are the colleagues the seeded Träger-Büro has invited
// (#4650), one per role it hands out besides the Verwaltung and the Leitung. The Träger-Portal
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
// The Träger-Büro already has its Leitung, so she gets Verwaltung without
// Leitung and Schulzugang (#5471).
var seedTraegerOperatorInvitee = struct {
	email string
	roles []string
}{"petra.hoffmann@ganztag-nord.example", []string{"administration", "billing"}}

// seedTraegerAdminRoles are the Träger roles of the bootstrap school admin.
var seedTraegerAdminRoles = []string{"administration", "leadership", "school_access", "billing"}

// seedTraegerBueroStep makes the bootstrap school admin the first person of
// the Träger-Büro (ADR 0084, #4649) through the operator route, so every dev
// and demo database can sign in to the Träger-Portal with the admin's
// credentials and sees the Träger's schools. The admin holds Leitung and
// Schulzugang besides Verwaltung (#5471), so the dev database manages the
// accounts and opens the schools, and the role Abrechnung, so it shows the
// contribution rules (#4664); all roles go in one assignment (#5144). The operator also invites a person
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
	if _, err := rt.Client.Post(path, map[string]any{"email": rt.Bootstrap.AdminEmail, "roles": seedTraegerAdminRoles}); err != nil {
		return fmt.Errorf("assign the Träger-Büro: %w", err)
	}
	fmt.Println("  Träger-Büro: school admin assigned with roles Verwaltung, Leitung, Schulzugang and Abrechnung")
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
	rt.Values[seedHolidayLegalPeriodKey] = periodID
	rt.Values[seedHolidayLegalPublishedKey] = published
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
	costBearerID, _ := rt.Values[seedCostBearerKey].(int64)
	runID, err := seedHolidayBillingRun(rt.Client, auth, periodID, costBearerID)
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

// seedHolidayAddressForm saves the next version of the period's form with
// its questions and the postal address as optional (#5625), so the
// registration, its export, the invoice and the letter show an address.
func seedHolidayAddressForm(client *Client, auth AuthRef, periodID int64) error {
	path := fmt.Sprintf("/traeger/holiday-periods/%d/form", periodID)
	raw, err := client.GetWithAuth(auth, path)
	if err != nil {
		return fmt.Errorf("read the holiday form: %w", err)
	}
	var form struct {
		Data struct {
			Fields      json.RawMessage `json:"fields"`
			LegalBlocks json.RawMessage `json:"legal_blocks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &form); err != nil {
		return fmt.Errorf("decode the holiday form: %w", err)
	}
	fields := form.Data.Fields
	if len(fields) == 0 {
		fields = json.RawMessage(`[]`)
	}
	body := map[string]any{"fields": fields, "address_requirement": "optional"}
	if len(form.Data.LegalBlocks) > 0 {
		body["legal_blocks"] = form.Data.LegalBlocks
	}
	if _, err := client.PutWithAuth(auth, path, body); err != nil {
		return fmt.Errorf("ask for the postal address in the holiday form: %w", err)
	}
	fmt.Println("  Träger-Büro: the holiday form asks for the postal address (optional)")
	return nil
}

// seedHolidayRegistrations lets the form ask for the postal address
// (#5625), registers two siblings through the public link (#4661), then a second parent registers the first sibling once more for
// the second week, a possible duplicate, stating benefits with the uploaded
// proof (#5238). The Träger-Büro asks for the debtor number at the
// confirmation (#5621), reviews the list and confirms the siblings'
// registration with their invented debtor number (#4663), which freezes its
// contribution (#4664); it confirms the duplicate with the cost takeover by
// a cost bearer (#5627). The form shows the siblings' contribution before
// it is sent.
func seedHolidayRegistrations(rt *Runtime, auth AuthRef, periodID int64, published []byte, homeSchool int64) (string, error) {
	if err := seedHolidayAddressForm(rt.Client, auth, periodID); err != nil {
		return "", err
	}
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
	proof, err := seedHolidayBenefitsProof(rt.Client, token)
	if err != nil {
		return "", err
	}
	raw, err = rt.Client.PostPublic("/traeger/holiday-registrations/"+token, seedHolidayDuplicate(registration, proof))
	if err != nil {
		return "", fmt.Errorf("register the possible duplicate: %w", err)
	}
	duplicate, err := decodeSeedEntityID(raw)
	if err != nil {
		return "", fmt.Errorf("read the possible duplicate: %w", err)
	}
	registrations := fmt.Sprintf("/traeger/holiday-periods/%d/registrations", periodID)
	if _, err := rt.Client.GetWithAuth(auth, registrations); err != nil {
		return "", fmt.Errorf("list the holiday registrations: %w", err)
	}
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/settings/values/"+seedDebtorNumberSettingKey, map[string]any{"value": true}); err != nil {
		return "", fmt.Errorf("ask for the debtor number at the confirmation: %w", err)
	}
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/settings/values/"+seedPaymentTextSettingKey, map[string]any{"value": seedPaymentText}); err != nil {
		return "", fmt.Errorf("write the payment text of the letters: %w", err)
	}
	confirm := fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d/confirm", periodID, siblings)
	if _, err := rt.Client.PostWithAuth(auth, confirm, map[string]any{"benefits_exemption": false, "debtor_number": seedHolidayDebtorNumber}); err != nil {
		return "", fmt.Errorf("confirm the holiday registration: %w", err)
	}
	fmt.Println("  Träger-Büro: siblings confirmed with their contribution and debtor number")
	if err := seedHolidayAfterConfirmation(rt, auth, token, periodID, siblings, duplicate, registration); err != nil {
		return "", err
	}
	return token, nil
}

// seedHolidaySiblingGroup lets the second parent register a third sibling
// in a form of their own, and the Träger-Büro merges it with the siblings'
// registration (#5628): the third child pays the sibling price although it
// stands in another registration. A demo server that refuses the order
// keeps the period without the group.
func seedHolidaySiblingGroup(rt *Runtime, auth AuthRef, token string, periodID int64, registration map[string]any, siblings int64) error {
	children := registration["children"].([]map[string]any)
	guestBookings := children[1]["bookings"].([]map[string]any)
	sibling := map[string]any{}
	for key, value := range registration {
		sibling[key] = value
	}
	delete(sibling, "second_guardian")
	sibling["guardian"] = map[string]any{"first_name": "Thomas", "last_name": "Ferienkind", "email": "thomas.ferienkind@example.org", "phone": "0151 0000002"}
	sibling["declaration_name"] = "Thomas Ferienkind"
	sibling["children"] = []map[string]any{{
		"first_name": "Mila", "last_name": "Ferienkind", "birth_date": "2020-02-21", "home_school_id": children[0]["home_school_id"],
		"bookings": guestBookings[len(guestBookings)-1:],
	}}
	raw, err := rt.Client.PostPublic("/traeger/holiday-registrations/"+token, sibling)
	var refused *APIError
	if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
		fmt.Printf("  Ferienanmeldung: third sibling refused (%s), no sibling group\n", refused.Code)
		return nil
	}
	if err != nil {
		return fmt.Errorf("register the third sibling: %w", err)
	}
	third, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the third sibling's registration: %w", err)
	}
	merge := fmt.Sprintf("/traeger/holiday-contributions/%d/sibling-groups", periodID)
	if _, err := rt.Client.PostWithAuth(auth, merge, map[string]any{"registration_ids": []int64{siblings, third}}); err != nil {
		return fmt.Errorf("merge the siblings of two registrations: %w", err)
	}
	fmt.Println("  Träger-Büro: the third sibling's registration merged with the siblings")
	return nil
}

// seedHolidayAfterConfirmation corrects the confirmed siblings' details
// (#5629), confirms the second registration with the cost takeover by a
// cost bearer (#5627) and merges a third sibling's registration with the
// siblings (#5628).
func seedHolidayAfterConfirmation(rt *Runtime, auth AuthRef, token string, periodID, siblings, duplicate int64, registration map[string]any) error {
	if err := seedHolidayRegistrationEdit(rt, auth, periodID, siblings, registration); err != nil {
		return err
	}
	if err := seedHolidayCostTakeover(rt, auth, periodID, duplicate); err != nil {
		return err
	}
	return seedHolidaySiblingGroup(rt, auth, token, periodID, registration, siblings)
}

// seedHolidayRegistrationEdit lets the Träger-Büro correct the confirmed
// registration (#5629): the family called with a new phone number and an
// added emergency contact. Name, e-mail and address stay those of the
// registration, so the confirmation letter still reaches the seeded
// parent's inbox with its address. The history then shows the change by its
// fields.
func seedHolidayRegistrationEdit(rt *Runtime, auth AuthRef, periodID, registrationID int64, registration map[string]any) error {
	guardian := map[string]any{}
	if registered, ok := registration["guardian"].(map[string]any); ok {
		maps.Copy(guardian, registered)
	}
	guardian["phone"] = "0151 0000009"
	path := fmt.Sprintf("/traeger/holiday-periods/%d/registrations/%d/details", periodID, registrationID)
	if _, err := rt.Client.PutWithAuth(auth, path, map[string]any{
		"guardian":         guardian,
		"guardian_address": registration["guardian_address"],
		"second_guardian":  registration["second_guardian"],
		"emergency_contacts": []map[string]any{
			{"name": "Helga Ferienkind", "phone": "05402 000002", "relationship": "Großmutter"},
			{"name": "Karin Ferienkind", "phone": "05402 000003", "relationship": "Tante"},
		},
		"pickup_persons": []map[string]any{{"name": "Peter Ferienkind", "relationship": "Onkel"}},
	}); err != nil {
		return fmt.Errorf("change the details of the holiday registration: %w", err)
	}
	fmt.Println("  Träger-Büro: changed the phone number and added an emergency contact")
	return nil
}

// seedCostBearerKey keeps the ID of the seeded cost bearer for the list of
// the billing run (#5627).
const seedCostBearerKey = "traeger.cost_bearer_id"

// seedHolidayCostTakeover lets the Abrechnung keep an invented youth office
// as cost bearer and the Träger-Büro confirm the second parent's
// registration, which states benefits with a proof, with the cost takeover
// by that office (#5627): the family pays 0 €, the office pays the rest.
func seedHolidayCostTakeover(rt *Runtime, auth AuthRef, periodID, registrationID int64) error {
	raw, err := rt.Client.PostWithAuth(auth, "/traeger/cost-bearers/", map[string]any{
		"name": "Fachdienst Jugend des Landkreises Musterstadt", "address": "Am Kreishaus 1\n12345 Musterstadt",
		"file_reference": "FJ 51-2027", "active": true,
	})
	if err != nil {
		return fmt.Errorf("keep the cost bearer: %w", err)
	}
	costBearerID, err := decodeSeedEntityID(raw)
	if err != nil {
		return fmt.Errorf("read the cost bearer: %w", err)
	}
	rt.Values[seedCostBearerKey] = costBearerID
	confirm := fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d/confirm", periodID, registrationID)
	if _, err := rt.Client.PostWithAuth(auth, confirm, map[string]any{
		"benefits_exemption": false, "cost_bearer_id": costBearerID, "debtor_number": seedCostTakeoverDebtorNumber,
	}); err != nil {
		return fmt.Errorf("confirm the holiday registration with the cost takeover: %w", err)
	}
	fmt.Println("  Träger-Büro: second registration confirmed, the cost bearer pays its contribution")
	return nil
}

// seedCostTakeoverDebtorNumber is the invented debtor number of the family
// whose contribution the cost bearer pays.
const seedCostTakeoverDebtorNumber = "10 4712"

// seedDebtorNumberSettingKey is the Träger switch that asks for the debtor
// number of the family at the confirmation (#5621); the demo Träger works
// like Bissendorf and switches it on.
const seedDebtorNumberSettingKey = "parent_billing.debtor_number_on_confirm"

// seedPaymentTextSettingKey is the payment text of the Träger in the
// confirmation letter (#5622). The demo Träger writes the Bissendorf text
// with the sentence for families with a direct debit mandate; the seeded
// run confirms only the siblings, who have a debtor number.
const seedPaymentTextSettingKey = "parent_billing.payment_text"

const seedPaymentText = "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist} auf das Konto unten. " +
	"Geben Sie als Verwendungszweck Ihre Debitorennummer {Debitorennummer} an. " +
	"Haben Sie uns bereits ein SEPA-Lastschriftmandat erteilt, buchen wir den Betrag ab."

// seedHolidayDebtorNumber is the invented debtor number of the confirmed
// siblings; the letter names it as the reference to pay with.
const seedHolidayDebtorNumber = "10 4711"

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

// seedHolidayBenefitsProof uploads the proof of benefits of the second
// parent through the public link (#5238) and returns its token.
func seedHolidayBenefitsProof(client *Client, token string) (string, error) {
	raw, err := client.PostPublicFile("/traeger/holiday-registrations/"+token+"/uploads", "file", "Bescheid-Buergergeld-2027.pdf",
		demoPDF("Bewilligungsbescheid Bürgergeld", []string{"Familie Ferienkind", "Bewilligungszeitraum Januar bis Juni 2027", "Kopie für die Ferienanmeldung"}))
	if err != nil {
		return "", fmt.Errorf("upload the proof of benefits: %w", err)
	}
	var upload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &upload); err != nil || upload.Data.Token == "" {
		return "", fmt.Errorf("the proof upload returned no token: %s", string(raw))
	}
	return upload.Data.Token, nil
}

// seedHolidayDuplicate is a second parent's registration of the first
// sibling of registration for the last booked week of the guest sibling. The
// parent states benefits and names the uploaded proof.
func seedHolidayDuplicate(registration map[string]any, proof string) map[string]any {
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
	duplicate["benefits_declared"] = true
	duplicate["benefits_proof_tokens"] = []string{proof}
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
// holiday guest for both weeks, with answers to the questions of the
// period's standard template (#5198) and the postal address the form asks
// for (#5625), under the consent texts that share the health information
// with the hosting OGS (#4722). All names and addresses are invented.
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
		"second_guardian":  map[string]any{"first_name": "Thomas", "last_name": "Ferienkind", "email": "thomas.ferienkind@example.org", "phone": "0151 0000002"},
		"guardian":         map[string]any{"first_name": "Sabine", "last_name": "Ferienkind", "email": "sabine.ferienkind@example.org", "phone": "0151 0000001"},
		"guardian_address": map[string]any{"street": "Sonnenweg 7", "postal_code": "49143", "city": "Bissendorf"},
		"children": []map[string]any{
			{"first_name": "Lotta", "last_name": "Ferienkind", "birth_date": "2018-04-12", "home_school_id": homeSchool,
				"allergies": "Erdnüsse", "bookings": []map[string]any{booking(0, 1)},
				"custom_data": map[string]any{"lebensmittelunvertraeglichkeiten": "Laktose", "tetanus_impfschutz": "ja",
					"darf_allein_nach_hause": false, "darf_an_ausfluegen_teilnehmen": true}},
			{"first_name": "Jonas", "last_name": "Ferienkind", "birth_date": "2016-09-03", "home_school_id": 0,
				"guest_school_name": "Grundschule am See", "medication": "Asthmaspray bei Bedarf",
				"bookings": []map[string]any{booking(0, 0), booking(1, 0)},
				"custom_data": map[string]any{"tetanus_impfschutz": "weiss_nicht", "darf_allein_nach_hause": true,
					"darf_an_ausfluegen_teilnehmen": true}},
		},
		"emergency_contacts": []map[string]any{{"name": "Helga Ferienkind", "phone": "05402 000002", "relationship": "Großmutter"}},
		"pickup_persons":     []map[string]any{{"name": "Peter Ferienkind", "relationship": "Onkel"}},
		"privacy_consent":    true, "health_consent": true, "binding_confirmed": true,
		"custody_confirmed": true, "declaration_name": "Sabine Ferienkind",
		"consent_version": 3,
		"captcha_token":   seedTurnstileTestToken,
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
		registration["declaration_name"] = first + " " + last
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
