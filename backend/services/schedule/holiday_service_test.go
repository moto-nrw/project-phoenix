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
	settings := &holidayMockSettings{region: "DE-SN"}
	svc := NewHolidayService(settings, nil)

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
	svc := NewHolidayService(&holidayMockSettings{region: "DE-NW"}, nil)

	set, err := svc.HolidayDates(context.Background(),
		timezone.NewDate(2026, time.May, 1), timezone.NewDate(2026, time.May, 31))
	require.NoError(t, err)
	assert.True(t, set[timezone.NewDate(2026, time.May, 1)], "Tag der Arbeit")
	assert.True(t, set[timezone.NewDate(2026, time.May, 25)], "Pfingstmontag")
	assert.False(t, set[timezone.NewDate(2026, time.May, 2)])
}

func TestHolidayServiceErrors(t *testing.T) {
	_, err := NewHolidayService(&holidayMockSettings{err: errors.New("boom")}, nil).
		HolidayDates(context.Background(), timezone.NewDate(2026, time.January, 1), timezone.NewDate(2026, time.January, 2))
	assert.Error(t, err)

	_, err = NewHolidayService(&holidayMockSettings{region: "XX"}, nil).
		HolidayDates(context.Background(), timezone.NewDate(2026, time.January, 1), timezone.NewDate(2026, time.January, 2))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported region")

	_, err = NewHolidayService(&holidayMockSettings{err: errors.New("boom")}, nil).
		HolidaysInRange(context.Background(), timezone.NewDate(2026, time.January, 1), timezone.NewDate(2026, time.January, 2))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve federal state setting")
}
