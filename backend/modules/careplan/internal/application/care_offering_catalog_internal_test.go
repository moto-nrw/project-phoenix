package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The stubs embed their port and override only what a test touches; an
// unimplemented call panics loudly rather than returning a zero value.

type catalogRecordsStub struct {
	ports.CatalogRecords
	offerings []careplan.CareOffering
	err       error
}

func (r catalogRecordsStub) ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
	return r.offerings, r.err
}

type catalogPhasesStub struct {
	phase careplan.OfferingPhase
	err   error
}

func (p catalogPhasesStub) Phase(context.Context, int64) (careplan.OfferingPhase, error) {
	return p.phase, p.err
}

type catalogBookingsStub struct {
	ports.CatalogBookings
	grades    []ports.OfferingGradeCount
	gradesErr error
	peaks     map[int64]int
	peakErr   error

	gotIDs        []int64
	gotPeakIDs    []int64
	gotFrom       calendar.Date
	gotUntil      calendar.Date
	peakCallCount int
}

func (b *catalogBookingsStub) OfferingGradeCounts(_ context.Context, ids []int64, from, until calendar.Date) ([]ports.OfferingGradeCount, error) {
	b.gotIDs, b.gotFrom, b.gotUntil = ids, from, until
	return b.grades, b.gradesErr
}

func (b *catalogBookingsStub) OfferingCapacityPeaks(_ context.Context, ids []int64, _, _ calendar.Date) (map[int64]int, error) {
	b.peakCallCount++
	b.gotPeakIDs = ids
	if b.peakErr != nil {
		return nil, b.peakErr
	}
	return b.peaks, nil
}

type catalogSettingsStub struct {
	gradeMax int
	err      error
}

func (s catalogSettingsStub) GradeLevelMax(context.Context) (int, error) {
	return s.gradeMax, s.err
}

type catalogTranslationsStub struct {
	ok  bool
	err error
}

func (s catalogTranslationsStub) NormalizeTranslations(raw json.RawMessage) (json.RawMessage, bool, error) {
	return raw, s.ok, s.err
}

type catalogTimetableStub struct{ ports.CatalogTimetable }
type catalogCalendarStub struct{ ports.CatalogCalendar }
type catalogSourceRulesStub struct{ ports.OfferingSourceRules }

const bookingStatsTestToday calendar.Date = "2026-08-24"

func catalogDependencies() CareOfferingCatalogDependencies {
	return CareOfferingCatalogDependencies{
		Records:      catalogRecordsStub{},
		Phases:       catalogPhasesStub{},
		Timetable:    catalogTimetableStub{},
		Calendar:     catalogCalendarStub{},
		Bookings:     &catalogBookingsStub{},
		Settings:     catalogSettingsStub{gradeMax: 4},
		Translations: catalogTranslationsStub{ok: true},
		SourceRules:  catalogSourceRulesStub{},
		MarkRollback: func(context.Context) {},
		Today:        func() calendar.Date { return bookingStatsTestToday },
	}
}

func newTestCatalog(t *testing.T, deps CareOfferingCatalogDependencies) *CareOfferingCatalog {
	t.Helper()
	catalog, err := NewCareOfferingCatalog(deps)
	require.NoError(t, err)
	return catalog
}

func bookingStatsCatalog(t *testing.T, phase careplan.OfferingPhase, offerings []careplan.CareOffering, bookings *catalogBookingsStub) *CareOfferingCatalog {
	t.Helper()
	deps := catalogDependencies()
	deps.Records = catalogRecordsStub{offerings: offerings}
	deps.Phases = catalogPhasesStub{phase: phase}
	deps.Bookings = bookings
	return newTestCatalog(t, deps)
}

func runningPhase() careplan.OfferingPhase {
	return careplan.OfferingPhase{
		ServiceStart: bookingStatsTestToday.AddDays(-30),
		ServiceEnd:   bookingStatsTestToday.AddDays(60),
	}
}

func intPtr(v int) *int { return &v }

func gradePtr(v int16) *int16 { return &v }

func gradeRule(t *testing.T, operator string, values ...int) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(availabilityRule{
		Match:      availabilityMatchAll,
		Conditions: []availabilityCondition{{Source: availabilitySourceGradeLevel, Operator: operator, Value: values}},
	})
	require.NoError(t, err)
	return raw
}

func TestValidateAvailabilityRuleUsesTenantGradeRange(t *testing.T) {
	t.Parallel()

	catalog := newTestCatalog(t, catalogDependencies())
	offering := careplan.CareOffering{AvailabilityRule: gradeRule(t, availabilityOperatorIn, 2, 5)}
	err := catalog.validateAvailabilityRule(context.Background(), offering)
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	require.ErrorContains(t, err, "condition 1")
	require.ErrorContains(t, err, "range 1-4")
}

func TestValidateAvailabilityRuleRefusesAnInvalidTenantRange(t *testing.T) {
	t.Parallel()

	deps := catalogDependencies()
	deps.Settings = catalogSettingsStub{gradeMax: 14}
	err := newTestCatalog(t, deps).validateAvailabilityRule(context.Background(),
		careplan.CareOffering{AvailabilityRule: gradeRule(t, availabilityOperatorIn, 2)})
	require.EqualError(t, err, "tenant grade range is invalid: maximum 14")
	require.NotErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid, "a broken tenant setting is no client error")
}

func TestNormalizeTranslationsClassifiesRefusalsAsInvalid(t *testing.T) {
	t.Parallel()

	deps := catalogDependencies()
	deps.Translations = catalogTranslationsStub{ok: false}
	err := newTestCatalog(t, deps).normalizeTranslations(&careplan.CareOffering{Translations: json.RawMessage(`[]`)})
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	require.EqualError(t, err, "invalid care offering configuration: translations must be a valid translation document")

	deps.Translations = catalogTranslationsStub{ok: true, err: errors.New("unknown attribute")}
	err = newTestCatalog(t, deps).normalizeTranslations(&careplan.CareOffering{Translations: json.RawMessage(`{}`)})
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
	require.EqualError(t, err, "invalid care offering configuration: validate care offering translations: unknown attribute")
}

func TestListBookingStats_ReportsCapacityAndGradeDistribution(t *testing.T) {
	t.Parallel()

	offerings := []careplan.CareOffering{
		{ID: 7, Name: "Randstunde", Capacity: intPtr(20)},
		{ID: 9, Name: "Ganztag"},
	}
	bookings := &catalogBookingsStub{
		peaks: map[int64]int{7: 14, 9: 3},
		grades: []ports.OfferingGradeCount{
			{CareOfferingID: 7, GradeLevel: gradePtr(1), Count: 12},
			{CareOfferingID: 7, GradeLevel: gradePtr(3), Count: 2},
			{CareOfferingID: 9, GradeLevel: nil, Count: 3},
		},
	}

	stats, err := bookingStatsCatalog(t, runningPhase(), offerings, bookings).ListBookingStats(context.Background(), 5)
	require.NoError(t, err)
	require.Len(t, stats, 2)

	assert.Equal(t, int64(7), stats[0].OfferingID)
	assert.Equal(t, 20, *stats[0].Capacity)
	assert.Equal(t, 14, stats[0].Booked)
	assert.Equal(t, map[int]int{1: 12, 3: 2}, stats[0].GradeLevels)
	assert.Zero(t, stats[0].UnknownGradeCount)

	assert.Nil(t, stats[1].Capacity, "an unlimited offering reports no capacity")
	assert.Equal(t, 3, stats[1].Booked)
	assert.Empty(t, stats[1].GradeLevels)
	assert.Equal(t, 3, stats[1].UnknownGradeCount, "children without a grade get their own bucket")
}

func TestListBookingStats_CountsInTheCapacityGatesWindow(t *testing.T) {
	t.Parallel()

	// The displayed occupancy must be the number the capacity gate will apply
	// on save, so the window has to match the gate's.
	phase := runningPhase()
	bookings := &catalogBookingsStub{peaks: map[int64]int{1: 0}}
	_, err := bookingStatsCatalog(t, phase, []careplan.CareOffering{{ID: 1, Name: "Ganztag"}}, bookings).
		ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, bookingStatsTestToday, bookings.gotFrom, "a running phase counts from today")
	assert.Equal(t, phase.ServiceEnd.AddDays(1), bookings.gotUntil, "the last service day is inclusive")
	assert.Equal(t, []int64{1}, bookings.gotIDs)
}

func TestListBookingStats_CountsFromTheStartOfAFuturePhase(t *testing.T) {
	t.Parallel()

	phase := careplan.OfferingPhase{
		ServiceStart: bookingStatsTestToday.AddDays(30),
		ServiceEnd:   bookingStatsTestToday.AddDays(200),
	}
	bookings := &catalogBookingsStub{peaks: map[int64]int{1: 0}}
	_, err := bookingStatsCatalog(t, phase, []careplan.CareOffering{{ID: 1, Name: "Ganztag"}}, bookings).
		ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, phase.ServiceStart, bookings.gotFrom)
}

func TestListBookingStats_FallsBackToTheFinalDayOfACompletedPhase(t *testing.T) {
	t.Parallel()

	// [today, end+1) is empty once a phase has ended. Reporting zero would
	// claim every offering is free; the final service day is the honest
	// answer for a historical phase.
	phase := careplan.OfferingPhase{
		ServiceStart: bookingStatsTestToday.AddDays(-200),
		ServiceEnd:   bookingStatsTestToday.AddDays(-30),
	}
	bookings := &catalogBookingsStub{peaks: map[int64]int{1: 4}}
	stats, err := bookingStatsCatalog(t, phase, []careplan.CareOffering{{ID: 1, Name: "Ganztag"}}, bookings).
		ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, phase.ServiceEnd, bookings.gotFrom)
	assert.Equal(t, phase.ServiceEnd.AddDays(1), bookings.gotUntil)
	assert.Equal(t, 4, stats[0].Booked)
}

func TestBookingStatsWindow_DefaultsToTodayWithoutPhaseDates(t *testing.T) {
	t.Parallel()

	today := bookingStatsTestToday
	from, until := bookingStatsWindowOn(careplan.OfferingPhase{}, today)
	assert.Equal(t, today, from)
	assert.Equal(t, today.AddDays(1), until)
	assert.True(t, from.Before(until), "the window is never empty")
}

func TestListBookingStats_ReturnsEmptyWithoutOfferings(t *testing.T) {
	t.Parallel()

	bookings := &catalogBookingsStub{}
	stats, err := bookingStatsCatalog(t, runningPhase(), nil, bookings).ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Empty(t, stats)
	assert.Zero(t, bookings.peakCallCount, "no offerings means no occupancy queries")
	assert.Nil(t, bookings.gotIDs)
}

// #2186 review: occupancy used to be one query per offering on top of the
// already-batched grade counts.
func TestListBookingStats_BatchesTheOccupancyQuery(t *testing.T) {
	t.Parallel()

	offerings := []careplan.CareOffering{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}, {ID: 3, Name: "C"}}
	bookings := &catalogBookingsStub{peaks: map[int64]int{1: 4, 3: 9}}

	stats, err := bookingStatsCatalog(t, runningPhase(), offerings, bookings).ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, 1, bookings.peakCallCount, "one query for every offering, not one each")
	assert.Equal(t, []int64{1, 2, 3}, bookings.gotPeakIDs)
	// Neither aggregate is phase-scoped: both must count the population the
	// capacity gate enforces on (#2186 review round 2).
	assert.Equal(t, []int64{1, 2, 3}, bookings.gotIDs)
	assert.Equal(t, 4, stats[0].Booked)
	assert.Zero(t, stats[1].Booked, "an offering with no bookings reads as zero")
	assert.Equal(t, 9, stats[2].Booked)
}

func TestListBookingStats_RejectsAnInvalidPhaseID(t *testing.T) {
	t.Parallel()

	_, err := bookingStatsCatalog(t, runningPhase(), nil, &catalogBookingsStub{}).ListBookingStats(context.Background(), 0)
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
}

func TestListBookingStats_ClassifiesAMissingPhaseAsInvalid(t *testing.T) {
	t.Parallel()

	deps := catalogDependencies()
	deps.Phases = catalogPhasesStub{err: fmt.Errorf("phase 5: %w", ports.ErrCatalogRowNotFound)}
	_, err := newTestCatalog(t, deps).ListBookingStats(context.Background(), 5)
	require.ErrorIs(t, err, careplan.ErrCareOfferingConfigInvalid)
}

// The catalog fails at composition instead of on the first request when a
// port is missing: missing wiring is a configuration error.
func TestNewCareOfferingCatalog_RequiresItsPorts(t *testing.T) {
	t.Parallel()

	withoutBookings := catalogDependencies()
	withoutBookings.Bookings = nil
	_, err := NewCareOfferingCatalog(withoutBookings)
	require.ErrorContains(t, err, "bookings")

	withoutPhases := catalogDependencies()
	withoutPhases.Phases = nil
	_, err = NewCareOfferingCatalog(withoutPhases)
	require.ErrorContains(t, err, "phases")
}

func TestListBookingStats_PropagatesRepositoryFailures(t *testing.T) {
	t.Parallel()

	offering := careplan.CareOffering{ID: 1, Name: "Ganztag"}

	deps := catalogDependencies()
	deps.Records = catalogRecordsStub{err: errors.New("offerings down")}
	deps.Phases = catalogPhasesStub{phase: runningPhase()}
	_, err := newTestCatalog(t, deps).ListBookingStats(context.Background(), 5)
	require.ErrorContains(t, err, "offerings down")

	_, err = bookingStatsCatalog(t, runningPhase(), []careplan.CareOffering{offering},
		&catalogBookingsStub{gradesErr: errors.New("grades down")}).ListBookingStats(context.Background(), 5)
	require.ErrorContains(t, err, "grades down")

	_, err = bookingStatsCatalog(t, runningPhase(), []careplan.CareOffering{offering},
		&catalogBookingsStub{peakErr: errors.New("peak down")}).ListBookingStats(context.Background(), 5)
	require.ErrorContains(t, err, "peak down")
}
