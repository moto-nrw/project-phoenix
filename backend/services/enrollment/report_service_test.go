package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	baseModels "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

func TestCareUsageRowCountsEffectiveDaysAsUnion(t *testing.T) {
	req := &enrollmentModels.Request{
		Model:             baseModels.Model{ID: 10},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}
	child := &enrollmentModels.RequestChild{
		Model:     baseModels.Model{ID: 20},
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
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue"}},
		{RequestChildID: 20, CareOfferingID: 2},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true, 2: true})

	require.Len(t, row.Offerings, 2)
	assert.Equal(t, []string{"mon", "tue", "wed"}, row.EffectiveDays)
	assert.Equal(t, 3, row.DayCount)
	assert.Equal(t, []string{"tue", "wed"}, row.Offerings[0].Days)
	assert.Equal(t, "available", row.Offerings[0].DaysSource)
	assert.Equal(t, []string{"mon", "tue"}, row.Offerings[1].Days)
	assert.Equal(t, "selected", row.Offerings[1].DaysSource)
}

func TestCareUsageRowDoesNotInflateMissingParentChoiceDays(t *testing.T) {
	req := &enrollmentModels.Request{
		Model:             baseModels.Model{ID: 10},
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}
	child := &enrollmentModels.RequestChild{
		Model:     baseModels.Model{ID: 20},
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
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 20, CareOfferingID: 1},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true})

	require.Len(t, row.Offerings, 1)
	assert.Empty(t, row.Offerings[0].Days)
	assert.NotNil(t, row.Offerings[0].Days)
	assert.Equal(t, "selected", row.Offerings[0].DaysSource)
	assert.Empty(t, row.EffectiveDays)
	assert.NotNil(t, row.EffectiveDays)
	assert.Equal(t, 0, row.DayCount)
}

func TestSortedDayCodesDedupesAndOrdersWeekdays(t *testing.T) {
	got := sortedDayCodes([]string{"fri", "mon", "mon", "wed", "tue"})
	assert.Equal(t, []string{"mon", "tue", "wed", "fri"}, got)
}

func TestSortedDayCodesReturnsEmptySliceForEmptyInput(t *testing.T) {
	got := sortedDayCodes(nil)
	assert.Empty(t, got)
	assert.NotNil(t, got)
}

func TestCareUsageRowMatchesFilters(t *testing.T) {
	grade := int16(2)
	dayCount := 3
	row := CareUsageRow{
		ChildFirstName:   "Lina",
		ChildLastName:    "Muster",
		TargetGradeLevel: &grade,
		Status:           enrollmentModels.ChildStatusApproved,
		Offerings: []CareUsageRowOffering{
			{ID: 10, Name: "OGS Ganztag", Days: []string{"mon", "wed", "fri"}},
		},
		EffectiveDays:     []string{"mon", "wed", "fri"},
		DayCount:          3,
		GuardianFirstName: "Eva",
		GuardianLastName:  "Muster",
		GuardianEmail:     "eva@example.test",
	}

	assert.True(t, careUsageRowMatches(row, CareUsageFilters{
		Status:     enrollmentModels.ChildStatusApproved,
		DayCount:   &dayCount,
		GradeLevel: &grade,
		Search:     "eva@example",
	}))
	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all"}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: enrollmentModels.ChildStatusRejected}))
	otherDayCount := 4
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", DayCount: &otherDayCount}))

	otherGrade := int16(3)
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", GradeLevel: &otherGrade}))
	assert.False(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", Search: "unbekannt"}))
}

func TestCareUsageRowMatchesZeroDayFilter(t *testing.T) {
	zero := 0
	row := CareUsageRow{
		Status:        enrollmentModels.ChildStatusApproved,
		EffectiveDays: []string{},
		DayCount:      0,
	}

	assert.True(t, careUsageRowMatches(row, CareUsageFilters{Status: "all", DayCount: &zero}))
}

func TestCareUsageRowExcludesNonIncludedOfferingsFromDayCount(t *testing.T) {
	req := &enrollmentModels.Request{Model: baseModels.Model{ID: 10}}
	child := &enrollmentModels.RequestChild{
		Model:  baseModels.Model{ID: 20},
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
	links := []*enrollmentModels.RequestChildOffering{
		{RequestChildID: 20, CareOfferingID: 1, SelectedDays: []string{"mon", "tue", "wed", "thu"}},
		{RequestChildID: 20, CareOfferingID: 2, SelectedDays: []string{"fri"}},
	}

	row := careUsageRow(req, child, links, offerings, map[int64]bool{1: true})

	require.Len(t, row.Offerings, 2)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, row.EffectiveDays)
	assert.Equal(t, 4, row.DayCount)
}
