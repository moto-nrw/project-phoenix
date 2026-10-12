package fileformat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// iservHeader is the column layout of a real IServ user export (#1446): the
// columns the user list showed, in that order. All values below are made up.
var iservHeader = []string{
	"Account", "Vorname", "Nachname", "Status", "Erstellt am", "Erstellt von",
	"Interne ID", "Benutzertyp", "Import-ID", "Klasse/Information",
	"Geburtsdatum", "E-Mail-Adresse", "Gruppen",
}

func iservRow(first, last, status, userType, class, birthday string) []string {
	account := strings.ToLower(first + "." + last)
	return []string{
		account, first, last, status, "24.09.2025", "Admin Beispiel",
		"0000-uuid", userType, "", class, birthday,
		account + "@schule.example", "Klasse " + class,
	}
}

// iservExportXLSX writes the export as IServ users see it in Excel: the file
// name in the first row, the header in the second.
func iservExportXLSX(t *testing.T, rows ...[]string) *importModels.StudentFile {
	t.Helper()
	buf := createTestXLSX(t, []string{"Benutzer_schule.example_19.05.2026"}, append([][]string{iservHeader}, rows...))
	file, err := NewXLSXParser().ParseStudentFile(buf, importModels.StudentFileOptions{})
	require.NoError(t, err)
	return &file
}

func TestIServExport_XLSXTitleRowAndPupilPreselection(t *testing.T) {
	t.Parallel()

	file := iservExportXLSX(t,
		iservRow("Mia", "Beispiel", "aktiviert", "Schüler(innen)", "1a", "15.08.2019"),
		iservRow("Paul", "Muster", "aktiviert", "Schüler(innen)", "2b", ""),
		iservRow("Lena", "Lehrkraft", "aktiviert", "Lehrer(innen)", "LEH", ""),
		iservRow("Sven", "Admin", "aktiviert", "", "", ""),
		iservRow("Tom", "Weg", "deaktiviert", "Schüler(innen)", "4c", ""),
	)

	require.Len(t, file.Rows, 2, "only the active pupils are imported")
	assert.Equal(t, "Mia", file.Rows[0].FirstName)
	assert.Equal(t, "Beispiel", file.Rows[0].LastName)
	assert.Equal(t, "1a", file.Rows[0].SchoolClass, "Klasse/Information is the class")
	assert.Equal(t, "15.08.2019", file.Rows[0].Birthday, "Geburtsdatum is the birthday")
	assert.Empty(t, file.Rows[0].GroupName, "IServ groups are no moto groups")
	assert.Equal(t, "2b", file.Rows[1].SchoolClass)

	require.NotNil(t, file.IServ)
	assert.Equal(t, 1, file.IServ.DeactivatedCount)
	assert.Equal(t, []importModels.IServUserType{
		{Value: "Schüler(innen)", Count: 2, Selected: true},
		{Value: "", Count: 1, Selected: false},
		{Value: "Lehrer(innen)", Count: 1, Selected: false},
	}, file.IServ.UserTypes)
}

func TestIServExport_ChosenUserTypes(t *testing.T) {
	t.Parallel()

	rows := [][]string{
		iservHeader,
		iservRow("Mia", "Beispiel", "aktiviert", "Kinder", "1a", ""),
		iservRow("Sven", "Admin", "aktiviert", "", "", ""),
		iservRow("Lena", "Lehrkraft", "aktiviert", "Personal", "LEH", ""),
	}

	t.Run("no pupil type name preselects nothing", func(t *testing.T) {
		t.Parallel()
		file, err := NewXLSXParser().ParseStudentFile(createTestXLSX(t, rows[0], rows[1:]), importModels.StudentFileOptions{})
		require.NoError(t, err, "an export without a chosen type is no error, the preview asks")
		assert.Empty(t, file.Rows)
		require.NotNil(t, file.IServ)
		assert.Len(t, file.IServ.UserTypes, 3)
	})

	t.Run("chosen types are imported, the empty type included", func(t *testing.T) {
		t.Parallel()
		file, err := NewXLSXParser().ParseStudentFile(createTestXLSX(t, rows[0], rows[1:]),
			importModels.StudentFileOptions{IServUserTypes: []string{"Kinder", ""}})
		require.NoError(t, err)
		require.Len(t, file.Rows, 2)
		assert.Equal(t, "Mia", file.Rows[0].FirstName)
		assert.Equal(t, "Sven", file.Rows[1].FirstName)
	})

	t.Run("an empty choice imports nobody", func(t *testing.T) {
		t.Parallel()
		file, err := NewXLSXParser().ParseStudentFile(createTestXLSX(t, rows[0], rows[1:]),
			importModels.StudentFileOptions{IServUserTypes: []string{}})
		require.NoError(t, err)
		assert.Empty(t, file.Rows)
		for _, userType := range file.IServ.UserTypes {
			assert.False(t, userType.Selected, userType.Value)
		}
	})
}

func TestIServExport_CSVSemicolonWindows1252(t *testing.T) {
	t.Parallel()

	text := "Vorname;Nachname;Status;Benutzertyp;Klasse/Information;Geburtsdatum\r\n" +
		"Jörg;Müller;aktiviert;Schüler;3a;01.02.2018\r\n" +
		"Anna;Lehrerin;aktiviert;Lehrer;ANN;\r\n"
	encoded, err := charmap.Windows1252.NewEncoder().String(text)
	require.NoError(t, err)

	file, err := NewCSVParser().ParseStudentFile(strings.NewReader(encoded), importModels.StudentFileOptions{})
	require.NoError(t, err)
	require.Len(t, file.Rows, 1)
	assert.Equal(t, "Jörg", file.Rows[0].FirstName)
	assert.Equal(t, "Müller", file.Rows[0].LastName)
	assert.Equal(t, "3a", file.Rows[0].SchoolClass)
	assert.Equal(t, "01.02.2018", file.Rows[0].Birthday)
	require.NotNil(t, file.IServ)
	assert.ElementsMatch(t, []importModels.IServUserType{
		{Value: "Schüler", Count: 1, Selected: true},
		{Value: "Lehrer", Count: 1, Selected: false},
	}, file.IServ.UserTypes)
}

func TestStudentFile_TemplateIsNoIServExport(t *testing.T) {
	t.Parallel()

	csv := "\uFEFFVorname,Nachname,Klasse\nMax,Mustermann,1A\n"
	file, err := NewCSVParser().ParseStudentFile(strings.NewReader(csv), importModels.StudentFileOptions{})
	require.NoError(t, err)
	assert.Nil(t, file.IServ)
	require.Len(t, file.Rows, 1)
	assert.Equal(t, "Max", file.Rows[0].FirstName, "a byte order mark does not hide the first column")
}

func TestStudentFile_HeaderSearchStaysBounded(t *testing.T) {
	t.Parallel()

	// A header below the allowed title rows is not searched for: the first
	// row stays the header and the usual errors follow.
	lines := strings.Repeat("Titel\n", maxTitleRows+1) + "Vorname,Nachname,Klasse\nMax,Mustermann,1A\n"
	rows, err := NewCSVParser().ParseStudents(strings.NewReader(lines))
	require.NoError(t, err)
	assert.Empty(t, rows[0].FirstName)
}
