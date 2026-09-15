package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

type holidayQueryStub struct {
	valid bool
	list  []CalendarHoliday
	dates map[string]bool
	err   error
}

func (s holidayQueryStub) ValidHolidayRegion(string) bool { return s.valid }
func (s holidayQueryStub) ListHolidays(context.Context, string, string, string) ([]CalendarHoliday, error) {
	return s.list, s.err
}
func (s holidayQueryStub) HolidayDates(context.Context, string, string, string) (map[string]bool, error) {
	return s.dates, s.err
}

type holidayMockSettings struct {
	region string
	err    error
	key    string
}

func (m *holidayMockSettings) ResolveString(_ context.Context, key string) (string, error) {
	m.key = key
	return m.region, m.err
}

func TestHolidayServiceResolvesRegionFromSetting(t *testing.T) {
	t.Parallel()

	settings := &holidayMockSettings{region: "DE-SN"}
	svc := NewHolidayService(settings, holidayQueryStub{valid: true, list: []CalendarHoliday{
		{Date: "2026-11-18", Name: "Buß- und Bettag"},
	}}, nil)

	list, err := svc.HolidaysInRange(context.Background(),
		timezone.NewDate(2026, time.November, 1), timezone.NewDate(2026, time.November, 30))
	require.NoError(t, err)
	assert.Equal(t, configModel.KeyFederalState, settings.key)

	// Sachsen has Buß- und Bettag (first Wednesday between Nov 16-22:
	// 2026-11-18); NRW would have none in this window besides Allerheiligen.
	names := make([]string, 0, len(list))
	for _, h := range list {
		names = append(names, h.Name)
	}
	assert.Contains(t, names, "Buß- und Bettag")
	assert.NotContains(t, names, "Allerheiligen")
}

func TestHolidayServiceDates(t *testing.T) {
	t.Parallel()

	svc := NewHolidayService(&holidayMockSettings{region: "DE-NW"}, holidayQueryStub{valid: true, dates: map[string]bool{
		"2026-05-01": true,
		"2026-05-25": true,
	}}, nil)

	set, err := svc.HolidayDates(context.Background(),
		timezone.NewDate(2026, time.May, 1), timezone.NewDate(2026, time.May, 31))
	require.NoError(t, err)
	assert.True(t, set[timezone.NewDate(2026, time.May, 1)], "Tag der Arbeit")
	assert.True(t, set[timezone.NewDate(2026, time.May, 25)], "Pfingstmontag")
	assert.False(t, set[timezone.NewDate(2026, time.May, 2)])
}

func TestHolidayServiceErrors(t *testing.T) {
	t.Parallel()

	query := holidayQueryStub{valid: true}
	_, err := NewHolidayService(&holidayMockSettings{err: errors.New("boom")}, query, nil).
		HolidayDates(context.Background(), timezone.NewDate(2026, time.January, 1), timezone.NewDate(2026, time.January, 2))
	assert.Error(t, err)

	_, err = NewHolidayService(&holidayMockSettings{region: "XX"}, holidayQueryStub{}, nil).
		HolidayDates(context.Background(), timezone.NewDate(2026, time.January, 1), timezone.NewDate(2026, time.January, 2))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported region")

	_, err = NewHolidayService(&holidayMockSettings{err: errors.New("boom")}, query, nil).
		HolidaysInRange(context.Background(), timezone.NewDate(2026, time.January, 1), timezone.NewDate(2026, time.January, 2))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve federal state setting")
}
