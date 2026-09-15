package importpkg

import (
	"testing"

	dataImportCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for the Stammdaten import (#2600): the staff import files real
// records, both importers support update mode, and the child import carries
// address, RFID and the guardian role preset.

func TestValidateStaffMasterFields(t *testing.T) {
	t.Parallel()

	row := importModels.StaffImportRow{
		Gender: "Weiblich", EmploymentType: "Teilzeit", WeeklyHours: "19,5",
		EntryDate: "01.08.2023", ContractEndDate: "31.07.2022", Birthday: "12.05.1988",
	}
	errs := validateStaffMasterFields(&row)

	assert.Equal(t, ports.GenderFemale, row.Gender)
	assert.Equal(t, ports.EmploymentTypePartTime, row.EmploymentType)
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

func TestMatchLinkedGuardian(t *testing.T) {
	t.Parallel()
	email := "anna@example.test"
	anna := ports.Guardian{ID: 1, FirstName: "Anna", LastName: "Muster", Email: &email}
	ben := ports.Guardian{ID: 2, FirstName: "Ben", LastName: "Muster"}
	ben2 := ports.Guardian{ID: 3, FirstName: "Ben", LastName: "Muster"}
	linked := []linkedGuardianProfile{
		{profile: anna, phones: []string{"+4915112345"}},
		{profile: ben, phones: []string{"022112345"}},
	}

	got := matchLinkedGuardian(linked, importModels.GuardianImportData{Email: "ANNA@example.test "})
	require.NotNil(t, got)
	assert.Equal(t, anna.ID, got.ID, "e-mail wins")

	got = matchLinkedGuardian(linked, importModels.GuardianImportData{
		FirstName: "Ben", LastName: "Muster",
		PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: "0221 123-45"}},
	})
	require.NotNil(t, got)
	assert.Equal(t, ben.ID, got.ID, "stored phone matches in normalized form")

	got = matchLinkedGuardian(linked, importModels.GuardianImportData{FirstName: "ben", LastName: "MUSTER"})
	require.NotNil(t, got)
	assert.Equal(t, ben.ID, got.ID, "unique name matches without contact data")

	assert.Nil(t, matchLinkedGuardian(linked, importModels.GuardianImportData{
		FirstName: "Ben", LastName: "Muster", Email: "other@example.test",
	}), "an unknown e-mail is not overridden by a name match")

	assert.Nil(t, matchLinkedGuardian(append(linked, linkedGuardianProfile{profile: ben2}),
		importModels.GuardianImportData{FirstName: "Ben", LastName: "Muster"}), "ambiguous names never match")

	assert.Equal(t, "+4915112345", normalizeImportPhone(" +49 151 123-45 "))
	assert.Equal(t, "", normalizeImportPhone("Handy"))
}

func newTestTransactions() importModels.Transactions { return dataImportCompose.NewTransactions() }

func newTestAuthorization() importModels.Authorization { return dataImportCompose.NewAuthorization() }

func newTestRolePolicy() importModels.SchoolRolePolicy {
	return dataImportCompose.NewSchoolRolePolicy()
}
