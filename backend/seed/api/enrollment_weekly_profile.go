package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"
)

const enrollmentWeeklyProfileKey = "anmeldung-wochenplan"

func enrollmentWeeklyProfileDefinition() demoProfileDefinition {
	settings := fullOperationSettings()
	settings[profileSettingAttendanceNFC] = SeedSetting{Value: json.RawMessage(`false`), ManagedBy: SettingManagedByOperator}
	settings[profileSettingCareConcept] = SeedSetting{Value: json.RawMessage(`"open_rooms"`), ManagedBy: SettingManagedByTenant}
	return demoProfileDefinition{
		Key: enrollmentWeeklyProfileKey, OrganizationName: "Demo-Träger Süd", OrganizationSlug: "demo-traeger-sued",
		SchoolName: "Demo-Schule Anmeldung und Wochenplan", SchoolSlug: enrollmentWeeklyProfileKey,
		SchoolAdminEmail: "anmeldung-wochenplan-admin@example.test", SchoolAdminPassword: "Wochenplan1234%",
		Settings: settings, Expected: SeedExpectedState{Students: 12, Contacts: 24, ParentAccounts: 12, HasEnrollment: true},
	}
}

type seedEnrollmentWeeklyProfileStep struct{ seeder *Seeder }

func (seedEnrollmentWeeklyProfileStep) Name() string { return "Seeding anmeldung-wochenplan profile" }

func (s seedEnrollmentWeeklyProfileStep) Run(ctx context.Context, primary *Runtime) error {
	child := *s.seeder
	child.profile, child.definition = enrollmentWeeklyProfileKey, enrollmentWeeklyProfileDefinition()
	child.options.TenantSlug, child.options.AdminEmail = "", ""
	rt := newRuntime(&child, primary.OperatorEmail, primary.OperatorPassword, primary.StaffPIN)
	rt.SetOperatorAuth(primary.OperatorAuth)
	defer primary.SetTenantAuth(primary.TenantAuth)
	workflow := Workflow{Name: enrollmentWeeklyProfileKey, Steps: []Step{
		bootstrapTenantStep{seeder: &child}, configureProfileStep{definition: child.definition},
	}}
	if err := workflow.Run(ctx, rt); err != nil {
		return child.formatProfileError(child.profile, workflow.Name, err)
	}
	state, err := seedWeeklyProfileContents(ctx, rt, &child)
	if err != nil {
		return child.formatProfileError(child.profile, "enrollment and weekly plans", err)
	}
	if err := linkDeveloperAdmin(ctx, primary, rt, state); err != nil {
		return err
	}
	if err := verifyProfileSettings(rt, child.definition); err != nil {
		return err
	}
	physical, err := listSeedDevices(rt, "terminal")
	if err != nil {
		return err
	}
	if len(physical) != 0 {
		return fmt.Errorf("%s must have no physical terminals, got %d", child.profile, len(physical))
	}
	virtual, err := verifyProfileDevices(rt, 0)
	if err != nil {
		return err
	}
	state.Devices[virtual.DeviceID] = virtual
	if err := verifyProfileStudents(rt, 12); err != nil {
		return err
	}
	state.Normalize()
	primary.Values[enrollmentWeeklyProfileKey] = state
	fmt.Printf("Verified profile %s: organization %s (%d), school %s (%d), admin %s / %s, phase %d, %d offerings, %d requests\n",
		child.profile, rt.Bootstrap.OrganizationSlug, rt.Bootstrap.OrganizationID, rt.Bootstrap.TenantSlug, rt.Bootstrap.SchoolID,
		rt.Bootstrap.AdminEmail, rt.Bootstrap.AdminPassword, state.Enrollment.PhaseID, len(state.Enrollment.Offerings), len(state.Enrollment.Requests))
	for _, parent := range state.Parents {
		fmt.Printf("  Parent %s: %s / %s, children %v\n", parent.Key, parent.Email, parent.Password, parent.StudentIDs)
	}
	for _, request := range state.Enrollment.Requests {
		fmt.Printf("  Request %s: %d (%s)\n", request.Key, request.RequestID, request.Status)
	}
	fmt.Printf("  Schema: %d; offerings: %s\n", state.Enrollment.SchemaID, formatSortedCountMap(state.Enrollment.Offerings))
	for _, student := range state.Students {
		fmt.Printf("  Child %s: %d\n", student.Key, student.ID)
	}
	fmt.Printf("  Developer admin: %s / %s\n", state.Accounts.Admin[0].Email, state.Accounts.Admin[0].Password)
	return nil
}

func seedWeeklyProfileContents(ctx context.Context, rt *Runtime, child *Seeder) (*SeedState, error) {
	step := parentEnrollmentSeedStep{seeder: child}
	settings, err := step.seedSettings(rt, rt.TenantAuth)
	if err != nil {
		return nil, err
	}
	// Immediate activation makes bookings visible without waiting for the scheduler.
	if _, err := rt.Client.Put("/api/settings/values/enrollment.default_activation_mode", map[string]any{"value": "immediate"}); err != nil {
		return nil, err
	}
	// The explicit invitation flow below exposes its seed token. Do not also
	// send an automatic invitation during approval, which would leave a pending
	// invitation and prevent that flow. Restore automatic invitations afterwards.
	if _, err := rt.Client.Put("/api/settings/values/enrollment.auto_invite_guardian_on_approval", map[string]any{"value": false}); err != nil {
		return nil, err
	}
	schemaID, err := step.createEnrollmentSchema(rt, rt.TenantAuth)
	if err != nil {
		return nil, err
	}
	phaseID, err := step.createEnrollmentPhase(rt, rt.TenantAuth, schemaID)
	if err != nil {
		return nil, err
	}
	if err := verifyWeeklyProfilePhase(rt, phaseID, schemaID); err != nil {
		return nil, err
	}
	offering := seedCareOffering{key: "wochenangebot", name: "Nachmittagsbetreuung", description: "Betreuung bis 16 Uhr", daysMode: "parent_choice", days: []string{"mon", "tue", "wed", "thu", "fri"}, countsAsCare: true, pickupTime: "16:00", sort: 10}
	raw, err := rt.Client.Post("/api/enrollment/care-offerings", careOfferingSeedBody(offering, phaseID))
	if err != nil {
		return nil, err
	}
	offeringID, err := parseEnvelopeStringID(raw)
	if err != nil {
		return nil, err
	}
	state := child.collectSeedState(NewFixedSeeder(rt.Client, rt.Verbose, child.options.StaffPassword), rt.StaffPIN, rt.Bootstrap)
	state.Enrollment = SeedEnrollmentState{SchemaID: schemaID, PhaseID: phaseID, Offerings: map[string]int64{offering.key: offeringID}}
	state.Enrollment.Settings, err = encodeSeedStateSettings(settings)
	if err != nil {
		return nil, err
	}
	var parentAuth AuthRef
	for index := range 16 {
		status := "approved"
		if index >= 12 {
			status = []string{"submitted", "waitlisted", "rejected", "withdrawn"}[index-12]
		}
		request, err := submitWeeklyProfileRequest(rt, step, state.Enrollment, index, status, parentAuth, state.Parents)
		if err != nil {
			return nil, err
		}
		state.Enrollment.Requests = append(state.Enrollment.Requests, request)
		if status != "approved" {
			continue
		}
		detail, err := step.loadEnrollmentRequestDetail(rt, rt.TenantAuth, request.RequestID)
		if err != nil {
			return nil, err
		}
		if len(detail.CreatedStudentIDs) != 1 {
			return nil, fmt.Errorf("request %d: expected one created child", request.RequestID)
		}
		studentID := detail.CreatedStudentIDs[0]
		firstName := weeklyProfileNames()[index]
		state.Students = append(state.Students, SeedStudent{Key: semanticKey(firstName + " Wochenplan"), ID: studentID, FirstName: firstName, LastName: "Wochenplan"})
		if err := writeWeeklyProfileSchedule(rt, studentID); err != nil {
			return nil, err
		}
		parent, auth, err := inviteWeeklyProfileParent(ctx, rt, step, studentID, index)
		if err != nil {
			return nil, err
		}
		state.Parents = append(state.Parents, parent)
		if index == 0 {
			parentAuth = auth
		}
		if err := verifyWeeklyProfileCare(rt, auth, studentID, offeringID); err != nil {
			return nil, err
		}
	}
	state.Credentials.Parents = state.Parents
	if _, err := rt.Client.Put("/api/settings/values/enrollment.auto_invite_guardian_on_approval", map[string]any{"value": true}); err != nil {
		return nil, err
	}
	return state, nil
}

func weeklyProfileNames() []string {
	return []string{"Anna", "Ben", "Clara", "David", "Emilia", "Felix", "Greta", "Hannes", "Ida", "Jonas", "Klara", "Leon", "Mila", "Noah", "Olivia", "Paul"}
}

func submitWeeklyProfileRequest(rt *Runtime, step parentEnrollmentSeedStep, enrollment SeedEnrollmentState, index int, status string, parentAuth AuthRef, parents []ParentCredentials) (SeedEnrollmentRequest, error) {
	key := fmt.Sprintf("kind-%02d-%s", index+1, status)
	email := fmt.Sprintf("wochenplan-eltern-%02d@example.test", index+1)
	if index == 12 {
		email = parents[0].Email
	}
	offeringID := enrollment.Offerings["wochenangebot"]
	body := step.enrollmentSubmissionWithDays(enrollment.PhaseID, enrollment.Offerings, weeklyProfileNames()[index], "Wochenplan", "2019-04-18", 1,
		"Alex", fmt.Sprintf("Wochenplan%02d", index+1), email, key, []int64{offeringID}, map[int64][]string{offeringID: {"mon"}})
	source := "public"
	var raw []byte
	var err error
	if index == 12 {
		source = "parent"
		raw, err = rt.Client.PostWithAuth(parentAuth, "/parent/enrollments/"+rt.Bootstrap.TenantSlug+"/submit", body)
	} else {
		raw, err = rt.Client.PostPublicWithHeaders("/api/enrollment/"+rt.Bootstrap.TenantSlug+"/submit", body, publicEnrollmentSeedHeaders(100+index))
	}
	if err != nil {
		return SeedEnrollmentRequest{}, err
	}
	request, err := parseEnrollmentSubmitResponse(raw, source)
	if err != nil {
		return request, err
	}
	detail, err := step.loadEnrollmentRequestDetail(rt, rt.TenantAuth, request.RequestID)
	if err != nil {
		return request, err
	}
	request.Key, request.Status, request.ChildIDs = key, status, detail.ChildIDs
	if len(request.ChildIDs) != 1 {
		return request, fmt.Errorf("request %s must have one child", key)
	}
	switch status {
	case "withdrawn":
		_, err = rt.Client.PostPublic("/api/enrollment/requests/"+request.StatusToken+"/withdraw", map[string]any{})
	case "submitted":
	default:
		err = step.decideEnrollmentChild(rt, rt.TenantAuth, request.RequestID, request.ChildIDs[0], status, "Beispiel für diesen Anmeldestatus")
	}
	if err != nil {
		return request, err
	}
	raw, err = rt.Client.GetWithAuth(rt.TenantAuth, fmt.Sprintf("/api/enrollment/admin/requests/%d", request.RequestID))
	if err != nil {
		return request, err
	}
	var check struct {
		Data struct {
			Children []struct {
				Status string `json:"status"`
			} `json:"children"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &check); err != nil {
		return request, err
	}
	if len(check.Data.Children) != 1 || check.Data.Children[0].Status != status {
		return request, fmt.Errorf("request %s: child decision does not match %s", key, status)
	}
	return request, nil
}

func writeWeeklyProfileSchedule(rt *Runtime, studentID int64) error {
	for _, schedule := range []struct{ path, timeField, clock string }{{"arrival-schedules", "expected_arrival", "12:00"}, {"pickup-schedules", "pickup_time", "15:00"}} {
		_, err := rt.Client.Put(fmt.Sprintf("/api/students/%d/%s", studentID, schedule.path), map[string]any{"schedules": []map[string]any{
			{"weekday": 1, schedule.timeField: schedule.clock}, {"weekday": 2, schedule.timeField: schedule.clock}, {"weekday": 4, schedule.timeField: schedule.clock},
		}})
		if err != nil {
			return err
		}
	}
	return nil
}

func inviteWeeklyProfileParent(ctx context.Context, rt *Runtime, step parentEnrollmentSeedStep, studentID int64, index int) (ParentCredentials, AuthRef, error) {
	email := fmt.Sprintf("wochenplan-eltern-%02d@example.test", index+1)
	raw, err := rt.Client.Get(fmt.Sprintf("/api/guardians/students/%d/guardians", studentID))
	if err != nil {
		return ParentCredentials{}, AuthRef{}, err
	}
	var response struct {
		Data []struct {
			Guardian struct {
				ID    int64  `json:"id"`
				Email string `json:"email"`
			} `json:"guardian"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return ParentCredentials{}, AuthRef{}, err
	}
	var guardianID int64
	for _, row := range response.Data {
		if row.Guardian.Email == email {
			guardianID = row.Guardian.ID
		}
	}
	if guardianID == 0 {
		return ParentCredentials{}, AuthRef{}, fmt.Errorf("child %d: guardian %s missing", studentID, email)
	}
	token, err := step.inviteGuardian(rt, rt.TenantAuth, guardianID)
	if err != nil {
		return ParentCredentials{}, AuthRef{}, err
	}
	password, err := step.parentPassword()
	if err != nil {
		return ParentCredentials{}, AuthRef{}, err
	}
	accountID, err := step.acceptGuardianInvitation(rt, token, password)
	if err != nil {
		return ParentCredentials{}, AuthRef{}, err
	}
	auth, err := rt.Adapter.LoginParent(ctx, email, password)
	parent := ParentCredentials{Key: fmt.Sprintf("eltern-%02d", index+1), Email: email, Password: password, Name: fmt.Sprintf("Alex Wochenplan%02d", index+1), AccountID: accountID, GuardianID: guardianID, StudentIDs: []int64{studentID}}
	return parent, auth, err
}

func verifyWeeklyProfilePhase(rt *Runtime, phaseID, schemaID int64) error {
	raw, err := rt.Client.Get(fmt.Sprintf("/api/enrollment/phases/%d", phaseID))
	if err != nil {
		return err
	}
	var response struct {
		Data struct {
			Active   bool      `json:"is_active"`
			SchemaID string    `json:"form_schema_id"`
			Opens    time.Time `json:"enrollment_open_at"`
			Closes   time.Time `json:"enrollment_close_at"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return err
	}
	now := time.Now()
	if !response.Data.Active || response.Data.SchemaID != strconv.FormatInt(schemaID, 10) || !response.Data.Opens.Before(now) || !response.Data.Closes.After(now) {
		return fmt.Errorf("phase %d: expected an active open phase with schema %d", phaseID, schemaID)
	}
	return nil
}

func verifyWeeklyProfileCare(rt *Runtime, auth AuthRef, studentID, offeringID int64) error {
	path := fmt.Sprintf("/parent/me/children/%d/", studentID)
	raw, err := rt.Client.GetWithAuth(auth, path+"care-offerings")
	if err != nil {
		return err
	}
	var bookings struct {
		Data struct {
			Offerings []struct {
				ID       string `json:"id"`
				Weekdays []int  `json:"weekdays"`
			} `json:"offerings"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &bookings); err != nil {
		return err
	}
	found := false
	for _, booking := range bookings.Data.Offerings {
		if booking.ID == strconv.FormatInt(offeringID, 10) && slices.Equal(booking.Weekdays, []int{1}) {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("child %d: Monday booking missing from parent API", studentID)
	}
	raw, err = rt.Client.GetWithAuth(auth, path+"care-schedule")
	if err != nil {
		return err
	}
	var plan struct {
		Data struct {
			Weekdays []struct {
				Weekday int    `json:"weekday"`
				Status  string `json:"status"`
				Arrival string `json:"arrival"`
				Pickup  string `json:"pickup"`
			} `json:"weekdays"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &plan); err != nil {
		return err
	}
	if len(plan.Data.Weekdays) != 5 {
		return fmt.Errorf("child %d: expected five care weekdays", studentID)
	}
	seen := make(map[int]bool)
	for _, day := range plan.Data.Weekdays {
		expected := "not_scheduled"
		if day.Weekday == 1 || day.Weekday == 2 || day.Weekday == 4 {
			expected = "scheduled"
		}
		if day.Weekday < 1 || day.Weekday > 5 || seen[day.Weekday] || day.Status != expected {
			return fmt.Errorf("child %d: weekly plan priority failed on weekday %d: %s", studentID, day.Weekday, day.Status)
		}
		if expected == "scheduled" && (day.Arrival != "12:00" || day.Pickup != "15:00") {
			return fmt.Errorf("child %d: weekly plan times differ", studentID)
		}
		seen[day.Weekday] = true
	}
	return nil
}

func linkDeveloperAdmin(ctx context.Context, primary, target *Runtime, state *SeedState) error {
	if primary.FixedSeeder == nil || len(primary.FixedSeeder.staffCredentials) == 0 {
		return fmt.Errorf("developer admin credentials missing")
	}
	admin := primary.FixedSeeder.staffCredentials[0]
	fs := NewFixedSeeder(target.Client, false, "")
	if err := fs.fetchRoles(ctx); err != nil {
		return err
	}
	roleID := fs.roleIDs["admin"]
	if roleID == 0 {
		return fmt.Errorf("target school admin role missing")
	}
	linked, err := target.Client.Post("/auth/link-to-tenant", map[string]any{"email": admin.Email, "role_id": roleID, "first_name": "Demo", "last_name": "Admin"})
	if err != nil {
		return err
	}
	var identity struct {
		Data struct {
			SchoolIdentity struct {
				StaffID   int64 `json:"staff_id,string"`
				TeacherID int64 `json:"teacher_id,string"`
			} `json:"school_identity"`
		} `json:"data"`
	}
	if err := parseJSON(linked, &identity); err != nil {
		return err
	}
	if identity.Data.SchoolIdentity.StaffID == 0 {
		return fmt.Errorf("developer admin staff identity missing")
	}
	state.Accounts.Admin = []AccountCredentials{{Key: "developer-admin", Email: admin.Email, Password: admin.Password, Name: admin.Name, StaffID: identity.Data.SchoolIdentity.StaffID, TeacherID: identity.Data.SchoolIdentity.TeacherID}}
	auth, err := target.Adapter.LoginTenant(ctx, admin.Email, admin.Password, primary.Bootstrap.TenantSlug)
	if err != nil {
		return err
	}
	raw, err := target.Client.PostWithAuth(auth, "/auth/switch-tenant", map[string]any{"tenant_slug": target.Bootstrap.TenantSlug})
	if err != nil {
		return fmt.Errorf("cross-organization tenant switch: %w", err)
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := parseJSON(raw, &tokens); err != nil {
		return err
	}
	if tokens.AccessToken == "" {
		return fmt.Errorf("tenant switch returned no access token")
	}
	switched := AuthRef{Kind: AuthBearer, Token: tokens.AccessToken, Label: admin.Email}
	verification := *target
	verification.TenantAuth = switched
	if err := verifyProfileSettings(&verification, enrollmentWeeklyProfileDefinition()); err != nil {
		return fmt.Errorf("switched tenant settings: %w", err)
	}
	if _, err := target.Client.GetWithAuth(switched, "/api/students?page=1&page_size=1"); err != nil {
		return err
	}
	raw, err = target.Client.GetWithAuth(target.TenantAuth, "/auth/account/tenants")
	if err != nil {
		return err
	}
	var tenants struct {
		Data []struct {
			TenantID int64 `json:"tenant_id"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &tenants); err != nil {
		return err
	}
	if len(tenants.Data) != 1 || tenants.Data[0].TenantID != target.Bootstrap.SchoolID {
		return fmt.Errorf("school admin is not isolated to own school")
	}
	_, err = target.Client.PostWithAuth(target.TenantAuth, "/auth/switch-tenant", map[string]any{"tenant_slug": primary.Bootstrap.TenantSlug})
	var denied *APIError
	if !errors.As(err, &denied) || (denied.StatusCode != 401 && denied.StatusCode != 403) {
		return fmt.Errorf("isolated school admin switch must be denied, got %v", err)
	}
	return nil
}
