package importpkg

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/auth/authtest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Tests for the Stammdaten import (#2600): the staff import files real
// records, both importers support update mode, and the child import carries
// address, RFID and the guardian role preset.

func newStammdatenStaffConfig(t *testing.T, db *bun.DB, invitations authsvc.InvitationService) (*StaffImportConfig, *repositories.Factory) {
	t.Helper()
	repos := repositories.NewFactory(db)
	config := NewStaffImportConfig(StaffImportDeps{
		InvitationService: invitations,
		AccountRepo:       repos.Account,
		AccountTenantRepo: repos.AccountTenant,
		RoleRepo:          repos.Role,
		PermissionRepo:    repos.Permission,
		SchoolRepo:        repos.School,
		PersonRepo:        repos.Person,
		StaffRepo:         repos.Staff,
		TeacherRepo:       repos.Teacher,
		MasterDataRepo:    repos.StaffMasterData,
		QualificationRepo: repos.StaffQualification,
	})
	return config, repos
}

func inTenantTx(t *testing.T, db *bun.DB, fn func(ctx context.Context) error) {
	t.Helper()
	require.NoError(t, tenant.WithTenantTx(testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		return fn(ctx)
	}))
}

func TestStaffImportConfig_Create_FilesStammdatensatzAndInvitation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	captured := &authsvc.InvitationRequest{}
	invitations := &authtest.InvitationServiceMock{
		CreateInvitationFn: func(_ context.Context, req authsvc.InvitationRequest) (*authModels.InvitationToken, error) {
			*captured = req
			return &authModels.InvitationToken{Model: base.Model{ID: 1}}, nil
		},
	}
	config, repos := newStammdatenStaffConfig(t, db, invitations)
	role := testpkg.CreateTestRoleForTenant(t, db, "Betreuungskraft", testpkg.Tenant(t))
	baseRole := authModels.BaseRoleUser
	role.BaseRole = &baseRole
	config.rolesByID = map[int64]*authModels.Role{role.ID: role}

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
	assert.Equal(t, userModels.EmploymentTypePartTime, *staff.EmploymentType)
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
	invitations := &authtest.InvitationServiceMock{
		CreateInvitationFn: func(context.Context, authsvc.InvitationRequest) (*authModels.InvitationToken, error) {
			invited = true
			return nil, nil
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
	existingMaster := &userModels.StaffMasterData{StaffID: staff.ID, AddressCity: &city}
	existingMaster.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StaffMasterData.Create(ctx, existingMaster))

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
	assert.Equal(t, userModels.EmploymentTypeFullTime, *updated.EmploymentType)

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

func TestValidateStaffMasterFields(t *testing.T) {
	t.Parallel()

	row := importModels.StaffImportRow{
		Gender: "Weiblich", EmploymentType: "Teilzeit", WeeklyHours: "19,5",
		EntryDate: "01.08.2023", ContractEndDate: "31.07.2022", Birthday: "12.05.1988",
	}
	errs := validateStaffMasterFields(&row)

	assert.Equal(t, userModels.GenderFemale, row.Gender)
	assert.Equal(t, userModels.EmploymentTypePartTime, row.EmploymentType)
	assert.Equal(t, "19.5", row.WeeklyHours)
	assert.Equal(t, "2023-08-01", row.EntryDate)
	assert.Equal(t, "1988-05-12", row.Birthday)
	require.Len(t, errs, 1)
	assert.Equal(t, "contract_end_date", errs[0].Field)

	bad := importModels.StaffImportRow{Gender: "x", EmploymentType: "Aushilfe", WeeklyHours: "90", Phone: "abc"}
	codes := map[string]bool{}
	for _, e := range validateStaffMasterFields(&bad) {
		codes[e.Code] = true
	}
	assert.True(t, codes["invalid_gender"])
	assert.True(t, codes["invalid_employment_type"])
	assert.True(t, codes["invalid_weekly_hours"])
	assert.True(t, codes["invalid_phone"])
}

func TestParseStaffQualifications(t *testing.T) {
	t.Parallel()

	entries, err := ParseStaffQualifications("Erste Hilfe (01.03.2024 bis 01.03.2026); Schwimmschein (2023-05-01);  ; Fortbildung Inklusion")
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, "Erste Hilfe", entries[0].Name)
	assert.Equal(t, timezone.NewDate(2024, 3, 1), *entries[0].AcquiredOn)
	assert.Equal(t, timezone.NewDate(2026, 3, 1), *entries[0].ExpiresOn)
	assert.Equal(t, "Schwimmschein", entries[1].Name)
	assert.Nil(t, entries[1].ExpiresOn)
	assert.Equal(t, "Fortbildung Inklusion", entries[2].Name)
	assert.Nil(t, entries[2].AcquiredOn)

	_, err = ParseStaffQualifications("Erste Hilfe (gestern)")
	require.Error(t, err)
	_, err = ParseStaffQualifications("Erste Hilfe (01.03.2026 bis 01.03.2024)")
	require.Error(t, err)
}

func TestMapGuardianRole(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"":                     "",
		"Hauptsorgeberechtigt": authorize.GuardianRolePrimaryGuardian,
		"sorgeberechtigte":     authorize.GuardianRoleLegalGuardian,
		"Mitsorgeberechtigt":   authorize.GuardianRoleCoGuardian,
		"Notfallkontakt":       authorize.GuardianRoleEmergency,
		"Nur Abholung":         authorize.GuardianRolePickupOnly,
		"pickup_only":          authorize.GuardianRolePickupOnly,
		"Sozialarbeit":         authorize.GuardianRoleSocialWorker,
	}
	for in, want := range cases {
		got, ok := MapGuardianRole(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got, in)
	}
	_, ok := MapGuardianRole("Chef")
	assert.False(t, ok)
}

func newStammdatenStudentConfig(t *testing.T, db *bun.DB) (*StudentImportConfig, *repositories.Factory) {
	t.Helper()
	repos := repositories.NewFactory(db)
	config := NewStudentImportConfig(StudentImportDeps{
		PersonRepo:          repos.Person,
		StudentRepo:         repos.Student,
		GuardianRepo:        repos.GuardianProfile,
		GuardianPhoneRepo:   repos.GuardianPhoneNumber,
		RelationRepo:        repos.StudentGuardian,
		PrivacyRepo:         repos.PrivacyConsent,
		ArrivalScheduleRepo: repos.StudentArrivalSchedule,
		PickupScheduleRepo:  repos.StudentPickupSchedule,
		RFIDCardRepo:        repos.RFIDCard,
		Resolver:            NewRelationshipResolver(repos.Group, repos.Room),
	}, db)
	return config, repos
}

func TestStudentImportConfig_CreateAndUpdate_Stammdaten(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	config, repos := newStammdatenStudentConfig(t, db)
	ctx := testpkg.Ctx(t)
	card := testpkg.CreateTestRFIDCard(t, db, "IMP")
	guardianEmail := fmt.Sprintf("erz.%s@example.test", card.ID)

	row := importModels.StudentImportRow{
		FirstName: "Max", LastName: "Import", SchoolClass: "1A", Birthday: "2018-02-03", TagID: card.ID,
		AddressStreet: "Kinderweg 3", AddressPostalCode: "50667", AddressCity: "Köln",
		Guardians: []importModels.GuardianImportData{{
			FirstName: "Maria", LastName: "Import", Email: guardianEmail, RelationshipType: "Mutter",
			GuardianRole: "Nur Abholung", PickupNotes: "nur dienstags", EmergencyPriority: 2, CanPickup: true,
		}},
		PrivacyAccepted: true, DataRetentionDays: 30,
	}
	importer := testpkg.CreateTestStaff(t, db, "Import", "Admin")

	var studentID int64
	inTenantTx(t, db, func(ctx context.Context) error {
		ctx = ContextWithImporterID(ctx, importer.ID)
		require.NoError(t, config.PreloadReferenceData(ctx))
		errs := config.Validate(ctx, &row)
		for _, e := range errs {
			assert.NotEqual(t, importModels.ErrorSeverityError, e.Severity, e.Message)
		}
		id, err := config.Create(ctx, row)
		require.NoError(t, err)
		studentID = id
		return nil
	})

	student, err := repos.Student.FindByID(ctx, studentID)
	require.NoError(t, err)
	assert.Equal(t, "Kinderweg 3", *student.AddressStreet)
	person, err := repos.Person.FindByID(ctx, student.PersonID)
	require.NoError(t, err)
	require.NotNil(t, person.TagID)
	assert.Equal(t, card.ID, *person.TagID)

	rels, err := repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, rels, 1)
	assert.Equal(t, authorize.GuardianRolePickupOnly, rels[0].GuardianRole)
	assert.Equal(t, "nur dienstags", *rels[0].PickupNotes)
	assert.Equal(t, 2, rels[0].EmergencyPriority)
	assert.False(t, authorize.StudentGuardianHasPermission(rels[0], authorize.GuardianPermissionPortalAccess))

	// Update: class changed, matched via the card; only given cells change.
	update := importModels.StudentImportRow{
		FirstName: "Max", LastName: "Import", SchoolClass: "2A", TagID: card.ID, AddressCity: "Bonn",
		Guardians:         []importModels.GuardianImportData{{Email: guardianEmail, GuardianRole: "Sorgeberechtigt"}},
		PickupSchedules:   []importModels.PickupScheduleImportData{{Weekday: 1, PickupTime: "15:00"}},
		DataRetentionDays: 30,
	}
	inTenantTx(t, db, func(ctx context.Context) error {
		ctx = ContextWithImporterID(ctx, importer.ID)
		require.NoError(t, config.PreloadReferenceData(ctx))
		errs := config.Validate(ctx, &update)
		for _, e := range errs {
			assert.NotEqual(t, importModels.ErrorSeverityError, e.Severity, e.Message)
		}
		id, err := config.FindExisting(ctx, update)
		require.NoError(t, err)
		require.NotNil(t, id, "the card must resolve the child across a class change")
		assert.Equal(t, studentID, *id)
		return config.Update(ctx, *id, update)
	})

	student, err = repos.Student.FindByID(ctx, studentID)
	require.NoError(t, err)
	assert.Equal(t, "2A", student.SchoolClass)
	assert.Equal(t, "Bonn", *student.AddressCity)
	assert.Equal(t, "Kinderweg 3", *student.AddressStreet, "empty cell keeps the street")
	rels, err = repos.StudentGuardian.FindByStudentID(ctx, studentID)
	require.NoError(t, err)
	require.Len(t, rels, 1, "existing guardian is merged, not duplicated")
	assert.Equal(t, authorize.GuardianRoleLegalGuardian, rels[0].GuardianRole)
	assert.Equal(t, "nur dienstags", *rels[0].PickupNotes)
	pickup, err := repos.StudentPickupSchedule.FindByStudentIDAndWeekday(ctx, studentID, 1)
	require.NoError(t, err)
	require.NotNil(t, pickup)
	assert.Equal(t, 15, pickup.PickupTime.Hour())
}

func TestStudentImportConfig_Validate_RejectsUnknownOrTakenTag(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	config, repos := newStammdatenStudentConfig(t, db)
	ctx := testpkg.Ctx(t)

	card := testpkg.CreateTestRFIDCard(t, db, "TAKEN")
	wearer := testpkg.CreateTestPerson(t, db, "Andere", "Person")
	require.NoError(t, repos.Person.LinkToRFIDCard(ctx, wearer.ID, card.ID))

	inTenantTx(t, db, func(ctx context.Context) error {
		require.NoError(t, config.PreloadReferenceData(ctx))

		unknown := importModels.StudentImportRow{FirstName: "A", LastName: "B", SchoolClass: "1A", TagID: "GIBTESNICHT"}
		assert.Contains(t, validationCodes(config.Validate(ctx, &unknown)), "rfid_unknown")

		taken := importModels.StudentImportRow{FirstName: "A", LastName: "B", SchoolClass: "1A", TagID: card.ID}
		assert.Contains(t, validationCodes(config.Validate(ctx, &taken)), "rfid_taken")

		own := importModels.StudentImportRow{FirstName: "Andere", LastName: "Person", SchoolClass: "1A", TagID: card.ID}
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

func TestMapStudentRow_GuardianNumberingWithGaps(t *testing.T) {
	t.Parallel()

	headers := []string{"Vorname", "Nachname", "Klasse", "Erz1.Email", "Erz3.Mobil", "Erz3.Rolle", "Erz3.Abholhinweis", "Erz3.Notfallpriorität"}
	mapping := make(map[string]int, len(headers))
	for i, h := range headers {
		mapping[normalizeHeaderKey(h)] = i
	}
	mapper := NewColumnMapper(mapping, []string{"Max", "Muster", "1A", "m@example.test", "0171-1", "Nur Abholung", "nur dienstags", "2"})

	row, err := MapStudentRow(mapper)
	require.NoError(t, err)
	require.Len(t, row.Guardians, 2, "Erz3 must be read although Erz2 columns are missing")
	assert.Equal(t, "Nur Abholung", row.Guardians[1].GuardianRole)
	assert.Equal(t, "nur dienstags", row.Guardians[1].PickupNotes)
	assert.Equal(t, 2, row.Guardians[1].EmergencyPriority)
}
