package importpkg

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// buildStaffXLSX builds an in-memory .xlsx from the given rows (row 0 = header).
func buildStaffXLSX(t *testing.T, rows [][]string) io.Reader {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	for r, row := range rows {
		for c, val := range row {
			cell, err := excelize.CoordinatesToCellName(c+1, r+1)
			require.NoError(t, err)
			require.NoError(t, f.SetCellStr(sheet, cell, val))
		}
	}
	buf := new(bytes.Buffer)
	require.NoError(t, f.Write(buf))
	return buf
}

func TestParseStaffCSV_MapsColumns(t *testing.T) {
	t.Parallel()

	csvData := "Vorname,Nachname,Email,Rolle,Position\n" +
		"Anna,Lehmann,anna@example.com,Lehrer,Klassenlehrerin\n" +
		"Bernd,Schulz,bernd@example.com,Admin,"

	rows, err := ParseStaffCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "Anna", rows[0].FirstName)
	assert.Equal(t, "Lehmann", rows[0].LastName)
	assert.Equal(t, "anna@example.com", rows[0].Email)
	assert.Equal(t, "Lehrer", rows[0].RoleName)
	assert.Equal(t, "Klassenlehrerin", rows[0].Position)

	assert.Equal(t, "Bernd", rows[1].FirstName)
	assert.Equal(t, "Admin", rows[1].RoleName)
	assert.Equal(t, "", rows[1].Position, "empty optional position stays empty")
}

func TestParseStaffCSV_CaseInsensitiveAndOptionalAnnotations(t *testing.T) {
	t.Parallel()

	// Lowercase headers and an "(optional)" annotation must still map.
	csvData := "vorname,nachname,email,rolle,position (optional)\n" +
		"Anna,Lehmann,anna@example.com,Lehrer,Foo"

	rows, err := ParseStaffCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Lehrer", rows[0].RoleName)
	assert.Equal(t, "Foo", rows[0].Position)
}

func TestParseStaffCSV_SkipsEmptyRows(t *testing.T) {
	t.Parallel()

	csvData := "Vorname,Nachname,Email,Rolle\n" +
		",,,\n" +
		"Anna,Lehmann,anna@example.com,Betreuer\n" +
		"   ,   ,   ,   \n"

	rows, err := ParseStaffCSV(strings.NewReader(csvData))

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Anna", rows[0].FirstName)
	assert.Equal(t, "Betreuer", rows[0].RoleName)
}

func TestParseStaffCSV_MissingRequiredColumn(t *testing.T) {
	t.Parallel()

	// Missing the "Rolle" column must be rejected before any row is parsed.
	csvData := "Vorname,Nachname,Email\nAnna,Lehmann,anna@example.com"

	_, err := ParseStaffCSV(strings.NewReader(csvData))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolle")
}

func TestParseStaffCSV_NoDataRows(t *testing.T) {
	t.Parallel()

	_, err := ParseStaffCSV(strings.NewReader("Vorname,Nachname,Email,Rolle"))
	require.Error(t, err)
}

func TestParseStaffXLSX_MapsColumns(t *testing.T) {
	t.Parallel()

	reader := buildStaffXLSX(t, [][]string{
		{"Vorname", "Nachname", "Email", "Rolle", "Position"},
		{"Anna", "Lehmann", "anna@example.com", "Lehrer", "Klassenlehrerin"},
		{"Bernd", "Schulz", "bernd@example.com", "Admin", ""},
	})

	rows, err := ParseStaffXLSX(reader)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "Anna", rows[0].FirstName)
	assert.Equal(t, "anna@example.com", rows[0].Email)
	assert.Equal(t, "Lehrer", rows[0].RoleName)
	assert.Equal(t, "Klassenlehrerin", rows[0].Position)
	assert.Equal(t, "Admin", rows[1].RoleName)
	assert.Equal(t, "", rows[1].Position)
}

func TestParseStaffXLSX_SkipsEmptyRows(t *testing.T) {
	t.Parallel()

	reader := buildStaffXLSX(t, [][]string{
		{"Vorname", "Nachname", "Email", "Rolle"},
		{"", "", "", ""},
		{"Anna", "Lehmann", "anna@example.com", "Betreuer"},
		{"   ", "   ", "   ", "   "},
	})

	rows, err := ParseStaffXLSX(reader)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Anna", rows[0].FirstName)
	assert.Equal(t, "Betreuer", rows[0].RoleName)
}

func TestParseStaffXLSX_MissingRequiredColumn(t *testing.T) {
	t.Parallel()

	reader := buildStaffXLSX(t, [][]string{
		{"Vorname", "Nachname", "Email"},
		{"Anna", "Lehmann", "anna@example.com"},
	})

	_, err := ParseStaffXLSX(reader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolle")
}

func TestParseStaffXLSX_NoDataRows(t *testing.T) {
	t.Parallel()

	reader := buildStaffXLSX(t, [][]string{
		{"Vorname", "Nachname", "Email", "Rolle"},
	})

	_, err := ParseStaffXLSX(reader)
	require.Error(t, err)
}

func TestStaffImportConfig_EntityNameAndUpdate(t *testing.T) {
	t.Parallel()

	config := NewStaffImportConfig(StaffImportDeps{})

	assert.Equal(t, "Mitarbeiter", config.EntityName())
	// Update writes the Stammdatensatz (#2600); without the repositories it
	// must fail loudly instead of silently skipping the row.
	require.Error(t, config.Update(context.Background(), 0, importModels.StaffImportRow{}))
}

func TestMapStaffRow_ReadsStammdatenColumns(t *testing.T) {
	t.Parallel()

	headers := []string{"Vorname", "Nachname", "Rolle", "Email (optional)", "Personalnummer (optional)", "Geschlecht (optional)", "Wochenstunden (optional)", "Qualifikationen (optional)", "Notfallkontakt Telefon (optional)"}
	mapping := make(map[string]int, len(headers))
	for i, h := range headers {
		mapping[normalizeHeaderKey(h)] = i
	}
	mapper := NewColumnMapper(mapping, []string{"Anna", "Lehmann", "Betreuer", "", "P-7", "w", "19,5", "Erste Hilfe", "+49 171 1"})

	row := MapStaffRow(mapper)

	assert.Equal(t, "P-7", row.PersonnelNumber)
	assert.Equal(t, "w", row.Gender)
	assert.Equal(t, "19,5", row.WeeklyHours)
	assert.Equal(t, "Erste Hilfe", row.Qualifications)
	assert.True(t, row.HasQualificationsColumn)
	assert.Equal(t, "+49 171 1", row.EmergencyContactPhone, "phone columns must keep the leading +")
	assert.Empty(t, row.Email, "e-mail is optional since #2600")
}
