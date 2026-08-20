package importpkg

import (
	"testing"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseBusDayColumns covers the optional per-day Buskind columns (#1582).
func TestParseBusDayColumns(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when no per-day column is present", func(t *testing.T) {
		mapper := NewColumnMapper(map[string]int{"bus": 0}, []string{"Ja"})
		assert.Nil(t, parseBusDayColumns(mapper))
	})

	t.Run("returns nil when per-day headers are present but all cells are blank", func(t *testing.T) {
		// The generated template always emits the Bus.Mo–Bus.Fr headers, so
		// header presence alone must not count as an override (#1580 review).
		mapping := map[string]int{
			"bus": 0, "bus.mo": 1, "bus.di": 2, "bus.mi": 3, "bus.do": 4, "bus.fr": 5,
		}
		values := []string{"Ja", "", "", "", "", ""}
		mapper := NewColumnMapper(mapping, values)
		assert.Nil(t, parseBusDayColumns(mapper))
	})

	t.Run("maps present per-day columns to canonical weekday keys", func(t *testing.T) {
		mapping := map[string]int{"bus.mo": 0, "bus.di": 1, "bus.fr": 2}
		values := []string{"Ja", "Nein", "Ja"}
		mapper := NewColumnMapper(mapping, values)

		days := parseBusDayColumns(mapper)

		require.NotNil(t, days)
		assert.True(t, days[usersModel.BusDayMonday])
		assert.False(t, days[usersModel.BusDayTuesday])
		assert.True(t, days[usersModel.BusDayFriday])
		// A column that is not present is absent from the map (not false).
		_, hasWed := days[usersModel.BusDayWednesday]
		assert.False(t, hasWed)
	})
}

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
		mapper := NewColumnMapper(
			map[string]int{"vorname": 0, "nachname": 1, "bus": 2},
			[]string{"Max", "Mustermann", "Ja"},
		)
		row, err := MapStudentRow(mapper)
		require.NoError(t, err)
		assert.True(t, row.BusPermission)
		assert.Nil(t, row.BusDays)
		// Resolves to all weekdays at persistence time.
		assert.Equal(t, usersModel.BusDaysFromLegacyFlag(true), busDaysFromImportRow(row))
	})

	t.Run("per-day Bus.Mo/Bus.Fr columns populate BusDays", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"vorname": 0, "nachname": 1, "bus.mo": 2, "bus.fr": 3},
			[]string{"Anna", "Schmidt", "Ja", "Ja"},
		)
		row, err := MapStudentRow(mapper)
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
		mapper := NewColumnMapper(
			map[string]int{
				"vorname": 0, "nachname": 1, "bus": 2,
				"bus.mo": 3, "bus.di": 4, "bus.mi": 5, "bus.do": 6, "bus.fr": 7,
			},
			[]string{"Max", "Mustermann", "Ja", "", "", "", "", ""},
		)
		row, err := MapStudentRow(mapper)
		require.NoError(t, err)
		assert.True(t, row.BusPermission)
		assert.Nil(t, row.BusDays, "blank per-day cells must not register as an override")
		assert.Equal(t, usersModel.BusDaysFromLegacyFlag(true), busDaysFromImportRow(row))
	})

	t.Run("an explicit per-day cell overrides the legacy Bus column", func(t *testing.T) {
		// Bus=Ja but only Bus.Mo filled → Monday only (per-day path wins once any
		// cell is explicit).
		mapper := NewColumnMapper(
			map[string]int{
				"vorname": 0, "nachname": 1, "bus": 2,
				"bus.mo": 3, "bus.di": 4, "bus.mi": 5, "bus.do": 6, "bus.fr": 7,
			},
			[]string{"Max", "Mustermann", "Ja", "Ja", "", "", "", ""},
		)
		row, err := MapStudentRow(mapper)
		require.NoError(t, err)
		assert.Equal(t, usersModel.BusDays{usersModel.BusDayMonday: true}, busDaysFromImportRow(row))
	})
}
