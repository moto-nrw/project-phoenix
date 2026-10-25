package listexport

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

func TestCSVExportIsAFlatTableForSpreadsheets(t *testing.T) {
	t.Parallel()
	doc := listrecords.Document{
		Title:   "Ferienanmeldungen",
		Filters: []string{"Status: Bestätigt"},
		Columns: []listrecords.Column{{ID: "name", Label: "Name"}, {ID: "note", Label: "Hinweis; Zeitmodell"}},
		Rows: []listrecords.Row{
			{GroupTitle: "Woche 1"},
			{Values: map[listrecords.ColumnID]string{"name": "Muster, Lia", "note": "=HYPERLINK(\"x\")"}},
			{Values: map[listrecords.ColumnID]string{"name": listrecords.StrongMarker + "Kim", "note": "bis 13 Uhr"}},
		},
	}

	file, err := NewService().Render(doc, listrecords.FormatCSV, "Ferien Anmeldungen")
	require.NoError(t, err)

	assert.Equal(t, "text/csv; charset=utf-8", file.ContentType)
	assert.Equal(t, "ferien-anmeldungen.csv", file.Filename)
	text := string(file.Data)
	require.True(t, strings.HasPrefix(text, "\xEF\xBB\xBF"), "German spreadsheet programs need the byte order mark")
	assert.Equal(t, []string{
		`Name;"Hinweis; Zeitmodell"`,
		`Muster, Lia;"'=HYPERLINK(""x"")"`,
		`Kim;bis 13 Uhr`,
	}, strings.Split(strings.TrimSuffix(strings.TrimPrefix(text, "\xEF\xBB\xBF"), "\n"), "\n"),
		"title, filters and group headings stay out; a formula is defused and style markers are removed")
}
