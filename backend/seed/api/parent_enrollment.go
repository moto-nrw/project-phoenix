package api

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
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
	enrollmentState.Settings, err = encodeSeedStateSettings(settings)
	if err != nil {
		return fmt.Errorf("encode enrollment settings: %w", err)
	}

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

func (s parentEnrollmentSeedStep) seedSettings(rt *Runtime, auth AuthRef) (map[string]any, error) {
	settings := map[string]any{
		"enrollment.enabled":                            true,
		"operations.parent_sick_note_enabled":           true,
		"operations.parent_notes_enabled":               true,
		"operations.parent_pickup_change_enabled":       true,
		"operations.parent_guardian_management_enabled": true,
		"guardians.parent_invite_mode":                  "direct",
		"guardians.parent_can_remove":                   true,
		"enrollment.require_captcha":                    false,
		"enrollment.offering_changes_enabled":           true,
	}
	for key, value := range settings {
		if _, err := rt.Client.PutWithAuth(auth, "/api/settings/values/"+key, map[string]any{"value": value}); err != nil {
			return nil, fmt.Errorf("set setting %s: %w", key, err)
		}
	}
	return settings, nil
}

func encodeSeedStateSettings(settings map[string]any) (map[string]json.RawMessage, error) {
	encoded := make(map[string]json.RawMessage, len(settings))
	for key, value := range settings {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("setting %s: %w", key, err)
		}
		encoded[key] = raw
	}
	return encoded, nil
}

func (s parentEnrollmentSeedStep) seedParentAccounts(ctx context.Context, rt *Runtime, adminAuth AuthRef) ([]ParentCredentials, map[string]AuthRef, error) {
	password, err := s.parentPassword()
	if err != nil {
		return nil, nil, err
	}

	guardianIndexes := []int{0, 1, 5, 19, 32, 54}
	parents := make([]ParentCredentials, 0, len(guardianIndexes))
	parentAuths := make(map[string]AuthRef, len(guardianIndexes))
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

func (s parentEnrollmentSeedStep) inviteGuardian(rt *Runtime, auth AuthRef, guardianID int64) (string, error) {
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

func (s parentEnrollmentSeedStep) seedEnrollment(rt *Runtime, adminAuth AuthRef, parents []ParentCredentials, parentAuths map[string]AuthRef) (SeedEnrollmentState, error) {
	state := SeedEnrollmentState{
		Offerings: make(map[string]int64),
		Settings:  make(map[string]json.RawMessage),
	}

	schemaID, err := s.createEnrollmentSchema(rt, adminAuth)
	if err != nil {
		return state, err
	}
	phaseID, err := s.createEnrollmentPhase(rt, adminAuth, schemaID)
	if err != nil {
		return state, err
	}
	state.PhaseID = phaseID
	lateInviteRaw, err := rt.Client.PostWithAuth(adminAuth, fmt.Sprintf("/api/enrollment/phases/%d/late-invites", phaseID), map[string]any{
		"guardian_email":      "spaete.anmeldung@example.test",
		"guardian_first_name": "Miriam",
		"guardian_last_name":  "Nachtrag",
		"reason":              "Zuzug nach Anmeldeschluss",
	})
	if err != nil {
		return state, fmt.Errorf("create late enrollment invite: %w", err)
	}
	lateInviteToken, err := parseLateInviteToken(lateInviteRaw)
	if err != nil {
		return state, fmt.Errorf("parse late enrollment invite: %w", err)
	}

	offerings, err := s.createCareOfferings(rt, adminAuth, phaseID)
	if err != nil {
		return state, err
	}
	if err := seedOfferingPlanningTemplate(rt, offerings["mittagessen"]); err != nil {
		return state, err
	}
	if err := seedCourseTemplate(rt, "Fußball-AG", 3, offerings["ag-fussball"], 12); err != nil {
		return state, err
	}
	if err := seedCourseTemplate(rt, "Theater-AG", 4, offerings["ag-theater"], 1); err != nil {
		return state, err
	}
	state.Offerings = offerings

	type enrollmentSubmission struct {
		source string
		body   map[string]any
		auth   AuthRef
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
		// Lea belegt den einzigen Platz der Theater-AG. Dadurch zeigt die
		// Eltern-App bei den anderen Familien einen vollen Kurs — und eine
		// Anfrage dort landet auf der Warteliste (#3075).
		seedSubmission("public", "Lea", "Sommer", "2019-04-18", 1,
			"Daniela", "Sommer", "daniela.sommer@example.test", "approved-3-days",
			// Der Kurs steht ohne Tagesauswahl in der Liste: seine Tage legt
			// die Schule fest (days_of_week_mode "fixed").
			[]string{"ogs-ganztag", "mittagessen", "ag-theater"}, map[string][]string{
				"ogs-ganztag": {"mon", "wed", "fri"},
				"mittagessen": {"mon", "wed", "fri"},
			}, "approved", "Demo-Zusage: drei Betreuungstage"),
		seedSubmission("public", "Emma", "Klein", "2019-01-11", 1,
			"Katharina", "Klein", "katharina.klein@example.test", "approved-1-day",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon"},
				"mittagessen": {"mon"},
			}, "approved", "Demo-Zusage: ein Betreuungstag"),
		seedSubmission("public", "Oskar", "Wolf", "2018-09-29", 1,
			"Martin", "Wolf", "martin.wolf@example.test", "approved-1-day",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"fri"},
				"mittagessen": {"fri"},
			}, "approved", "Demo-Zusage: ein Betreuungstag"),
		seedSubmission("public", "Mia", "Berger", "2018-05-17", 2,
			"Julia", "Berger", "julia.berger@example.test", "approved-1-day",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue"},
				"mittagessen": {"tue"},
			}, "approved", "Demo-Zusage: ein Betreuungstag"),
		seedSubmission("public", "Ben", "Hartmann", "2018-12-02", 1,
			"Nadine", "Hartmann", "nadine.hartmann@example.test", "approved-2-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue", "thu"},
				"mittagessen": {"tue", "thu"},
			}, "approved", "Demo-Zusage: zwei Betreuungstage"),
		seedSubmission("public", "Clara", "Neumann", "2017-10-09", 2,
			"Sven", "Neumann", "sven.neumann@example.test", "approved-2-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "wed"},
				"mittagessen": {"mon", "wed"},
			}, "approved", "Demo-Zusage: zwei Betreuungstage"),
		seedSubmission("public", "Paul", "Seidel", "2017-03-27", 3,
			"Anja", "Seidel", "anja.seidel@example.test", "approved-2-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon", "fri"},
				"mittagessen": {"mon", "fri"},
			}, "approved", "Demo-Zusage: zwei Betreuungstage"),
		seedSubmission("public", "Jonas", "Krüger", "2017-08-15", 2,
			"Petra", "Krüger", "petra.krueger@example.test", "approved-3-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon", "wed", "fri"},
				"mittagessen": {"mon", "wed", "fri"},
			}, "approved", "Demo-Zusage: drei Betreuungstage"),
		seedSubmission("public", "Amelie", "Vogt", "2016-11-20", 3,
			"Jan", "Vogt", "jan.vogt@example.test", "approved-3-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"tue", "thu", "fri"},
				"mittagessen": {"tue", "thu", "fri"},
			}, "approved", "Demo-Zusage: drei Betreuungstage"),
		seedSubmission("public", "Felix", "Braun", "2016-02-06", 4,
			"Birgit", "Braun", "birgit.braun.eltern@example.test", "approved-3-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"mon", "tue", "wed"},
				"mittagessen": {"mon", "tue", "wed"},
			}, "approved", "Demo-Zusage: drei Betreuungstage"),
		seedSubmission("public", "Hannah", "Schmitz", "2017-06-30", 2,
			"Carsten", "Schmitz", "carsten.schmitz@example.test", "approved-4-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "tue", "wed", "fri"},
				"mittagessen": {"mon", "tue", "wed", "fri"},
			}, "approved", "Demo-Zusage: vier Betreuungstage"),
		seedSubmission("public", "David", "Keller", "2016-04-03", 4,
			"Verena", "Keller", "verena.keller@example.test", "approved-4-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue", "wed", "thu", "fri"},
				"mittagessen": {"tue", "wed", "thu", "fri"},
			}, "approved", "Demo-Zusage: vier Betreuungstage"),
		seedSubmission("public", "Elias", "Sommerfeld", "2019-07-08", 1,
			"Marco", "Sommerfeld", "marco.sommerfeld@example.test", "approved-5-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "tue", "wed", "thu", "fri"},
				"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
			}, "approved", "Demo-Zusage: fünf Betreuungstage"),
		seedSubmission("public", "Sophie", "Adler", "2016-12-19", 3,
			"Florian", "Adler", "florian.adler@example.test", "approved-5-days-holiday",
			[]string{"ferienbetreuung", "mittagessen"}, map[string][]string{
				"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
			}, "approved", "Demo-Zusage: Ferienbetreuung"),
		seedSubmission("public", "Marie", "Busch", "2017-05-24", 3,
			"Silke", "Busch", "silke.busch@example.test", "approved-5-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "tue", "wed", "thu", "fri"},
				"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
			}, "approved", "Demo-Zusage: fünf Betreuungstage"),
		seedSubmission("public", "Mika", "Winter", "2018-11-03", 2,
			"Robert", "Winter", "robert.winter@example.test", "waitlisted-holiday",
			[]string{"ferienbetreuung", "mittagessen"}, map[string][]string{
				"mittagessen": {"mon", "tue", "wed", "thu", "fri"},
			}, "waitlisted", "Demo-Warteliste wegen begrenzter Plätze"),
		seedSubmission("public", "Tom", "Ahrens", "2017-09-13", 2,
			"Melanie", "Ahrens", "melanie.ahrens@example.test", "waitlisted-3-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "wed", "fri"},
				"mittagessen": {"mon", "wed", "fri"},
			}, "waitlisted", "Demo-Warteliste: drei Betreuungstage"),
		seedSubmission("public", "Ella", "Franke", "2019-03-01", 1,
			"Steffen", "Franke", "steffen.franke@example.test", "waitlisted-2-days",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue", "thu"},
				"mittagessen": {"tue", "thu"},
			}, "waitlisted", "Demo-Warteliste: zwei Betreuungstage"),
		seedSubmission("public", "Nora", "Brandt", "2017-07-22", 3,
			"Elena", "Brandt", "elena.brandt@example.test", "public-reject",
			[]string{"ogs-kurz", "mittagessen"}, map[string][]string{
				"ogs-kurz":    {"tue", "thu"},
				"mittagessen": {"tue", "thu"},
			}, "rejected", "Demo-Absage für Testdaten"),
		seedSubmission("public", "Mats", "Hoffmann", "2018-01-26", 1,
			"Kerstin", "Hoffmann", "kerstin.hoffmann@example.test", "rejected-3-days",
			[]string{"ogs-ganztag", "mittagessen"}, map[string][]string{
				"ogs-ganztag": {"mon", "wed", "fri"},
				"mittagessen": {"mon", "wed", "fri"},
			}, "rejected", "Demo-Absage: drei Betreuungstage"),
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
			status: "approved",
			reason: "Demo-Zusage: vier Betreuungstage",
		})
	}
	lateInviteBody := s.enrollmentSubmissionWithDays(phaseID, offerings, "Miriam", "Nachtrag", "2018-10-12", 2,
		"Miriam", "Nachtrag", "spaete.anmeldung@example.test", "late-invite-submission",
		[]int64{offerings["ogs-kurz"]}, map[int64][]string{offerings["ogs-kurz"]: {"tue", "thu"}},
	)
	lateInviteBody["late_invite_token"] = lateInviteToken
	submissions = append(submissions, enrollmentSubmission{
		source: "late_invite", path: "/api/enrollment/" + rt.Bootstrap.TenantSlug + "/submit", body: lateInviteBody,
	})

	immediateSeeded, withdrawnSeeded := false, false
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
			if err := s.decideSeedSubmission(rt, adminAuth, request, submission.status, submission.reason, &immediateSeeded); err != nil {
				return state, err
			}
			state.Requests[len(state.Requests)-1].Status = submission.status
		} else if !withdrawnSeeded && submission.source == "public" {
			if _, err := rt.Client.PostPublic(fmt.Sprintf("/api/enrollment/requests/%s/withdraw", request.StatusToken), map[string]any{}); err != nil {
				return state, fmt.Errorf("withdraw demo enrollment request: %w", err)
			}
			state.Requests[len(state.Requests)-1].Status = "withdrawn"
			withdrawnSeeded = true
		}
		if submission.source == "parent" && submission.status == "approved" {
			if err := s.seedOfferingChange(rt, adminAuth, submission.auth, request.RequestID, offerings); err != nil {
				return state, err
			}
		}
		if submission.status == "rejected" {
			if err := seedEnrollmentChangeConversation(rt, adminAuth, submission.body, request); err != nil {
				return state, err
			}
		}
	}
	if err := s.seedEnrollmentDeletion(rt, adminAuth, phaseID, offerings, len(submissions)); err != nil {
		return state, err
	}
	return state, nil
}

func (s parentEnrollmentSeedStep) decideSeedSubmission(rt *Runtime, auth AuthRef, request SeedEnrollmentRequest, status, reason string, immediateSeeded *bool) error {
	if len(request.ChildIDs) == 0 {
		return fmt.Errorf("submitted enrollment request %d has no child ids", request.RequestID)
	}
	decide := func() error {
		return s.decideEnrollmentChild(rt, auth, request.RequestID, request.ChildIDs[0], status, reason)
	}
	if status != "approved" || *immediateSeeded {
		return decide()
	}
	if err := withTemporarySeedSetting(rt, auth, "enrollment.default_activation_mode", "immediate", "scheduled", decide); err != nil {
		return err
	}
	*immediateSeeded = true
	return nil
}

func (s parentEnrollmentSeedStep) seedOfferingChange(rt *Runtime, adminAuth, parentAuth AuthRef, requestID int64, offerings map[string]int64) error {
	detail, err := s.loadEnrollmentRequestDetail(rt, adminAuth, requestID)
	if err != nil {
		return err
	}
	if len(detail.CreatedStudentIDs) == 0 {
		return fmt.Errorf("approved enrollment request %d has no created student", requestID)
	}
	studentID := detail.CreatedStudentIDs[0]
	effectiveFrom := todaySeedDate().AddDays(21).String()
	raw, err := rt.Client.PostWithAuth(parentAuth, fmt.Sprintf("/parent/me/children/%d/care-offerings/requests", studentID), map[string]any{
		"offerings":      []map[string]any{{"offering_id": strconv.FormatInt(offerings["ogs-kurz"], 10), "selected_days": []string{"mon", "tue", "wed", "thu"}}},
		"effective_from": effectiveFrom,
		"note":           "Bitte auf die längere Betreuung wechseln.",
	})
	if err != nil {
		return fmt.Errorf("request offering change: %w", err)
	}
	changeID, err := parsePendingOfferingRequestID(raw)
	if err != nil {
		return fmt.Errorf("parse offering change request: %w", err)
	}
	_, err = rt.Client.PostWithAuth(adminAuth, fmt.Sprintf("/api/students/offering-change-requests/%d/decide", changeID), map[string]any{
		"approve": true, "effective_from": effectiveFrom, "reason": "Demo-Zusage zum Angebotswechsel",
	})
	if err != nil {
		return fmt.Errorf("approve offering change: %w", err)
	}
	return nil
}

func seedEnrollmentChangeConversation(rt *Runtime, adminAuth AuthRef, original map[string]any, request SeedEnrollmentRequest) error {
	if len(request.ChildIDs) == 0 {
		return fmt.Errorf("rejected enrollment request %d has no child ids", request.RequestID)
	}
	body := maps.Clone(original)
	children, ok := body["children"].([]map[string]any)
	if !ok || len(children) == 0 {
		return fmt.Errorf("rejected enrollment request %d has no seed child body", request.RequestID)
	}
	children = slices.Clone(children)
	children[0] = maps.Clone(children[0])
	children[0]["id"] = strconv.FormatInt(request.ChildIDs[0], 10)
	body["children"] = children
	body["parent_note"] = "Wir haben die Angaben ergänzt und bitten um erneute Prüfung."
	raw, err := rt.Client.PostPublic(fmt.Sprintf("/api/enrollment/requests/%s/change-requests", request.StatusToken), body)
	if err != nil {
		return fmt.Errorf("create enrollment change request: %w", err)
	}
	changeID, err := parseEnvelopeStringID(raw)
	if err != nil {
		return fmt.Errorf("parse enrollment change request: %w", err)
	}
	if _, err := rt.Client.PostWithAuth(adminAuth, fmt.Sprintf("/api/enrollment/admin/change-requests/%d/question", changeID), map[string]any{
		"body": "Bitte bestätigen Sie die aktualisierten Betreuungstage.",
	}); err != nil {
		return fmt.Errorf("ask enrollment change question: %w", err)
	}
	if _, err := rt.Client.PostPublic(fmt.Sprintf("/api/enrollment/requests/%s/change-requests/%d/messages", request.StatusToken, changeID), map[string]any{
		"body": "Die Betreuungstage sind so richtig.",
	}); err != nil {
		return fmt.Errorf("reply to enrollment change question: %w", err)
	}
	return nil
}

func (s parentEnrollmentSeedStep) seedEnrollmentDeletion(rt *Runtime, auth AuthRef, phaseID int64, offerings map[string]int64, requestIndex int) error {
	body := s.enrollmentSubmissionWithDays(
		phaseID, offerings, "Tilda", "Löschdemo", "2019-08-13", 1,
		"Mara", "Löschdemo", "mara.loeschdemo@example.test", "deletion-audit",
		[]int64{offerings["ogs-kurz"]}, map[int64][]string{offerings["ogs-kurz"]: {"wed"}},
	)
	raw, err := rt.Client.PostPublicWithHeaders(
		"/api/enrollment/"+rt.Bootstrap.TenantSlug+"/submit", body, publicEnrollmentSeedHeaders(requestIndex),
	)
	if err != nil {
		return fmt.Errorf("submit enrollment deletion demo: %w", err)
	}
	request, err := parseEnrollmentSubmitResponse(raw, "deletion-audit")
	if err != nil {
		return err
	}
	requestPath := fmt.Sprintf("/api/enrollment/admin/requests/%d", request.RequestID)
	if _, err := rt.Client.GetWithAuth(auth, requestPath+"/delete-impact"); err != nil {
		return fmt.Errorf("preview enrollment deletion: %w", err)
	}
	if _, err := rt.Client.DeleteWithAuthBody(auth, requestPath, map[string]any{
		"reason": "Demo-Löschung zur Dokumentation des Datenschutz-Ablaufs",
	}); err != nil {
		return fmt.Errorf("delete enrollment demo request: %w", err)
	}
	return nil
}

func parseLateInviteToken(raw []byte) (string, error) {
	var envelope struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &envelope); err != nil {
		return "", err
	}
	if envelope.Data.Token == "" {
		return "", fmt.Errorf("late invite token missing")
	}
	return envelope.Data.Token, nil
}

func seedOfferingPlanningTemplate(rt *Runtime, offeringID int64) error {
	if rt.FixedSeeder == nil || offeringID == 0 {
		return fmt.Errorf("offering planning prerequisites not available")
	}
	roomID := rt.FixedSeeder.roomIDs["Mensa"]
	categoryID := rt.FixedSeeder.categoryIDs["Mensa"]
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if roomID == 0 || categoryID == 0 || len(staffIDs) == 0 {
		return fmt.Errorf("offering planning references not available")
	}
	today := todaySeedDate()
	_, err := rt.Client.Post("/api/timetable/templates", map[string]any{
		"name": "Mittagessen", "type": "care", "list_kind": "mensa",
		"target_group_type": "angebot", "source_care_offering_ids": []int64{offeringID},
		"weekdays": []int{1, 2, 3, 4, 5}, "start_time": "12:00", "end_time": "13:00",
		"room_id": roomID, "category_id": categoryID, "week_pattern": 0,
		"staff_ids": staffIDs[:1], "primary_staff_id": staffIDs[0],
		"materialize_from": today.String(), "materialize_to": today.AddDays(6).String(),
	})
	if err != nil {
		return fmt.Errorf("create offering planning template: %w", err)
	}
	return nil
}

// seedCourseTemplate makes one demo AG a Kurs in the sense of #3075: an
// activity Regeltermin fed by a care offering. Without it the Kurse section of
// the parents app is empty on every dev machine and nobody reviews it. The
// demo seeds two, one with room and one already full, so both states — a plain
// request and a waiting-list place — are visible without setting anything up.
func seedCourseTemplate(rt *Runtime, name string, weekday int, offeringID int64, maxParticipants int) error {
	if rt.FixedSeeder == nil || offeringID == 0 {
		return fmt.Errorf("course template prerequisites not available")
	}
	roomID := rt.FixedSeeder.roomIDs["Sporthalle"]
	categoryID := seedAnyCategoryID(rt)
	staffIDs := orderedSeedStaffIDs(rt.FixedSeeder)
	if roomID == 0 || categoryID == 0 || len(staffIDs) == 0 {
		return fmt.Errorf("course template references not available")
	}
	today := todaySeedDate()
	_, err := rt.Client.Post("/api/timetable/templates", map[string]any{
		"name": name, "type": "activity", "list_kind": "activity",
		"target_group_type": "angebot", "source_care_offering_ids": []int64{offeringID},
		"weekdays": []int{weekday}, "start_time": "14:00", "end_time": "15:00",
		"room_id": roomID, "category_id": categoryID, "week_pattern": 0,
		"max_participants": maxParticipants,
		"staff_ids":        staffIDs[:1], "primary_staff_id": staffIDs[0],
		"materialize_from": today.String(), "materialize_to": today.AddDays(6).String(),
	})
	if err != nil {
		return fmt.Errorf("create course template %s: %w", name, err)
	}
	return nil
}

func publicEnrollmentSeedHeaders(index int) map[string]string {
	return map[string]string{
		"X-Forwarded-For": fmt.Sprintf("198.51.100.%d", 10+index),
	}
}

func (s parentEnrollmentSeedStep) createEnrollmentSchema(rt *Runtime, auth AuthRef) (int64, error) {
	raw, err := rt.Client.PostWithAuth(auth, "/api/enrollment/schema/", map[string]any{
		"name": "Demo-Anmeldeformular",
		"fields": []map[string]any{
			{"key": "allergies", "label": "Allergien und Unverträglichkeiten", "type": "textarea", "sort_order": 10},
			{"key": "swimming_permission", "label": "Mein Kind darf schwimmen.", "type": "boolean", "sort_order": 20},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("create enrollment form schema: %w", err)
	}
	id, err := parseEnvelopeStringID(raw)
	if err != nil {
		return 0, fmt.Errorf("parse enrollment form schema: %w", err)
	}
	return id, nil
}

func (s parentEnrollmentSeedStep) createEnrollmentPhase(rt *Runtime, auth AuthRef, schemaID int64) (int64, error) {
	now := time.Now().UTC()
	openAt := now.Add(-24 * time.Hour).Format(time.RFC3339)
	closeAt := now.AddDate(0, 2, 0).Format(time.RFC3339)
	serviceStart := now.AddDate(0, -10, 0).Format("2006-01-02")
	serviceEnd := now.AddDate(1, 0, 0).Format("2006-01-02")
	body := map[string]any{
		"name":                         fmt.Sprintf("Demo Anmeldung %d/%d", now.Year(), now.Year()+1),
		"kind":                         "school_year",
		"service_start_date":           serviceStart,
		"service_end_date":             serviceEnd,
		"enrollment_open_at":           openAt,
		"enrollment_close_at":          closeAt,
		"show_status_reason_to_parent": true,
		"care_overflow_mode":           "waitlist",
		"care_offering_selection_mode": "at_least_one",
		"is_active":                    true,
		"form_schema_id":               strconv.FormatInt(schemaID, 10),
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

type seedCareOffering struct {
	key          string
	name         string
	description  string
	daysMode     string
	days         []string
	lunch        bool
	required     bool
	price        int
	capacity     *int
	sort         int
	countsAsCare bool
	pickupTime   string
}

func demoCareOfferings() []seedCareOffering {
	return []seedCareOffering{
		{key: "ogs-ganztag", name: "OGS Ganztag", description: "Betreuung bis 16 Uhr", daysMode: "parent_choice", days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: true, price: 16500, sort: 10, countsAsCare: true, pickupTime: "16:00"},
		{key: "ogs-kurz", name: "Kurzbetreuung", description: "Betreuung bis 14 Uhr", daysMode: "parent_choice", days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: false, price: 9000, sort: 20, countsAsCare: true, pickupTime: "14:00"},
		{key: "mittagessen", name: "Mittagessen", description: "Warme Mahlzeit an Betreuungstagen", daysMode: "parent_choice", days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: true, required: true, price: 5200, sort: 30},
		{key: "ferienbetreuung", name: "Ferienbetreuung Herbst", description: "Plätze für die Herbstferien", daysMode: "fixed", days: []string{"mon", "tue", "wed", "thu", "fri"}, lunch: true, price: 7500, capacity: intPtr(2), sort: 40, countsAsCare: true, pickupTime: "16:00"},
		// Die Demo-Kurse (#3075): Angebote, die an einer AG hängen. Zwei davon,
		// damit die Eltern-App beide Zustände zeigt — einer mit freien Plätzen,
		// einer voll, der eine Anfrage auf die Warteliste setzt.
		{key: "ag-fussball", name: "Fußball-AG", description: "Mittwochs auf dem Sportplatz", daysMode: "fixed", days: []string{"wed"}, capacity: intPtr(12), sort: 50},
		{key: "ag-theater", name: "Theater-AG", description: "Donnerstags in der Aula", daysMode: "fixed", days: []string{"thu"}, capacity: intPtr(1), sort: 51},
	}
}

func (s parentEnrollmentSeedStep) createCareOfferings(rt *Runtime, auth AuthRef, phaseID int64) (map[string]int64, error) {
	offerings := demoCareOfferings()
	created := make(map[string]int64, len(offerings))
	for _, offering := range offerings {
		body := careOfferingSeedBody(offering, phaseID)
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

func careOfferingSeedBody(offering seedCareOffering, phaseID int64) map[string]any {
	body := map[string]any{
		"phase_id":              phaseID,
		"name":                  offering.name,
		"description":           offering.description,
		"days_of_week_mode":     offering.daysMode,
		"available_days":        offering.days,
		"includes_holiday_care": offering.key == "ferienbetreuung",
		"includes_lunch":        offering.lunch,
		"price_cents":           offering.price,
		"is_active":             true,
		"is_required":           offering.required,
		"counts_as_care":        offering.countsAsCare,
		"sort_order":            offering.sort,
		"selection_rule":        "optional",
	}
	if offering.countsAsCare {
		pickupTimes := make(map[string]string, len(offering.days))
		for _, day := range offering.days {
			pickupTimes[day] = offering.pickupTime
		}
		body["pickup_times"] = pickupTimes
	}
	if offering.capacity != nil {
		body["capacity"] = *offering.capacity
	}
	return body
}

func (s parentEnrollmentSeedStep) enrollmentSubmissionWithDays(phaseID int64, offerings map[string]int64, childFirstName, childLastName, dob string, grade int16, guardianFirstName, guardianLastName, guardianEmail, source string, offeringIDs []int64, selectedDaysByOffering map[int64][]string) map[string]any {
	phone := "+49 221 555 990"
	// Names carry real umlauts (bbcdda558), but the canonical email pattern
	// (users.ValidateOptionalEmail) only accepts ASCII — transliterate for
	// the derived address or the public submit rejects the whole seed run.
	emailLocalPart := strings.NewReplacer(
		"ä", "ae",
		"ö", "oe",
		"ü", "ue",
		"ß", "ss",
	).Replace(strings.ReplaceAll(strings.ToLower(guardianFirstName+"."+guardianLastName), " ", "."))
	additionalEmail := emailLocalPart + ".2@example.test"
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
			"note":        "Demo-Datensatz für Anmeldung und Elternportal",
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
	StatusToken       string
	ChildIDs          []int64
	CreatedStudentIDs []int64
}

func (s parentEnrollmentSeedStep) loadEnrollmentRequestDetail(rt *Runtime, auth AuthRef, requestID int64) (enrollmentRequestDetail, error) {
	respBody, err := rt.Client.GetWithAuth(auth, fmt.Sprintf("/api/enrollment/admin/requests/%d", requestID))
	if err != nil {
		return enrollmentRequestDetail{}, fmt.Errorf("load enrollment request %d: %w", requestID, err)
	}
	var resp struct {
		Data struct {
			StatusToken string `json:"status_token"`
			Children    []struct {
				ID               string `json:"id"`
				CreatedStudentID any    `json:"created_student_id"`
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
		if child.CreatedStudentID != nil {
			createdID, parseErr := parseSeedID(child.CreatedStudentID)
			if parseErr == nil && createdID > 0 {
				detail.CreatedStudentIDs = append(detail.CreatedStudentIDs, createdID)
			}
		}
	}
	return detail, nil
}

func parsePendingOfferingRequestID(raw []byte) (int64, error) {
	var envelope struct {
		Data struct {
			PendingRequest *struct {
				ID string `json:"id"`
			} `json:"pending_request"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &envelope); err != nil {
		return 0, err
	}
	if envelope.Data.PendingRequest == nil {
		return 0, fmt.Errorf("pending_request missing")
	}
	id, err := strconv.ParseInt(envelope.Data.PendingRequest.ID, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid pending request id")
	}
	return id, nil
}

func (s parentEnrollmentSeedStep) decideEnrollmentChild(rt *Runtime, auth AuthRef, requestID, childID int64, status, reason string) error {
	_, err := rt.Client.PostWithAuth(auth, fmt.Sprintf("/api/enrollment/admin/requests/%d/children/%d/decide", requestID, childID), map[string]any{
		"status": status,
		"reason": reason,
	})
	if err != nil {
		return fmt.Errorf("decide enrollment request %d child %d as %s: %w", requestID, childID, status, err)
	}
	return nil
}

func (s parentEnrollmentSeedStep) seedParentPortalActions(rt *Runtime, parentAuth AuthRef, parent ParentCredentials) ([]SeedParentPortalAction, error) {
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
				"reason":      "Demo-Abholänderung aus dem Elternportal",
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
		body, err := rt.Client.PostWithAuth(parentAuth, action.path, action.body)
		if err != nil {
			return nil, fmt.Errorf("create parent portal action %s: %w", action.actionType, err)
		}
		if action.actionType == "care-exception" {
			if err := s.shareSeedPickupRequest(rt, parentAuth, parent, body); err != nil {
				return nil, err
			}
		}
		out = append(out, SeedParentPortalAction{
			ParentEmail: parent.Email,
			StudentID:   studentID,
			Type:        action.actionType,
		})
	}
	return out, nil
}

func (s parentEnrollmentSeedStep) shareSeedPickupRequest(
	rt *Runtime, parentAuth AuthRef, parent ParentCredentials, responseBody []byte,
) error {
	requestID, err := parseEnvelopeStringID(responseBody)
	if err != nil {
		return fmt.Errorf("parse seeded pickup request: %w", err)
	}
	for _, candidate := range rt.Parents {
		if candidate.Email == parent.Email || !slices.Contains(candidate.StudentIDs, parent.StudentIDs[0]) {
			continue
		}
		path := fmt.Sprintf("/parent/me/children/%d/request-sharing/pickup_change/%d", parent.StudentIDs[0], requestID)
		_, err = rt.Client.PutWithAuth(parentAuth, path, map[string]any{
			"recipient_guardian_profile_ids": []string{strconv.FormatInt(candidate.GuardianID, 10)},
		})
		if err != nil {
			return fmt.Errorf("share seeded pickup request: %w", err)
		}
		return nil
	}
	return fmt.Errorf("seeded pickup request has no second guardian recipient")
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
