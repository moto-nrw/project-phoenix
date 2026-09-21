package integration

import (
	"context"
	"fmt"
	"testing"

	dataImportCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"
	importService "github.com/moto-nrw/project-phoenix/services/import"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Tests for the Stammdaten import (#2600): the staff import files real
// records, both importers support update mode, and the child import carries
// address, RFID and the guardian role preset.

func newStammdatenStaffConfig(t *testing.T, db *bun.DB, invitations importModels.StaffInviter) (*importService.StaffImportConfig, *repositories.Factory) {
	t.Helper()
	owners := repositories.NewUnobservedTimetableDependencies(db)
	repos := repositories.NewFactory(db, owners)
	organizations, err := repositories.NewOrganizationTenancy(db)
	require.NoError(t, err)
	identity, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	config := importService.NewStaffImportConfig(importService.StaffImportDeps{RolePolicy: dataImportCompose.NewSchoolRolePolicy(), Authorization: dataImportCompose.NewAuthorization(), Transactions: dataImportCompose.NewTransactions(),
		Invitations:         invitations,
		FindInvitedPeople:   identity.FindInvitedPersonIDs,
		FindSchoolAccount:   dataImportCompose.NewSchoolAccountLookup(identity),
		Roles:               identity,
		FindRolePermissions: identity.FindRolePermissions,
		SchoolName:          dataImportCompose.NewSchoolName(organizations),
		Persons:             owners.Students,
		Membership:          owners.Membership,
		Records:             owners.Workforce,
	})
	return config, repos
}

func inTenantTx(t *testing.T, db *bun.DB, fn func(ctx context.Context) error) {
	t.Helper()
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	}))
}

func TestStaffImportConfig_Create_FilesStammdatensatzAndInvitation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	captured := &importModels.StaffInvitation{}
	invitations := &staffInvitationMock{
		InviteStaffFn: func(_ context.Context, req importModels.StaffInvitation) error {
			*captured = req
			return nil
		},
	}
	config, repos := newStammdatenStaffConfig(t, db, invitations)
	role := testpkg.CreateTestRoleForTenant(t, db, "Betreuungskraft", testpkg.Tenant(t))

	email := fmt.Sprintf("import.%d@example.test", role.ID)
	row := importModels.StaffImportRow{
		FirstName: "Anna", LastName: "Lehmann", Email: email, RoleID: role.ID, Position: "Gruppenleitung",
		Birthday: "1988-05-12", Gender: "female", PersonnelNumber: "P-2600", EmploymentType: "part_time",
		WeeklyHours: "19.5", EntryDate: "2023-08-01", AddressStreet: "Musterstr. 1", AddressPostalCode: "50667", AddressCity: "Köln",
		Phone: "0221-1234567", EmergencyContactName: "Peter Lehmann", EmergencyContactPhone: "0171-9876543",
		Qualifications: "Erste Hilfe (01.03.2024 bis 01.03.2026); Schwimmschein", StaffNotes: "importiert",
	}

	var staffID int64
	inTenantTx(t, db, func(ctx context.Context) error {
		require.NoError(t, config.PreloadReferenceData(ctx))
		id, err := config.Create(ctx, row)
		require.NoError(t, err)
		staffID = id
		return nil
	})

	ctx := testpkg.Ctx(t)
	staff, err := repos.Staff.FindWithPerson(ctx, staffID)
	require.NoError(t, err)
	require.NotNil(t, staff.Person)
	assert.Equal(t, "Anna", staff.Person.FirstName)
	require.NotNil(t, staff.Person.Birthday)
	assert.Equal(t, timezone.NewDate(1988, 5, 12), *staff.Person.Birthday)
	assert.Nil(t, staff.Person.AccountID, "the import must not invent an account")
	require.NotNil(t, staff.PersonnelNumber)
	assert.Equal(t, "P-2600", *staff.PersonnelNumber)
	require.NotNil(t, staff.EmploymentType)
	assert.Equal(t, ports.EmploymentTypePartTime, *staff.EmploymentType)
	assert.Equal(t, "importiert", staff.StaffNotes)

	teacher, err := repos.Teacher.FindByStaffID(ctx, staffID)
	require.NoError(t, err)
	require.NotNil(t, teacher, "a caregiver-tier role gets its profile")
	assert.Equal(t, "Gruppenleitung", teacher.Role)

	master, err := repos.StaffMasterData.FindByStaffID(ctx, staffID)
	require.NoError(t, err)
	require.NotNil(t, master)
	assert.Equal(t, "female", *master.Gender)
	assert.Equal(t, "Köln", *master.AddressCity)
	assert.Equal(t, "Peter Lehmann", *master.EmergencyContactName)
	assert.InDelta(t, 19.5, *master.WeeklyHours, 0.001)
	assert.Equal(t, timezone.NewDate(2023, 8, 1), *master.EntryDate)

	quals, err := repos.StaffQualification.ListByStaffID(ctx, staffID)
	require.NoError(t, err)
	require.Len(t, quals, 2)
	assert.Equal(t, "Erste Hilfe", quals[0].Name)
	require.NotNil(t, quals[0].ExpiresOn)
	assert.Equal(t, timezone.NewDate(2026, 3, 1), *quals[0].ExpiresOn)
	assert.Equal(t, "Schwimmschein", quals[1].Name)

	assert.Equal(t, email, captured.Email)
	require.NotNil(t, captured.PersonID, "the invitation must remember the imported person")
	assert.Equal(t, staff.PersonID, *captured.PersonID)

	// The same file resolving the row again finds the record it just wrote.
	inTenantTx(t, db, func(ctx context.Context) error {
		id, err := config.FindExisting(ctx, importModels.StaffImportRow{FirstName: "Anna", LastName: "Lehmann", PersonnelNumber: "p-2600"})
		require.NoError(t, err)
		require.NotNil(t, id)
		assert.Equal(t, staffID, *id)
		return nil
	})
}

func TestStaffImportConfig_Create_WithoutEmailSkipsInvitation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	invited := false
	invitations := &staffInvitationMock{
		InviteStaffFn: func(context.Context, importModels.StaffInvitation) error {
			invited = true
			return nil
		},
	}
	config, repos := newStammdatenStaffConfig(t, db, invitations)
	role := testpkg.CreateTestRoleForTenant(t, db, "Kueche", testpkg.Tenant(t))

	var staffID int64
	inTenantTx(t, db, func(ctx context.Context) error {
		require.NoError(t, config.PreloadReferenceData(ctx))
		id, err := config.Create(ctx, importModels.StaffImportRow{FirstName: "Cem", LastName: "Yilmaz", RoleID: role.ID})
		require.NoError(t, err)
		staffID = id
		return nil
	})

	assert.False(t, invited)
	staff, err := repos.Staff.FindWithPerson(testpkg.Ctx(t), staffID)
	require.NoError(t, err)
	assert.Equal(t, "Yilmaz", staff.Person.LastName)
	master, err := repos.StaffMasterData.FindByStaffID(testpkg.Ctx(t), staffID)
	require.NoError(t, err)
	assert.Nil(t, master, "no master data row without master data columns")
}

func TestStaffImportConfig_Update_PatchesOnlyGivenCells(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	config, repos := newStammdatenStaffConfig(t, db, nil)
	ctx := testpkg.Ctx(t)

	person := testpkg.CreateTestPerson(t, db, "Bernd", "Schulz")
	staff := testpkg.CreateTestStaffForPerson(t, db, person.ID)
	staff.StaffNotes = "alt"
	require.NoError(t, repos.Staff.Update(ctx, staff))
	city := "Bonn"
	inTenantTx(t, db, func(ctx context.Context) error {
		_, err := config.Records.CreateStaffMasterData(ctx, ports.StaffMasterData{StaffID: staff.ID, AddressCity: &city})
		return err
	})

	inTenantTx(t, db, func(ctx context.Context) error {
		require.NoError(t, config.PreloadReferenceData(ctx))
		id, err := config.FindExisting(ctx, importModels.StaffImportRow{FirstName: "bernd", LastName: "SCHULZ"})
		require.NoError(t, err)
		require.NotNil(t, id, "name match must resolve the existing staff member")
		assert.Equal(t, staff.ID, *id)
		return config.Update(ctx, *id, importModels.StaffImportRow{
			FirstName: "Bernd", LastName: "Schulz", PersonnelNumber: "P-9", WeeklyHours: "39", EmploymentType: "full_time",
		})
	})

	updated, err := repos.Staff.FindByID(ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, "alt", updated.StaffNotes, "empty cell keeps the stored value")
	require.NotNil(t, updated.PersonnelNumber)
	assert.Equal(t, "P-9", *updated.PersonnelNumber)
	assert.Equal(t, ports.EmploymentTypeFullTime, *updated.EmploymentType)

	master, err := repos.StaffMasterData.FindByStaffID(ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, "Bonn", *master.AddressCity, "empty address cell keeps the stored city")
	assert.InDelta(t, 39, *master.WeeklyHours, 0.001)
}

func TestStaffImportConfig_FindExisting_RefusesAmbiguousName(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	config, _ := newStammdatenStaffConfig(t, db, nil)
	testpkg.CreateTestStaff(t, db, "Doppel", "Name")
	testpkg.CreateTestStaff(t, db, "Doppel", "Name")

	inTenantTx(t, db, func(ctx context.Context) error {
		require.NoError(t, config.PreloadReferenceData(ctx))
		_, err := config.FindExisting(ctx, importModels.StaffImportRow{FirstName: "Doppel", LastName: "Name"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mehrere Personen")
		return nil
	})
}

func newStammdatenStudentConfig(t *testing.T, db *bun.DB) (*importService.StudentImportConfig, *repositories.Factory) {
	t.Helper()
	// Guardian commands need the owner's provider, which the legacy root
	// binds; the guardian-bearing cutover tests therefore live in the
	// services package (import_cutover_test.go). This config covers the
	// person, student, schedule and consent paths.
	owners := repositories.NewUnobservedTimetableDependencies(db)
	repos := repositories.NewFactory(db, owners)
	groups, err := repositories.NewSchoolStructure(db)
	require.NoError(t, err)
	rooms, err := repositories.NewFacilities(db)
	require.NoError(t, err)
	references := dataImportCompose.NewReferences(groups, rooms)
	rfid, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	config := importService.NewStudentImportConfig(importService.StudentImportDeps{Transactions: dataImportCompose.NewTransactions(),
		Persons:         owners.Students,
		Students:        owners.Students,
		Schedules:       repos.CarePlan(),
		PrivacyConsents: repositories.NewStudentPresenceForTests(db),
		FindRFIDCard:    rfid.FindRFIDCard,
		Resolver:        importService.NewRelationshipResolver(references.Groups, references.Rooms),
	})
	return config, repos
}

func TestStudentImportConfig_Validate_RejectsUnknownOrTakenTag(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	config, repos := newStammdatenStudentConfig(t, db)
	ctx := testpkg.Ctx(t)

	takenCard := testpkg.CreateTestRFIDCard(t, db, "TAKEN")
	nonStudentWearer := testpkg.CreateTestPerson(t, db, "Andere", "Person")
	require.NoError(t, repos.Person.LinkToRFIDCard(ctx, nonStudentWearer.ID, takenCard.ID))
	ownCard := testpkg.CreateTestRFIDCard(t, db, "OWN")
	studentWearer := testpkg.CreateTestStudent(t, db, "Eigene", "Person", "1A")
	require.NoError(t, repos.Person.LinkToRFIDCard(ctx, studentWearer.PersonID, ownCard.ID))

	inTenantTx(t, db, func(ctx context.Context) error {
		require.NoError(t, config.PreloadReferenceData(ctx))

		unknown := importModels.StudentImportRow{FirstName: "A", LastName: "B", SchoolClass: "1A", TagID: "GIBTESNICHT"}
		assert.Contains(t, validationCodes(config.Validate(ctx, &unknown)), "rfid_unknown")

		taken := importModels.StudentImportRow{FirstName: "A", LastName: "B", SchoolClass: "1A", TagID: takenCard.ID}
		assert.Contains(t, validationCodes(config.Validate(ctx, &taken)), "rfid_taken")

		own := importModels.StudentImportRow{FirstName: "Eigene", LastName: "Person", SchoolClass: "1A", TagID: ownCard.ID}
		assert.NotContains(t, validationCodes(config.Validate(ctx, &own)), "rfid_taken", "the wearer re-importing its own card passes")
		return nil
	})
}

func validationCodes(errs []importModels.ValidationError) []string {
	codes := make([]string, 0, len(errs))
	for _, e := range errs {
		codes = append(codes, e.Code)
	}
	return codes
}

type staffInvitationMock struct {
	InviteStaffFn func(context.Context, importModels.StaffInvitation) error
}

func (m *staffInvitationMock) InviteStaff(ctx context.Context, invitation importModels.StaffInvitation) error {
	return m.InviteStaffFn(ctx, invitation)
}
