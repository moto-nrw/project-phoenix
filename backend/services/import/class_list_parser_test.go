package importpkg

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// buildClassListXLSX builds an in-memory .xlsx from the given rows
// (row 0 = header).
func buildClassListXLSX(t *testing.T, rows [][]string) io.Reader {
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

func TestParseClassListCSV_MapsColumns(t *testing.T) {
	csvData := "Vorname,Nachname,Klasse\n" +
		"Zoe,Aalders,1a\n" +
		"Ben,Zorn,3c"

	rows, err := ParseClassListCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "Zoe", rows[0].FirstName)
	assert.Equal(t, "Aalders", rows[0].LastName)
	assert.Equal(t, "1a", rows[0].SchoolClass)
	assert.Equal(t, "Ben", rows[1].FirstName)
	assert.Equal(t, "3c", rows[1].SchoolClass)
}

func TestParseClassListCSV_SkipsEmptyRowsAndLowercaseHeader(t *testing.T) {
	csvData := "vorname,nachname,klasse\n" +
		"Zoe,Aalders,1a\n" +
		",,\n" +
		"Ben,Zorn,3c"

	rows, err := ParseClassListCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "Ben", rows[1].FirstName)
}

func TestParseClassListCSV_MissingColumn(t *testing.T) {
	csvData := "Vorname,Nachname\nZoe,Aalders"

	_, err := ParseClassListCSV(strings.NewReader(csvData))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fehlende erforderliche Spalten")
	assert.Contains(t, err.Error(), "klasse")
}

func TestParseClassListCSV_TemplateOnly(t *testing.T) {
	_, err := ParseClassListCSV(strings.NewReader("Vorname,Nachname,Klasse\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keine Datenzeilen")
}

func TestParseClassListCSV_EmptyFile(t *testing.T) {
	_, err := ParseClassListCSV(strings.NewReader(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read header")
}

func TestParseClassListXLSX_MapsColumns(t *testing.T) {
	reader := buildClassListXLSX(t, [][]string{
		{"Vorname", "Nachname", "Klasse"},
		{"Zoe", "Aalders", "1a"},
		{"", "", ""},
		{"Ben", "Zorn", "3c"},
	})

	rows, err := ParseClassListXLSX(reader)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "Zoe", rows[0].FirstName)
	assert.Equal(t, "Aalders", rows[0].LastName)
	assert.Equal(t, "1a", rows[0].SchoolClass)
	assert.Equal(t, "3c", rows[1].SchoolClass)
}

func TestParseClassListXLSX_MissingColumn(t *testing.T) {
	reader := buildClassListXLSX(t, [][]string{
		{"Vorname", "Klasse"},
		{"Zoe", "1a"},
	})

	_, err := ParseClassListXLSX(reader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fehlende erforderliche Spalten")
	assert.Contains(t, err.Error(), "nachname")
}

func TestParseClassListXLSX_TemplateOnly(t *testing.T) {
	reader := buildClassListXLSX(t, [][]string{
		{"Vorname", "Nachname", "Klasse"},
	})

	_, err := ParseClassListXLSX(reader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keine Datenzeilen")
}

func TestParseClassListXLSX_NoRowsAtAll(t *testing.T) {
	reader := buildClassListXLSX(t, [][]string{})

	_, err := ParseClassListXLSX(reader)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty Excel file")
}

func TestParseClassListXLSX_NotAnExcelFile(t *testing.T) {
	_, err := ParseClassListXLSX(strings.NewReader("kein excel"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open excel file")
}

// The template ships headers only, so the parsers drop no data row for what
// it is named (#2399 review round 10): a child really called "Lena Beispiel"
// or "Jonas Muster" must import like any other.
func TestParseClassListCSV_KeepsChildrenNamedLikeFormerTemplateExamples(t *testing.T) {
	csvData := "Vorname,Nachname,Klasse\n" +
		"Lena,Beispiel,1a\n" +
		"Jonas,Muster,3b\n" +
		"Zoe,Aalders,1a"

	rows, err := ParseClassListCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "Lena", rows[0].FirstName)
	assert.Equal(t, "Beispiel", rows[0].LastName)
	assert.Equal(t, "Muster", rows[1].LastName)
	assert.Equal(t, "Zoe", rows[2].FirstName)
}

func TestParseClassListXLSX_KeepsChildrenNamedLikeFormerTemplateExamples(t *testing.T) {
	reader := buildClassListXLSX(t, [][]string{
		{"Vorname", "Nachname", "Klasse"},
		{"Lena", "Beispiel", "1a"},
		{"Jonas", "Muster", "3b"},
		{"Ben", "Zorn", "3c"},
	})

	rows, err := ParseClassListXLSX(reader)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, "Beispiel", rows[0].LastName)
	assert.Equal(t, "Muster", rows[1].LastName)
	assert.Equal(t, "Ben", rows[2].FirstName)
}
