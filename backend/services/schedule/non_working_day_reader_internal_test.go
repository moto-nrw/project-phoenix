package schedule

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHolidayService struct {
	list  []Holiday
	dates map[timezone.Date]bool
	err   error
}

func (s *stubHolidayService) HolidaysInRange(_ context.Context, _, _ timezone.Date) ([]Holiday, error) {
	return s.list, s.err
}

func (s *stubHolidayService) HolidayDates(_ context.Context, _, _ timezone.Date) (map[timezone.Date]bool, error) {
	if s.err != nil {
		return nil, s.err
	}
	// Copy so union mutation in the resolver cannot leak into the stub.
	out := make(map[timezone.Date]bool, len(s.dates))
	for d := range s.dates {
		out[d] = true
	}
	return out, nil
}

type stubClosingDayService struct {
	ClosingDayService
	dates map[timezone.Date]bool
	err   error
}

func (s *stubClosingDayService) ClosingDayDates(_ context.Context, _, _ timezone.Date) (map[timezone.Date]bool, error) {
	return s.dates, s.err
}

func TestNonWorkingDayResolverUnionsDates(t *testing.T) {
	t.Parallel()

	holiday := timezone.NewDate(2026, 10, 3)
	closing := timezone.NewDate(2026, 10, 12)
	both := timezone.NewDate(2026, 11, 1) // Allerheiligen inside a closure week

	resolver := NewNonWorkingDayResolver(
		&stubHolidayService{dates: map[timezone.Date]bool{holiday: true, both: true}},
		&stubClosingDayService{dates: map[timezone.Date]bool{closing: true, both: true}},
	)

	set, err := resolver.HolidayDates(context.Background(), timezone.NewDate(2026, 10, 1), timezone.NewDate(2026, 11, 30))
	require.NoError(t, err)

	assert.True(t, set[holiday])
	assert.True(t, set[closing])
	assert.True(t, set[both], "a day that is holiday AND closing day collapses into one entry")
	assert.Len(t, set, 3)
}

func TestNonWorkingDayResolverPassthroughKeepsHolidaysPure(t *testing.T) {
	t.Parallel()

	resolver := NewNonWorkingDayResolver(
		&stubHolidayService{list: []Holiday{{Date: timezone.NewDate(2026, 5, 1), Name: "Tag der Arbeit"}}},
		&stubClosingDayService{dates: map[timezone.Date]bool{timezone.NewDate(2026, 5, 4): true}},
	)

	list, err := resolver.HolidaysInRange(context.Background(), timezone.NewDate(2026, 5, 1), timezone.NewDate(2026, 5, 31))
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Tag der Arbeit", list[0].Name, "closing days must never leak into HolidaysInRange")
}

func TestNonWorkingDayResolverPropagatesErrors(t *testing.T) {
	t.Parallel()

	resolver := NewNonWorkingDayResolver(
		&stubHolidayService{err: errors.New("holiday boom")},
		&stubClosingDayService{},
	)
	_, err := resolver.HolidayDates(context.Background(), timezone.NewDate(2026, 5, 1), timezone.NewDate(2026, 5, 31))
	require.Error(t, err)

	resolver = NewNonWorkingDayResolver(
		&stubHolidayService{dates: map[timezone.Date]bool{}},
		&stubClosingDayService{err: errors.New("closing boom")},
	)
	_, err = resolver.HolidayDates(context.Background(), timezone.NewDate(2026, 5, 1), timezone.NewDate(2026, 5, 31))
	require.Error(t, err)
}
