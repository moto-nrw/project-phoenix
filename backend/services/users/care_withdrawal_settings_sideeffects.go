package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
)

// RegisterCareWithdrawalSettingsSideEffects keeps pending booking-derived
// completion tasks from surviving a switch to weekly-plan-driven care.
func RegisterCareWithdrawalSettingsSideEffects(registry *sideeffects.Registry, lifecycle CareLifecycleService) {
	registry.Register(configModel.KeyEnrollmentBookingsAuthoritative, func(ctx context.Context, _ int64, value any) (func(), error) {
		authoritative, ok := value.(bool)
		if !ok {
			return nil, nil
		}
		_, err := lifecycle.ApplyBookingAuthoritySetting(ctx, timezone.TodayDate(), authoritative)
		return nil, err
	})
}
