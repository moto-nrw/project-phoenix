package importpkg

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// createTestXLSX creates an in-memory Excel file for testing
func createTestXLSX(t *testing.T, headers []string, rows [][]string) *bytes.Buffer {
	t.Helper()

	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheetName := f.GetSheetName(0)

	// Write headers
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheetName, cell, h)
	}

	// Write data rows
	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheetName, cell, val)
		}
	}

	buf, err := f.WriteToBuffer()
	require.NoError(t, err)

	return buf
}

// ============================================================================
// NewXLSXParser Tests
// ============================================================================

func TestNewXLSXParser(t *testing.T) {
	t.Parallel()

	t.Run("creates new parser", func(t *testing.T) {
		parser := NewXLSXParser()
		assert.NotNil(t, parser)
	})
}

// ============================================================================
// ParseStudents Tests
// ============================================================================

func TestXLSXParser_ParseStudents(t *testing.T) {
	t.Parallel()

	t.Run("parses valid Excel file with student data", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse"}
		rows := [][]string{
			{"Max", "Mustermann", "1A"},
			{"Anna", "Schmidt", "2B"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		students, err := parser.ParseStudents(buf)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, students, 2)
		assert.Equal(t, "Max", students[0].FirstName)
		assert.Equal(t, "Mustermann", students[0].LastName)
		assert.Equal(t, "1A", students[0].SchoolClass)
		assert.Equal(t, "Anna", students[1].FirstName)
	})

	t.Run("parses file with optional fields", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse", "Geburtstag", "RFID", "Gruppe"}
		rows := [][]string{
			{"Max", "Mustermann", "1A", "2015-05-15", "ABC123", "Schulklasse 1A"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		students, err := parser.ParseStudents(buf)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, students, 1)
		assert.Equal(t, "2015-05-15", students[0].Birthday)
		assert.Equal(t, "ABC123", students[0].TagID)
		assert.Equal(t, "Schulklasse 1A", students[0].GroupName)
	})

	t.Run("normalizes Excel date-formatted birthday cells", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()

		sheetName := f.GetSheetName(0)
		require.NoError(t, f.SetCellValue(sheetName, "A1", "Vorname"))
		require.NoError(t, f.SetCellValue(sheetName, "B1", "Nachname"))
		require.NoError(t, f.SetCellValue(sheetName, "C1", "Klasse"))
		require.NoError(t, f.SetCellValue(sheetName, "D1", "Geburtstag"))

		require.NoError(t, f.SetCellValue(sheetName, "A2", "Markus"))
		require.NoError(t, f.SetCellValue(sheetName, "B2", "Dell"))
		require.NoError(t, f.SetCellValue(sheetName, "C2", "4A"))
		require.NoError(t, f.SetCellValue(sheetName, "D2", time.Date(2017, time.May, 14, 0, 0, 0, 0, time.UTC)))

		dateStyle, err := f.NewStyle(&excelize.Style{NumFmt: 14})
		require.NoError(t, err)
		require.NoError(t, f.SetCellStyle(sheetName, "D2", "D2", dateStyle))

		buf, err := f.WriteToBuffer()
		require.NoError(t, err)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		assert.Equal(t, "2017-05-14", students[0].Birthday)
	})

	t.Run("preserves formatted postal codes with leading zeros", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()

		sheetName := f.GetSheetName(0)
		require.NoError(t, f.SetCellValue(sheetName, "A1", "Vorname"))
		require.NoError(t, f.SetCellValue(sheetName, "B1", "Nachname"))
		require.NoError(t, f.SetCellValue(sheetName, "C1", "Klasse"))
		require.NoError(t, f.SetCellValue(sheetName, "D1", "Erz1.Email"))
		require.NoError(t, f.SetCellValue(sheetName, "E1", "Erz1.PLZ"))

		require.NoError(t, f.SetCellValue(sheetName, "A2", "Markus"))
		require.NoError(t, f.SetCellValue(sheetName, "B2", "Dell"))
		require.NoError(t, f.SetCellValue(sheetName, "C2", "4A"))
		require.NoError(t, f.SetCellValue(sheetName, "D2", "maria.mueller@example.com"))
		require.NoError(t, f.SetCellValue(sheetName, "E2", 1067))

		plzFormat := "00000"
		plzStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &plzFormat})
		require.NoError(t, err)
		require.NoError(t, f.SetCellStyle(sheetName, "E2", "E2", plzStyle))

		buf, err := f.WriteToBuffer()
		require.NoError(t, err)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		require.Len(t, students[0].Guardians, 1)
		assert.Equal(t, "01067", students[0].Guardians[0].AddressPostalCode)
	})

	t.Run("keeps suspicious early Excel birthday serials invalid", func(t *testing.T) {
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()

		sheetName := f.GetSheetName(0)
		require.NoError(t, f.SetCellValue(sheetName, "A1", "Vorname"))
		require.NoError(t, f.SetCellValue(sheetName, "B1", "Nachname"))
		require.NoError(t, f.SetCellValue(sheetName, "C1", "Klasse"))
		require.NoError(t, f.SetCellValue(sheetName, "D1", "Geburtstag"))

		require.NoError(t, f.SetCellValue(sheetName, "A2", "Markus"))
		require.NoError(t, f.SetCellValue(sheetName, "B2", "Dell"))
		require.NoError(t, f.SetCellValue(sheetName, "C2", "4A"))
		require.NoError(t, f.SetCellValue(sheetName, "D2", 42))

		dateStyle, err := f.NewStyle(&excelize.Style{NumFmt: 14})
		require.NoError(t, err)
		require.NoError(t, f.SetCellStyle(sheetName, "D2", "D2", dateStyle))

		buf, err := f.WriteToBuffer()
		require.NoError(t, err)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		assert.Equal(t, "42", students[0].Birthday)
	})

	t.Run("parses guardians", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse", "Erz1.Vorname", "Erz1.Nachname", "Erz1.Email", "Erz1.Telefon"}
		rows := [][]string{
			{"Max", "Mustermann", "1A", "Maria", "Müller", "maria@example.com", "0123-456789"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		students, err := parser.ParseStudents(buf)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, students, 1)
		require.Len(t, students[0].Guardians, 1)
		assert.Equal(t, "Maria", students[0].Guardians[0].FirstName)
		assert.Equal(t, "maria@example.com", students[0].Guardians[0].Email)
	})

	t.Run("parses data retention days", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse", "Aufbewahrung(Tage)"}
		rows := [][]string{
			{"Max", "Mustermann", "1A", "14"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		students, err := parser.ParseStudents(buf)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, students, 1)
		assert.Equal(t, 14, students[0].DataRetentionDays)
	})

	t.Run("returns error for invalid retention days", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse", "Aufbewahrung(Tage)"}
		rows := [][]string{
			{"Max", "Mustermann", "1A", "30 Tage"}, // Invalid - should be just number
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		_, err := parser.ParseStudents(buf)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ungültiger Wert")
	})

	t.Run("returns error for empty file", func(t *testing.T) {
		// ARRANGE - Create Excel with only headers
		headers := []string{"Vorname", "Nachname", "Klasse"}
		rows := [][]string{} // No data rows
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		_, err := parser.ParseStudents(buf)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "keine Datenzeilen")
	})

	t.Run("skips empty rows", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse"}
		rows := [][]string{
			{"Max", "Mustermann", "1A"},
			{"", "", ""}, // Empty row - should be skipped
			{"Anna", "Schmidt", "2B"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		students, err := parser.ParseStudents(buf)

		// ASSERT
		require.NoError(t, err)
		assert.Len(t, students, 2)
	})

	t.Run("parses boolean fields", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse", "Bus", "Datenschutz"}
		rows := [][]string{
			{"Max", "Mustermann", "1A", "Ja", "Ja"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		students, err := parser.ParseStudents(buf)

		// ASSERT
		require.NoError(t, err)
		require.Len(t, students, 1)
		assert.True(t, students[0].BusPermission)
		assert.True(t, students[0].PrivacyAccepted)
	})

	t.Run("returns error for invalid Excel data", func(t *testing.T) {
		// ARRANGE
		parser := NewXLSXParser()
		invalidData := bytes.NewBufferString("not an excel file")

		// ACT
		_, err := parser.ParseStudents(invalidData)

		// ASSERT
		require.Error(t, err)
	})
}

// ============================================================================
// Guardian Profile Fields Tests
// ============================================================================

func TestXLSXParser_ParseStudents_GuardianProfileFields(t *testing.T) {
	t.Parallel()

	t.Run("parses guardian profile fields", func(t *testing.T) {
		headers := []string{
			"Vorname", "Nachname", "Klasse",
			"Erz1.Email", "Erz1.Straße", "Erz1.Stadt", "Erz1.PLZ",
			"Erz1.Notizen", "Erz1.Sprache",
		}
		rows := [][]string{
			{"Max", "Mustermann", "1A",
				"maria@example.com", "Musterstr. 1", "Köln", "50667",
				"Allergien beachten", "de"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		require.Len(t, students[0].Guardians, 1)

		g := students[0].Guardians[0]
		assert.Equal(t, "Musterstr. 1", g.AddressStreet)
		assert.Equal(t, "Köln", g.AddressCity)
		assert.Equal(t, "50667", g.AddressPostalCode)
		assert.Equal(t, "Allergien beachten", g.Notes)
		assert.Equal(t, "de", g.LanguagePreference)
	})

	t.Run("parses multiple guardians with profile fields", func(t *testing.T) {
		headers := []string{
			"Vorname", "Nachname", "Klasse",
			"Erz1.Email", "Erz1.Straße", "Erz1.Notizen",
			"Erz2.Email", "Erz2.Straße", "Erz2.Notizen",
		}
		rows := [][]string{
			{"Max", "Mustermann", "1A",
				"mother@example.com", "Hauptstr. 1", "Mutter-Notiz",
				"father@example.com", "Nebenstr. 5", "Vater-Notiz"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		require.Len(t, students[0].Guardians, 2)

		assert.Equal(t, "Hauptstr. 1", students[0].Guardians[0].AddressStreet)
		assert.Equal(t, "Mutter-Notiz", students[0].Guardians[0].Notes)

		assert.Equal(t, "Nebenstr. 5", students[0].Guardians[1].AddressStreet)
		assert.Equal(t, "Vater-Notiz", students[0].Guardians[1].Notes)
	})
}

// ============================================================================
// Pickup Schedule Tests
// ============================================================================

func TestXLSXParser_ParseStudents_ArrivalSchedule(t *testing.T) {
	t.Parallel()

	t.Run("parses all five weekdays with notes", func(t *testing.T) {
		headers := []string{
			"Vorname", "Nachname", "Klasse",
			"Ankunft.Mo", "Ankunft.Mo.Notizen",
			"Ankunft.Di", "Ankunft.Di.Notizen",
			"Ankunft.Mi", "Ankunft.Mi.Notizen",
			"Ankunft.Do", "Ankunft.Do.Notizen",
			"Ankunft.Fr", "Ankunft.Fr.Notizen",
		}
		rows := [][]string{
			{"Max", "Mustermann", "1A",
				"08:00", "Frühdienst",
				"08:10", "",
				"08:00", "",
				"08:15", "",
				"08:30", "Später Start"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		require.Len(t, students[0].ArrivalSchedules, 5)

		assert.Equal(t, 1, students[0].ArrivalSchedules[0].Weekday)
		assert.Equal(t, "08:00", students[0].ArrivalSchedules[0].ExpectedArrival)
		assert.Equal(t, "Frühdienst", students[0].ArrivalSchedules[0].Notes)
		assert.Equal(t, 5, students[0].ArrivalSchedules[4].Weekday)
		assert.Equal(t, "08:30", students[0].ArrivalSchedules[4].ExpectedArrival)
		assert.Equal(t, "Später Start", students[0].ArrivalSchedules[4].Notes)
	})
}

func TestXLSXParser_ParseStudents_PickupSchedule(t *testing.T) {
	t.Parallel()

	t.Run("parses all five weekdays with notes", func(t *testing.T) {
		headers := []string{
			"Vorname", "Nachname", "Klasse",
			"Abholung.Mo", "Abholung.Mo.Notizen",
			"Abholung.Di", "Abholung.Di.Notizen",
			"Abholung.Mi", "Abholung.Mi.Notizen",
			"Abholung.Do", "Abholung.Do.Notizen",
			"Abholung.Fr", "Abholung.Fr.Notizen",
		}
		rows := [][]string{
			{"Max", "Mustermann", "1A",
				"16:00", "Hort",
				"15:30", "",
				"16:00", "",
				"15:30", "",
				"14:00", "Frühschluss"},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		require.Len(t, students[0].PickupSchedules, 5)

		assert.Equal(t, 1, students[0].PickupSchedules[0].Weekday)
		assert.Equal(t, "16:00", students[0].PickupSchedules[0].PickupTime)
		assert.Equal(t, "Hort", students[0].PickupSchedules[0].Notes)

		assert.Equal(t, 5, students[0].PickupSchedules[4].Weekday)
		assert.Equal(t, "14:00", students[0].PickupSchedules[4].PickupTime)
		assert.Equal(t, "Frühschluss", students[0].PickupSchedules[4].Notes)
	})

	t.Run("parses partial pickup schedule", func(t *testing.T) {
		headers := []string{
			"Vorname", "Nachname", "Klasse",
			"Abholung.Mo", "Abholung.Mo.Notizen",
			"Abholung.Di", "Abholung.Di.Notizen",
			"Abholung.Mi", "Abholung.Mi.Notizen",
			"Abholung.Do", "Abholung.Do.Notizen",
			"Abholung.Fr", "Abholung.Fr.Notizen",
		}
		rows := [][]string{
			{"Max", "Mustermann", "1A",
				"16:00", "",
				"", "",
				"15:00", "",
				"", "",
				"", ""},
		}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		require.Len(t, students[0].PickupSchedules, 2, "should only have Mon and Wed")
		assert.Equal(t, 1, students[0].PickupSchedules[0].Weekday)
		assert.Equal(t, 3, students[0].PickupSchedules[1].Weekday)
	})

	t.Run("no pickup schedule columns", func(t *testing.T) {
		headers := []string{"Vorname", "Nachname", "Klasse"}
		rows := [][]string{{"Max", "Mustermann", "1A"}}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()
		students, err := parser.ParseStudents(buf)

		require.NoError(t, err)
		require.Len(t, students, 1)
		assert.Empty(t, students[0].PickupSchedules)
	})
}

// ============================================================================
// Full Row with All New Fields
// ============================================================================

func TestXLSXParser_ParseStudents_FullRowAllNewFields(t *testing.T) {
	t.Parallel()

	headers := []string{
		"Vorname", "Nachname", "Klasse",
		"Erz1.Vorname", "Erz1.Nachname", "Erz1.Email", "Erz1.Telefon",
		"Erz1.Verhältnis", "Erz1.Hauptansprechpartner", "Erz1.Notfall", "Erz1.Abholberechtigt",
		"Erz1.Straße", "Erz1.Stadt", "Erz1.PLZ",
		"Erz1.Notizen", "Erz1.Sprache",
		"Ankunft.Mo", "Ankunft.Mo.Notizen",
		"Ankunft.Fr", "Ankunft.Fr.Notizen",
		"Abholung.Mo", "Abholung.Mo.Notizen",
		"Abholung.Fr", "Abholung.Fr.Notizen",
	}
	rows := [][]string{
		{"Max", "Mustermann", "1A",
			"Maria", "Müller", "maria@example.com", "0123-456789",
			"Mutter", "Ja", "Ja", "Ja",
			"Musterstr. 1", "Köln", "50667",
			"Allergien", "de",
			"08:00", "Frühdienst",
			"08:30", "Später Start",
			"16:00", "Hort",
			"14:00", "Frühschluss"},
	}
	buf := createTestXLSX(t, headers, rows)

	parser := NewXLSXParser()
	students, err := parser.ParseStudents(buf)

	require.NoError(t, err)
	require.Len(t, students, 1)

	// Guardian
	require.Len(t, students[0].Guardians, 1)
	g := students[0].Guardians[0]
	assert.Equal(t, "Maria", g.FirstName)
	assert.Equal(t, "maria@example.com", g.Email)
	assert.Equal(t, "Musterstr. 1", g.AddressStreet)
	assert.Equal(t, "Köln", g.AddressCity)
	assert.Equal(t, "50667", g.AddressPostalCode)
	assert.Equal(t, "Allergien", g.Notes)
	assert.Equal(t, "de", g.LanguagePreference)

	require.Len(t, students[0].ArrivalSchedules, 2)
	assert.Equal(t, 1, students[0].ArrivalSchedules[0].Weekday)
	assert.Equal(t, "08:00", students[0].ArrivalSchedules[0].ExpectedArrival)
	assert.Equal(t, "Frühdienst", students[0].ArrivalSchedules[0].Notes)
	assert.Equal(t, 5, students[0].ArrivalSchedules[1].Weekday)
	assert.Equal(t, "08:30", students[0].ArrivalSchedules[1].ExpectedArrival)
	assert.Equal(t, "Später Start", students[0].ArrivalSchedules[1].Notes)

	// Pickup
	require.Len(t, students[0].PickupSchedules, 2)
	assert.Equal(t, 1, students[0].PickupSchedules[0].Weekday)
	assert.Equal(t, "16:00", students[0].PickupSchedules[0].PickupTime)
	assert.Equal(t, "Hort", students[0].PickupSchedules[0].Notes)
	assert.Equal(t, 5, students[0].PickupSchedules[1].Weekday)
	assert.Equal(t, "14:00", students[0].PickupSchedules[1].PickupTime)
	assert.Equal(t, "Frühschluss", students[0].PickupSchedules[1].Notes)
}

// ============================================================================
// ValidateHeader Tests
// ============================================================================

func TestXLSXParser_ValidateHeader(t *testing.T) {
	t.Parallel()

	t.Run("validates complete header", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse", "Geburtstag"}
		rows := [][]string{{"Max", "Mustermann", "1A", "2015-01-01"}}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		err := parser.ValidateHeader(buf)

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("returns error for missing required columns", func(t *testing.T) {
		// ARRANGE - Missing "Vorname"
		headers := []string{"Nachname", "Klasse"}
		rows := [][]string{{"Mustermann", "1A"}}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()

		// ACT
		err := parser.ValidateHeader(buf)

		// ASSERT
		require.Error(t, err)
		assert.Contains(t, err.Error(), "vorname")
	})

	t.Run("returns error for empty file", func(t *testing.T) {
		// ARRANGE - Create empty Excel file
		f := excelize.NewFile()
		defer func() { _ = f.Close() }()
		buf, _ := f.WriteToBuffer()

		parser := NewXLSXParser()

		// ACT
		err := parser.ValidateHeader(buf)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid Excel data", func(t *testing.T) {
		// ARRANGE
		parser := NewXLSXParser()
		invalidData := bytes.NewBufferString("not an excel file")

		// ACT
		err := parser.ValidateHeader(invalidData)

		// ASSERT
		require.Error(t, err)
	})
}

// ============================================================================
// GetColumnMapping Tests
// ============================================================================

func TestXLSXParser_GetColumnMapping(t *testing.T) {
	t.Parallel()

	t.Run("returns column mapping after parsing", func(t *testing.T) {
		// ARRANGE
		headers := []string{"Vorname", "Nachname", "Klasse"}
		rows := [][]string{{"Max", "Mustermann", "1A"}}
		buf := createTestXLSX(t, headers, rows)

		parser := NewXLSXParser()
		_, err := parser.ParseStudents(buf)
		require.NoError(t, err)

		// ACT
		mapping := parser.GetColumnMapping()

		// ASSERT
		assert.NotNil(t, mapping)
		assert.Equal(t, 0, mapping["vorname"])
		assert.Equal(t, 1, mapping["nachname"])
		assert.Equal(t, 2, mapping["klasse"])
	})
}

// ============================================================================
// isEmptyRow Tests
// ============================================================================

func TestIsEmptyRow(t *testing.T) {
	t.Parallel()

	t.Run("returns true for empty row", func(t *testing.T) {
		row := []string{"", "", ""}
		assert.True(t, isEmptyRow(row))
	})

	t.Run("returns true for whitespace row", func(t *testing.T) {
		row := []string{"  ", "\t", "  "}
		assert.True(t, isEmptyRow(row))
	})

	t.Run("returns false for non-empty row", func(t *testing.T) {
		row := []string{"", "test", ""}
		assert.False(t, isEmptyRow(row))
	})

	t.Run("returns true for nil slice", func(t *testing.T) {
		assert.True(t, isEmptyRow(nil))
	})
}
