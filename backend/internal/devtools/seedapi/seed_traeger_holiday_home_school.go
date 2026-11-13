package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// seedTraegerHolidayHomeSchoolStep plans a holiday week the second school of
// the demo Träger hosts and books a child of the demo school into it (#6679):
// the Träger-Büro confirms the registration, the child is a student of the
// demo school, and the page „Ferienwochen“ of the demo school shows it under
// „Kinder bei anderen OGS“ with the hosting school and a link to its file.
// Without a second school the step does nothing.
type seedTraegerHolidayHomeSchoolStep struct{}

func (seedTraegerHolidayHomeSchoolStep) Name() string {
	return "Seeding a demo child at another school's holiday week"
}

// seedHomeSchoolChild is the child of the demo school the other school cares
// for in the holidays.
var seedHomeSchoolChild = struct{ first, last, birth, parent, email string }{
	"Tom", "Nachbar", "2017-05-20", "Sina", "sina.nachbar@example.org",
}

func (seedTraegerHolidayHomeSchoolStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger home school seed prerequisites are not available")
	}
	if skipped, _ := rt.Values[seedHolidayRegistrationsSkippedKey].(bool); skipped {
		fmt.Println("  Träger-Büro: holiday registrations refused before, no week at another school")
		return nil
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	schools, err := seedTraegerSchoolIDs(rt.Client, auth)
	if err != nil {
		return err
	}
	host := int64(0)
	for _, school := range schools {
		if school != rt.Bootstrap.SchoolID {
			host = school
			break
		}
	}
	if host == 0 {
		fmt.Println("  Träger-Büro: no second school, no week at another school")
		return nil
	}
	periodID, weekID, modelID, token, err := seedHomeSchoolPeriod(rt.Client, auth, calendar.TodayDate(), host)
	if err != nil {
		return err
	}
	registration, ok, err := seedHomeSchoolRegistration(rt, token, weekID, modelID)
	if err != nil || !ok {
		return err
	}
	confirm := fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d/confirm", periodID, registration)
	if _, err := rt.Client.PostWithAuth(auth, confirm, map[string]any{"benefits_exemption": false, "debtor_number": "47030"}); err != nil {
		return fmt.Errorf("confirm the registration at another school: %w", err)
	}
	return seedHomeSchoolStudent(rt)
}

// seedHomeSchoolPeriod publishes the Osterferien week of the second school,
// moving it on while it holds a day without care (#4702).
func seedHomeSchoolPeriod(client *Client, auth AuthRef, today calendar.Date, host int64) (periodID, weekID, modelID int64, token string, err error) {
	monday := seedNextWinterMonday(today).AddDays(49)
	var raw []byte
	for shift := 0; ; shift++ {
		start := monday.AddDays(7 * shift)
		raw, err = client.PostWithAuth(auth, "/traeger/holiday-periods/", map[string]any{
			"name":                  fmt.Sprintf("Ferienbetreuung Osterferien %d an der Nachbarschule", start.Year()),
			"registration_opens_on": today.String(), "registration_closes_on": start.AddDays(-7).String(),
			"binding": true,
			"weeks": []map[string]any{{
				"starts_on": start.String(), "ends_on": start.AddDays(4).String(), "host_school_id": host,
				"time_models": []map[string]any{{"name": "bis 15 Uhr", "starts_at": "08:00", "ends_at": "15:00", "price_cents": 5500}},
			}},
		})
		var refused *APIError
		if shift+1 < seedHolidayPeriodShifts && errors.As(err, &refused) && refused.Code == seedHolidayWeekNoCareDayCode {
			continue
		}
		break
	}
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("create the holiday period at another school: %w", err)
	}
	if periodID, err = decodeSeedEntityID(raw); err != nil {
		return 0, 0, 0, "", fmt.Errorf("read the holiday period at another school: %w", err)
	}
	published, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/holiday-periods/%d/publish", periodID), map[string]any{})
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("publish the holiday period at another school: %w", err)
	}
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
	if err := json.Unmarshal(published, &envelope); err != nil || envelope.Data.PublicToken == "" ||
		len(envelope.Data.Weeks) == 0 || len(envelope.Data.Weeks[0].TimeModels) == 0 {
		return 0, 0, 0, "", fmt.Errorf("the holiday period at another school has no link or no bookable week")
	}
	week := envelope.Data.Weeks[0]
	fmt.Println("  Träger-Büro: Osterferien week planned at the second school")
	return periodID, week.ID, week.TimeModels[0].ID, envelope.Data.PublicToken, nil
}

// seedHomeSchoolRegistration registers the child of the demo school for the
// week of the second school. false reports a refused registration, which
// ends the step quietly.
func seedHomeSchoolRegistration(rt *Runtime, token string, weekID, modelID int64) (int64, bool, error) {
	child := seedHomeSchoolChild
	raw, err := rt.Client.PostPublicWithHeaders("/traeger/holiday-registrations/"+token, map[string]any{
		"guardian": map[string]any{"first_name": child.parent, "last_name": child.last, "email": child.email, "phone": "0151 0000301"},
		"children": []map[string]any{{
			"first_name": child.first, "last_name": child.last, "birth_date": child.birth, "home_school_id": rt.Bootstrap.SchoolID,
			"bookings": []map[string]any{{"week_id": weekID, "time_model_id": modelID}},
		}},
		"emergency_contacts": []map[string]any{{"name": "Opa " + child.last, "phone": "05402 000030", "relationship": "Großvater"}},
		"privacy_consent":    true, "binding_confirmed": true, "custody_confirmed": true,
		"declaration_name": child.parent + " " + child.last, "consent_version": seedHolidayConsentVersion, "retention_days": seedHolidayRetentionDays, "captcha_token": seedTurnstileTestToken,
	}, publicEnrollmentSeedHeaders(130))
	var refused *APIError
	if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
		fmt.Printf("  Ferienanmeldung: registration at another school refused (%s)\n", refused.Code)
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("register a child for another school's week: %w", err)
	}
	registration, err := decodeSeedEntityID(raw)
	if err != nil {
		return 0, false, fmt.Errorf("read the registration at another school: %w", err)
	}
	return registration, true, nil
}

// seedHomeSchoolStudent makes the child a student of the demo school and
// checks that the demo school sees its booking, assigned to the child's
// file. The step fails when the page would stay empty.
func seedHomeSchoolStudent(rt *Runtime) error {
	child := seedHomeSchoolChild
	groupID := firstSeedGroupID(rt.FixedSeeder)
	if groupID == 0 {
		return fmt.Errorf("the demo school has no group for its child at another school")
	}
	if _, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/students", map[string]any{
		"first_name": child.first, "last_name": child.last, "school_class": "Klasse 3a",
		"group_id": groupID, "birthday": child.birth, "pickup_status": "pickup",
	}); err != nil {
		return fmt.Errorf("create the demo school's child at another school: %w", err)
	}
	raw, err := rt.Client.GetWithAuth(rt.TenantAuth, "/api/holiday-weeks/home-school-children")
	if err != nil {
		return fmt.Errorf("read the demo school's children at other schools: %w", err)
	}
	type booking struct {
		StudentID *int64 `json:"student_id"`
	}
	var envelope struct {
		Data []booking `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode the demo school's children at other schools: %w", err)
	}
	if !slices.ContainsFunc(envelope.Data, func(entry booking) bool { return entry.StudentID != nil }) {
		return fmt.Errorf("the demo school sees no child of its own at another school's holiday week")
	}
	fmt.Printf("  Heimatschule: %d child(ren) at another school's holiday week\n", len(envelope.Data))
	return nil
}
