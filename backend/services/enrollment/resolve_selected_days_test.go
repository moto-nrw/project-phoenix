package enrollment

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// Pure-function tests for resolveSelectedDays — no DB required, so
// they run in every CI invocation regardless of test DB availability.
// resolveSelectedDays is the contract enforced at submit time: parent-
// supplied day picks are validated against the offering's day mode +
// available_days. The decision service also relies on the dedup pass.

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
	got, err := resolveSelectedDays(fixedOffering("mon", "tue"), nil)
	require.NoError(t, err)
	assert.Nil(t, got, "fixed offering writes nil so RC row reflects the offering's live schedule")
}

func TestResolveSelectedDays_FixedRejectsParentPicks(t *testing.T) {
	_, err := resolveSelectedDays(fixedOffering("mon", "tue"), []string{"mon"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fixed")
}

func TestResolveSelectedDays_ParentChoiceEmptyPicksRejected(t *testing.T) {
	_, err := resolveSelectedDays(parentChoiceOffering("mon", "tue", "wed"), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one day")
}

func TestResolveSelectedDays_ParentChoiceSubsetOK(t *testing.T) {
	got, err := resolveSelectedDays(parentChoiceOffering("mon", "tue", "wed", "thu", "fri"), []string{"tue", "thu"})
	require.NoError(t, err)
	assert.Equal(t, []string{"tue", "thu"}, got)
}

func TestResolveSelectedDays_ParentChoiceRejectsOutOfRangeDay(t *testing.T) {
	_, err := resolveSelectedDays(parentChoiceOffering("mon", "tue"), []string{"mon", "sat"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sat")
}

func TestResolveSelectedDays_ParentChoiceDedupes(t *testing.T) {
	got, err := resolveSelectedDays(parentChoiceOffering("mon", "tue", "wed"), []string{"mon", "mon", "tue"})
	require.NoError(t, err)
	assert.Equal(t, []string{"mon", "tue"}, got, "duplicate parent picks must collapse")
}

func TestResolveSelectedDays_RejectsUnknownMode(t *testing.T) {
	bogus := &enrollmentModels.CareOffering{
		DaysOfWeekMode: "weird",
		AvailableDays:  []string{"mon"},
	}
	_, err := resolveSelectedDays(bogus, []string{"mon"})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "unknown days_of_week_mode"))
}
