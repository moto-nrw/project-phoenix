package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	sharedDeveloperAdminKey = "entwickler-admin"
	manualAdminPosition     = "OGS-Büro"
)

type manualProfileStep struct {
	seeder *Seeder
}

func (manualProfileStep) Name() string { return "Seeding manual web-attendance profile" }

func (s manualProfileStep) Run(ctx context.Context, rt *Runtime) error {
	profile, err := s.seeder.seedManualProfile(ctx, rt)
	if err != nil {
		return err
	}
	rt.AdditionalProfiles[profile.Key] = profile
	return nil
}

type manualProfileData struct {
	students  map[string]SeedStudent
	guardians map[string]SeedEntityRef
	groups    map[string]SeedEntityRef
}

func (s *Seeder) seedManualProfile(ctx context.Context, rt *Runtime) (*SeedProfile, error) {
	if rt.Bootstrap == nil || rt.FixedSeeder == nil {
		return nil, fmt.Errorf("full-operation profile must be seeded first")
	}
	fullBootstrap, fullAuth := rt.Bootstrap, rt.TenantAuth
	defer func() {
		rt.Bootstrap, rt.TenantAuth = fullBootstrap, fullAuth
		rt.Client.BindAuth(fullAuth)
	}()

	definition := manualProfileDefinition()
	bootstrap, adminAuth, err := s.bootstrapManualProfile(ctx, rt, definition)
	if err != nil {
		return nil, err
	}
	rt.Bootstrap, rt.TenantAuth = bootstrap, adminAuth
	rt.Client.BindAuth(adminAuth)

	if err := (configureProfileStep{definition: definition}).Run(ctx, rt); err != nil {
		return nil, err
	}
	sharedAdmin, err := grantSharedDeveloperAdmin(rt, bootstrap)
	if err != nil {
		return nil, err
	}
	data, err := seedManualProfileData(rt)
	if err != nil {
		return nil, err
	}
	virtualDevice, err := verifyManualProfile(ctx, rt, definition, data, fullBootstrap, sharedAdmin)
	if err != nil {
		return nil, err
	}
	return buildManualSeedProfile(rt, definition, bootstrap, sharedAdmin, virtualDevice, data), nil
}

func (s *Seeder) bootstrapManualProfile(ctx context.Context, rt *Runtime, definition demoProfileDefinition) (*bootstrapSeedState, AuthRef, error) {
	rt.SetOperatorAuth(rt.OperatorAuth)
	name, slug := definition.SchoolName, definition.SchoolSlug
	if s.options.Randomize {
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		name = fmt.Sprintf("%s %s", name, suffix)
		slug = truncateSeedSubdomain(fmt.Sprintf("%s-%s", slug, suffix))
	}
	schoolID, tenantSlug, err := s.createSeedSchool(rt.Bootstrap.OrganizationID, name, slug, slug)
	if err != nil {
		return nil, AuthRef{}, wrapConflictError(err, "school")
	}
	email, password, err := s.profileAdminCredentialsFor(definition)
	if err != nil {
		return nil, AuthRef{}, err
	}
	if err := s.inviteSeedAdmin(schoolID, email, password, manualAdminPosition); err != nil {
		return nil, AuthRef{}, err
	}
	auth, err := rt.Adapter.LoginTenant(ctx, email, password, tenantSlug)
	if err != nil {
		return nil, AuthRef{}, fmt.Errorf("login as manual profile admin: %w", err)
	}
	return &bootstrapSeedState{
		OrganizationID: rt.Bootstrap.OrganizationID, OrganizationName: rt.Bootstrap.OrganizationName,
		OrganizationSlug: rt.Bootstrap.OrganizationSlug, SchoolID: schoolID, SchoolName: name,
		SchoolSlug: slug, TenantSlug: tenantSlug, AdminEmail: email, AdminPassword: password,
		AdminName: "Seed Admin", AdminPosition: manualAdminPosition,
	}, auth, nil
}

func grantSharedDeveloperAdmin(rt *Runtime, manual *bootstrapSeedState) (AccountCredentials, error) {
	credential, member, err := sharedDeveloperAdmin(rt.FixedSeeder)
	if err != nil {
		return AccountCredentials{}, err
	}
	roleID := rt.FixedSeeder.roleIDs["admin"]
	if roleID <= 0 {
		return AccountCredentials{}, fmt.Errorf("shared developer admin role not available")
	}
	path := fmt.Sprintf("/operator/accounts/%d/tenants", credential.AccountID)
	body := map[string]any{
		"school_id": manual.SchoolID, "role_id": roleID,
		"first_name": member.FirstName, "last_name": member.LastName, "position": member.Position,
	}
	if _, err := rt.Client.PostWithAuth(rt.OperatorAuth, path, body); err != nil {
		return AccountCredentials{}, fmt.Errorf("grant shared developer admin access: %w", err)
	}
	return credential, nil
}

func sharedDeveloperAdmin(fs *FixedSeeder) (AccountCredentials, DemoStaffMember, error) {
	if fs == nil || len(DemoStaff) == 0 {
		return AccountCredentials{}, DemoStaffMember{}, fmt.Errorf("shared developer admin is unavailable")
	}
	member := DemoStaff[0]
	name := member.FirstName + " " + member.LastName
	accountID := fs.accountIDs[name]
	if accountID <= 0 || len(fs.staffCredentials) == 0 {
		return AccountCredentials{}, DemoStaffMember{}, fmt.Errorf("shared developer admin account is unavailable")
	}
	created := fs.staffCredentials[0]
	return AccountCredentials{
		Key: sharedDeveloperAdminKey, AccountID: accountID, Email: created.Email,
		Password: created.Password, PIN: created.PIN, Name: name, StaffID: fs.staffIDs[name],
		TeacherID: fs.teacherIDs[name],
	}, member, nil
}

func verifyTenantAccess(ctx context.Context, rt *Runtime, credential AccountCredentials, full, manual *bootstrapSeedState) error {
	fullAuth, err := rt.Adapter.LoginTenant(ctx, credential.Email, credential.Password, full.TenantSlug)
	if err != nil {
		return fmt.Errorf("login shared developer admin: %w", err)
	}
	if err := verifySharedTenantList(rt, fullAuth, full.TenantSlug, manual.TenantSlug); err != nil {
		return err
	}
	manualAuth, err := switchSeedTenant(rt, fullAuth, manual.TenantSlug)
	if err != nil {
		return fmt.Errorf("switch shared developer admin to %s: %w", manual.TenantSlug, err)
	}
	if _, err := switchSeedTenant(rt, manualAuth, full.TenantSlug); err != nil {
		return fmt.Errorf("switch shared developer admin to %s: %w", full.TenantSlug, err)
	}
	if err := expectTenantSwitchDenied(rt, rt.TenantAuth, full.TenantSlug); err != nil {
		return fmt.Errorf("manual school admin isolation: %w", err)
	}
	fullAdminAuth, err := rt.Adapter.LoginTenant(ctx, full.AdminEmail, full.AdminPassword, full.TenantSlug)
	if err != nil {
		return fmt.Errorf("login full-operation school admin: %w", err)
	}
	if err := expectTenantSwitchDenied(rt, fullAdminAuth, manual.TenantSlug); err != nil {
		return fmt.Errorf("full-operation school admin isolation: %w", err)
	}
	return nil
}

func verifySharedTenantList(rt *Runtime, auth AuthRef, slugs ...string) error {
	raw, err := rt.Client.GetWithAuth(auth, "/auth/account/tenants")
	if err != nil {
		return fmt.Errorf("read shared developer admin schools: %w", err)
	}
	var envelope struct {
		Data []struct {
			Slug string `json:"slug"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode shared developer admin schools: %w", err)
	}
	available := make(map[string]bool, len(envelope.Data))
	for _, school := range envelope.Data {
		available[school.Slug] = true
	}
	for _, slug := range slugs {
		if !available[slug] {
			return fmt.Errorf("shared developer admin cannot access school %s", slug)
		}
	}
	return nil
}

func switchSeedTenant(rt *Runtime, auth AuthRef, slug string) (AuthRef, error) {
	raw, err := rt.Client.PostWithAuth(auth, "/auth/switch-tenant", map[string]any{"tenant_slug": slug})
	if err != nil {
		return AuthRef{}, err
	}
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return AuthRef{}, fmt.Errorf("decode tenant switch: %w", err)
	}
	if tokens.AccessToken == "" {
		return AuthRef{}, fmt.Errorf("tenant switch returned no access token")
	}
	return AuthRef{Kind: AuthBearer, Label: slug, Token: tokens.AccessToken}, nil
}

func expectTenantSwitchDenied(rt *Runtime, auth AuthRef, slug string) error {
	_, err := rt.Client.PostWithAuth(auth, "/auth/switch-tenant", map[string]any{"tenant_slug": slug})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
		return fmt.Errorf("switch to %s: expected 401, got %v", slug, err)
	}
	return nil
}

func seedManualProfileData(rt *Runtime) (manualProfileData, error) {
	groups, err := seedManualGroups(rt)
	if err != nil {
		return manualProfileData{}, err
	}
	students, guardians, err := seedManualStudents(rt, groups)
	if err != nil {
		return manualProfileData{}, err
	}
	if err := seedManualAttendance(rt, students); err != nil {
		return manualProfileData{}, err
	}
	return manualProfileData{students: students, guardians: guardians, groups: groups}, nil
}

func seedManualGroups(rt *Runtime) (map[string]SeedEntityRef, error) {
	groups := make(map[string]SeedEntityRef, 2)
	for index := 1; index <= 2; index++ {
		name := fmt.Sprintf("Bezugsgruppe %d", index)
		raw, err := rt.Client.Post("/api/groups", map[string]any{"name": name, "teacher_ids": []int64{}})
		if err != nil {
			return nil, fmt.Errorf("create manual profile group %s: %w", name, err)
		}
		id, err := decodeSeedEntityID(raw)
		if err != nil {
			return nil, fmt.Errorf("decode manual profile group %s: %w", name, err)
		}
		groups[fmt.Sprintf("bezugsgruppe-%d", index)] = SeedEntityRef{ID: id, Name: name}
	}
	return groups, nil
}

func seedManualStudents(rt *Runtime, groups map[string]SeedEntityRef) (map[string]SeedStudent, map[string]SeedEntityRef, error) {
	if len(DemoStudents) < 12 || len(DemoGuardians) < 12 {
		return nil, nil, fmt.Errorf("manual profile requires 12 demo students and contacts")
	}
	students := make(map[string]SeedStudent, 12)
	guardians := make(map[string]SeedEntityRef, 12)
	for index, source := range DemoStudents[:12] {
		groupKey := fmt.Sprintf("bezugsgruppe-%d", 1+index/6)
		group := groups[groupKey]
		body := manualStudentPayload(index, source, group.ID)
		raw, err := rt.Client.Post("/api/students", body)
		if err != nil {
			return nil, nil, fmt.Errorf("create manual profile student %d: %w", index+1, err)
		}
		id, err := decodeSeedEntityID(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("decode manual profile student %d: %w", index+1, err)
		}
		key := semanticKey(source.FirstName + " " + source.LastName)
		students[key] = SeedStudent{
			Key: key, ID: id, FirstName: source.FirstName, LastName: source.LastName,
			GroupKey: groupKey, Class: source.Class,
		}
		guardian, err := readManualGuardian(rt, id, DemoGuardians[index])
		if err != nil {
			return nil, nil, err
		}
		guardians[fmt.Sprintf("kontakt-%02d", index+1)] = guardian
	}
	return students, guardians, nil
}

func manualStudentPayload(index int, student DemoStudent, groupID int64) map[string]any {
	arrival, pickup := manualWeeklySchedules(index)
	contact := DemoGuardians[index]
	phone := contact.MobilePhone
	if phone == "" {
		phone = contact.Phone
	}
	return map[string]any{
		"first_name": student.FirstName, "last_name": student.LastName,
		"school_class": student.Class, "group_id": groupID,
		"birthday":      fmt.Sprintf("2018-%02d-%02d", index%12+1, index%28+1),
		"pickup_status": "Wird abgeholt", "arrival_schedules": arrival, "pickup_schedules": pickup,
		"guardians": []map[string]any{{
			"first_name": contact.FirstName, "last_name": contact.LastName,
			"email":             fmt.Sprintf("manual-contact-%02d@example.test", index+1),
			"relationship_type": "parent", "is_primary": true,
			"is_emergency_contact": true, "can_pickup": true, "emergency_priority": 1,
			"phone_numbers": []map[string]any{{"phone_number": phone, "phone_type": "mobile", "is_primary": true}},
		}},
	}
}

func manualWeeklySchedules(index int) ([]map[string]any, []map[string]any) {
	arrival := make([]map[string]any, 0, 5)
	pickup := make([]map[string]any, 0, 5)
	for weekday := 1; weekday <= 5; weekday++ {
		arrival = append(arrival, map[string]any{
			"weekday": weekday, "expected_arrival": fmt.Sprintf("%02d:45", 11+(index+weekday)%2),
		})
		pickup = append(pickup, map[string]any{
			"weekday": weekday, "pickup_time": fmt.Sprintf("%02d:30", 15+(index+weekday)%2),
		})
	}
	return arrival, pickup
}

func readManualGuardian(rt *Runtime, studentID int64, source DemoGuardian) (SeedEntityRef, error) {
	path := fmt.Sprintf("/api/guardians/students/%d/guardians", studentID)
	raw, err := rt.Client.Get(path)
	if err != nil {
		return SeedEntityRef{}, fmt.Errorf("read contact for student %d: %w", studentID, err)
	}
	var envelope struct {
		Data []struct {
			Guardian struct {
				ID int64 `json:"id"`
			} `json:"guardian"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data) != 1 || envelope.Data[0].Guardian.ID <= 0 {
		return SeedEntityRef{}, fmt.Errorf("student %d does not have exactly one persisted contact", studentID)
	}
	return SeedEntityRef{ID: envelope.Data[0].Guardian.ID, Name: source.FirstName + " " + source.LastName}, nil
}

func decodeSeedEntityID(raw []byte) (int64, error) {
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, err
	}
	if envelope.Data.ID <= 0 {
		return 0, fmt.Errorf("response contains no entity id")
	}
	return envelope.Data.ID, nil
}

func seedManualAttendance(rt *Runtime, students map[string]SeedStudent) error {
	keys := sortedManualStudentKeys(students)
	if len(keys) < 8 {
		return fmt.Errorf("manual attendance requires 8 students, got %d", len(keys))
	}
	for index, key := range keys[:8] {
		student := students[key]
		if err := postSchoolAttendance(rt, student.ID, "in", "checked_in"); err != nil {
			return err
		}
		if index >= 4 {
			if err := postSchoolAttendance(rt, student.ID, "out", "checked_out"); err != nil {
				return err
			}
		}
	}
	return nil
}

func postSchoolAttendance(rt *Runtime, studentID int64, action, expectedStatus string) error {
	path := fmt.Sprintf("/api/students/%d/school-checkin", studentID)
	raw, err := rt.Client.Post(path, map[string]any{"action": action})
	if err != nil {
		return fmt.Errorf("record web attendance for student %d: %w", studentID, err)
	}
	var envelope struct {
		Data struct {
			Status  string `json:"status"`
			Changed bool   `json:"changed"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode web attendance for student %d: %w", studentID, err)
	}
	if envelope.Data.Status != expectedStatus || !envelope.Data.Changed {
		return fmt.Errorf("web attendance for student %d: expected changed %s, got changed=%t status=%s", studentID, expectedStatus, envelope.Data.Changed, envelope.Data.Status)
	}
	return nil
}

func sortedManualStudentKeys(students map[string]SeedStudent) []string {
	keys := make([]string, 0, len(students))
	for key := range students {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func verifyManualProfile(ctx context.Context, rt *Runtime, definition demoProfileDefinition, data manualProfileData, full *bootstrapSeedState, shared AccountCredentials) (SeedDevice, error) {
	if err := verifyProfileSettings(rt, definition); err != nil {
		return SeedDevice{}, err
	}
	virtual, err := verifyManualDevices(rt)
	if err != nil {
		return SeedDevice{}, err
	}
	if err := verifyManualStudents(rt, definition.Expected, data); err != nil {
		return SeedDevice{}, err
	}
	if err := verifyManualStaff(rt, definition.Expected.Staff); err != nil {
		return SeedDevice{}, err
	}
	if err := verifyManualVisits(rt); err != nil {
		return SeedDevice{}, err
	}
	if err := verifyManualVisitHistories(rt, data.students); err != nil {
		return SeedDevice{}, err
	}
	if err := verifyTenantAccess(ctx, rt, shared, full, rt.Bootstrap); err != nil {
		return SeedDevice{}, err
	}
	fmt.Printf("Verified API contract for profile %s\n", definition.Key)
	return virtual, nil
}

func verifyManualDevices(rt *Runtime) (SeedDevice, error) {
	physical, err := listSeedDevices(rt, "terminal")
	if err != nil {
		return SeedDevice{}, fmt.Errorf("read manual profile physical devices: %w", err)
	}
	if len(physical) != 0 {
		return SeedDevice{}, fmt.Errorf("manual profile physical devices: expected 0, got %d", len(physical))
	}
	virtual, err := listSeedDevices(rt, "virtual")
	if err != nil {
		return SeedDevice{}, fmt.Errorf("read manual profile virtual devices: %w", err)
	}
	for _, device := range virtual {
		if device.DeviceID != webManualDeviceID || device.DeviceType != "virtual" {
			continue
		}
		if err := verifyProtectedDevice(rt, device.ID); err != nil {
			return SeedDevice{}, err
		}
		device.Protected = true
		return device, nil
	}
	return SeedDevice{}, fmt.Errorf("virtual web device %s missing", webManualDeviceID)
}

func verifyProtectedDevice(rt *Runtime, deviceID int64) error {
	if deviceID <= 0 {
		return fmt.Errorf("virtual web device has no id")
	}
	path := fmt.Sprintf("/operator/devices/%d/transfer-status", deviceID)
	raw, err := rt.Client.GetWithAuth(rt.OperatorAuth, path)
	if err != nil {
		return fmt.Errorf("read virtual web device protection: %w", err)
	}
	var envelope struct {
		Data struct {
			IsProtected bool `json:"is_protected"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode virtual web device protection: %w", err)
	}
	if !envelope.Data.IsProtected {
		return fmt.Errorf("virtual web device %s is not protected", webManualDeviceID)
	}
	return nil
}

func verifyManualStudents(rt *Runtime, expected SeedExpectedState, data manualProfileData) error {
	raw, err := rt.Client.Get("/api/students?page=1&page_size=100")
	if err != nil {
		return fmt.Errorf("read manual profile students: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID             int64   `json:"id"`
			SchoolClass    string  `json:"school_class"`
			GroupID        int64   `json:"group_id"`
			Location       string  `json:"current_location"`
			ActualPickupAt *string `json:"actual_pickup_time"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode manual profile students: %w", err)
	}
	if len(envelope.Data) != expected.Students {
		return fmt.Errorf("manual profile students: expected %d, got %d", expected.Students, len(envelope.Data))
	}
	if err := verifyManualStudentRows(envelope.Data, expected); err != nil {
		return err
	}
	return verifyManualContactsAndPlans(rt, data)
}

func verifyManualStudentRows(rows []struct {
	ID             int64   `json:"id"`
	SchoolClass    string  `json:"school_class"`
	GroupID        int64   `json:"group_id"`
	Location       string  `json:"current_location"`
	ActualPickupAt *string `json:"actual_pickup_time"`
}, expected SeedExpectedState) error {
	present, checkedOut := 0, 0
	for _, student := range rows {
		if strings.TrimSpace(student.SchoolClass) == "" || student.GroupID <= 0 {
			return fmt.Errorf("manual profile student %d has no class or group reference", student.ID)
		}
		switch student.Location {
		case "Anwesend":
			present++
		case "Abwesend":
			if student.ActualPickupAt != nil {
				checkedOut++
			}
		default:
			return fmt.Errorf("manual profile student %d has room-tracking location %q", student.ID, student.Location)
		}
	}
	if present != expected.PresentStudents || checkedOut != expected.CheckedOutStudents {
		return fmt.Errorf("manual attendance: expected %d present and %d checked out, got %d and %d", expected.PresentStudents, expected.CheckedOutStudents, present, checkedOut)
	}
	return nil
}

func verifyManualStaff(rt *Runtime, expected int) error {
	raw, err := rt.Client.Get("/api/staff/")
	if err != nil {
		return fmt.Errorf("read manual profile staff: %w", err)
	}
	var envelope struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode manual profile staff: %w", err)
	}
	if len(envelope.Data) != expected {
		return fmt.Errorf("manual profile staff: expected %d, got %d", expected, len(envelope.Data))
	}
	return nil
}

func verifyManualContactsAndPlans(rt *Runtime, data manualProfileData) error {
	for _, key := range sortedManualStudentKeys(data.students) {
		student := data.students[key]
		if err := verifyManualCollection(rt, fmt.Sprintf("/api/guardians/students/%d/guardians", student.ID), "guardian", student.ID); err != nil {
			return err
		}
		if err := verifyManualSchedule(rt, fmt.Sprintf("/api/students/%d/arrival-schedules", student.ID), "arrival", student.ID); err != nil {
			return err
		}
		if err := verifyManualSchedule(rt, fmt.Sprintf("/api/students/%d/pickup-schedules", student.ID), "pickup", student.ID); err != nil {
			return err
		}
	}
	raw, err := rt.Client.Get("/api/students/arrival-settings")
	if err != nil {
		return fmt.Errorf("read care-day source: %w", err)
	}
	var settings struct {
		Data struct {
			CareDaysSource string `json:"care_days_source"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return fmt.Errorf("decode care-day source: %w", err)
	}
	if settings.Data.CareDaysSource != "weekly_plan" {
		return fmt.Errorf("manual profile care days are not weekly-plan-driven")
	}
	return nil
}

func verifyManualCollection(rt *Runtime, path, kind string, studentID int64) error {
	raw, err := rt.Client.Get(path)
	if err != nil {
		return fmt.Errorf("read %s for student %d: %w", kind, studentID, err)
	}
	var envelope struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode %s for student %d: %w", kind, studentID, err)
	}
	if len(envelope.Data) != 1 {
		return fmt.Errorf("student %d: expected one %s, got %d", studentID, kind, len(envelope.Data))
	}
	return nil
}

func verifyManualSchedule(rt *Runtime, path, kind string, studentID int64) error {
	raw, err := rt.Client.Get(path)
	if err != nil {
		return fmt.Errorf("read %s plan for student %d: %w", kind, studentID, err)
	}
	var envelope struct {
		Data struct {
			Schedules []json.RawMessage `json:"schedules"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode %s plan for student %d: %w", kind, studentID, err)
	}
	if len(envelope.Data.Schedules) != 5 {
		return fmt.Errorf("student %d: expected five-day %s plan, got %d days", studentID, kind, len(envelope.Data.Schedules))
	}
	return nil
}

func verifyManualVisits(rt *Runtime) error {
	raw, err := rt.Client.Get("/api/active/visits")
	if err != nil {
		return fmt.Errorf("read manual profile room visits: %w", err)
	}
	var envelope struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode manual profile room visits: %w", err)
	}
	if len(envelope.Data) != 0 {
		return fmt.Errorf("manual profile must not contain room visits, got %d", len(envelope.Data))
	}
	return nil
}

func verifyManualVisitHistories(rt *Runtime, students map[string]SeedStudent) error {
	for _, key := range sortedManualStudentKeys(students) {
		student := students[key]
		path := fmt.Sprintf("/api/students/%d/visit-history", student.ID)
		raw, err := rt.Client.Get(path)
		if err != nil {
			return fmt.Errorf("read room history for student %d: %w", student.ID, err)
		}
		var envelope struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return fmt.Errorf("decode room history for student %d: %w", student.ID, err)
		}
		if len(envelope.Data) != 0 {
			return fmt.Errorf("student %d has %d room-history entries", student.ID, len(envelope.Data))
		}
	}
	return nil
}

func buildManualSeedProfile(rt *Runtime, definition demoProfileDefinition, bootstrap *bootstrapSeedState, shared AccountCredentials, virtual SeedDevice, data manualProfileData) *SeedProfile {
	return &SeedProfile{
		Key: definition.Key, Name: bootstrap.SchoolName,
		Organization: SeedOrganizationRef{
			ID: bootstrap.OrganizationID, Name: bootstrap.OrganizationName, Slug: bootstrap.OrganizationSlug,
		},
		School: SeedSchoolRef{
			ID: bootstrap.SchoolID, Name: bootstrap.SchoolName,
			Slug: bootstrap.SchoolSlug, TenantSlug: bootstrap.TenantSlug,
		},
		Settings: cloneProfileSettings(definition.Settings),
		Credentials: SeedStateCredentials{
			Operator:    &SeedOperatorCredentials{Email: rt.OperatorEmail, Password: rt.OperatorPassword},
			SchoolAdmin: makeBootstrapSeedState(bootstrap).SchoolAdmin,
			DevicePIN:   rt.StaffPIN, Accounts: SeedStateAccounts{Admin: []AccountCredentials{shared}},
		},
		Devices: map[string]SeedDevice{"web-anwesenheit": virtual},
		Entities: SeedProfileEntities{
			Students: data.students, Guardians: data.guardians, Groups: data.groups,
			Rooms: map[string]SeedEntityRef{}, Activities: map[string]SeedEntityRef{},
		},
		Expected:  definition.Expected,
		Scenarios: SeedStateScenarios{DefaultPlayer: "web", DefaultMode: "binary"},
	}
}
