package departure_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func busOnMonday() departure.Plan {
	return departure.Plan{
		BusDays:       departure.BusDays{departure.PickupDayMonday: true},
		PickupDays:    departure.PickupDays{},
		DepartureDays: departure.DepartureDays{departure.PickupDayMonday: departure.DepartureBus},
		AllowedDepartureModes: departure.AllowedDepartureModes{
			departure.PickupDayMonday: []departure.DepartureMode{departure.DepartureBus},
		},
	}
}

func TestResolvePrefersTheSuppliedModesWhenTheLegacyMirrorsAreUnchanged(t *testing.T) {
	t.Parallel()

	current := busOnMonday()
	// The caller moved the unified plan and carried the hydrated mirrors along
	// unchanged, so the unified change is the intentional one.
	write := busOnMonday()
	write.DepartureDays = departure.DepartureDays{departure.PickupDayThursday: departure.DeparturePickup}

	got := write.Resolve(&current).DepartureDays()

	assert.Equal(t, departure.DepartureAlone, got.ModeFor(departure.PickupDayMonday))
	assert.Equal(t, departure.DeparturePickup, got.ModeFor(departure.PickupDayThursday))
}

func TestResolveFoldsAChangedLegacyMirrorIntoTheStoredModes(t *testing.T) {
	t.Parallel()

	current := busOnMonday()
	// A legacy client speaks only through bus_days; its unified fields are the
	// ones it read, so the legacy move is what it actually asked for.
	write := busOnMonday()
	write.BusDays = departure.BusDays{departure.PickupDayTuesday: true}

	got := write.Resolve(&current).DepartureDays()

	assert.Equal(t, departure.DepartureAlone, got.ModeFor(departure.PickupDayMonday))
	assert.Equal(t, departure.DepartureBus, got.ModeFor(departure.PickupDayTuesday))
}

func TestResolveOnACreateReadsTheSuppliedProjection(t *testing.T) {
	t.Parallel()

	unified := departure.Plan{
		DepartureDays: departure.DepartureDays{departure.PickupDayWednesday: departure.DeparturePickup},
	}
	assert.Equal(t, departure.DeparturePickup,
		unified.Resolve(nil).DepartureDays().ModeFor(departure.PickupDayWednesday))

	legacy := departure.Plan{BusDays: departure.BusDays{departure.PickupDayFriday: true}}
	assert.Equal(t, departure.DepartureBus,
		legacy.Resolve(nil).DepartureDays().ModeFor(departure.PickupDayFriday))
}

func TestResolveReadsTheLegacyStatusStringWhenNoPickupMapIsSupplied(t *testing.T) {
	t.Parallel()

	status := departure.PickupStatusPickedUp
	write := departure.Plan{PickupStatus: &status}

	got := write.Resolve(nil)

	assert.True(t, got.HasMode(departure.DeparturePickup),
		"the oldest projection still names a pickup child")
}

func TestUntouchedPlanStaysUntouched(t *testing.T) {
	t.Parallel()

	var empty departure.Plan
	assert.False(t, empty.Touched(), "a caller that loaded no plan must not rewrite one")
	assert.Equal(t, empty, empty.Align(nil), "aligning nothing changes nothing")
}

// A caller that never touched the plan carries the hydrated fields unchanged.
// Rebasing them onto the freshly locked state is what keeps its unrelated
// write from reverting a companion edit that committed in between.
func TestRebaseMovesUntouchedFieldsOntoTheLockedState(t *testing.T) {
	t.Parallel()

	baseline := busOnMonday()
	current := departure.Plan{
		BusDays:       departure.BusDays{departure.PickupDayMonday: true},
		PickupDays:    departure.PickupDays{},
		DepartureDays: departure.DepartureDays{departure.PickupDayMonday: departure.DepartureAccompanied},
		AllowedDepartureModes: departure.AllowedDepartureModes{
			departure.PickupDayMonday: []departure.DepartureMode{departure.DepartureAccompanied},
		},
	}
	write := busOnMonday()

	rebased := write.Rebase(&baseline, &current)

	assert.True(t, departure.DaysEqual(current.DepartureDays, rebased.DepartureDays),
		"the untouched plan now reads the committed state")
	require.True(t, departure.AllowedModesEqual(current.AllowedDepartureModes, rebased.AllowedDepartureModes))
}

func TestRebaseLeavesAnIntentionalChangeAlone(t *testing.T) {
	t.Parallel()

	baseline := busOnMonday()
	current := busOnMonday()
	write := busOnMonday()
	write.DepartureDays = departure.DepartureDays{departure.PickupDayTuesday: departure.DepartureBus}

	rebased := write.Rebase(&baseline, &current)

	assert.Equal(t, departure.DepartureBus, rebased.DepartureDays.ModeFor(departure.PickupDayTuesday),
		"a field the caller really changed still wins over the stored state")
}

func TestRebaseWithoutABaselineTakesTheWriteAtFaceValue(t *testing.T) {
	t.Parallel()

	current := busOnMonday()
	write := busOnMonday()

	assert.Equal(t, write, write.Rebase(nil, &current))
	assert.Equal(t, write, write.Rebase(&current, nil))
}

// Align exists so a validation sees the plan that will be persisted. A legacy
// client removing the accompanied mode through DepartureDays must not be
// judged against the stale mode set it carried along.
func TestAlignRewritesEveryProjectionFromTheResolvedModes(t *testing.T) {
	t.Parallel()

	current := departure.Plan{
		BusDays:       departure.BusDays{},
		PickupDays:    departure.PickupDays{},
		DepartureDays: departure.DepartureDays{departure.PickupDayMonday: departure.DepartureAccompanied},
		AllowedDepartureModes: departure.AllowedDepartureModes{
			departure.PickupDayMonday: []departure.DepartureMode{departure.DepartureAccompanied},
		},
	}
	write := current
	write.DepartureDays = departure.DepartureDays{departure.PickupDayMonday: departure.DepartureAlone}

	aligned := write.Align(&current)

	assert.False(t, aligned.AllowedDepartureModes.HasMode(departure.DepartureAccompanied),
		"the mode set follows the projection the caller actually moved")
	assert.Equal(t, departure.DepartureAlone, aligned.DepartureDays.ModeFor(departure.PickupDayMonday))
}

// Effective is the read-side precedence every hydrated row passes through.
func TestEffectivePrefersTheStoredModeSet(t *testing.T) {
	t.Parallel()

	stored := departure.Plan{
		AllowedDepartureModes: departure.AllowedDepartureModes{
			departure.PickupDayMonday: []departure.DepartureMode{departure.DepartureBus},
		},
		// Deliberately contradictory: the mode set is authoritative.
		DepartureDays: departure.DepartureDays{departure.PickupDayFriday: departure.DeparturePickup},
		BusDays:       departure.BusDays{departure.PickupDayTuesday: true},
	}

	got := stored.Effective()

	assert.Equal(t, departure.DepartureBus, got.DepartureDays.ModeFor(departure.PickupDayMonday))
	assert.True(t, got.BusDays[departure.PickupDayMonday])
	assert.False(t, got.BusDays[departure.PickupDayTuesday])
	assert.Equal(t, departure.DepartureAlone, got.DepartureDays.ModeFor(departure.PickupDayFriday))
}

func TestEffectiveFallsBackToDepartureDaysThenTheLegacyMaps(t *testing.T) {
	t.Parallel()

	unified := departure.Plan{
		DepartureDays: departure.DepartureDays{departure.PickupDayWednesday: departure.DeparturePickup},
		BusDays:       departure.BusDays{departure.PickupDayMonday: true},
	}
	got := unified.Effective()
	assert.True(t, got.PickupDays[departure.PickupDayWednesday])
	assert.False(t, got.BusDays[departure.PickupDayMonday],
		"departure_days is authoritative once it carries a non-alone day")

	// An empty departure_days cannot tell "alone every day" from "not
	// backfilled", so a row written straight to bus_days still reads as one.
	legacy := departure.Plan{
		DepartureDays: departure.DepartureDays{},
		BusDays:       departure.BusDays{departure.PickupDayMonday: true},
	}
	assert.Equal(t, departure.DepartureBus,
		legacy.Effective().DepartureDays.ModeFor(departure.PickupDayMonday))
}

func TestEffectiveOnAnEmptyRowIsAllAlone(t *testing.T) {
	t.Parallel()

	got := departure.Plan{}.Effective()

	assert.False(t, got.AllowedDepartureModes.HasAny())
	for _, day := range departure.PickupDayOrder {
		assert.Equal(t, departure.DepartureAlone, got.DepartureDays.ModeFor(day))
	}
}
