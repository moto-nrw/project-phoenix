package selection

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The per-day rule (#4514): a group with at_most_one_per_day accepts several
// variants per child, but never two on the same weekday.

const (
	shortCareID   int64 = 1 // "Verlässlichkeit bis 13 Uhr", parent picks days
	longCareID    int64 = 2 // "Nachmittagsgruppe bis 17 Uhr", parent picks days
	fixedFridayID int64 = 3 // fixed Friday variant of the same group
	extraCourseID int64 = 4 // combinable offering outside the group
	autoCareID    int64 = 5 // per-day variant added automatically by the course
)

func perDayCatalog() map[int64]*Offering {
	variant := func(id int64, order int, mode string, days ...string) *Offering {
		return &Offering{
			ID: id, SortOrder: order, DaysOfWeekMode: mode, AvailableDays: days, CountsAsCare: true,
			SelectionGroup: "Betreuungszeit", SelectionRule: SelectionRuleAtMostOnePerDay,
		}
	}
	return map[int64]*Offering{
		shortCareID:   variant(shortCareID, 1, daysOfWeekModeParentChoice, "mon", "tue", "wed", "thu", "fri"),
		longCareID:    variant(longCareID, 2, daysOfWeekModeParentChoice, "mon", "tue", "wed", "thu", "fri"),
		fixedFridayID: variant(fixedFridayID, 3, daysOfWeekModeFixed, "fri"),
		extraCourseID: {ID: extraCourseID, SortOrder: 4, DaysOfWeekMode: daysOfWeekModeParentChoice, AvailableDays: []string{"mon", "wed"}},
	}
}

func dayPicks(rows ...DaySelection) Child {
	child := Child{OfferingDays: rows}
	for _, row := range rows {
		child.OfferingIDs = append(child.OfferingIDs, row.OfferingID)
	}
	return child
}

func requireDayConflict(t *testing.T, err error, child int, day string, ids ...int64) {
	t.Helper()
	require.ErrorIs(t, err, ErrCareOfferingDayConflict)
	require.ErrorIs(t, err, ErrCareOfferingRule, "keeps matching the group-rule value")
	require.ErrorIs(t, err, ErrInvalidSubmission, "maps to 400")
	var conflict *DayConflictError
	require.True(t, errors.As(err, &conflict))
	assert.Equal(t, child, conflict.Child)
	assert.Equal(t, "Betreuungszeit", conflict.Group)
	assert.Equal(t, day, conflict.Day)
	assert.ElementsMatch(t, ids, conflict.OfferingIDs)
}

func TestDayRule_DifferentVariantsOnDifferentDaysAreAllowed(t *testing.T) {
	t.Parallel()
	children := []Child{dayPicks(
		DaySelection{OfferingID: shortCareID, SelectedDays: []string{"mon"}},
		DaySelection{OfferingID: longCareID, SelectedDays: []string{"tue"}},
	)}
	got, err := MaterializeAdjustments(children, perDayCatalog(), "optional", Grandfathered{}, false)
	require.NoError(t, err)
	require.Len(t, got[0], 2)
}

func TestDayRule_TwoVariantsOnTheSameDayConflict(t *testing.T) {
	t.Parallel()
	children := []Child{dayPicks(
		DaySelection{OfferingID: shortCareID, SelectedDays: []string{"mon", "wed"}},
		DaySelection{OfferingID: longCareID, SelectedDays: []string{"tue", "wed"}},
	)}
	_, err := MaterializeAdjustments(children, perDayCatalog(), "optional", Grandfathered{}, false)
	requireDayConflict(t, err, 0, "wed", shortCareID, longCareID)

	child := children[0]
	_, err = MaterializeChild(0, &child, perDayCatalog(), "optional")
	requireDayConflict(t, err, 0, "wed", shortCareID, longCareID)
}

func TestDayRule_SiblingsMayPickDifferentVariantsOnTheSameDay(t *testing.T) {
	t.Parallel()
	children := []Child{
		dayPicks(DaySelection{OfferingID: shortCareID, SelectedDays: []string{"mon"}}),
		dayPicks(DaySelection{OfferingID: longCareID, SelectedDays: []string{"mon"}}),
	}
	_, err := MaterializeAdjustments(children, perDayCatalog(), "optional", Grandfathered{}, false)
	require.NoError(t, err)

	children = append(children, dayPicks(
		DaySelection{OfferingID: shortCareID, SelectedDays: []string{"thu"}},
		DaySelection{OfferingID: longCareID, SelectedDays: []string{"thu"}},
	))
	_, err = MaterializeAdjustments(children, perDayCatalog(), "optional", Grandfathered{}, false)
	requireDayConflict(t, err, 2, "thu", shortCareID, longCareID)
}

func TestDayRule_FixedDaysCount(t *testing.T) {
	t.Parallel()
	catalog := perDayCatalog()
	child := dayPicks(DaySelection{OfferingID: shortCareID, SelectedDays: []string{"mon", "fri"}})
	child.OfferingIDs = append(child.OfferingIDs, fixedFridayID)
	_, err := MaterializeAdjustments([]Child{child}, catalog, "optional", Grandfathered{}, false)
	requireDayConflict(t, err, 0, "fri", shortCareID, fixedFridayID)

	child = dayPicks(DaySelection{OfferingID: shortCareID, SelectedDays: []string{"mon"}})
	child.OfferingIDs = append(child.OfferingIDs, fixedFridayID)
	_, err = MaterializeAdjustments([]Child{child}, catalog, "optional", Grandfathered{}, false)
	require.NoError(t, err)
}

func TestDayRule_AutomaticallyAddedDaysCount(t *testing.T) {
	t.Parallel()
	catalog := perDayCatalog()
	// The course adds the per-day variant on its own days, Monday and
	// Wednesday. A manual variant on Monday then conflicts.
	catalog[autoCareID] = &Offering{
		ID: autoCareID, SortOrder: 5, DaysOfWeekMode: daysOfWeekModeParentChoice,
		AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"}, CountsAsCare: true,
		SelectionGroup: "Betreuungszeit", SelectionRule: SelectionRuleAtMostOnePerDay,
		AutoAddTriggerOfferingIDs: []int64{extraCourseID},
	}
	child := dayPicks(
		DaySelection{OfferingID: extraCourseID, SelectedDays: []string{"mon", "wed"}},
		DaySelection{OfferingID: shortCareID, SelectedDays: []string{"mon"}},
	)
	_, err := MaterializeAdjustments([]Child{child}, catalog, "optional", Grandfathered{}, false)
	requireDayConflict(t, err, 0, "mon", shortCareID, autoCareID)

	child = dayPicks(
		DaySelection{OfferingID: extraCourseID, SelectedDays: []string{"mon", "wed"}},
		DaySelection{OfferingID: shortCareID, SelectedDays: []string{"tue"}},
	)
	got, err := MaterializeAdjustments([]Child{child}, catalog, "optional", Grandfathered{}, false)
	require.NoError(t, err)
	require.Len(t, got[0], 3)
}

func TestDayRule_OfferingsOutsideTheGroupAreCombinable(t *testing.T) {
	t.Parallel()
	child := dayPicks(
		DaySelection{OfferingID: shortCareID, SelectedDays: []string{"mon"}},
		DaySelection{OfferingID: extraCourseID, SelectedDays: []string{"mon"}},
	)
	_, err := MaterializeAdjustments([]Child{child}, perDayCatalog(), "optional", Grandfathered{}, false)
	require.NoError(t, err)
}

func TestDayRule_DoesNotRequireABookingOnEveryDayOrAtAll(t *testing.T) {
	t.Parallel()
	_, err := MaterializeAdjustments([]Child{{}}, perDayCatalog(), "optional", Grandfathered{}, false)
	require.NoError(t, err)
	child := dayPicks(DaySelection{OfferingID: longCareID, SelectedDays: []string{"wed"}})
	_, err = MaterializeAdjustments([]Child{child}, perDayCatalog(), "optional", Grandfathered{}, false)
	require.NoError(t, err)
}

func TestDayRule_MixedWithAnotherRuleInTheGroupIsRejected(t *testing.T) {
	t.Parallel()
	catalog := perDayCatalog()
	catalog[longCareID].SelectionRule = selectionRuleAtMostOne
	child := dayPicks(DaySelection{OfferingID: shortCareID, SelectedDays: []string{"mon"}})
	_, err := MaterializeAdjustments([]Child{child}, catalog, "optional", Grandfathered{}, false)
	require.ErrorIs(t, err, ErrCareOfferingRule)
	require.NotErrorIs(t, err, ErrCareOfferingDayConflict)
	assert.Contains(t, err.Error(), "conflicting rules")
}
