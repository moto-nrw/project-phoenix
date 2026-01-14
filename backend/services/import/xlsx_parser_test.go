package importpkg

import (
	"bytes"
	"testing"

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
	t.Run("creates new parser", func(t *testing.T) {
		parser := NewXLSXParser()
		assert.NotNil(t, parser)
	})
}

// ============================================================================
// ParseStudents Tests
// ============================================================================

func TestXLSXParser_ParseStudents(t *testing.T) {
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
			{"", "", ""},           // Empty row - should be skipped
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
// ValidateHeader Tests
// ============================================================================

func TestXLSXParser_ValidateHeader(t *testing.T) {
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
