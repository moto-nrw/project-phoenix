package api

import (
	"fmt"
	"strings"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
)

func (s *Seeder) buildE2EManifest(rt *Runtime) (*contract.Manifest, error) {
	if rt == nil {
		return nil, fmt.Errorf("cannot build e2e manifest: runtime is nil")
	}
	if rt.Bootstrap == nil {
		return nil, fmt.Errorf("cannot build e2e manifest: bootstrap state is nil")
	}
	if rt.FixedSeeder == nil {
		return nil, fmt.Errorf("cannot build e2e manifest: fixed seeder is nil")
	}
	if rt.StaffPIN == "" {
		return nil, fmt.Errorf("cannot build e2e manifest: staff PIN missing")
	}

	adminActor, err := resolveE2EAdminActor(rt.FixedSeeder, rt.SecondTenant)
	if err != nil {
		return nil, err
	}
	staffActor, err := resolveE2EStaffActor(rt.FixedSeeder)
	if err != nil {
		return nil, err
	}
	fixtures, err := buildE2EFixtures(rt.FixedSeeder, adminActor)
	if err != nil {
		return nil, err
	}

	deviceAPIKey, ok := rt.FixedSeeder.deviceKeys[e2eScenarioCheckinDeviceKey]
	if !ok || deviceAPIKey == "" {
		return nil, fmt.Errorf(
			"cannot build e2e manifest: device %q missing API key",
			e2eScenarioCheckinDeviceKey,
		)
	}

	manifest := &contract.Manifest{
		Version: contract.ManifestVersion,
		Runtime: s.options.E2ERuntime,
		Scenario: contract.ScenarioMetadata{
			Name: s.options.Scenario,
			Mode: "single-tenant",
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
				Key:    e2eScenarioCheckinDeviceKey,
				APIKey: deviceAPIKey,
				PIN:    rt.StaffPIN,
			},
		},
		Fixtures: fixtures,
	}

	if rt.SecondTenant != nil {
		manifest.Scenario.Mode = "multi-tenant"
		manifest.Tenants.Secondary = &contract.Tenant{
			OrganizationID: rt.SecondTenant.OrganizationID,
			SchoolID:       rt.SecondTenant.SchoolID,
			Slug:           rt.SecondTenant.Slug,
			Name:           rt.SecondTenant.Name,
		}
		manifest.Switching = &contract.Switching{
			LinkEmail: rt.SecondTenant.LinkEmail,
			Verified:  true,
			Actor:     adminActor,
		}
	}

	manifest.Normalize()
	return manifest, nil
}

func resolveE2EAdminActor(fs *FixedSeeder, secondTenant *SeedStateSecondTenant) (contract.Actor, error) {
	adminCred, err := resolveE2EAdminCredential(fs, secondTenant)
	if err != nil {
		return contract.Actor{}, fmt.Errorf("cannot build e2e manifest: %w", err)
	}
	return actorFromStaffCredential(fs, adminCred, "admin")
}

func resolveE2EStaffActor(fs *FixedSeeder) (contract.Actor, error) {
	staffCred, ok := staffCredentialByEmail(fs, e2eScenarioStaffEmail)
	if !ok {
		return contract.Actor{}, fmt.Errorf(
			"cannot build e2e manifest: scenario staff account %q is not present in fixed seeder output",
			e2eScenarioStaffEmail,
		)
	}
	return actorFromStaffCredential(fs, staffCred, "user")
}

func buildE2EFixtures(fs *FixedSeeder, adminActor contract.Actor) (contract.Fixtures, error) {
	searchPrimary, err := studentRefByName(fs, e2eScenarioSearchStudentPrimary)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e manifest: %w", err)
	}
	searchSecondary, err := studentRefByName(fs, e2eScenarioSearchStudentSecondary)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e manifest: %w", err)
	}
	groupPrimary, err := groupRefByKey(fs, e2eScenarioVisibleGroupKeyA)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e manifest: %w", err)
	}
	groupSecondary, err := groupRefByKey(fs, e2eScenarioVisibleGroupKeyB)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e manifest: %w", err)
	}
	checkinStudent, err := studentRefByName(fs, e2eScenarioCheckinStudent)
	if err != nil {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e manifest: %w", err)
	}

	checkinRoomID, ok := fs.roomIDs[e2eScenarioCheckinRoomName]
	if !ok || checkinRoomID <= 0 {
		return contract.Fixtures{}, fmt.Errorf(
			`cannot build e2e manifest: room %q missing from fixed seeder output`,
			e2eScenarioCheckinRoomName,
		)
	}
	checkinActivityID, ok := fs.activityIDs[e2eScenarioCheckinActivityName]
	if !ok || checkinActivityID <= 0 {
		return contract.Fixtures{}, fmt.Errorf(
			`cannot build e2e manifest: activity %q missing from fixed seeder output`,
			e2eScenarioCheckinActivityName,
		)
	}
	checkinRFID, ok := fs.studentRFID[checkinStudent.ID]
	if !ok || checkinRFID == "" {
		return contract.Fixtures{}, fmt.Errorf("cannot build e2e manifest: RFID tag missing for checkin fixture student %d", checkinStudent.ID)
	}

	return contract.Fixtures{
		Students: contract.StudentFixtures{
			SearchPair: contract.StudentPair{
				Primary:   searchPrimary,
				Secondary: searchSecondary,
			},
		},
		Groups: contract.GroupFixtures{
			VisiblePair: contract.GroupPair{
				Primary:   groupPrimary,
				Secondary: groupSecondary,
			},
		},
		Checkin: contract.CheckinFixture{
			Student:    checkinStudent,
			Room:       contract.RoomRef{ID: checkinRoomID, Name: e2eScenarioCheckinRoomName},
			Activity:   contract.ActivityRef{ID: checkinActivityID, Name: e2eScenarioCheckinActivityName},
			DeviceKey:  e2eScenarioCheckinDeviceKey,
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

func studentRefByName(fs *FixedSeeder, ref namedStudentRef) (contract.StudentRef, error) {
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

func demoStudentByName(ref namedStudentRef) (DemoStudent, error) {
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
