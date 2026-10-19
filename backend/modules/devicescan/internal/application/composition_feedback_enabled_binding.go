package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

var ErrSettingsNotConfigured = errors.New("settings service is not configured")

type ScanSettings interface {
	FeedbackEnabled(ctx context.Context) (bool, error)
	RoomCapacityDetailsDisclosed(ctx context.Context) (bool, error)
	ActivityCapacityDetailsDisclosed(ctx context.Context) (bool, error)
	DailyCheckoutTime(ctx context.Context) (string, error)
	PerStudentCheckoutEnabled(ctx context.Context) (bool, error)
	PerStudentCheckoutDeltaMinutes(ctx context.Context) (int, error)
	DailyCheckoutFromAllRoomsEnabled(ctx context.Context) (bool, error)
}

func FeedbackEnabledBinding(ctx context.Context, source ScanSettings) (bool, error) {
	if source == nil {
		return false, ErrSettingsNotConfigured
	}
	return source.FeedbackEnabled(ctx)
}
func CapacityDetailsDisclosedBinding(ctx context.Context, source ScanSettings, kind ports.CapacityKind) (bool, error) {
	if source == nil {
		return false, ErrSettingsNotConfigured
	}
	if kind == ports.CapacityActivity {
		return source.ActivityCapacityDetailsDisclosed(ctx)
	}
	return source.RoomCapacityDetailsDisclosed(ctx)
}
func DailyCheckoutTimeBinding(ctx context.Context, source ScanSettings) (string, error) {
	if source == nil {
		return "", ErrSettingsNotConfigured
	}
	return source.DailyCheckoutTime(ctx)
}
func PerStudentCheckoutEnabledBinding(ctx context.Context, source ScanSettings) (bool, error) {
	if source == nil {
		return false, nil
	}
	return source.PerStudentCheckoutEnabled(ctx)
}
func PerStudentCheckoutDeltaMinutesBinding(ctx context.Context, source ScanSettings) (int, error) {
	if source == nil {
		return 0, ErrSettingsNotConfigured
	}
	return source.PerStudentCheckoutDeltaMinutes(ctx)
}
func DailyCheckoutFromAllRoomsEnabledBinding(ctx context.Context, source ScanSettings) (bool, error) {
	if source == nil {
		return false, nil
	}
	return source.DailyCheckoutFromAllRoomsEnabled(ctx)
}
