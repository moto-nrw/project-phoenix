package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
)

// registerCareWithdrawalSettingsSideEffects applies the booking-led care
// switch through Care Plan inside the settings write transaction: enabling it
// is refused while a cared-for child has no care day, and disabling it closes
// the booking-derived completion tasks weekly-plan care does not keep.
func registerCareWithdrawalSettingsSideEffects(registry *sideeffects.Registry, authority careplan.BookingAuthority) {
	registry.Register(configModels.KeyEnrollmentBookingsAuthoritative, func(ctx context.Context, _ int64, value any) (func(), error) {
		authoritative, ok := value.(bool)
		if !ok {
			return nil, nil
		}
		_, err := authority.ApplyBookingAuthoritySetting(ctx, timezone.TodayDate(), authoritative)
		return nil, err
	})
}
