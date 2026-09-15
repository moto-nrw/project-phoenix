package importpkg

import (
	"strings"
	"testing"
	"unicode/utf8"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	usersModel "github.com/moto-nrw/project-phoenix/services/import/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseDepartureDayColumns covers the unified per-day Gehweise columns (#1610).

// TestBoundedNotePtr covers the import companion-note cap (#1694): trim, drop
// empty to nil, and truncate by rune count (multibyte-safe) rather than reject.
func TestBoundedNotePtr(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
