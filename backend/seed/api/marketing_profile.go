package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"
)

const (
	marketingProfileKey = "marketing"
	// marketingReferenceClock is the Berlin wall-clock time the product
	// screenshots show. The screenshot pipeline pins the browser clock to it
	// on a weekday. Every weekly-plan time below is derived from it: children
	// seeded as present arrive before it, children not yet arrived are
	// expected after it, and pickups fall in the afternoon. All five weekdays
	// share the plan, so the picture is consistent on any weekday.
	//
	// Live presence cannot follow this clock: the server stamps check-ins
	// and check-outs with its own time when the seed runs, and there is no way
	// to fake it through the API. The seed therefore records the presence
	// states through the web-attendance API at seed time; only the plan times
	// are relative to the reference clock.
	marketingReferenceClock = "10:15"

	marketingAdminFirstName = "Katrin"
	marketingAdminLastName  = "Brandt"
	marketingStaffPassword  = "Sonnenhang1234%"
)

// marketingPresence is the state a child shows at the reference clock.
type marketingPresence string

const (
	marketingPresent    marketingPresence = "present"     // checked in, picked up in the afternoon
	marketingPickedUp   marketingPresence = "picked_up"   // checked in and already picked up early
	marketingNotArrived marketingPresence = "not_arrived" // expected after the reference clock
)

type marketingChild struct {
	firstName string
	lastName  string
	class     string
	birthday  string
	group     string
	presence  marketingPresence
}

type marketingFamily struct {
	firstName     string
	lastName      string
	phone         string
	parentAccount bool
	children      []marketingChild
}

type marketingGroup struct {
	key    string
	name   string
	leader string
}

type marketingStaffMember struct {
	firstName string
	lastName  string
}

func marketingProfileDefinition() demoProfileDefinition {
	settings := fullOperationSettings()
	settings[profileSettingPresenceMode] = SeedSetting{Value: json.RawMessage(`"` + profilePresenceBinary + `"`), ManagedBy: SettingManagedByOperator}
	settings[profileSettingAttendanceNFC] = SeedSetting{Value: json.RawMessage(`false`), ManagedBy: SettingManagedByOperator}
	settings[profileSettingCareConcept] = SeedSetting{Value: json.RawMessage(`"` + profileCareConceptOpenRooms + `"`), ManagedBy: SettingManagedByTenant}
	settings[profileSettingEnrollmentEnabled] = SeedSetting{Value: json.RawMessage(`false`), ManagedBy: SettingManagedByTenant}
	settings[profileSettingCareOfferingsEnabled] = SeedSetting{Value: json.RawMessage(`false`), ManagedBy: SettingManagedByTenant}
	return demoProfileDefinition{
		Key: marketingProfileKey, OrganizationName: "Demo-Träger Marketing", OrganizationSlug: "demo-traeger-marketing",
		SchoolName: "OGS Sonnenhang", SchoolSlug: marketingProfileKey,
		SchoolAdminEmail: "marketing-admin@example.test", SchoolAdminPassword: "Marketing1234%",
		Settings: settings,
		Expected: SeedExpectedState{
			Students: 12, Groups: 2, Staff: 4, Contacts: 11, ParentAccounts: 4,
			HasAttendance: true, PresentStudents: 7, CheckedOutStudents: 2, WeeklyPlans: 12,
		},
	}
}

func marketingGroups() []marketingGroup {
	return []marketingGroup{
		{key: "fuchsbau", name: "Fuchsbau", leader: "Miriam Sommer"},
		{key: "eulennest", name: "Eulennest", leader: "Jonas Albrecht"},
	}
}

func marketingStaff() []marketingStaffMember {
	return []marketingStaffMember{{firstName: "Miriam", lastName: "Sommer"}, {firstName: "Jonas", lastName: "Albrecht"}}
}

// marketingFamilies lists one contact per family. The first family has two
// children, so one parent account shows two children in the parents portal.
func marketingFamilies() []marketingFamily {
	return []marketingFamily{
		{firstName: "Sarah", lastName: "Yilmaz", phone: "+49 151 23450101", parentAccount: true, children: []marketingChild{
			{firstName: "Emir", lastName: "Yilmaz", class: "Klasse 3a", birthday: "2017-03-14", group: "fuchsbau", presence: marketingPresent},
			{firstName: "Elif", lastName: "Yilmaz", class: "Klasse 1b", birthday: "2019-06-02", group: "eulennest", presence: marketingPresent},
		}},
		{firstName: "Thomas", lastName: "Becker", phone: "+49 151 23450102", parentAccount: true, children: []marketingChild{
			{firstName: "Lina", lastName: "Becker", class: "Klasse 2a", birthday: "2018-01-21", group: "fuchsbau", presence: marketingPresent},
		}},
		{firstName: "Anna", lastName: "Hoffmann", phone: "+49 151 23450103", parentAccount: true, children: []marketingChild{
			{firstName: "Paul", lastName: "Hoffmann", class: "Klasse 1a", birthday: "2019-09-09", group: "eulennest", presence: marketingNotArrived},
		}},
		{firstName: "Julia", lastName: "Wagner", phone: "+49 151 23450104", parentAccount: true, children: []marketingChild{
			{firstName: "Mia", lastName: "Wagner", class: "Klasse 4a", birthday: "2016-11-30", group: "fuchsbau", presence: marketingPickedUp},
		}},
		{firstName: "Markus", lastName: "Schulz", phone: "+49 151 23450105", children: []marketingChild{
			{firstName: "Ben", lastName: "Schulz", class: "Klasse 2b", birthday: "2018-04-17", group: "eulennest", presence: marketingPresent},
		}},
		{firstName: "Nadine", lastName: "Krüger", phone: "+49 151 23450106", children: []marketingChild{
			{firstName: "Emma", lastName: "Krüger", class: "Klasse 3b", birthday: "2017-07-25", group: "fuchsbau", presence: marketingPresent},
		}},
		{firstName: "Daniel", lastName: "Weber", phone: "+49 151 23450107", children: []marketingChild{
			{firstName: "Noah", lastName: "Weber", class: "Klasse 1a", birthday: "2019-02-11", group: "eulennest", presence: marketingPresent},
		}},
		{firstName: "Katharina", lastName: "Lehmann", phone: "+49 151 23450108", children: []marketingChild{
			{firstName: "Lea", lastName: "Lehmann", class: "Klasse 4b", birthday: "2016-05-08", group: "fuchsbau", presence: marketingNotArrived},
		}},
		{firstName: "Sven", lastName: "Hartmann", phone: "+49 151 23450109", children: []marketingChild{
			{firstName: "Finn", lastName: "Hartmann", class: "Klasse 2a", birthday: "2018-10-03", group: "eulennest", presence: marketingPresent},
		}},
		{firstName: "Aylin", lastName: "Demir", phone: "+49 151 23450110", children: []marketingChild{
			{firstName: "Mira", lastName: "Demir", class: "Klasse 3a", birthday: "2017-12-19", group: "fuchsbau", presence: marketingPickedUp},
		}},
		{firstName: "Christina", lastName: "Neumann", phone: "+49 151 23450111", children: []marketingChild{
			{firstName: "Theo", lastName: "Neumann", class: "Klasse 1b", birthday: "2019-08-27", group: "eulennest", presence: marketingNotArrived},
		}},
	}
}

func marketingContactEmail(family marketingFamily) string {
	return emailLocalPart(family.firstName, family.lastName) + "@example.test"
}

// marketingClock returns the reference clock shifted by offset minutes.
func marketingClock(offsetMinutes int) string {
	reference, err := time.Parse("15:04", marketingReferenceClock)
	if err != nil {
		panic(fmt.Sprintf("invalid marketing reference clock %q: %v", marketingReferenceClock, err))
	}
	return reference.Add(time.Duration(offsetMinutes) * time.Minute).Format("15:04")
}

// marketingPlanTimes derives a child's expected arrival and pickup from the
// reference clock. Present children alternate between two afternoon pickups.
func marketingPlanTimes(presence marketingPresence, index int) (arrival, pickup string) {
	switch presence {
	case marketingPickedUp:
		return marketingClock(-150), marketingClock(-15) // 07:45, picked up early at 10:00
	case marketingNotArrived:
		return marketingClock(90), marketingClock(345) // 11:45 after school, 16:00
	default:
		return marketingClock(-150), marketingClock(285 + 60*(index%2)) // 07:45, 15:00 or 16:00
	}
}

func marketingWeeklySchedules(presence marketingPresence, index int) ([]map[string]any, []map[string]any) {
	arrivalTime, pickupTime := marketingPlanTimes(presence, index)
	arrival := make([]map[string]any, 0, 5)
	pickup := make([]map[string]any, 0, 5)
	for weekday := 1; weekday <= 5; weekday++ {
		arrival = append(arrival, map[string]any{"weekday": weekday, "expected_arrival": arrivalTime})
		pickup = append(pickup, map[string]any{"weekday": weekday, "pickup_time": pickupTime})
	}
	return arrival, pickup
}

type seedMarketingProfileStep struct{ seeder *Seeder }

func (seedMarketingProfileStep) Name() string { return "Seeding marketing profile" }

func (s seedMarketingProfileStep) Run(ctx context.Context, primary *Runtime) error {
	child := *s.seeder
	child.profile, child.definition = marketingProfileKey, marketingProfileDefinition()
	child.options.TenantSlug, child.options.AdminEmail = "", ""
	rt := newRuntime(&child, primary.OperatorEmail, primary.OperatorPassword, primary.StaffPIN)
	rt.SetOperatorAuth(primary.OperatorAuth)
	defer primary.SetTenantAuth(primary.TenantAuth)
	workflow := Workflow{Name: marketingProfileKey, Steps: []Step{
		bootstrapTenantStep{seeder: &child}, configureProfileStep{definition: child.definition},
	}}
	if err := workflow.Run(ctx, rt); err != nil {
		return child.formatProfileError(child.profile, workflow.Name, err)
	}
	profile, err := seedMarketingProfile(ctx, primary, rt, &child)
	if err != nil {
		return child.formatProfileError(child.profile, "marketing contents", err)
	}
	primary.AdditionalProfiles[marketingProfileKey] = profile
	printMarketingProfile(profile)
	return nil
}

func seedMarketingProfile(ctx context.Context, primary, rt *Runtime, child *Seeder) (*SeedProfile, error) {
	schoolAdmin, err := renameMarketingAdmin(rt)
	if err != nil {
		return nil, err
	}
	data, staff, err := seedMarketingContents(ctx, rt, child)
	if err != nil {
		return nil, err
	}
	parents, err := inviteMarketingParents(ctx, rt, parentEnrollmentSeedStep{seeder: child}, data)
	if err != nil {
		return nil, err
	}
	developer := &SeedState{Settings: cloneProfileSettings(child.definition.Settings)}
	if err := linkDeveloperAdmin(ctx, primary, rt, developer); err != nil {
		return nil, err
	}
	if err := verifyMarketingProfile(rt, child.definition, data); err != nil {
		return nil, err
	}
	virtual, err := verifyProfileDevices(rt, 0)
	if err != nil {
		return nil, err
	}
	profile := buildManualSeedProfile(rt, child.definition, rt.Bootstrap, AccountCredentials{}, virtual, data)
	profile.Devices = map[string]SeedDevice{virtual.DeviceID: virtual}
	// The screenshot pipeline signs in with accounts.admin[0]: the school's own
	// admin comes first, the shared developer admin after it.
	profile.Credentials.Accounts = SeedStateAccounts{Admin: append([]AccountCredentials{schoolAdmin}, developer.Accounts.Admin...), Betreuer: staff}
	profile.Credentials.Parents = parents
	return profile, nil
}

// renameMarketingAdmin gives the invited school admin a real-looking name,
// since screenshots show the signed-in user, and reads the admin's staff id.
func renameMarketingAdmin(rt *Runtime) (AccountCredentials, error) {
	if _, err := rt.Client.Put("/api/me/profile", map[string]any{"first_name": marketingAdminFirstName, "last_name": marketingAdminLastName}); err != nil {
		return AccountCredentials{}, fmt.Errorf("rename marketing school admin: %w", err)
	}
	name := marketingAdminFirstName + " " + marketingAdminLastName
	rt.Bootstrap.AdminName = name
	raw, err := rt.Client.Get("/api/staff/by-role?role=admin")
	if err != nil {
		return AccountCredentials{}, fmt.Errorf("read marketing school admin identity: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID        int64 `json:"id"`
			AccountID int64 `json:"account_id"`
			TeacherID int64 `json:"teacher_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return AccountCredentials{}, fmt.Errorf("decode marketing school admin identity: %w", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].ID <= 0 {
		return AccountCredentials{}, fmt.Errorf("marketing school: expected exactly one admin, got %d", len(envelope.Data))
	}
	admin := envelope.Data[0]
	return AccountCredentials{
		Key: "schul-admin", AccountID: admin.AccountID, Email: rt.Bootstrap.AdminEmail, Password: rt.Bootstrap.AdminPassword,
		Name: name, StaffID: admin.ID, TeacherID: admin.TeacherID,
	}, nil
}

func seedMarketingContents(ctx context.Context, rt *Runtime, child *Seeder) (manualProfileData, []AccountCredentials, error) {
	staff, err := seedMarketingStaff(ctx, rt, child)
	if err != nil {
		return manualProfileData{}, nil, err
	}
	groups, err := seedMarketingGroups(rt, staff)
	if err != nil {
		return manualProfileData{}, nil, err
	}
	students, guardians, err := seedMarketingChildren(rt, groups)
	if err != nil {
		return manualProfileData{}, nil, err
	}
	if err := seedMarketingAttendance(rt, students); err != nil {
		return manualProfileData{}, nil, err
	}
	return manualProfileData{students: students, guardians: guardians, groups: groups}, staff, nil
}

func seedMarketingStaff(ctx context.Context, rt *Runtime, child *Seeder) ([]AccountCredentials, error) {
	roles := NewFixedSeeder(rt.Client, false, "")
	if err := roles.fetchRoles(ctx); err != nil {
		return nil, err
	}
	roleID := roles.roleIDs["user"]
	if roleID == 0 {
		return nil, fmt.Errorf("marketing school: user role missing")
	}
	password := marketingStaffPassword
	if child.options.StaffPassword != "" {
		password = child.options.StaffPassword
	}
	groupByLeader := make(map[string]string)
	for _, group := range marketingGroups() {
		groupByLeader[group.leader] = group.key
	}
	staff := make([]AccountCredentials, 0, len(marketingStaff()))
	for _, member := range marketingStaff() {
		name := member.firstName + " " + member.lastName
		local := emailLocalPart(member.firstName, member.lastName)
		raw, err := rt.Client.Post("/auth/register", map[string]any{
			"email": local + "@example.test", "username": "sonnenhang-" + local, "password": password, "confirm_password": password,
			"role_id": roleID, "first_name": member.firstName, "last_name": member.lastName,
		})
		if err != nil {
			return nil, fmt.Errorf("create marketing staff %s: %w", name, err)
		}
		var account struct {
			Data struct {
				ID             int64 `json:"id"`
				SchoolIdentity struct {
					StaffID   int64 `json:"staff_id,string"`
					TeacherID int64 `json:"teacher_id,string"`
				} `json:"school_identity"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &account); err != nil {
			return nil, fmt.Errorf("decode marketing staff %s: %w", name, err)
		}
		if account.Data.SchoolIdentity.StaffID <= 0 || account.Data.SchoolIdentity.TeacherID <= 0 {
			return nil, fmt.Errorf("marketing staff %s has no caregiver profile", name)
		}
		staff = append(staff, AccountCredentials{
			Key: semanticKey(name), AccountID: account.Data.ID, Email: local + "@example.test", Password: password, Name: name,
			StaffID: account.Data.SchoolIdentity.StaffID, TeacherID: account.Data.SchoolIdentity.TeacherID, Group: groupByLeader[name],
		})
	}
	return staff, nil
}

func seedMarketingGroups(rt *Runtime, staff []AccountCredentials) (map[string]SeedEntityRef, error) {
	groups := make(map[string]SeedEntityRef, len(marketingGroups()))
	for _, group := range marketingGroups() {
		teachers := []int64{}
		for _, member := range staff {
			if member.Group == group.key {
				teachers = append(teachers, member.TeacherID)
			}
		}
		raw, err := rt.Client.Post("/api/groups", map[string]any{"name": group.name, "teacher_ids": teachers})
		if err != nil {
			return nil, fmt.Errorf("create marketing group %s: %w", group.name, err)
		}
		id, err := decodeSeedEntityID(raw)
		if err != nil {
			return nil, fmt.Errorf("decode marketing group %s: %w", group.name, err)
		}
		groups[group.key] = SeedEntityRef{ID: id, Name: group.name}
	}
	return groups, nil
}

// seedMarketingChildren creates each child with its family contact. A
// sibling links the contact its older sibling created instead of a copy.
func seedMarketingChildren(rt *Runtime, groups map[string]SeedEntityRef) (map[string]SeedStudent, map[string]SeedEntityRef, error) {
	students := make(map[string]SeedStudent)
	guardians := make(map[string]SeedEntityRef)
	index := 0
	for _, family := range marketingFamilies() {
		contactName := family.firstName + " " + family.lastName
		var contactID int64
		for _, source := range family.children {
			group, ok := groups[source.group]
			if !ok {
				return nil, nil, fmt.Errorf("marketing group %s missing", source.group)
			}
			raw, err := rt.Client.Post("/api/students", marketingStudentPayload(source, index, group.ID, family, contactID))
			if err != nil {
				return nil, nil, fmt.Errorf("create marketing child %d: %w", index+1, err)
			}
			id, err := decodeSeedEntityID(raw)
			if err != nil {
				return nil, nil, fmt.Errorf("decode marketing child %d: %w", index+1, err)
			}
			guardian, err := readManualGuardian(rt, id, DemoGuardian{FirstName: family.firstName, LastName: family.lastName})
			if err != nil {
				return nil, nil, err
			}
			if contactID != 0 && guardian.ID != contactID {
				return nil, nil, fmt.Errorf("marketing sibling %d is not linked to contact %d", id, contactID)
			}
			contactID = guardian.ID
			guardians[semanticKey(contactName)] = guardian
			key := semanticKey(source.firstName + " " + source.lastName)
			students[key] = SeedStudent{Key: key, ID: id, FirstName: source.firstName, LastName: source.lastName, GroupKey: source.group, Class: source.class}
			index++
		}
	}
	return students, guardians, nil
}

func marketingStudentPayload(source marketingChild, index int, groupID int64, family marketingFamily, contactID int64) map[string]any {
	arrival, pickup := marketingWeeklySchedules(source.presence, index)
	contact := map[string]any{
		"relationship_type": "parent", "is_primary": true, "is_emergency_contact": true, "can_pickup": true, "emergency_priority": 1,
	}
	if contactID != 0 {
		contact["guardian_profile_id"] = contactID
	} else {
		contact["first_name"], contact["last_name"], contact["email"] = family.firstName, family.lastName, marketingContactEmail(family)
		contact["phone_numbers"] = []map[string]any{{"phone_number": family.phone, "phone_type": "mobile", "is_primary": true}}
	}
	return map[string]any{
		"first_name": source.firstName, "last_name": source.lastName, "school_class": source.class, "group_id": groupID,
		"birthday": source.birthday, "pickup_status": "Wird abgeholt",
		"arrival_schedules": arrival, "pickup_schedules": pickup, "guardians": []map[string]any{contact},
	}
}

func seedMarketingAttendance(rt *Runtime, students map[string]SeedStudent) error {
	for _, family := range marketingFamilies() {
		for _, source := range family.children {
			student := students[semanticKey(source.firstName+" "+source.lastName)]
			if source.presence == marketingNotArrived {
				continue
			}
			if err := postSchoolAttendance(rt, student.ID, "in", "checked_in"); err != nil {
				return err
			}
			if source.presence == marketingPickedUp {
				if err := postSchoolAttendance(rt, student.ID, "out", "checked_out"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func inviteMarketingParents(ctx context.Context, rt *Runtime, step parentEnrollmentSeedStep, data manualProfileData) ([]ParentCredentials, error) {
	password, err := step.parentPassword()
	if err != nil {
		return nil, err
	}
	parents := []ParentCredentials{}
	for _, family := range marketingFamilies() {
		if !family.parentAccount {
			continue
		}
		name := family.firstName + " " + family.lastName
		guardian := data.guardians[semanticKey(name)]
		studentIDs := make([]int64, 0, len(family.children))
		for _, source := range family.children {
			studentIDs = append(studentIDs, data.students[semanticKey(source.firstName+" "+source.lastName)].ID)
		}
		token, err := step.inviteGuardian(rt, rt.TenantAuth, guardian.ID)
		if err != nil {
			return nil, fmt.Errorf("invite marketing parent %s: %w", semanticKey(name), err)
		}
		accountID, err := step.acceptGuardianInvitation(rt, token, password)
		if err != nil {
			return nil, fmt.Errorf("accept marketing parent invitation %s: %w", semanticKey(name), err)
		}
		email := marketingContactEmail(family)
		if err := verifyMarketingParentChildren(ctx, rt, email, password, studentIDs); err != nil {
			return nil, err
		}
		parents = append(parents, ParentCredentials{
			Key: semanticKey(name), Email: email, Password: password, Name: name,
			AccountID: accountID, GuardianID: guardian.ID, StudentIDs: studentIDs,
		})
	}
	return parents, nil
}

// verifyMarketingParentChildren signs in to the parents portal and checks that
// the account sees exactly its own children.
func verifyMarketingParentChildren(ctx context.Context, rt *Runtime, email, password string, expected []int64) error {
	auth, err := rt.Adapter.LoginParent(ctx, email, password)
	if err != nil {
		return fmt.Errorf("marketing parent login: %w", err)
	}
	raw, err := rt.Client.GetWithAuth(auth, "/parent/me/children")
	if err != nil {
		return fmt.Errorf("read marketing parent children: %w", err)
	}
	var envelope struct {
		Data []struct {
			StudentID string `json:"student_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode marketing parent children: %w", err)
	}
	actual := make([]int64, 0, len(envelope.Data))
	for _, row := range envelope.Data {
		id, err := strconv.ParseInt(row.StudentID, 10, 64)
		if err != nil {
			return fmt.Errorf("decode marketing parent child id %q: %w", row.StudentID, err)
		}
		actual = append(actual, id)
	}
	slices.Sort(actual)
	wanted := slices.Sorted(slices.Values(expected))
	if !slices.Equal(actual, wanted) {
		return fmt.Errorf("marketing parent sees children %v, expected %v", actual, wanted)
	}
	return nil
}

func verifyMarketingProfile(rt *Runtime, definition demoProfileDefinition, data manualProfileData) error {
	if err := verifyProfileSettings(rt, definition); err != nil {
		return err
	}
	physical, err := listSeedDevices(rt, "terminal")
	if err != nil {
		return err
	}
	if len(physical) != 0 {
		return fmt.Errorf("%s must have no physical terminals, got %d", definition.Key, len(physical))
	}
	if err := verifyManualStudents(rt, definition.Expected, data); err != nil {
		return err
	}
	if err := verifyManualStaff(rt, definition.Expected.Staff); err != nil {
		return err
	}
	return verifyManualVisits(rt)
}

func printMarketingProfile(profile *SeedProfile) {
	admin := profile.Credentials.Accounts.Admin[0]
	fmt.Printf("Verified profile %s: organization %s (%d), school %s (%d), admin %s / %s, reference clock %s\n",
		profile.Key, profile.Organization.Slug, profile.Organization.ID, profile.School.TenantSlug, profile.School.ID,
		admin.Email, admin.Password, marketingReferenceClock)
	for _, parent := range profile.Credentials.Parents {
		fmt.Printf("  Parent %s: %s / %s, children %v\n", parent.Key, parent.Email, parent.Password, parent.StudentIDs)
	}
}
