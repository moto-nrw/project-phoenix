package schedule

import (
	"context"
	"errors"
	"strings"
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

func (m *mockClosingDayRepo) Update(_ context.Context, _ *schedule.ClosingDay) error {
	return m.err
}

func (m *mockClosingDayRepo) Delete(_ context.Context, _ any) error {
	return m.err
}

func (m *mockClosingDayRepo) FindByTenantID(_ context.Context) ([]*schedule.ClosingDay, error) {
	return m.days, m.err
}

func (m *mockClosingDayRepo) FindByID(_ context.Context, _ any) (*schedule.ClosingDay, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.days[0], nil
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

	err = svc.Create(context.Background(), closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), " \t\n "))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason is required")

	longReason := strings.Repeat("x", schedule.ClosingDayReasonMaxLength+1)
	err = svc.Create(context.Background(), closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), longReason))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason cannot exceed 255 characters")

	unicodeReason := strings.Repeat("ä", schedule.ClosingDayReasonMaxLength)
	require.NoError(t, svc.Create(context.Background(), closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), unicodeReason)))
	err = svc.Create(context.Background(), closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), unicodeReason+"ä"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason cannot exceed 255 characters")

	err = svc.Create(context.Background(), &schedule.ClosingDay{EndDate: timezone.NewDate(2026, 8, 7), Reason: "Ohne Start"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "start_date is required")

	err = svc.Create(context.Background(), &schedule.ClosingDay{StartDate: timezone.NewDate(2026, 8, 1), Reason: "Ohne Ende"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "end_date is required")
}

func TestClosingDayCreateWrapsRepoError(t *testing.T) {
	svc := NewClosingDayService(&mockClosingDayRepo{err: errors.New("boom")})
	valid := closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), "Sommerschließung")

	err := svc.Create(context.Background(), valid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create closing day")

	require.NoError(t, NewClosingDayService(&mockClosingDayRepo{}).Create(context.Background(), valid))
}

func TestClosingDayUpdate(t *testing.T) {
	valid := closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), "Sommerschließung")

	require.NoError(t, NewClosingDayService(&mockClosingDayRepo{}).Update(context.Background(), valid))

	err := NewClosingDayService(&mockClosingDayRepo{}).Update(context.Background(), closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), ""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reason is required")

	err = NewClosingDayService(&mockClosingDayRepo{err: errors.New("boom")}).Update(context.Background(), valid)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update closing day")
}

func TestClosingDayDelete(t *testing.T) {
	require.NoError(t, NewClosingDayService(&mockClosingDayRepo{}).Delete(context.Background(), 1))

	err := NewClosingDayService(&mockClosingDayRepo{err: errors.New("boom")}).Delete(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete closing day")
}

func TestClosingDayGetAll(t *testing.T) {
	day := closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), "Sommerschließung")

	days, err := NewClosingDayService(&mockClosingDayRepo{days: []*schedule.ClosingDay{day}}).GetAll(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []*schedule.ClosingDay{day}, days)

	_, err = NewClosingDayService(&mockClosingDayRepo{err: errors.New("boom")}).GetAll(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get all closing days")
}

func TestClosingDayGetByID(t *testing.T) {
	day := closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), "Sommerschließung")

	got, err := NewClosingDayService(&mockClosingDayRepo{days: []*schedule.ClosingDay{day}}).GetByID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, day, got)

	_, err = NewClosingDayService(&mockClosingDayRepo{err: errors.New("boom")}).GetByID(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get closing day")
}

func TestClosingDaysInRange(t *testing.T) {
	day := closingRange(timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 7), "Sommerschließung")

	days, err := NewClosingDayService(&mockClosingDayRepo{days: []*schedule.ClosingDay{day}}).ClosingDaysInRange(context.Background(), timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 31))
	require.NoError(t, err)
	assert.Equal(t, []*schedule.ClosingDay{day}, days)

	_, err = NewClosingDayService(&mockClosingDayRepo{err: errors.New("boom")}).ClosingDaysInRange(context.Background(), timezone.NewDate(2026, 8, 1), timezone.NewDate(2026, 8, 31))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closing days in range")
}
