package api

import (
	"context"
	"encoding/json"
	"fmt"
)

const enrollmentBookingsProfileKey = "anmeldung-buchungen"

func enrollmentBookingsProfileDefinition() demoProfileDefinition {
	definition := enrollmentWeeklyProfileDefinition()
	definition.Key = enrollmentBookingsProfileKey
	definition.SchoolSlug = enrollmentBookingsProfileKey
	definition.SchoolName = "Demo-Schule Anmeldung und Buchungen"
	definition.SchoolAdminEmail = "anmeldung-buchungen-admin@example.test"
	definition.SchoolAdminPassword = "Buchungen1234%"
	definition.Settings[profileSettingPresenceMode] = SeedSetting{Value: json.RawMessage(`"binary"`), ManagedBy: SettingManagedByOperator}
	definition.Settings[profileSettingAttendanceNFC] = SeedSetting{Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator}
	definition.Settings[profileSettingGroupMode] = SeedSetting{Value: json.RawMessage(`"open_care"`), ManagedBy: SettingManagedByTenant}
	definition.Settings[profileSettingBookingsAuthoritative] = SeedSetting{Value: json.RawMessage(`true`), ManagedBy: SettingManagedByOperator}
	definition.Settings[profileSettingEnrollmentEnabled] = SeedSetting{Value: json.RawMessage(`false`), ManagedBy: SettingManagedByTenant}
	definition.Expected.PhysicalDevices = 1
	definition.Expected.HasAttendance = true
	definition.Expected.HasHistory = true
	return definition
}

type seedEnrollmentBookingsProfileStep struct{ seeder *Seeder }

func (seedEnrollmentBookingsProfileStep) Name() string { return "Seeding anmeldung-buchungen profile" }

func (s seedEnrollmentBookingsProfileStep) Run(ctx context.Context, primary *Runtime) error {
	weekly, ok := primary.Values[enrollmentWeeklyProfileKey].(*SeedState)
	if !ok {
		return fmt.Errorf("%s must be seeded first", enrollmentWeeklyProfileKey)
	}
	child := *s.seeder
	child.profile, child.definition = enrollmentBookingsProfileKey, enrollmentBookingsProfileDefinition()
	child.options.TenantSlug, child.options.AdminEmail = "", ""
	rt := newRuntime(&child, primary.OperatorEmail, primary.OperatorPassword, primary.StaffPIN)
	rt.SetOperatorAuth(primary.OperatorAuth)
	rt.Bootstrap = &bootstrapSeedState{OrganizationID: weekly.Bootstrap.OrganizationID, OrganizationName: weekly.Bootstrap.OrganizationName, OrganizationSlug: weekly.Bootstrap.OrganizationSlug}
	defer primary.SetTenantAuth(primary.TenantAuth)
	bootstrap, auth, err := child.bootstrapManualProfile(ctx, rt, child.definition)
	if err != nil {
		return err
	}
	rt.Bootstrap = bootstrap
	rt.SetTenantAuth(auth)
	initial := child.definition
	initial.Settings = cloneProfileSettings(initial.Settings)
	initial.Settings[profileSettingBookingsAuthoritative] = SeedSetting{Value: json.RawMessage(`false`), ManagedBy: SettingManagedByOperator}
	initial.Settings[profileSettingEnrollmentEnabled] = SeedSetting{Value: json.RawMessage(`true`), ManagedBy: SettingManagedByTenant}
	if err := (configureProfileStep{definition: initial}).Run(ctx, rt); err != nil {
		return err
	}
	if err := verifyProfileSettings(rt, initial); err != nil {
		return err
	}
	state, err := seedWeeklyProfileContents(ctx, rt, &child)
	if err != nil {
		return child.formatProfileError(child.profile, "approved bookings", err)
	}
	if err := enableSeedBookingAuthority(rt, bootstrap.SchoolID); err != nil {
		return err
	}
	if err := verifyBookingProfileCare(ctx, rt, state, 12); err != nil {
		return err
	}
	if err := seedCareWithdrawalStates(rt, state.Enrollment.Requests[9:12]); err != nil {
		return err
	}
	for index, key := range []string{"abschluss-geplant", "abschluss-faellig", "abschluss-erledigt"} {
		state.Enrollment.Requests[9+index].Key = key
		state.Students[9+index].Key = key
	}
	if _, err := rt.Client.Put("/api/settings/values/"+profileSettingEnrollmentEnabled, map[string]any{"value": false}); err != nil {
		return err
	}
	// Closed enrollment must not disable the already approved care bookings.
	if err := verifyBookingProfileCare(ctx, rt, state, 9); err != nil {
		return err
	}
	state.Enrollment.Settings[profileSettingEnrollmentEnabled] = json.RawMessage(`false`)
	if err := verifyBookingWithdrawals(rt, state); err != nil {
		return err
	}
	if err := seedBookingProfileAttendance(rt, state); err != nil {
		return err
	}
	if err := linkDeveloperAdmin(ctx, primary, rt, state); err != nil {
		return err
	}
	if err := verifyProfileSettings(rt, child.definition); err != nil {
		return err
	}
	state.Scenarios = SeedStateScenarios{DefaultPlayer: "web", DefaultMode: "binary"}
	state.Normalize()
	primary.Values[enrollmentBookingsProfileKey] = state
	fmt.Printf("Verified profile %s: organization %s (%d), school %s (%d), admin %s / %s, phase %d\n", child.profile, bootstrap.OrganizationSlug, bootstrap.OrganizationID, bootstrap.TenantSlug, bootstrap.SchoolID, bootstrap.AdminEmail, bootstrap.AdminPassword, state.Enrollment.PhaseID)
	return nil
}

func verifyBookingProfileCare(ctx context.Context, rt *Runtime, state *SeedState, count int) error {
	raw, err := rt.Client.Get("/api/students/arrival-settings")
	if err != nil {
		return err
	}
	var source struct {
		Data struct {
			Source string `json:"care_days_source"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &source); err != nil {
		return err
	}
	if source.Data.Source != "bookings" {
		return fmt.Errorf("care days are not booking-led")
	}
	// The first nine children retain active bookings; the final three exercise
	// withdrawal states, including a child with no current care days.
	for index := range count {
		parent := state.Parents[index]
		auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
		if err != nil {
			return err
		}
		if err := verifyEnrollmentProfileCare(rt, auth, state.Students[index].ID, state.Enrollment.Offerings["wochenangebot"], true); err != nil {
			return err
		}
	}
	return nil
}

func verifyBookingWithdrawals(rt *Runtime, state *SeedState) error {
	for index, expected := range []struct{ state, urgency, outcome string }{
		{"pending", "planned", ""}, {"pending", "overdue", ""}, {"resolved", "planned", "care_ended"},
	} {
		student := state.Students[9+index]
		raw, err := rt.Client.Get(fmt.Sprintf("/api/students/care-withdrawals?student_id=%d&state=%s", student.ID, expected.state))
		if err != nil {
			return err
		}
		var response struct {
			Data struct {
				Items []struct {
					StudentID string `json:"student_id"`
					State     string `json:"state"`
					Urgency   string `json:"urgency"`
					Outcome   string `json:"outcome"`
				} `json:"items"`
			} `json:"data"`
		}
		if err := parseJSON(raw, &response); err != nil {
			return err
		}
		if len(response.Data.Items) != 1 {
			return fmt.Errorf("%s: expected one withdrawal", student.Key)
		}
		row := response.Data.Items[0]
		if row.StudentID != fmt.Sprint(student.ID) || row.State != expected.state || row.Urgency != expected.urgency || row.Outcome != expected.outcome {
			return fmt.Errorf("%s: withdrawal state differs from %s/%s/%s", student.Key, expected.state, expected.urgency, expected.outcome)
		}
	}
	return nil
}

func seedBookingProfileAttendance(rt *Runtime, state *SeedState) error {
	if _, err := rt.Client.Put("/api/settings/values/security.ogs_device_pin", map[string]any{"value": rt.StaffPIN}); err != nil {
		return err
	}
	raw, err := rt.Client.Post("/api/iot/", map[string]any{"device_id": "BUCHUNGEN-NFC-001", "name": "Eingang", "device_type": "terminal", "status": "active"})
	if err != nil {
		return err
	}
	var response struct {
		Data SeedDevice `json:"data"`
	}
	if err := parseJSON(raw, &response); err != nil {
		return err
	}
	device := response.Data
	if device.ID <= 0 || device.APIKey == "" {
		return fmt.Errorf("physical terminal returned no ID or API key")
	}
	device.DeviceID, device.DeviceType = "BUCHUNGEN-NFC-001", "terminal"
	state.Devices[device.DeviceID] = device
	raw, err = rt.Client.DeviceGet("/api/iot/config", device.APIKey, rt.StaffPIN)
	if err != nil {
		return err
	}
	var config struct {
		Data struct {
			PresenceMode string `json:"presence_mode"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &config); err != nil {
		return err
	}
	if config.Data.PresenceMode != "binary" {
		return fmt.Errorf("terminal configuration is not binary")
	}
	if err := seedBookingTerminalAttendance(rt, state.Students[8].ID, device); err != nil {
		return err
	}
	students := make(map[string]SeedStudent)
	for _, student := range state.Students[:9] {
		students[student.Key] = student
	}
	if err := seedManualAttendance(rt, students); err != nil {
		return err
	}
	if err := verifyManualVisits(rt); err != nil {
		return err
	}
	if err := verifyManualVisitHistories(rt, students); err != nil {
		return err
	}
	virtual, err := verifyProfileDevices(rt, 1)
	if err != nil {
		return err
	}
	state.Devices[virtual.DeviceID] = virtual
	return nil
}

func seedBookingTerminalAttendance(rt *Runtime, studentID int64, device SeedDevice) error {
	rfid := fmt.Sprintf("B00C%08X", studentID)
	if _, err := rt.Client.DevicePost(fmt.Sprintf("/api/students/%d/rfid", studentID), map[string]string{"rfid_tag": rfid}, device.APIKey, rt.StaffPIN); err != nil {
		return err
	}
	for _, action := range []string{"checked_in", "checked_out"} {
		raw, err := rt.Client.DevicePost("/api/iot/checkin", map[string]any{"student_rfid": rfid, "action": "checkin"}, device.APIKey, rt.StaffPIN)
		if err != nil {
			return err
		}
		var response struct {
			Data struct {
				Action string `json:"action"`
			} `json:"data"`
		}
		if err := parseJSON(raw, &response); err != nil {
			return err
		}
		if response.Data.Action != action {
			return fmt.Errorf("binary NFC attendance: expected %s, got %s", action, response.Data.Action)
		}
	}
	return nil
}
