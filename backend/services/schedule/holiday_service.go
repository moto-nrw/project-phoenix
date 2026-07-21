package schedule

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/holidays"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// HolidayService resolves the tenant's public holidays (gesetzliche
// Feiertage) from the operations.federal_state setting (#1418 3a). The
// dates are computed locally (internal/holidays) — there is no table and
// no external API; the setting is the single source of truth.
type HolidayService interface {
	// HolidaysInRange returns the holidays in [from, to] for the current
	// tenant (from the request context).
	HolidaysInRange(ctx context.Context, from, to timezone.Date) ([]holidays.Holiday, error)
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
	logger   *slog.Logger
}

// NewHolidayService creates the holiday resolver backed by the settings
// service.
func NewHolidayService(settings holidaySettingsResolver, logger *slog.Logger) HolidayService {
	return &holidayService{settings: settings, logger: logger}
}

func (s *holidayService) region(ctx context.Context) (string, error) {
	region, err := s.settings.ResolveString(ctx, configModel.KeyFederalState)
	if err != nil {
		return "", fmt.Errorf("failed to resolve federal state setting: %w", err)
	}
	if !holidays.ValidRegion(region) {
		return "", fmt.Errorf("federal state setting has unsupported region %q", region)
	}
	return region, nil
}

func (s *holidayService) HolidaysInRange(ctx context.Context, from, to timezone.Date) ([]holidays.Holiday, error) {
	region, err := s.region(ctx)
	if err != nil {
		return nil, err
	}
	return holidays.InRange(region, from, to)
}

func (s *holidayService) HolidayDates(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	region, err := s.region(ctx)
	if err != nil {
		return nil, err
	}
	return holidays.DateSet(region, from, to)
}
