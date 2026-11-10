package fileformat

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"
)

func TestCSVParsersReadGermanExcel(t *testing.T) {
	t.Parallel()
	for _, format := range []string{"utf8", "bom", "semicolon", "windows1252"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			content := "Vorname,Nachname,Klasse,Rolle,Vorjahresübertrag\nJörg,Müller,1a,Betreuer,2\n"
			if format == "semicolon" || format == "windows1252" {
				content = strings.ReplaceAll(content, ",", ";")
			}
			if format == "bom" {
				content = "\ufeff" + content
			}
			data := []byte(content)
			if format == "windows1252" {
				var err error
				data, err = charmap.Windows1252.NewEncoder().Bytes(data)
				require.NoError(t, err)
			}
			students, err := NewCSVParser().ParseStudents(bytes.NewReader(data))
			require.NoError(t, err)
			require.Len(t, students, 1)
			require.Equal(t, "Jörg", students[0].FirstName)
			require.Equal(t, "Müller", students[0].LastName)
			staff, err := ParseStaffCSV(bytes.NewReader(data))
			require.NoError(t, err)
			require.Len(t, staff, 1)
			require.Equal(t, "Müller", staff[0].LastName)
			classes, err := ParseClassListCSV(bytes.NewReader(data))
			require.NoError(t, err)
			require.Len(t, classes, 1)
			require.Equal(t, "Müller", classes[0].LastName)
			balances, err := ParseOpeningBalanceCSV(bytes.NewReader(data))
			require.NoError(t, err)
			require.Len(t, balances, 1)
			require.Equal(t, "Müller", balances[0].LastName)
			require.Equal(t, "2", balances[0].VacationCarryover)
		})
	}
}
