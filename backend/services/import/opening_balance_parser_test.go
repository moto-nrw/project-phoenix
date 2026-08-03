package importpkg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Opening balance file parsing (#2132). The parser uses encoding/csv with the
// standard comma separator, so German decimals ("12,5") must be quoted in the
// file; the fixtures below mirror that.

const openingBalanceCSVHeader = "Personalnummer,Vorname,Nachname,Stundensaldo,Jahresanspruch,Vorjahresübertrag,Resturlaub\n"

func TestParseOpeningBalanceCSV_MapsColumns(t *testing.T) {
	csvData := openingBalanceCSVHeader +
		"P-100,Anna,Lehmann,\"12,5\",30,\"2\",\"17,5\"\n" +
		",Bernd,Schulz,,,,\"5\""

	rows, err := ParseOpeningBalanceCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, "P-100", rows[0].PersonnelNumber)
	assert.Equal(t, "Anna", rows[0].FirstName)
	assert.Equal(t, "Lehmann", rows[0].LastName)
	assert.Equal(t, "12,5", rows[0].HoursBalance)
	assert.Equal(t, "30", rows[0].VacationEntitled)
	assert.Equal(t, "2", rows[0].VacationCarryover)
	assert.Equal(t, "17,5", rows[0].VacationRemaining)

	// Every value column is optional — a vacation-only row is legitimate.
	assert.Empty(t, rows[1].PersonnelNumber)
	assert.Equal(t, "Bernd", rows[1].FirstName)
	assert.Empty(t, rows[1].HoursBalance)
	assert.Equal(t, "5", rows[1].VacationRemaining)
}

func TestParseOpeningBalanceCSV_HeaderIsCaseInsensitiveAndOptionalAnnotated(t *testing.T) {
	csvData := "vorname,nachname,stundensaldo (optional),resturlaub (optional)\n" +
		"Anna,Lehmann,\"12,5\",\"17,5\""

	rows, err := ParseOpeningBalanceCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "12,5", rows[0].HoursBalance)
	assert.Equal(t, "17,5", rows[0].VacationRemaining)
}

func TestParseOpeningBalanceCSV_PersonnelNumberDoesNotRequireNames(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "ohne Vorname",
			header: "Personalnummer,Nachname,Stundensaldo\nP-100,Lehmann,\"12,5\"",
		},
		{
			name:   "ohne Nachname",
			header: "Personalnummer,Vorname,Stundensaldo\nP-100,Anna,\"12,5\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := ParseOpeningBalanceCSV(strings.NewReader(tc.header))
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.Equal(t, "P-100", rows[0].PersonnelNumber)
		})
	}
}

func TestParseOpeningBalanceCSV_SkipsEmptyRows(t *testing.T) {
	csvData := openingBalanceCSVHeader +
		",,,,,,\n" +
		"P-100,Anna,Lehmann,\"12,5\",,,\n" +
		"   ,   ,   ,   ,   ,   ,   "

	rows, err := ParseOpeningBalanceCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Anna", rows[0].FirstName)
}

func TestParseOpeningBalanceCSV_TemplateWithoutDataRows(t *testing.T) {
	rows, err := ParseOpeningBalanceCSV(strings.NewReader(openingBalanceCSVHeader))

	require.Error(t, err)
	assert.Nil(t, rows)
	assert.Contains(t, err.Error(), "keine Datenzeilen")
}

func TestParseOpeningBalanceCSV_ShortRowKeepsMappedColumns(t *testing.T) {
	// Trailing columns omitted entirely (a common spreadsheet export shape).
	csvData := openingBalanceCSVHeader + "P-100,Anna,Lehmann"

	rows, err := ParseOpeningBalanceCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Anna", rows[0].FirstName)
	assert.Empty(t, rows[0].HoursBalance)
	assert.Empty(t, rows[0].VacationRemaining)
}

func TestParseOpeningBalanceXLSX_MapsColumns(t *testing.T) {
	reader := buildStaffXLSX(t, [][]string{
		{"Personalnummer", "Vorname", "Nachname", "Stundensaldo", "Jahresanspruch", "Vorjahresübertrag", "Resturlaub"},
		{"P-100", "Anna", "Lehmann", "12,5", "30", "2", "17,5"},
		{"", "", "", "", "", "", ""},
		{"", "Bernd", "Schulz", "", "", "", "5"},
	})

	rows, err := ParseOpeningBalanceXLSX(reader)
	require.NoError(t, err)
	require.Len(t, rows, 2, "the blank sheet row must be skipped")
	assert.Equal(t, "12,5", rows[0].HoursBalance)
	assert.Equal(t, "17,5", rows[0].VacationRemaining)
	assert.Equal(t, "Bernd", rows[1].FirstName)
	assert.Equal(t, "5", rows[1].VacationRemaining)
}

func TestParseOpeningBalanceXLSX_PersonnelNumberDoesNotRequireNames(t *testing.T) {
	reader := buildStaffXLSX(t, [][]string{
		{"Personalnummer", "Nachname", "Stundensaldo"},
		{"P-100", "Lehmann", "12,5"},
	})

	rows, err := ParseOpeningBalanceXLSX(reader)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "P-100", rows[0].PersonnelNumber)
}

// TestParseOpeningBalanceCSV_NegativeValuesSurviveTheSanitizer is the
// end-to-end check for the case the whole takeover exists for: a staff member
// who arrives with a MINUS on the Stundenkonto (or an overdrawn vacation
// account). The value must reach Validate parseable.
func TestParseOpeningBalanceCSV_NegativeValuesSurviveTheSanitizer(t *testing.T) {
	csvData := openingBalanceCSVHeader + "P-100,Anna,Lehmann,\"-3,25\",30,,\"-2\""

	rows, err := ParseOpeningBalanceCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "-3,25", rows[0].HoursBalance)
	assert.Equal(t, "-2", rows[0].VacationRemaining)

	c := newOpeningConfig()
	row := rows[0]
	errs := c.Validate(t.Context(), &row)

	require.Empty(t, errs, "a negative opening balance must import without a validation error")
	require.NotNil(t, row.HoursBalanceMinutes)
	assert.Equal(t, -195, *row.HoursBalanceMinutes)
	require.NotNil(t, row.VacationRemainingDays)
	assert.InDelta(t, -2, *row.VacationRemainingDays, 0.0001)
}

func TestParseOpeningBalanceCSV_RejectsMissingIdentityColumns(t *testing.T) {
	// Neither a Personalnummer nor both name columns: nothing can be matched.
	csvData := "vorname,stundensaldo\nAnna,\"12,5\""

	_, err := ParseOpeningBalanceCSV(strings.NewReader(csvData))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fehlende erforderliche Spalten")
	assert.Contains(t, err.Error(), "nachname")
}

func TestParseOpeningBalanceCSV_RejectsEmptyFile(t *testing.T) {
	_, err := ParseOpeningBalanceCSV(strings.NewReader(""))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read header")
}

func TestParseOpeningBalanceXLSX_RejectsNonExcelInput(t *testing.T) {
	_, err := ParseOpeningBalanceXLSX(strings.NewReader("Personalnummer;Vorname\nP-100;Anna"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "open excel file")
}

func TestParseOpeningBalanceXLSX_RejectsMissingIdentityColumns(t *testing.T) {
	reader := buildStaffXLSX(t, [][]string{
		{"Vorname", "Stundensaldo"},
		{"Anna", "12,5"},
	})

	_, err := ParseOpeningBalanceXLSX(reader)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fehlende erforderliche Spalten")
}

func TestParseOpeningBalanceXLSX_TemplateWithoutDataRows(t *testing.T) {
	reader := buildStaffXLSX(t, [][]string{
		{"Personalnummer", "Vorname", "Nachname", "Stundensaldo"},
	})

	_, err := ParseOpeningBalanceXLSX(reader)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "keine Datenzeilen")
}
