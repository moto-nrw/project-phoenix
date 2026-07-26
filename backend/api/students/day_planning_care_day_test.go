package students

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// stubCareDayService returns a canned verdict per student and records the IDs
// it was asked about.
type stubCareDayService struct {
	verdicts map[int64]scheduleService.CareDayStatus
	err      error
	askedFor []int64
}

func (s *stubCareDayService) ResolveForDate(
	_ context.Context, studentIDs []int64, _ timezone.Date,
) (map[int64]scheduleService.CareDayStatus, error) {
	s.askedFor = studentIDs
	if s.err != nil {
		return nil, s.err
	}
	return s.verdicts, nil
}

func (s *stubCareDayService) ResolveForRange(
	context.Context, []int64, timezone.Date, timezone.Date,
) (map[int64]map[timezone.Date]scheduleService.CareDayStatus, error) {
	panic("not used by the day-planning filter")
}

func timetableSet(ids ...int64) map[int64]struct{} {
	out := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out
}

// A child assigned to a block on a weekday their care plan does not cover must
// lose the timetable signal — otherwise the student search reports "kommt
// heute" while the timetable roster leaves them out (#1747).
func TestFilterTimetableIDsByCareDayDropsNotScheduled(t *testing.T) {
	stub := &stubCareDayService{verdicts: map[int64]scheduleService.CareDayStatus{
		10: scheduleService.CareDayScheduled,
		11: scheduleService.CareDayNotScheduled,
	}}
	rs := &Resource{ResourceConfig: ResourceConfig{CareDayService: stub}}

	got, err := rs.filterTimetableIDsByCareDay(
		context.Background(), timetableSet(10, 11), timezone.NewDate(2026, 7, 20),
	)

	require.NoError(t, err)
	assert.Equal(t, timetableSet(10), got)
	assert.ElementsMatch(t, []int64{10, 11}, stub.askedFor)
}

// Children with no care plan on file resolve to unknown and must keep their
// timetable signal: a school that does not maintain arrival/pickup plans still
// sees its full roster.
func TestFilterTimetableIDsByCareDayKeepsUnknown(t *testing.T) {
	stub := &stubCareDayService{verdicts: map[int64]scheduleService.CareDayStatus{
		20: scheduleService.CareDayUnknown,
	}}
	rs := &Resource{ResourceConfig: ResourceConfig{CareDayService: stub}}

	got, err := rs.filterTimetableIDsByCareDay(
		context.Background(), timetableSet(20), timezone.NewDate(2026, 7, 20),
	)

	require.NoError(t, err)
	assert.Equal(t, timetableSet(20), got)
}

// A cancelled day loses the signal too. On a weekday ResolveDayPlanning
// answers on the exception anyway, so dropping it changes nothing there — but
// the bulk arrival/pickup loaders skip weekends without reading exceptions, so
// a weekend "kommt heute nicht" would otherwise arrive as a bare HasTimetable
// and be reported as "kommt heute" while the roster says abgemeldet (#1747).
func TestFilterTimetableIDsByCareDayDropsCancelled(t *testing.T) {
	stub := &stubCareDayService{verdicts: map[int64]scheduleService.CareDayStatus{
		21: scheduleService.CareDayCancelled,
	}}
	rs := &Resource{ResourceConfig: ResourceConfig{CareDayService: stub}}

	got, err := rs.filterTimetableIDsByCareDay(
		context.Background(), timetableSet(21), timezone.NewDate(2026, 7, 18),
	)

	require.NoError(t, err)
	assert.Empty(t, got)
}

// A student the resolver returns no entry for must not silently lose the
// signal — an empty verdict is not a "not booked" statement.
func TestFilterTimetableIDsByCareDayKeepsMissingVerdict(t *testing.T) {
	rs := &Resource{ResourceConfig: ResourceConfig{
		CareDayService: &stubCareDayService{verdicts: map[int64]scheduleService.CareDayStatus{}},
	}}

	got, err := rs.filterTimetableIDsByCareDay(
		context.Background(), timetableSet(30), timezone.NewDate(2026, 7, 20),
	)

	require.NoError(t, err)
	assert.Equal(t, timetableSet(30), got)
}

// Without the service wired the pre-#1747 behaviour stands: every assignment
// counts.
func TestFilterTimetableIDsByCareDayWithoutServiceIsUnfiltered(t *testing.T) {
	rs := &Resource{}

	got, err := rs.filterTimetableIDsByCareDay(
		context.Background(), timetableSet(40, 41), timezone.NewDate(2026, 7, 20),
	)

	require.NoError(t, err)
	assert.Equal(t, timetableSet(40, 41), got)
}

func TestFilterTimetableIDsByCareDayPropagatesError(t *testing.T) {
	stub := &stubCareDayService{err: errors.New("boom")}
	rs := &Resource{ResourceConfig: ResourceConfig{CareDayService: stub}}

	_, err := rs.filterTimetableIDsByCareDay(
		context.Background(), timetableSet(50), timezone.NewDate(2026, 7, 20),
	)

	require.Error(t, err)
}
