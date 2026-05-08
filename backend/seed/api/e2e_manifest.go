package api

import (
	"fmt"
	"strings"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
)

func (s *Seeder) buildE2EState(rt *Runtime) (*contract.State, error) {
	if rt == nil {
		return nil, fmt.Errorf("cannot build e2e state: runtime is nil")
	}
	if rt.Bootstrap == nil {
		return nil, fmt.Errorf("cannot build e2e state: bootstrap state is nil")
	}
	if rt.FixedSeeder == nil {
		return nil, fmt.Errorf("cannot build e2e state: fixed seeder is nil")
	}
	if rt.StaffPIN == "" {
		return nil, fmt.Errorf("cannot build e2e state: staff PIN missing")
	}
	scenarioDefinition, ok := scenarios.Lookup(s.options.Scenario)
	if !ok {
		return nil, fmt.Errorf(
			"cannot build e2e state: unknown scenario %q",
			s.options.Scenario,
		)
	}

	adminActor, err := resolveE2EAdminActor(rt.FixedSeeder, rt.SecondTenant)
	if err != nil {
		return nil, err
	}
	staffActor, err := resolveE2EStaffActor(rt.FixedSeeder, scenarioDefinition)
	if err != nil {
		return nil, err
	}
	fixtures, err := buildE2EFixtures(rt.FixedSeeder, adminActor, scenarioDefinition)
	if err != nil {
		return nil, err
	}

	deviceAPIKey, ok := rt.FixedSeeder.deviceKeys[scenarioDefinition.Fixtures.Checkin.DeviceKey]
	if !ok || deviceAPIKey == "" {
		return nil, fmt.Errorf(
			"cannot build e2e state: device %q missing API key",
			scenarioDefinition.Fixtures.Checkin.DeviceKey,
		)
	}

	state := &contract.State{
		Version: contract.StateVersion,
		Runtime: s.options.E2ERuntime,
		Setup: contract.Setup{
			Auth: contract.AuthSetup{
				Roles:                     append([]string(nil), scenarioDefinition.Auth.Roles...),
				RequiresSecondaryTenant:   scenarioDefinition.Auth.RequiresSecondaryTenant,
				RequiresVerifiedSwitching: scenarioDefinition.Auth.RequiresVerifiedSwitching,
			},
		},
		World: contract.World{
			Scenario: contract.ScenarioMetadata{
				Name: scenarioDefinition.Name,
				Mode: scenarioDefinition.Mode,
			},
			Tenants: contract.Tenants{
				Primary: contract.Tenant{
					OrganizationID: rt.Bootstrap.OrganizationID,
					SchoolID:       rt.Bootstrap.SchoolID,
					Slug:           rt.Bootstrap.TenantSlug,
					Name:           rt.Bootstrap.SchoolName,
				},
			},
			Actors: contract.Actors{
				Admin: adminActor,
				Staff: staffActor,
				Operator: &contract.OperatorCredentials{
					Email:    rt.OperatorEmail,
					Password: rt.OperatorPassword,
				},
			},
			Devices: contract.Devices{
				DefaultCheckin: contract.Device{
					Key:    scenarioDefinition.Fixtures.Checkin.DeviceKey,
					APIKey: deviceAPIKey,
					PIN:    rt.StaffPIN,
				},
			},
		},
		Fixtures: fixtures,
		Assertions: contract.Assertions{
			Switching: contract.SwitchingAssertion{
				Required: scenarioDefinition.Auth.RequiresVerifiedSwitching,
			},
		},
	}

	if rt.SecondTenant != nil {
		state.World.Tenants.Secondary = &contract.Tenant{
			OrganizationID: rt.SecondTenant.OrganizationID,
			SchoolID:       rt.SecondTenant.SchoolID,
			Slug:           rt.SecondTenant.Slug,
			Name:           rt.SecondTenant.Name,
		}
		state.Assertions.Switching = contract.SwitchingAssertion{
			Required:  true,
			Verified:  true,
			LinkEmail: rt.SecondTenant.LinkEmail,
			Actor:     &adminActor,
		}
	}

	state.Normalize()
	if err := state.Validate(); err != nil {
		return nil, fmt.Errorf("cannot build e2e state: %w", err)
	}
	return state, nil
}

func resolveE2EAdminActor(fs *FixedSeeder, secondTenant *SeedStateSecondTenant) (contract.Actor, error) {
	adminCred, err := resolveE2EAdminCredential(fs, secondTenant)
	if err != nil {
		return contract.Actor{}, fmt.Errorf("cannot build e2e state: %w", err)
	}
	return actorFromStaffCredential(fs, adminCred, "admin")
}

func resolveE2EStaffActor(fs *FixedSeeder, definition scenarios.Definition) (contract.Actor, error) {
	staffEmail := definition.Fixtures.StaffEmail
	staffCred, ok := staffCredentialByEmail(fs, staffEmail)
	if !ok {
		return contract.Actor{}, fmt.Errorf(
			"cannot build e2e state: scenario staff account %q is not present in fixed seeder output",
			staffEmail,
		)
	}
	return actorFromStaffCredential(fs, staffCred, "user")
}

func buildE2EFixtures(fs *FixedSeeder, adminActor contract.Actor, definition scenarios.Definition) (contract.Fixtures, error) {
	searchPrimary, err := studentRefByName(fs, definition.Fixtures.Students.SearchPairPrimary)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e state: %w", err)
	}
	searchSecondary, err := studentRefByName(fs, definition.Fixtures.Students.SearchPairSecondary)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e state: %w", err)
	}
	sickPresent, err := studentRefByName(fs, definition.Fixtures.Students.SickPresent.Student)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e state: %w", err)
	}
	groupPrimary, err := groupRefByKey(fs, definition.Fixtures.Groups.VisiblePrimaryKey)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e state: %w", err)
	}
	groupSecondary, err := groupRefByKey(fs, definition.Fixtures.Groups.VisibleSecondaryKey)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e state: %w", err)
	}
	checkinStudent, err := studentRefByName(fs, definition.Fixtures.Checkin.Student)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e state: %w", err)
	}

	checkinRoomName := definition.Fixtures.Checkin.RoomName
	checkinRoomID, ok := fs.roomIDs[checkinRoomName]
	if !ok || checkinRoomID <= 0 {
		return contract.Fixtures{}, fmt.Errorf(
			`cannot build e2e state: room %q missing from fixed seeder output`,
			checkinRoomName,
		)
	}
	checkinActivityName := definition.Fixtures.Checkin.ActivityName
	checkinActivityID, ok := fs.activityIDs[checkinActivityName]
	if !ok || checkinActivityID <= 0 {
		return contract.Fixtures{}, fmt.Errorf(
			`cannot build e2e state: activity %q missing from fixed seeder output`,
			checkinActivityName,
		)
	}
	checkinRFID, ok := fs.studentRFID[checkinStudent.ID]
	if !ok || checkinRFID == "" {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e state: RFID tag missing for checkin fixture student %d", checkinStudent.ID)
	}

	return contract.Fixtures{
		Students: contract.StudentFixtures{
			SearchPair: contract.StudentPair{
				Primary:   searchPrimary,
				Secondary: searchSecondary,
			},
			SickPresent: sickPresent,
		},
		Groups: contract.GroupFixtures{
			VisiblePair: contract.GroupPair{
				Primary:   groupPrimary,
				Secondary: groupSecondary,
			},
		},
		Checkin: contract.CheckinFixture{
			Student:    checkinStudent,
			Room:       contract.RoomRef{ID: checkinRoomID, Name: checkinRoomName},
			Activity:   contract.ActivityRef{ID: checkinActivityID, Name: checkinActivityName},
			DeviceKey:  definition.Fixtures.Checkin.DeviceKey,
			RFIDTag:    checkinRFID,
			Supervisor: adminActor,
		},
	}, nil
}

func resolveE2EAdminCredential(fs *FixedSeeder, secondTenant *SeedStateSecondTenant) (StaffCredentials, error) {
	if secondTenant != nil && secondTenant.LinkEmail != "" {
		if cred, ok := staffCredentialByEmail(fs, secondTenant.LinkEmail); ok {
			return cred, nil
		}
		return StaffCredentials{}, fmt.Errorf(
			`switching account %q is not present in fixed seeder output`,
			secondTenant.LinkEmail,
		)
	}
	for _, cred := range fs.staffCredentials {
		if cred.Position == "OGS-Büro" {
			return cred, nil
		}
	}
	return StaffCredentials{}, fmt.Errorf("no OGS-Büro admin account present in fixed seeder output")
}

func staffCredentialByEmail(fs *FixedSeeder, email string) (StaffCredentials, bool) {
	if fs == nil {
		return StaffCredentials{}, false
	}
	for _, cred := range fs.staffCredentials {
		if cred.Email == email {
			return cred, true
		}
	}
	return StaffCredentials{}, false
}

func actorFromStaffCredential(fs *FixedSeeder, cred StaffCredentials, role string) (contract.Actor, error) {
	if fs == nil {
		return contract.Actor{}, fmt.Errorf("cannot build e2e actor: fixed seeder is nil")
	}
	staffID, ok := fs.staffIDs[cred.Name]
	if !ok || staffID <= 0 {
		return contract.Actor{}, fmt.Errorf("cannot build e2e actor: missing staff ID for %q", cred.Email)
	}
	return contract.Actor{
		Email:       cred.Email,
		Password:    cred.Password,
		DisplayName: cred.Name,
		Role:        role,
		StaffID:     staffID,
	}, nil
}

func studentRefByName(fs *FixedSeeder, ref scenarios.StudentNameRef) (contract.StudentRef, error) {
	if fs == nil {
		return contract.StudentRef{}, fmt.Errorf("fixed seeder is nil")
	}
	studentID, err := lookupSeededStudentID(fs, ref)
	if err != nil {
		return contract.StudentRef{}, err
	}
	demoStudent, err := demoStudentByName(ref)
	if err != nil {
		return contract.StudentRef{}, err
	}
	return contract.StudentRef{
		ID:        studentID,
		FirstName: demoStudent.FirstName,
		LastName:  demoStudent.LastName,
		GroupKey:  demoStudent.GroupKey,
		Class:     demoStudent.Class,
	}, nil
}

func demoStudentByName(ref scenarios.StudentNameRef) (DemoStudent, error) {
	for _, student := range DemoStudents {
		if student.FirstName == ref.FirstName && student.LastName == ref.LastName {
			return student, nil
		}
	}
	return DemoStudent{}, fmt.Errorf(`seeded student "%s %s" is missing`, ref.FirstName, ref.LastName)
}

func groupRefByKey(fs *FixedSeeder, key string) (contract.GroupRef, error) {
	if fs == nil {
		return contract.GroupRef{}, fmt.Errorf("fixed seeder is nil")
	}
	id, ok := fs.groupIDs[key]
	if !ok || id <= 0 {
		return contract.GroupRef{}, fmt.Errorf("seeded group %q is missing", key)
	}
	return contract.GroupRef{
		ID:          id,
		Key:         key,
		DisplayName: displayNameForGroupKey(key),
	}, nil
}

func displayNameForGroupKey(key string) string {
	if key == "" {
		return ""
	}
	return strings.ToUpper(key[:1]) + key[1:]
}
