package importpkg

import (
	"strings"
	"testing"
	"unicode/utf8"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseDepartureDayColumns covers the unified per-day Gehweise columns (#1610).
func TestParseDepartureDayColumns(t *testing.T) {
	t.Run("returns nil when no Gehweise column has a value", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"gehweise.mo": 0, "gehweise.di": 1},
			[]string{"", ""},
		)
		days, err := parseDepartureDayColumns(mapper)
		require.NoError(t, err)
		assert.Nil(t, days)
	})

	t.Run("maps German and canonical values to modes", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"gehweise.mo": 0, "gehweise.di": 1, "gehweise.mi": 2, "gehweise.do": 3},
			[]string{"alleine", "Fährt Bus", "wird abgeholt", "pickup"},
		)
		days, err := parseDepartureDayColumns(mapper)
		require.NoError(t, err)
		require.NotNil(t, days)
		assert.Equal(t, "alone", days["mon"])
		assert.Equal(t, "bus", days["tue"])
		assert.Equal(t, "pickup", days["wed"])
		assert.Equal(t, "pickup", days["thu"])
	})

	t.Run("maps the accompanied mode (Mit anderem Kind) values", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"gehweise.mo": 0, "gehweise.di": 1, "gehweise.mi": 2},
			[]string{"Mit anderem Kind", "begleitet", "accompanied"},
		)
		days, err := parseDepartureDayColumns(mapper)
		require.NoError(t, err)
		require.NotNil(t, days)
		assert.Equal(t, string(usersModel.DepartureAccompanied), days["mon"])
		assert.Equal(t, string(usersModel.DepartureAccompanied), days["tue"])
		assert.Equal(t, string(usersModel.DepartureAccompanied), days["wed"])
	})

	t.Run("rejects unrecognized cell values", func(t *testing.T) {
		mapper := NewColumnMapper(map[string]int{"gehweise.mo": 0}, []string{"taxi"})
		days, err := parseDepartureDayColumns(mapper)
		require.Error(t, err)
		assert.Nil(t, days)
		assert.Contains(t, err.Error(), "gehweise.mo")
		assert.Contains(t, err.Error(), "taxi")
	})

	t.Run("MapStudentRow returns invalid Gehweise value as row error", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"vorname": 0, "nachname": 1, "klasse": 2, "gehweise.di": 3},
			[]string{"Max", "Muster", "1a", "buss"},
		)
		_, err := MapStudentRow(mapper)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "gehweise.di")
		assert.Contains(t, err.Error(), "buss")
	})

	t.Run("MapStudentRow reads the Begleitung companion note (#1694)", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"vorname": 0, "nachname": 1, "klasse": 2, "gehweise.mo": 3, "begleitung": 4},
			[]string{"Max", "Muster", "1a", "mit anderem Kind", "Geschwisterkind Lena"},
		)
		row, err := MapStudentRow(mapper)
		require.NoError(t, err)
		assert.Equal(t, "Geschwisterkind Lena", row.DepartureCompanionNote)
	})
}

// TestBoundedNotePtr covers the import companion-note cap (#1694): trim, drop
// empty to nil, and truncate by rune count (multibyte-safe) rather than reject.
func TestBoundedNotePtr(t *testing.T) {
	t.Run("empty and whitespace become nil", func(t *testing.T) {
		assert.Nil(t, boundedNotePtr(""))
		assert.Nil(t, boundedNotePtr("   "))
	})

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		got := boundedNotePtr("  Geschwisterkind Lena  ")
		require.NotNil(t, got)
		assert.Equal(t, "Geschwisterkind Lena", *got)
	})

	t.Run("truncates to the rune cap without splitting multibyte runes", func(t *testing.T) {
		in := strings.Repeat("ä", usersModel.MaxDepartureCompanionNoteLen+50)
		got := boundedNotePtr(in)
		require.NotNil(t, got)
		assert.Equal(t, usersModel.MaxDepartureCompanionNoteLen, utf8.RuneCountInString(*got))
		assert.True(t, utf8.ValidString(*got))
	})
}

// TestDeparturePlanFromImportRow covers the unified-vs-legacy precedence (#1610):
// the new Gehweise columns win; otherwise the legacy Bus(.Mo..Fr) + Abholstatus
// columns are folded so old templates keep importing.
func TestDeparturePlanFromImportRow(t *testing.T) {
	t.Run("Gehweise columns take precedence", func(t *testing.T) {
		row := importModels.StudentImportRow{
			DepartureDays: map[string]string{"mon": "bus", "wed": "pickup"},
			// Legacy fields that must be ignored when Gehweise is present.
			BusPermission: true,
			PickupStatus:  usersModel.PickupStatusPickedUp,
		}
		got := departurePlanFromImportRow(row)
		assert.Equal(t, usersModel.DepartureBus, got.ModeFor("mon"))
		assert.Equal(t, usersModel.DeparturePickup, got.ModeFor("wed"))
		assert.Equal(t, usersModel.DepartureAlone, got.ModeFor("fri"))
	})

	t.Run("legacy Abholstatus folds to pickup on all weekdays", func(t *testing.T) {
		row := importModels.StudentImportRow{PickupStatus: usersModel.PickupStatusPickedUp}
		got := departurePlanFromImportRow(row)
		assert.Equal(t, usersModel.DeparturePickup, got.ModeFor("mon"))
		assert.Equal(t, usersModel.DeparturePickup, got.ModeFor("fri"))
	})

	t.Run("legacy per-day Bus folds to bus; pickup wins on contradiction", func(t *testing.T) {
		row := importModels.StudentImportRow{
			BusDays:      map[string]bool{"mon": true, "tue": true},
			PickupStatus: usersModel.PickupStatusPickedUp, // all weekdays picked up
		}
		got := departurePlanFromImportRow(row)
		// Monday is both bus and pickup in the legacy data → pickup wins.
		assert.Equal(t, usersModel.DeparturePickup, got.ModeFor("mon"))
	})
}
