package enrollment

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Pure-function tests for resolveManualSelectedDays — no DB required, so
// they run in every CI invocation regardless of test DB availability.
// resolveManualSelectedDays is the contract enforced at submit time:
// parent-supplied day picks are validated against the offering's day mode
// + available_days. The decision service also relies on the dedup pass.
// The "parent-choice offering needs at least one pick" rule lives at the
// caller (materializeOfferingSelections) via
// errParentChoiceOfferingMissingDays.

func fixedOffering(days ...string) *enrollmentModels.CareOffering {
	return &enrollmentModels.CareOffering{
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  days,
	}
}

func parentChoiceOffering(days ...string) *enrollmentModels.CareOffering {
	return &enrollmentModels.CareOffering{
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  days,
	}
}

func TestResolveSelectedDays_FixedWithoutPicksReturnsNil(t *testing.T) {
	got, err := resolveManualSelectedDays(fixedOffering("mon", "tue"), nil)
	require.NoError(t, err)
	assert.Nil(t, got, "fixed offering writes nil so RC row reflects the offering's live schedule")
}

func TestResolveSelectedDays_FixedRejectsParentPicks(t *testing.T) {
	_, err := resolveManualSelectedDays(fixedOffering("mon", "tue"), []string{"mon"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fixed")
}

// Empty picks on a parent-choice offering pass resolveManualSelectedDays
// (it returns an empty set) — the rejection is the caller's job, using the
// errParentChoiceOfferingMissingDays sentinel. Pin both halves of that
// contract here.
func TestResolveSelectedDays_ParentChoiceEmptyPicksRejected(t *testing.T) {
	got, err := resolveManualSelectedDays(parentChoiceOffering("mon", "tue", "wed"), nil)
	require.NoError(t, err)
	assert.Empty(t, got)

	require.Error(t, errParentChoiceOfferingMissingDays)
	assert.Contains(t, errParentChoiceOfferingMissingDays.Error(), "at least one day")
}

func TestResolveSelectedDays_ParentChoiceSubsetOK(t *testing.T) {
	got, err := resolveManualSelectedDays(parentChoiceOffering("mon", "tue", "wed", "thu", "fri"), []string{"tue", "thu"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tue", "thu"}, got)
}

func TestResolveSelectedDays_ParentChoiceRejectsOutOfRangeDay(t *testing.T) {
	_, err := resolveManualSelectedDays(parentChoiceOffering("mon", "tue"), []string{"mon", "sat"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sat")
}

func TestResolveSelectedDays_ParentChoiceDedupes(t *testing.T) {
	got, err := resolveManualSelectedDays(parentChoiceOffering("mon", "tue", "wed"), []string{"mon", "mon", "tue"})
	require.NoError(t, err)
	assert.Equal(t, []string{"mon", "tue"}, got, "duplicate parent picks must collapse")
}

// Day-validation failures on parent input must map to a 400 at the HTTP
// layer. mapSubmitError does that by matching ErrInvalidSubmission, so
// these errors have to wrap it — otherwise a bad day pick surfaces as a
// 500 (looks like a server bug, and the frontend has no German message).
func TestResolveSelectedDays_ParentInputErrorsWrapInvalidSubmission(t *testing.T) {
	cases := map[string]func() error{
		"subset violation": func() error {
			_, err := resolveManualSelectedDays(parentChoiceOffering("mon", "tue"), []string{"sat"})
			return err
		},
		"missing picks": func() error {
			// Rejected by the caller via the sentinel, not by
			// resolveManualSelectedDays itself.
			return errParentChoiceOfferingMissingDays
		},
		"fixed rejects picks": func() error {
			_, err := resolveManualSelectedDays(fixedOffering("mon"), []string{"mon"})
			return err
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			err := run()
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidSubmission),
				"parent day-validation error must wrap ErrInvalidSubmission so it maps to 400, got %q", err)
		})
	}
}

// The three parent-input errors additionally carry their own sentinel so
// the HTTP layer can attach stable codes for localized messages (#1885).
func TestResolveSelectedDays_ParentInputErrorsCarrySpecificSentinels(t *testing.T) {
	_, err := resolveManualSelectedDays(parentChoiceOffering("mon", "tue"), []string{"sat"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrSelectedDayNotAvailable),
		"subset violation must wrap ErrSelectedDayNotAvailable, got %q", err)

	assert.True(t, errors.Is(errParentChoiceOfferingMissingDays, ErrDaySelectionRequired))

	_, err = resolveManualSelectedDays(fixedOffering("mon"), []string{"mon"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDaySelectionNotAllowed),
		"fixed-mode picks must wrap ErrDaySelectionNotAllowed, got %q", err)
}

func TestResolveSelectedDays_RejectsUnknownMode(t *testing.T) {
	bogus := &enrollmentModels.CareOffering{
		DaysOfWeekMode: "weird",
		AvailableDays:  []string{"mon"},
	}
	_, err := resolveManualSelectedDays(bogus, []string{"mon"})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "unknown days_of_week_mode"))
}
