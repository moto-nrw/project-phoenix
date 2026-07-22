package schedule

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockClosingDayRepo overrides only the methods the service under test uses;
// the embedded interface panics on anything else, which is exactly what we
// want in a focused unit test.
type mockClosingDayRepo struct {
	schedule.ClosingDayRepository
	days []*schedule.ClosingDay
	err  error
}

func (m *mockClosingDayRepo) FindOverlappingRange(_ context.Context, _, _ timezone.Date) ([]*schedule.ClosingDay, error) {
	return m.days, m.err
}

func (m *mockClosingDayRepo) Create(_ context.Context, _ *schedule.ClosingDay) error {
	return m.err
}

func closingRange(start, end timezone.Date, reason string) *schedule.ClosingDay {
	return &schedule.ClosingDay{StartDate: start, EndDate: end, Reason: reason}
}

func TestClosingDayDatesExpandsRanges(t *testing.T) {
	svc := NewClosingDayService(&mockClosingDayRepo{days: []*schedule.ClosingDay{
		closingRange(timezone.NewDate(2026, 12, 24), timezone.NewDate(2026, 12, 27), "Weihnachten"),
		closingRange(timezone.NewDate(2026, 12, 31), timezone.NewDate(2026, 12, 31), "Silvester"),
	}})

	set, err := svc.ClosingDayDates(context.Background(), timezone.NewDate(2026, 12, 1), timezone.NewDate(2026, 12, 31))
	require.NoError(t, err)

	assert.Len(t, set, 5)
	assert.True(t, set[timezone.NewDate(2026, 12, 24)])
	assert.True(t, set[timezone.NewDate(2026, 12, 27)])
	assert.True(t, set[timezone.NewDate(2026, 12, 31)])
	assert.False(t, set[timezone.NewDate(2026, 12, 28)])
}

func TestClosingDayDatesClampsToWindow(t *testing.T) {
	// Range extends past both window edges; only in-window days may appear.
	svc := NewClosingDayService(&mockClosingDayRepo{days: []*schedule.ClosingDay{
		closingRange(timezone.NewDate(2026, 7, 20), timezone.NewDate(2026, 8, 7), "Sommerschließung"),
	}})

	set, err := svc.ClosingDayDates(context.Background(), timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 3))
	require.NoError(t, err)

	assert.Len(t, set, 3)
	assert.True(t, set[timezone.NewDate(2026, 8, 1)])
	assert.True(t, set[timezone.NewDate(2026, 8, 3)])
	assert.False(t, set[timezone.NewDate(2026, 7, 31)])
	assert.False(t, set[timezone.NewDate(2026, 8, 4)])
}

func TestClosingDayDatesEmptyOnInvertedWindow(t *testing.T) {
	svc := NewClosingDayService(&mockClosingDayRepo{})

	set, err := svc.ClosingDayDates(context.Background(), timezone.NewDate(2026, 8, 3), timezone.NewDate(2026, 8, 1))
	require.NoError(t, err)
	assert.Empty(t, set)
}

func TestClosingDayDatesPropagatesRepoError(t *testing.T) {
	svc := NewClosingDayService(&mockClosingDayRepo{err: errors.New("boom")})

	_, err := svc.ClosingDayDates(context.Background(), timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 3))
	require.Error(t, err)
}

func TestClosingDayCreateRejectsInvalid(t *testing.T) {
	svc := NewClosingDayService(&mockClosingDayRepo{})

	err := svc.Create(context.Background(), closingRange(timezone.NewDate(2026, 8, 7), timezone.NewDate(2026, 8, 1), "Verkehrt"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end_date must not be before start_date")

	err = svc.Create(context.Background(), closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason is required")
}
