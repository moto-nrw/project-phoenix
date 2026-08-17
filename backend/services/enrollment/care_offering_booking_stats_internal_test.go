package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// The three stubs embed their interface and override only what
// ListBookingStats touches; an unimplemented call panics loudly rather than
// silently returning a zero value.

type bookingStatsPhaseRepo struct {
	enrollmentModels.PhaseRepository
	phase *enrollmentModels.Phase
	err   error
}

func (r bookingStatsPhaseRepo) FindByID(context.Context, int64) (*enrollmentModels.Phase, error) {
	return r.phase, r.err
}

type bookingStatsOfferingRepo struct {
	enrollmentModels.CareOfferingRepository
	offerings []*enrollmentModels.CareOffering
	err       error
}

func (r bookingStatsOfferingRepo) ListByPhase(context.Context, int64) ([]*enrollmentModels.CareOffering, error) {
	return r.offerings, r.err
}

type bookingStatsLinkRepo struct {
	enrollmentModels.RequestChildOfferingRepository
	grades    []*enrollmentModels.CareOfferingGradeLevelCount
	gradesErr error
	peaks     map[int64]int
	peakErr   error

	gotIDs        []int64
	gotPeakIDs    []int64
	gotFrom       timezone.Date
	gotUntil      timezone.Date
	peakCallCount int
}

func (r *bookingStatsLinkRepo) CountActiveGradeLevelsByCareOfferingIDs(
	_ context.Context, ids []int64, from, until timezone.Date,
) ([]*enrollmentModels.CareOfferingGradeLevelCount, error) {
	r.gotIDs = ids
	r.gotFrom = from
	r.gotUntil = until
	return r.grades, r.gradesErr
}

func (r *bookingStatsLinkRepo) CountMaxActiveByCareOfferingIDsInRange(
	_ context.Context, ids []int64, _, _ timezone.Date,
) (map[int64]int, error) {
	r.peakCallCount++
	r.gotPeakIDs = ids
	if r.peakErr != nil {
		return nil, r.peakErr
	}
	return r.peaks, nil
}

func intPtr(v int) *int { return &v }

func bookingStatsService(
	phase *enrollmentModels.Phase,
	offerings []*enrollmentModels.CareOffering,
	links *bookingStatsLinkRepo,
) *careOfferingService {
	return &careOfferingService{CareOfferingServiceConfig: CareOfferingServiceConfig{
		Repo:                     bookingStatsOfferingRepo{offerings: offerings},
		PhaseRepo:                bookingStatsPhaseRepo{phase: phase},
		RequestChildOfferingRepo: links,
	}}
}

func runningPhase() *enrollmentModels.Phase {
	today := timezone.TodayDate()
	return &enrollmentModels.Phase{
		ServiceStartDate: today.AddDays(-30),
		ServiceEndDate:   today.AddDays(60),
	}
}

func TestListBookingStats_ReportsCapacityAndGradeDistribution(t *testing.T) {
	offerings := []*enrollmentModels.CareOffering{
		{Name: "Randstunde", Capacity: intPtr(20)},
		{Name: "Ganztag"},
	}
	offerings[0].ID = 7
	offerings[1].ID = 9
	links := &bookingStatsLinkRepo{
		peaks: map[int64]int{7: 14, 9: 3},
		grades: []*enrollmentModels.CareOfferingGradeLevelCount{
			{CareOfferingID: 7, GradeLevel: gradePtr(1), Count: 12},
			{CareOfferingID: 7, GradeLevel: gradePtr(3), Count: 2},
			{CareOfferingID: 9, GradeLevel: nil, Count: 3},
		},
	}

	stats, err := bookingStatsService(runningPhase(), offerings, links).
		ListBookingStats(context.Background(), 5)
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
	// The displayed occupancy must be the number the capacity gate will apply
	// on save, so the window has to match applyCapacityOverflowCore's.
	today := timezone.TodayDate()
	phase := &enrollmentModels.Phase{
		ServiceStartDate: today.AddDays(-30),
		ServiceEndDate:   today.AddDays(60),
	}
	offering := &enrollmentModels.CareOffering{Name: "Ganztag"}
	offering.ID = 1
	links := &bookingStatsLinkRepo{peaks: map[int64]int{1: 0}}

	_, err := bookingStatsService(phase, []*enrollmentModels.CareOffering{offering}, links).
		ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, today, links.gotFrom, "a running phase counts from today")
	assert.Equal(t, phase.ServiceEndDate.AddDays(1), links.gotUntil, "the last service day is inclusive")
	assert.Equal(t, []int64{1}, links.gotIDs)
}

func TestListBookingStats_CountsFromTheStartOfAFuturePhase(t *testing.T) {
	today := timezone.TodayDate()
	phase := &enrollmentModels.Phase{
		ServiceStartDate: today.AddDays(30),
		ServiceEndDate:   today.AddDays(200),
	}
	offering := &enrollmentModels.CareOffering{Name: "Ganztag"}
	offering.ID = 1
	links := &bookingStatsLinkRepo{peaks: map[int64]int{1: 0}}

	_, err := bookingStatsService(phase, []*enrollmentModels.CareOffering{offering}, links).
		ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, phase.ServiceStartDate, links.gotFrom)
}

func TestListBookingStats_FallsBackToTheFinalDayOfACompletedPhase(t *testing.T) {
	// [today, end+1) is empty once a phase has ended. Reporting zero would
	// claim every offering is free; the final service day is the honest
	// answer for a historical phase.
	today := timezone.TodayDate()
	phase := &enrollmentModels.Phase{
		ServiceStartDate: today.AddDays(-200),
		ServiceEndDate:   today.AddDays(-30),
	}
	offering := &enrollmentModels.CareOffering{Name: "Ganztag"}
	offering.ID = 1
	links := &bookingStatsLinkRepo{peaks: map[int64]int{1: 4}}

	stats, err := bookingStatsService(phase, []*enrollmentModels.CareOffering{offering}, links).
		ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, phase.ServiceEndDate, links.gotFrom)
	assert.Equal(t, phase.ServiceEndDate.AddDays(1), links.gotUntil)
	assert.Equal(t, 4, stats[0].Booked)
}

func TestBookingStatsWindow_DefaultsToTodayWithoutPhaseDates(t *testing.T) {
	today := timezone.TodayDate()

	from, until := bookingStatsWindow(nil)
	assert.Equal(t, today, from)
	assert.Equal(t, today.AddDays(1), until)

	from, until = bookingStatsWindow(&enrollmentModels.Phase{})
	assert.Equal(t, today, from)
	assert.Equal(t, today.AddDays(1), until)
	assert.True(t, from.Before(until), "the window is never empty")
}

func TestListBookingStats_ReturnsEmptyWithoutOfferings(t *testing.T) {
	links := &bookingStatsLinkRepo{}

	stats, err := bookingStatsService(runningPhase(), nil, links).
		ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Empty(t, stats)
	assert.Zero(t, links.peakCallCount, "no offerings means no occupancy queries")
	assert.Nil(t, links.gotIDs)
}

// #2186 review: occupancy used to be one query per offering on top of the
// already-batched grade counts.
func TestListBookingStats_BatchesTheOccupancyQuery(t *testing.T) {
	offerings := []*enrollmentModels.CareOffering{{Name: "A"}, {Name: "B"}, {Name: "C"}}
	for i, offering := range offerings {
		offering.ID = int64(i + 1)
	}
	links := &bookingStatsLinkRepo{peaks: map[int64]int{1: 4, 3: 9}}

	stats, err := bookingStatsService(runningPhase(), offerings, links).
		ListBookingStats(context.Background(), 5)
	require.NoError(t, err)

	assert.Equal(t, 1, links.peakCallCount, "one query for every offering, not one each")
	assert.Equal(t, []int64{1, 2, 3}, links.gotPeakIDs)
	// Neither aggregate is phase-scoped: both must count the same population
	// the capacity gate enforces on, or the dialog offers a slot the gate
	// then refuses (#2186 review round 2).
	assert.Equal(t, []int64{1, 2, 3}, links.gotIDs)
	assert.Equal(t, 4, stats[0].Booked)
	assert.Zero(t, stats[1].Booked, "an offering with no bookings reads as zero")
	assert.Equal(t, 9, stats[2].Booked)
}

func TestListBookingStats_RejectsAnInvalidPhaseID(t *testing.T) {
	_, err := bookingStatsService(runningPhase(), nil, &bookingStatsLinkRepo{}).
		ListBookingStats(context.Background(), 0)
	require.ErrorIs(t, err, ErrCareOfferingInvalid)
}

func TestListBookingStats_ClassifiesAMissingPhaseAsInvalid(t *testing.T) {
	service := &careOfferingService{CareOfferingServiceConfig: CareOfferingServiceConfig{
		Repo:                     bookingStatsOfferingRepo{},
		PhaseRepo:                bookingStatsPhaseRepo{err: sql.ErrNoRows},
		RequestChildOfferingRepo: &bookingStatsLinkRepo{},
	}}

	_, err := service.ListBookingStats(context.Background(), 5)
	require.ErrorIs(t, err, ErrCareOfferingInvalid)
}

func TestListBookingStats_RequiresItsRepositories(t *testing.T) {
	noLinks := &careOfferingService{CareOfferingServiceConfig: CareOfferingServiceConfig{
		Repo:      bookingStatsOfferingRepo{},
		PhaseRepo: bookingStatsPhaseRepo{phase: runningPhase()},
	}}
	_, err := noLinks.ListBookingStats(context.Background(), 5)
	require.ErrorContains(t, err, "request child offering repository")

	noPhases := &careOfferingService{CareOfferingServiceConfig: CareOfferingServiceConfig{
		Repo:                     bookingStatsOfferingRepo{},
		RequestChildOfferingRepo: &bookingStatsLinkRepo{},
	}}
	_, err = noPhases.ListBookingStats(context.Background(), 5)
	require.ErrorContains(t, err, "phase repository")
}

func TestListBookingStats_PropagatesRepositoryFailures(t *testing.T) {
	offering := &enrollmentModels.CareOffering{Name: "Ganztag"}
	offering.ID = 1

	offeringsFailed := &careOfferingService{CareOfferingServiceConfig: CareOfferingServiceConfig{
		Repo:                     bookingStatsOfferingRepo{err: errors.New("offerings down")},
		PhaseRepo:                bookingStatsPhaseRepo{phase: runningPhase()},
		RequestChildOfferingRepo: &bookingStatsLinkRepo{},
	}}
	_, err := offeringsFailed.ListBookingStats(context.Background(), 5)
	require.ErrorContains(t, err, "offerings down")

	gradesFailed := bookingStatsService(
		runningPhase(),
		[]*enrollmentModels.CareOffering{offering},
		&bookingStatsLinkRepo{gradesErr: errors.New("grades down")},
	)
	_, err = gradesFailed.ListBookingStats(context.Background(), 5)
	require.ErrorContains(t, err, "grades down")

	peakFailed := bookingStatsService(
		runningPhase(),
		[]*enrollmentModels.CareOffering{offering},
		&bookingStatsLinkRepo{peakErr: errors.New("peak down")},
	)
	_, err = peakFailed.ListBookingStats(context.Background(), 5)
	require.ErrorContains(t, err, "peak down")
}
