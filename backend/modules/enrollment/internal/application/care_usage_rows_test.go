package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func TestCareUsageRowCountsEffectiveDaysAsUnion(t *testing.T) {
	t.Parallel()

	req := &enrollmentModels.Request{
		ID:                10,
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}
	child := &reportChild{
		ID:        20,
		RequestID: 10,
		FirstName: "Lina",
		LastName:  "Muster",
		Status:    enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Regelbetreuung",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
		2: {
			Name:           "AG",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays:  []string{"tue", "wed"},
		},
	}
	links := []*enrollment.RequestChildOfferingRecord{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue"}},
		{RequestChildID: 20, CareOfferingID: 2},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true, 2: true}, nil)

	require.Len(t, row.Offerings, 2)
	assert.Equal(t, []string{"mon", "tue", "wed"}, row.EffectiveDays)
	assert.Equal(t, 3, row.DayCount)
	assert.Equal(t, []string{"tue", "wed"}, row.Offerings[0].Days)
	assert.Equal(t, "available", row.Offerings[0].DaysSource)
	assert.Equal(t, []string{"mon", "tue"}, row.Offerings[1].Days)
	assert.Equal(t, "selected", row.Offerings[1].DaysSource)
}

func TestCareUsageRowDoesNotInflateMissingParentChoiceDays(t *testing.T) {
	t.Parallel()

	req := &enrollmentModels.Request{
		ID:                10,
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}
	child := &reportChild{
		ID:        20,
		RequestID: 10,
		FirstName: "Lina",
		LastName:  "Muster",
		Status:    enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Regelbetreuung",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
	}
	links := []*enrollment.RequestChildOfferingRecord{
		{RequestChildID: 20, CareOfferingID: 1},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true}, nil)

	require.Len(t, row.Offerings, 1)
	assert.Empty(t, row.Offerings[0].Days)
	assert.NotNil(t, row.Offerings[0].Days)
	assert.Equal(t, "selected", row.Offerings[0].DaysSource)
	assert.Empty(t, row.EffectiveDays)
	assert.NotNil(t, row.EffectiveDays)
	assert.Equal(t, 0, row.DayCount)
}

func TestSortedDayCodesDedupesAndOrdersWeekdays(t *testing.T) {
	t.Parallel()

	got := sortedDayCodes([]string{"fri", "mon", "mon", "wed", "tue"})
	assert.Equal(t, []string{"mon", "tue", "wed", "fri"}, got)
}

func TestSortedDayCodesReturnsEmptySliceForEmptyInput(t *testing.T) {
	t.Parallel()

	got := sortedDayCodes(nil)
	assert.Empty(t, got)
	assert.NotNil(t, got)
}

func TestCareUsageRowMatchesFilters(t *testing.T) {
	t.Parallel()

	grade := int16(2)
	dayCount := 3
	row := enrollment.CareUsageRow{
		ChildFirstName:   "Lina",
		ChildLastName:    "Muster",
		TargetGradeLevel: &grade,
		Status:           enrollmentModels.ChildStatusApproved,
		Offerings: []enrollment.CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed", "fri"}},
		},
		EffectiveDays:     []string{"mon", "wed", "fri"},
		DayCount:          3,
		PickupByDay:       map[string]string{"mon": "14:30", "wed": "16:00", "fri": "14:30"},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}

	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{
		Status:     enrollmentModels.ChildStatusApproved,
		DayCount:   &dayCount,
		GradeLevel: &grade,
		Search:     "eva@example",
	}))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all"}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: enrollmentModels.ChildStatusRejected}))
	otherDayCount := 4
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", DayCount: &otherDayCount}))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "mon"}))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "wed", PickupTime: "16:00"}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "wed", PickupTime: "16:00", Search: "unbekannt"}))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", PickupTime: "14:30"}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "tue"}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "wed", PickupTime: "14:30"}))

	otherGrade := int16(3)
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", GradeLevel: &otherGrade}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Search: "unbekannt"}))
}

func TestCareUsageRowMatchesExplicitOfferingFilter(t *testing.T) {
	t.Parallel()

	row := enrollment.CareUsageRow{
		Status: enrollmentModels.ChildStatusApproved,
		Offerings: []enrollment.CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed", "fri"}},
			{ID: 11, Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon", "wed", "fri"},
		DayCount:      3,
	}

	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{11},
	}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{12},
	}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{},
	}))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{
		Status:          "all",
		CareOfferingIDs: []int64{12},
	}))
}

func TestCareUsageRowMatchesBookedPickupDayFilters(t *testing.T) {
	t.Parallel()

	row := enrollment.CareUsageRow{
		Status: enrollmentModels.ChildStatusApproved,
		Offerings: []enrollment.CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed"}},
			{ID: 11, Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon", "wed"},
		PickupByDay:   map[string]string{"mon": "14:30", "wed": "16:00", "fri": "15:30"},
	}

	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "fri"}))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", PickupTime: "15:30"}))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "fri", PickupTime: "15:30"}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "fri", PickupTime: "14:30"}))
}

func TestCareUsageRowMatchesWeekdayWithoutPickupTimes(t *testing.T) {
	t.Parallel()

	row := enrollment.CareUsageRow{
		Status: enrollmentModels.ChildStatusApproved,
		Offerings: []enrollment.CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon"}},
			{ID: 11, Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon"},
		PickupByDay:   map[string]string{},
	}

	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "mon"}))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "fri"}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", Weekday: "mon", PickupTime: "14:30"}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", PickupTime: "14:30"}))
	assert.Empty(t, careUsageBookedPickupDays(row, enrollment.CareUsageFilters{Status: "all"}))
}

func TestCareUsageRowMatchesScopedBookedPickupDayFilters(t *testing.T) {
	t.Parallel()

	row := enrollment.CareUsageRow{
		Status: enrollmentModels.ChildStatusApproved,
		Offerings: []enrollment.CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed"}},
			{ID: 11, Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon", "wed"},
		PickupByDay:   map[string]string{"mon": "14:30", "wed": "16:00", "fri": "15:30"},
	}
	ogsOnly := enrollment.CareUsageFilters{Status: "all", CareOfferingIDsSet: true, CareOfferingIDs: []int64{10}}
	randstundeOnly := enrollment.CareUsageFilters{Status: "all", CareOfferingIDsSet: true, CareOfferingIDs: []int64{11}}

	assert.Equal(t, []string{"mon", "wed"}, careUsageBookedPickupDays(row, ogsOnly))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{10},
		Weekday:            "fri",
	}))
	assert.False(t, careUsageRowMatches(row, enrollment.CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{10},
		PickupTime:         "15:30",
	}))
	assert.Equal(t, []string{"fri"}, careUsageBookedPickupDays(row, randstundeOnly))
	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{
		Status:             "all",
		CareOfferingIDsSet: true,
		CareOfferingIDs:    []int64{11},
		Weekday:            "fri",
		PickupTime:         "15:30",
	}))
}

func TestCareUsageRowMatchesZeroDayFilter(t *testing.T) {
	t.Parallel()

	zero := 0
	row := enrollment.CareUsageRow{
		Status:        enrollmentModels.ChildStatusApproved,
		EffectiveDays: []string{},
		DayCount:      0,
	}

	assert.True(t, careUsageRowMatches(row, enrollment.CareUsageFilters{Status: "all", DayCount: &zero}))
}

func TestCareUsageRowExcludesNonIncludedOfferingsFromDayCount(t *testing.T) {
	t.Parallel()

	req := &enrollmentModels.Request{ID: 10}
	child := &reportChild{
		ID:     20,
		Status: enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
		2: {
			Name:           "Randstunde",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"fri"},
		},
	}
	links := []*enrollment.RequestChildOfferingRecord{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue", "wed", "thu"}},
		{RequestChildID: 20, CareOfferingID: 2, SelectedDays: []string{"fri"}},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true}, nil)

	require.Len(t, row.Offerings, 2)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, row.EffectiveDays)
	assert.Equal(t, 4, row.DayCount)
}

func TestCareUsageBookedPickupDaysIncludesNonCareOfferingDays(t *testing.T) {
	t.Parallel()

	row := enrollment.CareUsageRow{
		Offerings: []enrollment.CareUsageRowOffering{
			{Name: "OGS Ganztag", Days: []string{"mon", "wed"}},
			{Name: "Randstunde", Days: []string{"fri"}},
		},
		EffectiveDays: []string{"mon", "wed"},
		PickupByDay:   map[string]string{"mon": "14:30", "wed": "16:00", "fri": "14:30"},
	}

	assert.Equal(t, []string{"mon", "wed", "fri"}, careUsageBookedPickupDays(row, enrollment.CareUsageFilters{Status: "all"}))
}

func TestCareUsageRowKeepsOfferingsVisibleWhenNoOfferingsAreIncluded(t *testing.T) {
	t.Parallel()

	req := &enrollmentModels.Request{ID: 10}
	child := &reportChild{
		ID:     20,
		Status: enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Ganztag",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
	}
	links := []*enrollment.RequestChildOfferingRecord{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue"}},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{}, nil)

	require.Len(t, row.Offerings, 1)
	assert.Equal(t, []string{"mon", "tue"}, row.Offerings[0].Days)
	assert.Empty(t, row.EffectiveDays)
	assert.NotNil(t, row.EffectiveDays)
	assert.Equal(t, 0, row.DayCount)
}

func TestNormalizedCareUsageOfferingIDsDefaultsOnlyWhenSelectionIsMissing(t *testing.T) {
	t.Parallel()

	offerings := []*enrollmentModels.CareOffering{
		{ID: 1, Name: "Ganztag", CountsAsCare: true},
		{ID: 2, Name: "Randstunde", CountsAsCare: false},
		{ID: 3, Name: "Kurzbetreuung", CountsAsCare: true},
	}

	assert.Equal(t, []int64{1, 3}, normalizedCareUsageOfferingIDs(nil, offerings, false))
	assert.Equal(t, []int64{}, normalizedCareUsageOfferingIDs(nil, offerings, true))
	assert.Equal(t, []int64{2}, normalizedCareUsageOfferingIDs([]int64{2, 2, -1}, offerings, true))
}

func TestCareUsageRowCarriesManualAndAutomaticDays(t *testing.T) {
	t.Parallel()

	req := &enrollmentModels.Request{ID: 10}
	child := &reportChild{
		ID:     20,
		Status: enrollmentModels.ChildStatusApproved,
	}
	offerings := map[int64]*enrollmentModels.CareOffering{
		1: {
			Name:           "Randstunde",
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
			AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		},
	}
	links := []*enrollment.RequestChildOfferingRecord{
		{
			RequestChildID:        20,
			CareOfferingID:        1,
			SelectedDays:          []string{"mon", "tue", "wed", "thu", "fri"},
			ManualSelectedDays:    []string{"fri"},
			AutomaticSelectedDays: []string{"mon", "tue", "wed", "thu"},
		},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true}, nil)

	require.Len(t, row.Offerings, 1)
	assert.Equal(t, []string{"fri"}, row.Offerings[0].ManualSelectedDays)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, row.Offerings[0].AutomaticSelectedDays)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, row.Offerings[0].Days)
}
