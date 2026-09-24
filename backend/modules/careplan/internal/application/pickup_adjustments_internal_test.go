package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type directPickupCoordinatorStub struct {
	catalog *careplan.OfferingChangeCatalog
	extra   map[int64][]careplan.OfferingChangeSelection
}

const pickupAdjustmentTestToday calendar.Date = "2026-08-24"

func (s directPickupCoordinatorStub) PrepareDirectOfferingAdjustment(context.Context, careplan.DirectOfferingAdjustmentInput) error {
	return nil
}

func (s directPickupCoordinatorStub) ApplyDirectOfferingAdjustment(context.Context, careplan.DirectOfferingAdjustmentInput) error {
	return nil
}

func (s directPickupCoordinatorStub) PreviewDirectOfferingAdjustment(
	_ context.Context,
	input careplan.DirectOfferingAdjustmentInput,
) (*careplan.DirectOfferingAdjustmentPreview, error) {
	materialized := cloneOfferingSelections(input.Selections)
	if target := selectedCareOfferingID(s.catalog, input.Selections); target > 0 {
		materialized = append(materialized, s.extra[target]...)
	}
	rows := make([]careplan.OfferingSelection, 0, len(materialized))
	for _, selected := range materialized {
		rows = append(rows, careplan.OfferingSelection{OfferingID: selected.OfferingID, SelectedDays: selected.SelectedDays})
	}
	profile, err := materializedPickupTimes(rows, s.catalog)
	return &careplan.DirectOfferingAdjustmentPreview{Catalog: s.catalog, MaterializedPickupTimes: profile}, err
}

func selectedCareOfferingID(catalog *careplan.OfferingChangeCatalog, selections []careplan.OfferingChangeSelection) int64 {
	for _, selected := range selections {
		for _, item := range catalog.Items {
			if item.OfferingID == selected.OfferingID && item.CountsAsCare {
				return item.OfferingID
			}
		}
	}
	return 0
}

func pickupAdjustmentsWith(offerings careplan.DirectOfferingAdjustments) *PickupAdjustments {
	return &PickupAdjustments{deps: PickupAdjustmentDependencies{Offerings: offerings}}
}

func TestMatchingPickupOfferings_ReturnsEveryOtherExactActiveCareProfile(t *testing.T) {
	t.Parallel()

	catalog := &careplan.OfferingChangeCatalog{Items: []careplan.OfferingChangeCatalogItem{
		{
			OfferingID: 1, Name: "Bis 16 Uhr", IsActive: true, Selected: true, CountsAsCare: true,
			DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "tue", "wed", "thu"},
			PickupTimes: map[string]string{"mon": "16:00", "tue": "16:00", "wed": "16:00", "thu": "16:00"},
		},
		{
			OfferingID: 2, Name: "Bis 14:30", IsActive: true, CountsAsCare: true,
			DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "tue", "wed", "thu"},
			PickupTimes: map[string]string{"mon": "14:30", "tue": "14:30", "wed": "14:30", "thu": "14:30"},
		},
		{
			OfferingID: 3, Name: "Ebenfalls 14:30", IsActive: true, CountsAsCare: true,
			DaysOfWeekMode: "fixed", AvailableDays: []string{"thu", "wed", "tue", "mon"},
			PickupTimes: map[string]string{"mon": "14:30", "tue": "14:30", "wed": "14:30", "thu": "14:30"},
		},
		{
			OfferingID: 4, Name: "Inaktiv", IsActive: false, CountsAsCare: true,
			DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "tue", "wed", "thu"},
			PickupTimes: map[string]string{"mon": "14:30", "tue": "14:30", "wed": "14:30", "thu": "14:30"},
		},
	}}
	proposed := map[int]careplan.PickupAdjustmentSchedule{
		1: {Weekday: 1, PickupTime: "14:30"},
		2: {Weekday: 2, PickupTime: "14:30"},
		3: {Weekday: 3, PickupTime: "14:30"},
		4: {Weekday: 4, PickupTime: "14:30"},
	}

	service := pickupAdjustmentsWith(directPickupCoordinatorStub{catalog: catalog})
	matches, err := service.matchingPickupOfferings(context.Background(), careplan.PickupAdjustmentPreviewInput{
		StudentID: 1, CareDays: []int{1, 2, 3, 4}, EffectiveFrom: pickupAdjustmentTestToday,
	}, catalog, proposed)
	require.NoError(t, err)

	require.Len(t, matches, 2)
	assert.Equal(t, []int64{2, 3}, []int64{matches[0].OfferingID, matches[1].OfferingID})
	assert.Empty(t, matches[0].SelectedDays)
	assert.Empty(t, matches[1].SelectedDays)
}

func TestMatchingPickupOfferings_DoesNotGuessForIntermediateTimeOrDifferentCareDays(t *testing.T) {
	t.Parallel()

	catalog := &careplan.OfferingChangeCatalog{Items: []careplan.OfferingChangeCatalogItem{
		{
			OfferingID: 2, Name: "Bis 14:30", IsActive: true, CountsAsCare: true,
			DaysOfWeekMode: "fixed", AvailableDays: []string{"mon", "tue", "wed", "thu"},
			PickupTimes: map[string]string{"mon": "14:30", "tue": "14:30", "wed": "14:30", "thu": "14:30"},
		},
	}}

	service := pickupAdjustmentsWith(directPickupCoordinatorStub{catalog: catalog})
	matches, err := service.matchingPickupOfferings(context.Background(), careplan.PickupAdjustmentPreviewInput{
		StudentID: 1, CareDays: []int{1, 2, 3, 4}, EffectiveFrom: pickupAdjustmentTestToday,
	}, catalog, map[int]careplan.PickupAdjustmentSchedule{
		1: {Weekday: 1, PickupTime: "13:45"}, 2: {Weekday: 2, PickupTime: "13:45"},
		3: {Weekday: 3, PickupTime: "13:45"}, 4: {Weekday: 4, PickupTime: "13:45"},
	})
	require.NoError(t, err)
	assert.Empty(t, matches)
	matches, err = service.matchingPickupOfferings(context.Background(), careplan.PickupAdjustmentPreviewInput{
		StudentID: 1, CareDays: []int{1, 2, 3}, EffectiveFrom: pickupAdjustmentTestToday,
	}, catalog, map[int]careplan.PickupAdjustmentSchedule{
		1: {Weekday: 1, PickupTime: "14:30"}, 2: {Weekday: 2, PickupTime: "14:30"},
		3: {Weekday: 3, PickupTime: "14:30"},
	})
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestMatchingPickupOfferings_UsesChosenCareDaysForParentChoiceOffering(t *testing.T) {
	t.Parallel()

	catalog := &careplan.OfferingChangeCatalog{Items: []careplan.OfferingChangeCatalogItem{
		{
			OfferingID: 8, Name: "Flexible Tage", IsActive: true, CountsAsCare: true,
			DaysOfWeekMode: "parent_choice", AvailableDays: []string{"mon", "tue", "wed", "thu", "fri"},
			PickupTimes: map[string]string{"mon": "15:00", "thu": "15:00"},
		},
	}}
	proposed := map[int]careplan.PickupAdjustmentSchedule{
		1: {Weekday: 1, PickupTime: "15:00"},
		4: {Weekday: 4, PickupTime: "15:00"},
	}

	service := pickupAdjustmentsWith(directPickupCoordinatorStub{catalog: catalog})
	matches, err := service.matchingPickupOfferings(context.Background(), careplan.PickupAdjustmentPreviewInput{
		StudentID: 1, CareDays: []int{1, 4}, EffectiveFrom: pickupAdjustmentTestToday,
	}, catalog, proposed)
	require.NoError(t, err)

	require.Len(t, matches, 1)
	assert.Equal(t, []string{"mon", "thu"}, matches[0].SelectedDays)
}

func TestMatchingPickupOfferings_UsesMaterializedCoBookings(t *testing.T) {
	t.Parallel()

	catalog := &careplan.OfferingChangeCatalog{Items: []careplan.OfferingChangeCatalogItem{
		{OfferingID: 2, Name: "Bis 14:30", IsActive: true, CountsAsCare: true,
			DaysOfWeekMode: "fixed", AvailableDays: []string{"mon"}, PickupTimes: map[string]string{"mon": "14:30"}},
		{OfferingID: 9, Name: "Mitbuchung bis 16:00", IsActive: true, CountsAsCare: true,
			DaysOfWeekMode: "fixed", AvailableDays: []string{"mon"}, PickupTimes: map[string]string{"mon": "16:00"}},
	}}
	service := pickupAdjustmentsWith(directPickupCoordinatorStub{
		catalog: catalog,
		extra:   map[int64][]careplan.OfferingChangeSelection{2: {{OfferingID: 9}}},
	})
	matches, err := service.matchingPickupOfferings(context.Background(), careplan.PickupAdjustmentPreviewInput{
		StudentID: 1, CareDays: []int{1}, EffectiveFrom: pickupAdjustmentTestToday,
	}, catalog, map[int]careplan.PickupAdjustmentSchedule{1: {Weekday: 1, PickupTime: "14:30"}})

	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestEffectiveProposedPickupPlanUsesOfferingForBlankCareDay(t *testing.T) {
	t.Parallel()
	offering := careplan.PickupWeek{
		1: {PickupTime: pickupTestTime(t, "15:00")},
	}

	proposed := effectiveProposedPickupPlan([]int{1}, nil, offering)

	assert.Equal(t, "15:00", proposed[1].PickupTime)
	assert.False(t, pickupPlanDeviates([]int{1}, proposed, offering))
}

func TestPickupPlanDeviatesWhenCareDayWasRemoved(t *testing.T) {
	t.Parallel()

	offering := careplan.PickupWeek{
		1: {PickupTime: pickupTestTime(t, "15:00")},
		4: {PickupTime: pickupTestTime(t, "15:00")},
	}
	proposed := map[int]careplan.PickupAdjustmentSchedule{
		1: {Weekday: 1, PickupTime: "15:00"},
	}

	assert.True(t, pickupPlanDeviates([]int{1}, proposed, offering))
}

func TestSelectsExactPickupOfferingRejectsArbitraryCareSelection(t *testing.T) {
	t.Parallel()

	matches := []careplan.PickupOfferingMatch{{OfferingID: 2, Selections: []careplan.OfferingChangeSelection{
		{OfferingID: 4}, {OfferingID: 2, SelectedDays: []string{"mon", "thu"}},
	}}}

	assert.True(t, selectsExactPickupOffering([]careplan.OfferingChangeSelection{
		{OfferingID: 4},
		{OfferingID: 2, SelectedDays: []string{"thu", "mon"}},
	}, matches))
	assert.False(t, selectsExactPickupOffering([]careplan.OfferingChangeSelection{
		{OfferingID: 2, SelectedDays: []string{"mon"}},
	}, matches))
	assert.False(t, selectsExactPickupOffering([]careplan.OfferingChangeSelection{
		{OfferingID: 2, SelectedDays: []string{"mon", "thu"}},
		{OfferingID: 9},
	}, matches))
}

func TestPickupAdjustmentTokenChangesWithExistingNotes(t *testing.T) {
	t.Parallel()

	firstNote := "Bus"
	secondNote := "Wird abgeholt"
	base := careplan.PickupWeek{
		1: {PickupTime: pickupTestTime(t, "15:00"), Notes: &firstNote},
	}
	changed := careplan.PickupWeek{
		1: {PickupTime: pickupTestTime(t, "15:00"), Notes: &secondNote},
	}
	// The identity fingerprint keeps the tokenized content visible.
	service := &PickupAdjustments{deps: PickupAdjustmentDependencies{Fingerprint: func(content []byte) string { return string(content) }}}
	input := careplan.PickupAdjustmentPreviewInput{StudentID: 7}
	preview := &careplan.PickupAdjustmentPreview{}
	first, err := service.pickupAdjustmentToken(pickupTokenContent{TenantID: 3, Input: input, Preview: preview, Current: base})
	require.NoError(t, err)
	second, err := service.pickupAdjustmentToken(pickupTokenContent{TenantID: 3, Input: input, Preview: preview, Current: changed})
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestSamePickupAdjustmentTokenRequiresEqualSHA256Values(t *testing.T) {
	t.Parallel()

	token := strings.Repeat("a", 64)
	other := strings.Repeat("b", 64)
	assert.True(t, samePickupAdjustmentToken(token, " "+token+" "))
	assert.False(t, samePickupAdjustmentToken(token, other))
	assert.False(t, samePickupAdjustmentToken("abcd", "abcd"), "a token that is no SHA-256 value never matches")
	assert.False(t, samePickupAdjustmentToken("", ""))
}

func TestPickupPlanLabelIncludesNotes(t *testing.T) {
	t.Parallel()

	note := "Fährt mit dem Bus"
	label := pickupPlanLabel(careplan.PickupWeek{
		1: {PickupTime: pickupTestTime(t, "15:00"), Notes: &note},
	})

	assert.Equal(t, "Mo 15:00 Uhr (Notiz: Fährt mit dem Bus)", label)
}

func TestPickupAdjustmentAppliesArrivalSchedulesOnlyForImmediateExceptions(t *testing.T) {
	t.Parallel()

	today := calendar.NewDate(2026, 8, 24)
	assert.True(t, appliesArrivalSchedulesOn(careplan.PickupAdjustmentResolutionException, today, today))
	assert.False(t, appliesArrivalSchedulesOn(careplan.PickupAdjustmentResolutionOffering, today, today))
	assert.False(t, appliesArrivalSchedulesOn(careplan.PickupAdjustmentResolutionException, today.AddDays(1), today))
}

func pickupTestTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return parsed
}
