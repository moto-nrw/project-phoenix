package importpkg

import (
	"testing"

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
		assert.Nil(t, parseDepartureDayColumns(mapper))
	})

	t.Run("maps German and canonical values to modes", func(t *testing.T) {
		mapper := NewColumnMapper(
			map[string]int{"gehweise.mo": 0, "gehweise.di": 1, "gehweise.mi": 2, "gehweise.do": 3},
			[]string{"alleine", "Fährt Bus", "wird abgeholt", "pickup"},
		)
		days := parseDepartureDayColumns(mapper)
		require.NotNil(t, days)
		assert.Equal(t, "alone", days["mon"])
		assert.Equal(t, "bus", days["tue"])
		assert.Equal(t, "pickup", days["wed"])
		assert.Equal(t, "pickup", days["thu"])
	})

	t.Run("skips unrecognized cell values", func(t *testing.T) {
		mapper := NewColumnMapper(map[string]int{"gehweise.mo": 0}, []string{"taxi"})
		assert.Nil(t, parseDepartureDayColumns(mapper))
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
