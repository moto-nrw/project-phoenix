package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/compose/vocabulary"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RegisterCareWithdrawalSettingsSideEffects applies the booking-led care
// switch through Care Plan inside the settings write transaction: enabling it
// is refused while a cared-for child has no care day, and disabling it closes
// the booking-derived completion tasks weekly-plan care does not keep.
func RegisterCareWithdrawalSettingsSideEffects(registry *settingsCompose.SideEffectRegistry, authority careplan.BookingAuthority) {
	registry.Register(configModels.KeyEnrollmentBookingsAuthoritative, func(ctx context.Context, _ int64, value any) (func(), error) {
		authoritative, ok := value.(bool)
		if !ok {
			return nil, nil
		}
		_, err := authority.ApplyBookingAuthoritySetting(ctx, timezone.TodayDate(), authoritative)
		return nil, err
	})
}
