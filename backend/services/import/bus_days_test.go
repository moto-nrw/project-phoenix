package importpkg

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/dataimport/fileformat"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	usersModel "github.com/moto-nrw/project-phoenix/services/import/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseBusDayColumns covers the optional per-day Buskind columns (#1582).

// TestBusDaysFromImportRow covers the legacy-vs-per-day precedence (#1582).
func TestBusDaysFromImportRow(t *testing.T) {
	t.Parallel()

	t.Run("legacy Bus=true maps to all weekdays", func(t *testing.T) {
		row := importModels.StudentImportRow{BusPermission: true}
		got := busDaysFromImportRow(row)
		assert.Equal(t, usersModel.BusDaysFromLegacyFlag(true), got)
	})

	t.Run("legacy Bus=false maps to no days", func(t *testing.T) {
		row := importModels.StudentImportRow{BusPermission: false}
		assert.False(t, busDaysFromImportRow(row).HasAny())
	})

	t.Run("per-day columns take precedence over the legacy flag", func(t *testing.T) {
		// Legacy says true (all days) but the per-day columns scope it to Mo+Fr.
		row := importModels.StudentImportRow{
			BusPermission: true,
			BusDays: map[string]bool{
				usersModel.BusDayMonday:  true,
				usersModel.BusDayFriday:  true,
				usersModel.BusDayTuesday: false,
			},
		}
		got := busDaysFromImportRow(row)
		assert.Equal(t, usersModel.BusDays{
			usersModel.BusDayMonday: true,
			usersModel.BusDayFriday: true,
		}, got)
	})
}

// TestMapStudentRow_BusDays wires the CSV columns through MapStudentRow.
func TestMapStudentRow_BusDays(t *testing.T) {
	t.Parallel()

	t.Run("legacy Bus column sets BusPermission and leaves BusDays nil", func(t *testing.T) {
		mapper := fileformat.NewColumnMapper(
			map[string]int{"vorname": 0, "nachname": 1, "bus": 2},
			[]string{"Max", "Mustermann", "Ja"},
		)
		row, err := fileformat.MapStudentRow(mapper)
		require.NoError(t, err)
		assert.True(t, row.BusPermission)
		assert.Nil(t, row.BusDays)
		// Resolves to all weekdays at persistence time.
		assert.Equal(t, usersModel.BusDaysFromLegacyFlag(true), busDaysFromImportRow(row))
	})

	t.Run("per-day Bus.Mo/Bus.Fr columns populate BusDays", func(t *testing.T) {
		mapper := fileformat.NewColumnMapper(
			map[string]int{"vorname": 0, "nachname": 1, "bus.mo": 2, "bus.fr": 3},
			[]string{"Anna", "Schmidt", "Ja", "Ja"},
		)
		row, err := fileformat.MapStudentRow(mapper)
		require.NoError(t, err)
		require.NotNil(t, row.BusDays)
		assert.Equal(t, usersModel.BusDays{
			usersModel.BusDayMonday: true,
			usersModel.BusDayFriday: true,
		}, busDaysFromImportRow(row))
	})

	t.Run("legacy Bus=Ja with present-but-blank per-day headers maps to all weekdays", func(t *testing.T) {
		// New-template regression (#1580 review): the Bus.Mo–Bus.Fr headers are
		// always emitted; when the user leaves them blank, the legacy "Bus"
		// column must win — Bus=Ja → all weekdays.
		mapper := fileformat.NewColumnMapper(
			map[string]int{
				"vorname": 0, "nachname": 1, "bus": 2,
				"bus.mo": 3, "bus.di": 4, "bus.mi": 5, "bus.do": 6, "bus.fr": 7,
			},
			[]string{"Max", "Mustermann", "Ja", "", "", "", "", ""},
		)
		row, err := fileformat.MapStudentRow(mapper)
		require.NoError(t, err)
		assert.True(t, row.BusPermission)
		assert.Nil(t, row.BusDays, "blank per-day cells must not register as an override")
		assert.Equal(t, usersModel.BusDaysFromLegacyFlag(true), busDaysFromImportRow(row))
	})

	t.Run("an explicit per-day cell overrides the legacy Bus column", func(t *testing.T) {
		// Bus=Ja but only Bus.Mo filled → Monday only (per-day path wins once any
		// cell is explicit).
		mapper := fileformat.NewColumnMapper(
			map[string]int{
				"vorname": 0, "nachname": 1, "bus": 2,
				"bus.mo": 3, "bus.di": 4, "bus.mi": 5, "bus.do": 6, "bus.fr": 7,
			},
			[]string{"Max", "Mustermann", "Ja", "Ja", "", "", "", ""},
		)
		row, err := fileformat.MapStudentRow(mapper)
		require.NoError(t, err)
		assert.Equal(t, usersModel.BusDays{usersModel.BusDayMonday: true}, busDaysFromImportRow(row))
	})
}
