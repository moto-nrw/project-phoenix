package schedule

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

type Holiday struct {
	Date timezone.Date `json:"date"`
	Name string        `json:"name"`
}

type CalendarHoliday struct {
	Date string
	Name string
}

type HolidayCalendar interface {
	ValidHolidayRegion(string) bool
	ListHolidays(context.Context, string, string, string) ([]CalendarHoliday, error)
	HolidayDates(context.Context, string, string, string) (map[string]bool, error)
}

// HolidayService resolves the tenant's public holidays (gesetzliche
// Feiertage) from the operations.federal_state setting (#1418 3a). The
// dates are computed locally by the School Calendar capability — there is no
// table or external API; the setting is the single source of truth.
type HolidayService interface {
	// HolidaysInRange returns the holidays in [from, to] for the current
	// tenant (from the request context).
	HolidaysInRange(ctx context.Context, from, to timezone.Date) ([]Holiday, error)
	// HolidayDates returns the same holidays as a date set for O(1) lookups
	// in Soll computations.
	HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error)
}

// holidaySettingsResolver is the slice of config.SettingsService the
// holiday service needs.
type holidaySettingsResolver interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

type holidayService struct {
	settings holidaySettingsResolver
	calendar HolidayCalendar
	logger   *slog.Logger
}

// NewHolidayService creates the holiday resolver backed by the settings
// service.
func NewHolidayService(settings holidaySettingsResolver, calendar HolidayCalendar, logger *slog.Logger) HolidayService {
	if settings == nil || calendar == nil {
		panic("holiday service: settings and school calendar are required")
	}
	return &holidayService{settings: settings, calendar: calendar, logger: logger}
}

func (s *holidayService) region(ctx context.Context) (string, error) {
	region, err := s.settings.ResolveString(ctx, configModel.KeyFederalState)
	if err != nil {
		return "", fmt.Errorf("failed to resolve federal state setting: %w", err)
	}
	if !s.calendar.ValidHolidayRegion(region) {
		return "", fmt.Errorf("federal state setting has unsupported region %q", region)
	}
	return region, nil
}

func (s *holidayService) HolidaysInRange(ctx context.Context, from, to timezone.Date) ([]Holiday, error) {
	region, err := s.region(ctx)
	if err != nil {
		return nil, err
	}
	values, err := s.calendar.ListHolidays(ctx, region, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	result := make([]Holiday, 0, len(values))
	for _, value := range values {
		result = append(result, Holiday{Date: timezone.Date(value.Date), Name: value.Name})
	}
	return result, nil
}

func (s *holidayService) HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	region, err := s.region(ctx)
	if err != nil {
		return nil, err
	}
	values, err := s.calendar.HolidayDates(ctx, region, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	result := make(map[timezone.Date]bool, len(values))
	for date := range values {
		result[timezone.Date(date)] = true
	}
	return result, nil
}
