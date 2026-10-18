package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RegisterCareWithdrawalSettingsSideEffects applies the booking-led care
// switch through Care Plan inside the settings write transaction: enabling it
// is refused while a cared-for child has no care day, and disabling it closes
// the booking-derived completion tasks weekly-plan care does not keep. The
// refusal reaches the settings routes as Settings Platform's own
// BookingAuthorityBlockedError (#4072).
func RegisterCareWithdrawalSettingsSideEffects(registry *settingsCompose.SideEffectRegistry, authority careplan.BookingAuthority) {
	registry.Register(configModels.KeyEnrollmentBookingsAuthoritative, func(ctx context.Context, _ int64, value any) (func(), error) {
		authoritative, ok := value.(bool)
		if !ok {
			return nil, nil
		}
		_, err := authority.ApplyBookingAuthoritySetting(ctx, timezone.TodayDate(), authoritative)
		if errors.Is(err, careplan.ErrBookingAuthorityBlocked) {
			return nil, &settings.BookingAuthorityBlockedError{Reason: err}
		}
		return nil, err
	})
}
