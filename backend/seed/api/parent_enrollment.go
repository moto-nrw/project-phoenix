package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

type parentEnrollmentSeedStep struct {
	seeder *Seeder
}

func (parentEnrollmentSeedStep) Name() string { return "Parent portal and enrollment seeding" }

func (s parentEnrollmentSeedStep) Run(ctx context.Context, rt *Runtime) error {
	if rt.FixedSeeder == nil {
		return fmt.Errorf("fixed seeder not available")
	}
	if rt.Bootstrap == nil {
		return fmt.Errorf("bootstrap state not available")
	}

	adminAuth, err := rt.Adapter.LoginTenant(ctx, rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, rt.Bootstrap.TenantSlug)
	if err != nil {
		return fmt.Errorf("login seed school admin: %w", err)
	}
	rt.SetTenantAuth(adminAuth)

	settings, err := s.seedSettings(rt, adminAuth)
	if err != nil {
		return err
	}

	parents, parentAuths, err := s.seedParentAccounts(ctx, rt, adminAuth)
	if err != nil {
		return err
	}
	rt.Parents = parents

	enrollmentState, err := s.seedEnrollment(rt, adminAuth, parents, parentAuths)
	if err != nil {
		return err
	}
	enrollmentState.Settings = settings

	if len(parents) > 0 {
		actions, err := s.seedParentPortalActions(rt, parentAuths[parents[0].Email], parents[0])
		if err != nil {
			return err
		}
		enrollmentState.ParentActions = actions
	}

	rt.Enrollment = enrollmentState
	rt.SetTenantAuth(adminAuth)
	fmt.Printf("Seeded %d parent accounts and %d enrollment requests\n", len(rt.Parents), len(rt.Enrollment.Requests))
	fmt.Println()
	return nil
}

func (s parentEnrollmentSeedStep) seedSettings(rt *Runtime, auth phoenixapi.AuthRef) (map[string]any, error) {
	settings := map[string]any{
		configModels.KeyEnrollmentEnabled:               true,
		configModels.KeyParentSickNoteEnabled:           true,
		configModels.KeyParentNotesEnabled:              true,
		configModels.KeyParentPickupChangeEnabled:       true,
		configModels.KeyParentGuardianManagementEnabled: true,
		configModels.KeyGuardianParentInviteMode:        configModels.ParentInviteModeDirect,
		configModels.KeyGuardianParentCanRemove:         true,
		configModels.KeyEnrollmentRequireCaptcha:        false,
	}
	for key, value := range settings {
		if _, err := rt.Client.PutWithAuth(auth, "/api/settings/values/"+key, map[string]any{"value": value}); err != nil {
			return nil, fmt.Errorf("set setting %s: %w", key, err)
		}
	}
	return settings, nil
}

func (s parentEnrollmentSeedStep) seedParentAccounts(ctx context.Context, rt *Runtime, adminAuth phoenixapi.AuthRef) ([]ParentCredentials, map[string]phoenixapi.AuthRef, error) {
	password, err := s.parentPassword()
	if err != nil {
		return nil, nil, err
	}

	guardianIndexes := []int{0, 5, 19, 32, 54}
	parents := make([]ParentCredentials, 0, len(guardianIndexes))
	parentAuths := make(map[string]phoenixapi.AuthRef, len(guardianIndexes))
	for _, idx := range guardianIndexes {
		if idx < 0 || idx >= len(DemoGuardians) {
			return nil, nil, fmt.Errorf("demo guardian index %d out of range", idx)
		}
		guardian := DemoGuardians[idx]
		guardianKey := fmt.Sprintf("%s %s", guardian.FirstName, guardian.LastName)
		guardianID, ok := rt.FixedSeeder.guardianIDs[guardianKey]
		if !ok || guardianID == 0 {
			return nil, nil, fmt.Errorf("guardian %s was not created", guardianKey)
		}
		studentID, ok := rt.FixedSeeder.studentIDByIndex[guardian.StudentIndex]
		if !ok || studentID == 0 {
			return nil, nil, fmt.Errorf("student index %d for guardian %s was not created", guardian.StudentIndex, guardianKey)
		}

		token, err := s.inviteGuardian(rt, adminAuth, guardianID)
		if err != nil {
			return nil, nil, fmt.Errorf("invite guardian %s: %w", guardianKey, err)
		}
		accountID, err := s.acceptGuardianInvitation(rt, token, password)
		if err != nil {
			return nil, nil, fmt.Errorf("accept guardian invitation for %s: %w", guardianKey, err)
		}

		auth, err := rt.Adapter.LoginParent(ctx, guardian.Email, password)
		if err != nil {
			return nil, nil, fmt.Errorf("login parent %s: %w", guardian.Email, err)
		}

		parent := ParentCredentials{
			Email:      guardian.Email,
			Password:   password,
			Name:       guardianKey,
			AccountID:  accountID,
			GuardianID: guardianID,
			StudentIDs: []int64{studentID},
		}
		parents = append(parents, parent)
		parentAuths[parent.Email] = auth
	}
	return parents, parentAuths, nil
}

func (s parentEnrollmentSeedStep) parentPassword() (string, error) {
	if strings.TrimSpace(s.seeder.options.StaffPassword) != "" {
		return s.seeder.options.StaffPassword, nil
	}
	return defaultSeedParentPassword, nil
}

func (s parentEnrollmentSeedStep) inviteGuardian(rt *Runtime, auth phoenixapi.AuthRef, guardianID int64) (string, error) {
	respBody, err := rt.Client.PostWithAuthAndHeaders(
		auth,
		fmt.Sprintf("/api/guardians/%d/invite", guardianID),
		map[string]any{},
		map[string]string{seedTokenHeader: "true"},
	)
	if err != nil {
		return "", err
	}
	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := parseJSON(respBody, &resp); err != nil {
		return "", fmt.Errorf("parse guardian invitation response: %w", err)
	}
	if resp.Data.Token == "" {
		return "", fmt.Errorf("guardian invitation response did not include seed token")
	}
	return resp.Data.Token, nil
}

func (s parentEnrollmentSeedStep) acceptGuardianInvitation(rt *Runtime, token, password string) (int64, error) {
	respBody, err := rt.Client.PostPublic("/auth/guardian-invitations/"+token+"/accept", map[string]any{
		"password":         password,
		"confirm_password": password,
	})
	if err != nil {
		return 0, err
	}
	var resp struct {
		Data struct {
			AccountID int64 `json:"account_id"`
		} `json:"data"`
	}
	if err := parseJSON(respBody, &resp); err != nil {
		return 0, fmt.Errorf("parse guardian invitation accept response: %w", err)
	}
	if resp.Data.AccountID == 0 {
		return 0, fmt.Errorf("guardian invitation accept response did not include account_id")
	}
	return resp.Data.AccountID, nil
}

func (s parentEnrollmentSeedStep) seedEnrollment(rt *Runtime, adminAuth phoenixapi.AuthRef, parents []ParentCredentials, parentAuths map[string]phoenixapi.AuthRef) (SeedEnrollmentState, error) {
	state := SeedEnrollmentState{
		Offerings: make(map[string]int64),
		Settings:  make(map[string]any),
	}

	phaseID, err := s.createEnrollmentPhase(rt, adminAuth)
	if err != nil {
		return state, err
	}
	state.PhaseID = phaseID

	offerings, err := s.createCareOfferings(rt, adminAuth, phaseID)
	if err != nil {
		return state, err
	}
	state.Offerings = offerings

	type enrollmentSubmission struct {
		source string
		body   map[string]any
		auth   phoenixapi.AuthRef
		path   string
		status string
		reason string
	}
	seedSubmission := func(source, childFirstName, childLastName, dob string, grade int16, guardianFirstName, guardianLastName, guardianEmail, seedSource string, offeringKeys []string, selectedDays map[string][]string, status, reason string) enrollmentSubmission {
		offeringIDs := make([]int64, 0, len(offeringKeys))
		selectedDaysByID := make(map[int64][]string, len(selectedDays))
		for _, key := range offeringKeys {
			id := offerings[key]
			offeringIDs = append(offeringIDs, id)
			if days, ok := selectedDays[key]; ok {
				selectedDaysByID[id] = days
			}
		}
		return enrollmentSubmission{
			source: source,
			path:   "/api/enrollment/" + rt.Bootstrap.TenantSlug + "/submit",
			body: s.enrollmentSubmissionWithDays(phaseID, offerings, childFirstName, childLastName, dob, grade,
				guardianFirstName, guardianLastName, guardianEmail, seedSource, offeringIDs, selectedDaysByID,
			),
			status: status,
			reason: reason,
		}
	}
	submissions := []enrollmentSubmission{
		seedSubmission("public", "Lea", "Sommer", "2019-04-18", 1,
			"Daniela", "Sommer", "daniela.sommer@example.test", "approved-3-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "wed", "fri"},
				"mittagessen": {"mon", "wed", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: drei Betreuungstage"),
		seedSubmission("public", "Emma", "Klein", "2019-01-11", 1,
			"Katharina", "Klein", "katharina.klein@example.test", "approved-1-day",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon"},
				"mittagessen": {"mon"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: ein Betreuungstag"),
		seedSubmission("public", "Oskar", "Wolf", "2018-09-29", 1,
			"Martin", "Wolf", "martin.wolf@example.test", "approved-1-day",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"fri"},
				"mittagessen": {"fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: ein Betreuungstag"),
		seedSubmission("public", "Mia", "Berger", "2018-05-17", 2,
			"Julia", "Berger", "julia.berger@example.test", "approved-1-day",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue"},
				"mittagessen": {"tue"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: ein Betreuungstag"),
		seedSubmission("public", "Ben", "Hartmann", "2018-12-02", 1,
			"Nadine", "Hartmann", "nadine.hartmann@example.test", "approved-2-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue", "thu"},
				"mittagessen": {"tue", "thu"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: zwei Betreuungstage"),
		seedSubmission("public", "Clara", "Neumann", "2017-10-09", 2,
			"Sven", "Neumann", "sven.neumann@example.test", "approved-2-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "wed"},
				"mittagessen": {"mon", "wed"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: zwei Betreuungstage"),
		seedSubmission("public", "Paul", "Seidel", "2017-03-27", 3,
			"Anja", "Seidel", "anja.seidel@example.test", "approved-2-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon", "fri"},
				"mittagessen": {"mon", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: zwei Betreuungstage"),
		seedSubmission("public", "Jonas", "Krueger", "2017-08-15", 2,
			"Petra", "Krueger", "petra.krueger@example.test", "approved-3-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon", "wed", "fri"},
				"mittagessen": {"mon", "wed", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: drei Betreuungstage"),
		seedSubmission("public", "Amelie", "Vogt", "2016-11-20", 3,
			"Jan", "Vogt", "jan.vogt@example.test", "approved-3-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"tue", "thu", "fri"},
				"mittagessen": {"tue", "thu", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: drei Betreuungstage"),
		seedSubmission("public", "Felix", "Braun", "2016-02-06", 4,
			"Birgit", "Braun", "birgit.braun.eltern@example.test", "approved-3-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon", "tue", "wed"},
				"mittagessen": {"mon", "tue", "wed"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: drei Betreuungstage"),
		seedSubmission("public", "Hannah", "Schmitz", "2017-06-30", 2,
			"Carsten", "Schmitz", "carsten.schmitz@example.test", "approved-4-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "tue", "wed", "fri"},
				"mittagessen": {"mon", "tue", "wed", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: vier Betreuungstage"),
		seedSubmission("public", "David", "Keller", "2016-04-03", 4,
			"Verena", "Keller", "verena.keller@example.test", "approved-4-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue", "wed", "thu", "fri"},
				"mittagessen": {"tue", "wed", "thu", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: vier Betreuungstage"),
		seedSubmission("public", "Elias", "Sommerfeld", "2019-07-08", 1,
			"Marco", "Sommerfeld", "marco.sommerfeld@example.test", "approved-5-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "tue", "wed", "thu", "fri"},
				"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: fuenf Betreuungstage"),
		seedSubmission("public", "Sophie", "Adler", "2016-12-19", 3,
			"Florian", "Adler", "florian.adler@example.test", "approved-5-days-holiday",
			[]string{"ferienbetreuung", "mittagessen"}, map[string][]string{
				"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: Ferienbetreuung"),
		seedSubmission("public", "Marie", "Busch", "2017-05-24", 3,
			"Silke", "Busch", "silke.busch@example.test", "approved-5-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "tue", "wed", "thu", "fri"},
				"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
			}, enrollmentModels.ChildStatusApproved, "Demo-Zusage: fuenf Betreuungstage"),
		seedSubmission("public", "Mika", "Winter", "2018-11-03", 2,
			"Robert", "Winter", "robert.winter@example.test", "waitlisted-holiday",
			[]string{"ferienbetreuung", "mittagessen"}, map[string][]string{
				"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
			}, enrollmentModels.ChildStatusWaitlisted, "Demo-Warteliste wegen begrenzter Plaetze"),
		seedSubmission("public", "Tom", "Ahrens", "2017-09-13", 2,
			"Melanie", "Ahrens", "melanie.ahrens@example.test", "waitlisted-3-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "wed", "fri"},
				"mittagessen": {"mon", "wed", "fri"},
			}, enrollmentModels.ChildStatusWaitlisted, "Demo-Warteliste: drei Betreuungstage"),
		seedSubmission("public", "Ella", "Franke", "2019-03-01", 1,
			"Steffen", "Franke", "steffen.franke@example.test", "waitlisted-2-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue", "thu"},
				"mittagessen": {"tue", "thu"},
			}, enrollmentModels.ChildStatusWaitlisted, "Demo-Warteliste: zwei Betreuungstage"),
		seedSubmission("public", "Nora", "Brandt", "2017-07-22", 3,
			"Elena", "Brandt", "elena.brandt@example.test", "public-reject",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue", "thu"},
				"mittagessen": {"tue", "thu"},
			}, enrollmentModels.ChildStatusRejected, "Demo-Absage fuer Testdaten"),
		seedSubmission("public", "Mats", "Hoffmann", "2018-01-26", 1,
			"Kerstin", "Hoffmann", "kerstin.hoffmann@example.test", "rejected-3-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "wed", "fri"},
				"mittagessen": {"mon", "wed", "fri"},
			}, enrollmentModels.ChildStatusRejected, "Demo-Absage: drei Betreuungstage"),
		{
			source: "public",
			path:   "/api/enrollment/" + rt.Bootstrap.TenantSlug + "/submit",
			body: s.enrollmentSubmissionWithDays(phaseID, offerings, "Jan", "Peters", "2016-10-16", 3,
				"Andrea", "Peters", "andrea.peters@example.test", "submitted-5-days",
				[]int64{offerings["ogs-ganztag"], offerings["mittagessen"]}, map[int64][]string{
					offerings["ogs-ganztag"]: {"mon", "tue", "wed", "thu", "fri"},
					offerings["mittagessen"]: {"mon", "tue", "wed", "thu", "fri"},
				},
			),
		},
		seedSubmission("public", "Lara", "Simon", "2018-06-04", 2,
			"Oliver", "Simon", "oliver.simon@example.test", "submitted-1-day",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon"},
				"mittagessen": {"mon"},
			}, "", ""),
		seedSubmission("public", "Emil", "Graf", "2016-08-21", 4,
			"Susanne", "Graf", "susanne.graf@example.test", "submitted-4-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "tue", "wed", "thu"},
				"mittagessen": {"mon", "tue", "wed", "thu"},
			}, "", ""),
	}
	if len(parents) > 1 {
		parent := parents[1]
		submissions = append(submissions, enrollmentSubmission{
			source: "parent",
			auth:   parentAuths[parent.Email],
			path:   "/parent/enrollments/" + rt.Bootstrap.TenantSlug + "/submit",
			body: s.enrollmentSubmissionWithDays(phaseID, offerings, "Lina", "Richter", "2019-02-14", 1,
				"Thomas", "Richter", parent.Email, "parent-authenticated",
				[]int64{offerings["ogs-ganztag"], offerings["mittagessen"]}, map[int64][]string{
					offerings["ogs-ganztag"]: {"mon", "tue", "wed", "thu"},
					offerings["mittagessen"]: {"mon", "tue", "wed", "thu"},
				},
			),
			status: enrollmentModels.ChildStatusApproved,
			reason: "Demo-Zusage: vier Betreuungstage",
		})
	}

	for index, submission := range submissions {
		if submission.source == "parent" {
			if _, err := rt.Client.GetWithAuth(submission.auth, "/parent/enrollments/"+rt.Bootstrap.TenantSlug+"/profile"); err != nil {
				return state, fmt.Errorf("load parent enrollment profile before submit: %w", err)
			}
		}

		var respBody []byte
		var submitErr error
		if submission.source == "parent" {
			respBody, submitErr = rt.Client.PostWithAuth(submission.auth, submission.path, submission.body)
		} else {
			respBody, submitErr = rt.Client.PostPublicWithHeaders(
				submission.path,
				submission.body,
				publicEnrollmentSeedHeaders(index),
			)
		}
		if submitErr != nil {
			return state, fmt.Errorf("submit %s enrollment: %w", submission.source, submitErr)
		}
		request, err := parseEnrollmentSubmitResponse(respBody, submission.source)
		if err != nil {
			return state, err
		}
		detail, err := s.loadEnrollmentRequestDetail(rt, adminAuth, request.RequestID)
		if err != nil {
			return state, err
		}
		request.StatusToken = detail.StatusToken
		request.ChildIDs = detail.ChildIDs
		state.Requests = append(state.Requests, request)
		if submission.status != "" {
			if len(request.ChildIDs) == 0 {
				return state, fmt.Errorf("submitted enrollment request %d has no child ids", request.RequestID)
			}
			childID := request.ChildIDs[0]
			if err := s.decideEnrollmentChild(rt, adminAuth, request.RequestID, childID, submission.status, submission.reason); err != nil {
				return state, err
			}
			state.Requests[len(state.Requests)-1].Status = submission.status
		}
	}
	return state, nil
}

func publicEnrollmentSeedHeaders(index int) map[string]string {
	return map[string]string{
		"X-Forwarded-For": fmt.Sprintf("198.51.100.%d", 10+index),
	}
}

func (s parentEnrollmentSeedStep) createEnrollmentPhase(rt *Runtime, auth phoenixapi.AuthRef) (int64, error) {
	now := time.Now().UTC()
	openAt := now.Add(-24 * time.Hour).Format(time.RFC3339)
	closeAt := now.AddDate(0, 2, 0).Format(time.RFC3339)
	serviceStart := now.AddDate(0, 2, 0).Format("2006-01-02")
	serviceEnd := now.AddDate(1, 1, 0).Format("2006-01-02")
	body := map[string]any{
		"name":                         fmt.Sprintf("Demo Anmeldung %d/%d", now.Year(), now.Year()+1),
		"kind":                         enrollmentModels.PhaseKindSchoolYear,
		"service_start_date":           serviceStart,
		"service_end_date":             serviceEnd,
		"enrollment_open_at":           openAt,
		"enrollment_close_at":          closeAt,
		"show_status_reason_to_parent": true,
		"care_overflow_mode":           enrollmentModels.PhaseCareOverflowWaitlist,
		"care_offering_selection_mode": enrollmentModels.PhaseCareOfferingSelectionAtLeastOne,
		"is_active":                    true,
	}
	respBody, err := rt.Client.PostWithAuth(auth, "/api/enrollment/phases", body)
	if err != nil {
		return 0, fmt.Errorf("create enrollment phase: %w", err)
	}
	id, err := parseEnvelopeStringID(respBody)
	if err != nil {
		return 0, fmt.Errorf("parse enrollment phase response: %w", err)
	}
	return id, nil
}

func (s parentEnrollmentSeedStep) createCareOfferings(rt *Runtime, auth phoenixapi.AuthRef, phaseID int64) (map[string]int64, error) {
	offerings := []struct {
		key         string
		name        string
		description string
		daysMode    string
		days        []string
		lunch       bool
		required    bool
		price       int
		capacity    *int
		sort        int
	}{
		{key: "ogs-ganztag", name: "OGS Ganztag", description: "Betreuung bis 16 Uhr", daysMode: enrollmentModels.DaysOfWeekModeParentChoice, days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: true, price: 16500, sort: 10},
		{key: "ogs-kurz", name: "Kurzbetreuung", description: "Betreuung bis 14 Uhr", daysMode: enrollmentModels.DaysOfWeekModeParentChoice, days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: false, price: 9000, sort: 20},
		{key: "mittagessen", name: "Mittagessen", description: "Warme Mahlzeit an Betreuungstagen", daysMode: enrollmentModels.DaysOfWeekModeParentChoice, days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: true, required: true, price: 5200, sort: 30},
		{key: "ferienbetreuung", name: "Ferienbetreuung Herbst", description: "Plaetze fuer die Herbstferien", daysMode: enrollmentModels.DaysOfWeekModeFixed, days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: true, price: 7500, capacity: intPtr(2), sort: 40},
	}

	created := make(map[string]int64, len(offerings))
	for _, offering := range offerings {
		description := offering.description
		price := offering.price
		body := map[string]any{
			"phase_id":              phaseID,
			"name":                  offering.name,
			"description":           description,
			"days_of_week_mode":     offering.daysMode,
			"available_days":        offering.days,
			"includes_holiday_care": offering.key == "ferienbetreuung",
			"includes_lunch":        offering.lunch,
			"price_cents":           price,
			"is_active":             true,
			"is_required":           offering.required,
			"sort_order":            offering.sort,
			"selection_rule":        enrollmentModels.SelectionRuleOptional,
		}
		if offering.capacity != nil {
			body["capacity"] = *offering.capacity
		}
		respBody, err := rt.Client.PostWithAuth(auth, "/api/enrollment/care-offerings", body)
		if err != nil {
			return nil, fmt.Errorf("create care offering %s: %w", offering.key, err)
		}
		id, err := parseEnvelopeStringID(respBody)
		if err != nil {
			return nil, fmt.Errorf("parse care offering %s response: %w", offering.key, err)
		}
		created[offering.key] = id
	}
	return created, nil
}

func (s parentEnrollmentSeedStep) enrollmentSubmissionWithDays(phaseID int64, offerings map[string]int64, childFirstName, childLastName, dob string, grade int16, guardianFirstName, guardianLastName, guardianEmail, source string, offeringIDs []int64, selectedDaysByOffering map[int64][]string) map[string]any {
	phone := "+49 221 555 990"
	additionalEmail := strings.ReplaceAll(strings.ToLower(guardianFirstName+"."+guardianLastName), " ", ".") + ".2@example.test"
	offeringDays := []map[string]any{}
	for _, offeringID := range offeringIDs {
		days, ok := selectedDaysByOffering[offeringID]
		if !ok {
			days = defaultSeedSelectedDays(offerings, offeringID)
		}
		if len(days) == 0 {
			continue
		}
		offeringDays = append(offeringDays, map[string]any{
			"offering_id":   offeringID,
			"selected_days": days,
		})
	}
	return map[string]any{
		"phase_id":            phaseID,
		"guardian_first_name": guardianFirstName,
		"guardian_last_name":  guardianLastName,
		"guardian_email":      guardianEmail,
		"guardian_phone":      phone,
		"additional_guardians": []map[string]any{
			{
				"first_name": "Co",
				"last_name":  guardianLastName,
				"email":      additionalEmail,
			},
		},
		"consent_flags": map[string]any{
			"data_processing": true,
			"photos":          source != "public-reject",
		},
		"custom_data": map[string]any{
			"seed_source": source,
			"note":        "Demo-Datensatz fuer Anmeldung und Elternportal",
		},
		"children": []map[string]any{
			{
				"first_name":         childFirstName,
				"last_name":          childLastName,
				"date_of_birth":      dob,
				"target_grade_level": grade,
				"offering_ids":       offeringIDs,
				"offering_days":      offeringDays,
				"custom_data": map[string]any{
					"allergies": "keine",
				},
			},
		},
	}
}

func defaultSeedSelectedDays(offerings map[string]int64, offeringID int64) []string {
	switch offeringID {
	case offerings["ogs-ganztag"]:
		return []string{"mon", "wed", "fri"}
	case offerings["ogs-kurz"]:
		return []string{"tue", "thu"}
	case offerings["mittagessen"]:
		return []string{"mon", "wed", "fri"}
	default:
		return nil
	}
}

func parseEnrollmentSubmitResponse(respBody []byte, source string) (SeedEnrollmentRequest, error) {
	var resp struct {
		Data struct {
			RequestID string `json:"request_id"`
			StatusURL string `json:"status_url"`
		} `json:"data"`
	}
	if err := parseJSON(respBody, &resp); err != nil {
		return SeedEnrollmentRequest{}, fmt.Errorf("parse enrollment submit response: %w", err)
	}
	requestID, err := strconv.ParseInt(resp.Data.RequestID, 10, 64)
	if err != nil || requestID == 0 {
		return SeedEnrollmentRequest{}, fmt.Errorf("enrollment submit response has invalid request_id %q", resp.Data.RequestID)
	}
	return SeedEnrollmentRequest{
		RequestID:   requestID,
		Source:      source,
		StatusURL:   resp.Data.StatusURL,
		StatusToken: statusTokenFromURL(resp.Data.StatusURL),
	}, nil
}

type enrollmentRequestDetail struct {
	StatusToken string
	ChildIDs    []int64
}

func (s parentEnrollmentSeedStep) loadEnrollmentRequestDetail(rt *Runtime, auth phoenixapi.AuthRef, requestID int64) (enrollmentRequestDetail, error) {
	respBody, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/api/enrollment/admin/requests/%d", requestID))
	if err != nil {
		return enrollmentRequestDetail{}, fmt.Errorf("load enrollment request %d: %w", requestID, err)
	}
	var resp struct {
		Data struct {
			StatusToken string `json:"status_token"`
			Children    []struct {
				ID string `json:"id"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := parseJSON(respBody, &resp); err != nil {
		return enrollmentRequestDetail{}, fmt.Errorf("parse enrollment request detail: %w", err)
	}
	detail := enrollmentRequestDetail{StatusToken: resp.Data.StatusToken}
	for _, child := range resp.Data.Children {
		childID, err := strconv.ParseInt(child.ID, 10, 64)
		if err != nil || childID == 0 {
			return enrollmentRequestDetail{}, fmt.Errorf("enrollment request %d has invalid child id %q", requestID, child.ID)
		}
		detail.ChildIDs = append(detail.ChildIDs, childID)
	}
	return detail, nil
}

func (s parentEnrollmentSeedStep) decideEnrollmentChild(rt *Runtime, auth phoenixapi.AuthRef, requestID, childID int64, status, reason string) error {
	_, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/api/enrollment/admin/requests/%d/children/%d/decide", requestID, childID), map[string]any{
		"status": status,
		"reason": reason,
	})
	if err != nil {
		return fmt.Errorf("decide enrollment request %d child %d as %s: %w", requestID, childID, status, err)
	}
	return nil
}

func (s parentEnrollmentSeedStep) seedParentPortalActions(rt *Runtime, parentAuth phoenixapi.AuthRef, parent ParentCredentials) ([]SeedParentPortalAction, error) {
	if len(parent.StudentIDs) == 0 {
		return nil, nil
	}
	studentID := parent.StudentIDs[0]
	if _, err := rt.Client.GetWithAuth(parentAuth, "/parent/me/profile"); err != nil {
		return nil, fmt.Errorf("load parent profile: %w", err)
	}
	if _, err := rt.Client.GetWithAuth(parentAuth, "/parent/me/children"); err != nil {
		return nil, fmt.Errorf("load parent children: %w", err)
	}
	if _, err := rt.Client.GetWithAuth(parentAuth, fmt.Sprintf("/parent/me/children/%d/features", studentID)); err != nil {
		return nil, fmt.Errorf("load parent child features: %w", err)
	}

	actions := []struct {
		actionType string
		path       string
		body       map[string]any
	}{
		{
			actionType: "sick-note",
			path:       fmt.Sprintf("/parent/me/children/%d/sick-note", studentID),
			body: map[string]any{
				"dates":  []string{time.Now().AddDate(0, 0, 2).Format("2006-01-02")},
				"reason": "Demo-Krankmeldung aus dem Elternportal",
			},
		},
		{
			actionType: "care-exception",
			path:       fmt.Sprintf("/parent/me/children/%d/care-exception", studentID),
			body: map[string]any{
				"date":        time.Now().AddDate(0, 0, 3).Format("2006-01-02"),
				"pickup_time": "14:30",
			},
		},
		{
			actionType: "related-account",
			path:       fmt.Sprintf("/parent/me/children/%d/related-accounts", studentID),
			body: map[string]any{
				"email":      "oma.schneider@example.test",
				"first_name": "Elke",
				"last_name":  "Schneider",
			},
		},
	}

	out := make([]SeedParentPortalAction, 0, len(actions))
	for _, action := range actions {
		if _, err := rt.Client.PostWithAuth(parentAuth, action.path, action.body); err != nil {
			return nil, fmt.Errorf("create parent portal action %s: %w", action.actionType, err)
		}
		out = append(out, SeedParentPortalAction{
			ParentEmail: parent.Email,
			StudentID:   studentID,
			Type:        action.actionType,
		})
	}
	return out, nil
}

func parseEnvelopeStringID(respBody []byte) (int64, error) {
	var resp struct {
		Data struct {
			ID any `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return 0, err
	}
	id, err := parseSeedID(resp.Data.ID)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid id %v", resp.Data.ID)
	}
	return id, nil
}

func parseSeedID(value any) (int64, error) {
	switch v := value.(type) {
	case string:
		return strconv.ParseInt(v, 10, 64)
	case float64:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	default:
		return 0, fmt.Errorf("unsupported id type %T", value)
	}
}

func statusTokenFromURL(raw string) string {
	if parsed, err := url.Parse(raw); err == nil {
		return path.Base(parsed.Path)
	}
	return path.Base(raw)
}

func intPtr(v int) *int {
	return &v
}
