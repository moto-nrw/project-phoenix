package importpkg

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStaffCSV_MapsColumns(t *testing.T) {
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
	// Lowercase headers and an "(optional)" annotation must still map.
	csvData := "vorname,nachname,email,rolle,position (optional)\n" +
		"Anna,Lehmann,anna@example.com,Lehrer,Foo"

	rows, err := ParseStaffCSV(strings.NewReader(csvData))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Lehrer", rows[0].RoleName)
	assert.Equal(t, "Foo", rows[0].Position)
}

func TestParseStaffCSV_MissingRequiredColumn(t *testing.T) {
	// Missing the "Rolle" column must be rejected before any row is parsed.
	csvData := "Vorname,Nachname,Email\nAnna,Lehmann,anna@example.com"

	_, err := ParseStaffCSV(strings.NewReader(csvData))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rolle")
}

func TestParseStaffCSV_NoDataRows(t *testing.T) {
	_, err := ParseStaffCSV(strings.NewReader("Vorname,Nachname,Email,Rolle"))
	require.Error(t, err)
}
