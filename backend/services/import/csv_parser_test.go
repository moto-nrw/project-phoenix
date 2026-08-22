package importpkg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSVParser_ParseStudents_Basic(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Gruppe,Geburtstag,Datenschutz,Aufbewahrung(Tage)
Max,Mustermann,1A,Gruppe 1A,2015-08-15,Ja,30
Anna,Schmidt,2B,Gruppe 2B,2014-03-22,Nein,15`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 2)

	// Check first student
	assert.Equal(t, "Max", rows[0].FirstName)
	assert.Equal(t, "Mustermann", rows[0].LastName)
	assert.Equal(t, "1A", rows[0].SchoolClass)
	assert.Equal(t, "Gruppe 1A", rows[0].GroupName)
	assert.Equal(t, "2015-08-15", rows[0].Birthday)
	assert.True(t, rows[0].PrivacyAccepted)
	assert.Equal(t, 30, rows[0].DataRetentionDays)

	// Check second student
	assert.Equal(t, "Anna", rows[1].FirstName)
	assert.Equal(t, "Schmidt", rows[1].LastName)
	assert.False(t, rows[1].PrivacyAccepted)
	assert.Equal(t, 15, rows[1].DataRetentionDays)
}

func TestCSVParser_ParseStudents_SingleGuardian(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Vorname,Erz1.Nachname,Erz1.Email,Erz1.Telefon,Erz1.Verhältnis,Erz1.Hauptansprechpartner
Max,Mustermann,1A,Maria,Müller,maria@example.com,0123-456789,Mutter,Ja`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)

	// Check guardian
	assert.Len(t, rows[0].Guardians, 1, "Should have 1 guardian")
	guardian := rows[0].Guardians[0]
	assert.Equal(t, "Maria", guardian.FirstName)
	assert.Equal(t, "Müller", guardian.LastName)
	assert.Equal(t, "maria@example.com", guardian.Email)
	assert.Equal(t, "0123-456789", guardian.Phone)
	assert.Equal(t, "Mutter", guardian.RelationshipType)
	assert.True(t, guardian.IsPrimary)
}

func TestCSVParser_ParseStudents_MultipleGuardians(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Email,Erz1.Telefon,Erz1.Hauptansprechpartner,Erz2.Email,Erz2.Telefon,Erz2.Hauptansprechpartner,Erz3.Email
Max,Mustermann,1A,maria@example.com,111,Ja,hans@example.com,222,Nein,oma@example.com`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)

	// Should auto-detect 3 guardians
	assert.Len(t, rows[0].Guardians, 3, "Should have 3 guardians")

	// Guardian 1
	assert.Equal(t, "maria@example.com", rows[0].Guardians[0].Email)
	assert.Equal(t, "111", rows[0].Guardians[0].Phone)
	assert.True(t, rows[0].Guardians[0].IsPrimary)

	// Guardian 2
	assert.Equal(t, "hans@example.com", rows[0].Guardians[1].Email)
	assert.Equal(t, "222", rows[0].Guardians[1].Phone)
	assert.False(t, rows[0].Guardians[1].IsPrimary)

	// Guardian 3 (email only)
	assert.Equal(t, "oma@example.com", rows[0].Guardians[2].Email)
	assert.Empty(t, rows[0].Guardians[2].Phone)
}

func TestCSVParser_ParseStudents_EmptyGuardians(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Email,Erz1.Telefon,Erz2.Email,Erz2.Telefon
Max,Mustermann,1A,maria@example.com,111,,""`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)

	// Should only have 1 guardian (Erz2 is empty)
	assert.Len(t, rows[0].Guardians, 1, "Should skip empty guardian")
	assert.Equal(t, "maria@example.com", rows[0].Guardians[0].Email)
}

func TestCSVParser_ParseStudents_NoGuardians(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse
Max,Mustermann,1A`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Empty(t, rows[0].Guardians, "Should have no guardians")
}

func TestCSVParser_ParseStudents_BooleanParsing(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Datenschutz,Bus,Erz1.Hauptansprechpartner,Erz1.Email
Max,Mustermann,1A,Ja,Nein,Yes,test@example.com
Anna,Schmidt,2B,yes,no,1,test2@example.com
Tom,Test,3C,true,false,ja,test3@example.com
Lisa,Muster,4D,JA,NEIN,JA,test4@example.com`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 4)

	// Test various boolean formats
	tests := []struct {
		rowIdx          int
		privacyAccepted bool
		busPermission   bool
		guardianPrimary bool
	}{
		{0, true, false, true}, // Ja/Nein/Yes
		{1, true, false, true}, // yes/no/1
		{2, true, false, true}, // true/false/ja
		{3, true, false, true}, // JA/NEIN/JA
	}

	for _, tt := range tests {
		assert.Equal(t, tt.privacyAccepted, rows[tt.rowIdx].PrivacyAccepted, "Row %d privacy", tt.rowIdx)
		assert.Equal(t, tt.busPermission, rows[tt.rowIdx].BusPermission, "Row %d bus", tt.rowIdx)
		if len(rows[tt.rowIdx].Guardians) > 0 {
			assert.Equal(t, tt.guardianPrimary, rows[tt.rowIdx].Guardians[0].IsPrimary, "Row %d guardian primary", tt.rowIdx)
		}
	}
}

func TestCSVParser_ParseStudents_OptionalFields(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Gruppe,Gesundheitsinfo,Betreuernotizen,Zusatzinfo,RFID
Max,Mustermann,1A,,,,,
Anna,Schmidt,2B,Gruppe 2B,Allergie: Nüsse,Ruhiges Kind,Sehr gut,ABC123`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 2)

	// First row - all optional fields empty
	assert.Empty(t, rows[0].GroupName)
	assert.Empty(t, rows[0].HealthInfo)
	assert.Empty(t, rows[0].SupervisorNotes)
	assert.Empty(t, rows[0].ExtraInfo)
	assert.Empty(t, rows[0].TagID)

	// Second row - all optional fields filled
	assert.Equal(t, "Gruppe 2B", rows[1].GroupName)
	assert.Equal(t, "Allergie: Nüsse", rows[1].HealthInfo)
	assert.Equal(t, "Ruhiges Kind", rows[1].SupervisorNotes)
	assert.Equal(t, "Sehr gut", rows[1].ExtraInfo)
	assert.Equal(t, "ABC123", rows[1].TagID)
}

func TestCSVParser_ParseStudents_DefaultDataRetention(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse
Max,Mustermann,1A`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, 30, rows[0].DataRetentionDays, "Should default to 30 days")
}

func TestCSVParser_ParseStudents_CaseInsensitiveColumns(t *testing.T) {
	t.Parallel()

	csvData := `VORNAME,NACHNAME,KLASSE,erz1.email
Max,Mustermann,1A,test@example.com`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "Max", rows[0].FirstName)
	assert.Equal(t, "Mustermann", rows[0].LastName)
	assert.Len(t, rows[0].Guardians, 1)
	assert.Equal(t, "test@example.com", rows[0].Guardians[0].Email)
}

func TestCSVParser_ValidateHeader_RequiredColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		csvData   string
		wantError bool
	}{
		{
			name:      "all required columns present",
			csvData:   "Vorname,Nachname,Klasse",
			wantError: false,
		},
		{
			name:      "missing Vorname",
			csvData:   "Nachname,Klasse",
			wantError: true,
		},
		{
			name:      "missing Nachname",
			csvData:   "Vorname,Klasse",
			wantError: true,
		},
		{
			name:      "missing Klasse",
			csvData:   "Vorname,Nachname",
			wantError: true,
		},
		{
			name:      "extra columns ok",
			csvData:   "Vorname,Nachname,Klasse,Gruppe,Extra1,Extra2",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewCSVParser()
			err := parser.ValidateHeader(strings.NewReader(tt.csvData))

			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCSVParser_ParseStudents_GuardianExtensibility(t *testing.T) {
	t.Parallel()

	// Test with 5 guardians to ensure unlimited extensibility
	csvData := `Vorname,Nachname,Klasse,Erz1.Email,Erz2.Email,Erz3.Email,Erz4.Email,Erz5.Email
Max,Mustermann,1A,g1@ex.com,g2@ex.com,g3@ex.com,g4@ex.com,g5@ex.com`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Len(t, rows[0].Guardians, 5, "Should auto-detect 5 guardians")

	// Verify all guardian emails
	expectedEmails := []string{"g1@ex.com", "g2@ex.com", "g3@ex.com", "g4@ex.com", "g5@ex.com"}
	for i, guardian := range rows[0].Guardians {
		assert.Equal(t, expectedEmails[i], guardian.Email, "Guardian %d email mismatch", i+1)
	}
}

func TestCSVParser_ParseStudents_TrimSpaces(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Gruppe
  Max  ,  Mustermann  ,  1A  ,  Gruppe 1A  `

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)

	// All fields should be trimmed
	assert.Equal(t, "Max", rows[0].FirstName)
	assert.Equal(t, "Mustermann", rows[0].LastName)
	assert.Equal(t, "1A", rows[0].SchoolClass)
	assert.Equal(t, "Gruppe 1A", rows[0].GroupName)
}

func TestCSVParser_ParseStudents_ComplexGuardianData(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Vorname,Erz1.Nachname,Erz1.Email,Erz1.Telefon,Erz1.Mobil,Erz1.Verhältnis,Erz1.Hauptansprechpartner,Erz1.Notfall,Erz1.Abholberechtigt
Max,Mustermann,1A,Maria,Müller,maria@example.com,0123-456789,0176-12345678,Mutter,Ja,Ja,Ja`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Len(t, rows[0].Guardians, 1)

	guardian := rows[0].Guardians[0]
	assert.Equal(t, "Maria", guardian.FirstName)
	assert.Equal(t, "Müller", guardian.LastName)
	assert.Equal(t, "maria@example.com", guardian.Email)
	assert.Equal(t, "0123-456789", guardian.Phone)
	assert.Equal(t, "0176-12345678", guardian.MobilePhone)
	assert.Equal(t, "Mutter", guardian.RelationshipType)
	assert.True(t, guardian.IsPrimary)
	assert.True(t, guardian.IsEmergencyContact)
	assert.True(t, guardian.CanPickup)
}

func TestCSVParser_ParseStudents_PartialGuardianData(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Email,Erz1.Telefon,Erz2.Email,Erz2.Telefon
Max,Mustermann,1A,maria@example.com,,hans@example.com,`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	assert.Len(t, rows, 1)

	// Both guardians should be added (email is sufficient)
	assert.Len(t, rows[0].Guardians, 2)
	assert.Equal(t, "maria@example.com", rows[0].Guardians[0].Email)
	assert.Empty(t, rows[0].Guardians[0].Phone)
	assert.Equal(t, "hans@example.com", rows[0].Guardians[1].Email)
	assert.Empty(t, rows[0].Guardians[1].Phone)
}

func TestCSVParser_ParseStudents_EmptyRows(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse
Max,Mustermann,1A
,,
Anna,Schmidt,2B`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	// Should parse 3 rows (including the empty one)
	assert.Len(t, rows, 3)

	// Empty row should have empty fields
	assert.Empty(t, rows[1].FirstName)
	assert.Empty(t, rows[1].LastName)
	assert.Empty(t, rows[1].SchoolClass)
}

func TestCSVParser_ParseStudents_GuardianProfileFields(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Email,Erz1.Straße,Erz1.Stadt,Erz1.PLZ,Erz1.Notizen,Erz1.Sprache
Max,Mustermann,1A,maria@example.com,Musterstr. 1,Köln,50667,Allergien beachten,de`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Guardians, 1)

	g := rows[0].Guardians[0]
	assert.Equal(t, "maria@example.com", g.Email)
	assert.Equal(t, "Musterstr. 1", g.AddressStreet)
	assert.Equal(t, "Köln", g.AddressCity)
	assert.Equal(t, "50667", g.AddressPostalCode)
	assert.Equal(t, "Allergien beachten", g.Notes)
	assert.Equal(t, "de", g.LanguagePreference)
}

func TestCSVParser_ParseStudents_GuardianProfileFieldsEmpty(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Email,Erz1.Straße,Erz1.Stadt,Erz1.PLZ,Erz1.Notizen,Erz1.Sprache
Max,Mustermann,1A,maria@example.com,,,,,`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Guardians, 1)

	g := rows[0].Guardians[0]
	assert.Empty(t, g.AddressStreet)
	assert.Empty(t, g.AddressCity)
	assert.Empty(t, g.AddressPostalCode)
	assert.Empty(t, g.Notes)
	assert.Empty(t, g.LanguagePreference)
}

func TestCSVParser_ParseStudents_MultipleGuardiansWithProfileFields(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Email,Erz1.Straße,Erz1.Notizen,Erz2.Email,Erz2.Straße,Erz2.Notizen
Max,Mustermann,1A,mother@example.com,Hauptstr. 1,Mutter-Notiz,father@example.com,Nebenstr. 5,Vater-Notiz`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].Guardians, 2)

	assert.Equal(t, "Hauptstr. 1", rows[0].Guardians[0].AddressStreet)
	assert.Equal(t, "Mutter-Notiz", rows[0].Guardians[0].Notes)

	assert.Equal(t, "Nebenstr. 5", rows[0].Guardians[1].AddressStreet)
	assert.Equal(t, "Vater-Notiz", rows[0].Guardians[1].Notes)
}

func TestCSVParser_ParseStudents_PickupSchedule(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Abholung.Mo,Abholung.Mo.Notizen,Abholung.Di,Abholung.Di.Notizen,Abholung.Mi,Abholung.Mi.Notizen,Abholung.Do,Abholung.Do.Notizen,Abholung.Fr,Abholung.Fr.Notizen
Max,Mustermann,1A,16:00,Hort,15:30,,16:00,,15:30,,14:00,Frühschluss`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].PickupSchedules, 5)

	// Monday
	assert.Equal(t, 1, rows[0].PickupSchedules[0].Weekday)
	assert.Equal(t, "16:00", rows[0].PickupSchedules[0].PickupTime)
	assert.Equal(t, "Hort", rows[0].PickupSchedules[0].Notes)

	// Tuesday
	assert.Equal(t, 2, rows[0].PickupSchedules[1].Weekday)
	assert.Equal(t, "15:30", rows[0].PickupSchedules[1].PickupTime)
	assert.Empty(t, rows[0].PickupSchedules[1].Notes)

	// Friday
	assert.Equal(t, 5, rows[0].PickupSchedules[4].Weekday)
	assert.Equal(t, "14:00", rows[0].PickupSchedules[4].PickupTime)
	assert.Equal(t, "Frühschluss", rows[0].PickupSchedules[4].Notes)
}

func TestCSVParser_ParseStudents_ArrivalSchedule(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Ankunft.Mo,Ankunft.Mo.Notizen,Ankunft.Di,Ankunft.Di.Notizen,Ankunft.Mi,Ankunft.Mi.Notizen,Ankunft.Do,Ankunft.Do.Notizen,Ankunft.Fr,Ankunft.Fr.Notizen
Max,Mustermann,1A,08:00,Frühdienst,08:10,,08:00,,08:15,,08:30,Später Start`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].ArrivalSchedules, 5)

	assert.Equal(t, 1, rows[0].ArrivalSchedules[0].Weekday)
	assert.Equal(t, "08:00", rows[0].ArrivalSchedules[0].ExpectedArrival)
	assert.Equal(t, "Frühdienst", rows[0].ArrivalSchedules[0].Notes)
	assert.Equal(t, 5, rows[0].ArrivalSchedules[4].Weekday)
	assert.Equal(t, "08:30", rows[0].ArrivalSchedules[4].ExpectedArrival)
	assert.Equal(t, "Später Start", rows[0].ArrivalSchedules[4].Notes)
}

func TestCSVParser_ParseStudents_PickupSchedulePartial(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Abholung.Mo,Abholung.Mo.Notizen,Abholung.Di,Abholung.Di.Notizen,Abholung.Mi,Abholung.Mi.Notizen,Abholung.Do,Abholung.Do.Notizen,Abholung.Fr,Abholung.Fr.Notizen
Max,Mustermann,1A,16:00,,,,,,,,,`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, rows[0].PickupSchedules, 1, "should only have Monday")
	assert.Equal(t, 1, rows[0].PickupSchedules[0].Weekday)
	assert.Equal(t, "16:00", rows[0].PickupSchedules[0].PickupTime)
}

func TestCSVParser_ParseStudents_NoPickupScheduleColumns(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse
Max,Mustermann,1A`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Empty(t, rows[0].PickupSchedules, "should have no pickup schedules")
}

func TestCSVParser_ParseStudents_FullRowWithAllNewFields(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Vorname,Erz1.Nachname,Erz1.Email,Erz1.Telefon,Erz1.Verhältnis,Erz1.Hauptansprechpartner,Erz1.Notfall,Erz1.Abholberechtigt,Erz1.Straße,Erz1.Stadt,Erz1.PLZ,Erz1.Notizen,Erz1.Sprache,Ankunft.Mo,Ankunft.Mo.Notizen,Ankunft.Fr,Ankunft.Fr.Notizen,Abholung.Mo,Abholung.Mo.Notizen,Abholung.Fr,Abholung.Fr.Notizen
Max,Mustermann,1A,Maria,Müller,maria@example.com,0123-456789,Mutter,Ja,Ja,Ja,Musterstr. 1,Köln,50667,Allergien,de,08:00,Frühdienst,08:30,Später Start,16:00,Hort,14:00,Frühschluss`

	parser := NewCSVParser()
	rows, err := parser.ParseStudents(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)

	// Guardian with all fields
	require.Len(t, rows[0].Guardians, 1)
	g := rows[0].Guardians[0]
	assert.Equal(t, "Maria", g.FirstName)
	assert.Equal(t, "Müller", g.LastName)
	assert.Equal(t, "maria@example.com", g.Email)
	assert.Equal(t, "Mutter", g.RelationshipType)
	assert.True(t, g.IsPrimary)
	assert.Equal(t, "Musterstr. 1", g.AddressStreet)
	assert.Equal(t, "Köln", g.AddressCity)
	assert.Equal(t, "50667", g.AddressPostalCode)
	assert.Equal(t, "Allergien", g.Notes)
	assert.Equal(t, "de", g.LanguagePreference)

	require.Len(t, rows[0].ArrivalSchedules, 2)
	assert.Equal(t, 1, rows[0].ArrivalSchedules[0].Weekday)
	assert.Equal(t, "08:00", rows[0].ArrivalSchedules[0].ExpectedArrival)
	assert.Equal(t, "Frühdienst", rows[0].ArrivalSchedules[0].Notes)
	assert.Equal(t, 5, rows[0].ArrivalSchedules[1].Weekday)
	assert.Equal(t, "08:30", rows[0].ArrivalSchedules[1].ExpectedArrival)
	assert.Equal(t, "Später Start", rows[0].ArrivalSchedules[1].Notes)

	// Pickup schedules
	require.Len(t, rows[0].PickupSchedules, 2)
	assert.Equal(t, 1, rows[0].PickupSchedules[0].Weekday)
	assert.Equal(t, "16:00", rows[0].PickupSchedules[0].PickupTime)
	assert.Equal(t, "Hort", rows[0].PickupSchedules[0].Notes)
	assert.Equal(t, 5, rows[0].PickupSchedules[1].Weekday)
	assert.Equal(t, "14:00", rows[0].PickupSchedules[1].PickupTime)
	assert.Equal(t, "Frühschluss", rows[0].PickupSchedules[1].Notes)
}

func TestCSVParser_GetColumnMapping(t *testing.T) {
	t.Parallel()

	csvData := `Vorname,Nachname,Klasse,Erz1.Email,Erz2.Email
Max,Mustermann,1A,test1@ex.com,test2@ex.com`

	parser := NewCSVParser()
	_, err := parser.ParseStudents(strings.NewReader(csvData))
	require.NoError(t, err)

	mapping := parser.GetColumnMapping()
	assert.NotEmpty(t, mapping)

	// Check expected columns are mapped
	assert.Contains(t, mapping, "vorname")
	assert.Contains(t, mapping, "nachname")
	assert.Contains(t, mapping, "klasse")
	assert.Contains(t, mapping, "erz1.email")
	assert.Contains(t, mapping, "erz2.email")
}
