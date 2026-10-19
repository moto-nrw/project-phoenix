package api

import (
	"context"
	"errors"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/realtime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SettingsWrites are the settings side effects the API dispatches on a
// value change and the tenant settings operations.
type SettingsWrites struct {
	SideEffects configModels.SideEffectDispatcher
	Tenant      configModels.TenantOperationsService
}

// buildSettingsWrites binds the side effects of Facilities (the Schulhof and
// WC checkout switches), Care Plan (the booking authority) and the photo
// lifecycle (the feature toggle), and the tenant operations over them.
func buildSettingsWrites(
	env owners.RootEnvironment, settings owners.SettingsOwner, payroll settingsCompose.PayrollStatusGetter, yard FacilitiesYard,
	authority careplan.BookingAuthority, photos StudentPhotoFeatureToggle, hub *realtime.Hub,
) (SettingsWrites, error) {
	schulhof, wc := facilitiesCompose.SettingHandlers(yard.Schulhof, yard.WC)
	registry, err := settingsCompose.NewSideEffects(env.Observe.SettingsSideEffectFailure,
		settingsCompose.SettingSideEffect{Key: configModels.KeyCheckoutSchulhofEnabled, Handle: schulhof},
		settingsCompose.SettingSideEffect{Key: configModels.KeyCheckoutWCEnabled, Handle: wc},
		careWithdrawalSettingSideEffect(authority),
		settingsCompose.SettingSideEffect{Key: configModels.KeyStudentPhotosEnabled, Handle: photos.HandleFeatureToggle},
	)
	if err != nil {
		return SettingsWrites{}, err
	}
	return SettingsWrites{
		SideEffects: registry,
		Tenant:      settingsCompose.NewTenantOperations(settings.Service, payroll, settings.Rows, registry.Dispatch, owners.SettingsChangedNotifier(hub)),
	}, nil
}

// careWithdrawalSettingSideEffect applies a change of
// enrollment.bookings_authoritative through Care Plan's booking authority.
func careWithdrawalSettingSideEffect(authority careplan.BookingAuthority) settingsCompose.SettingSideEffect {
	return settingsCompose.SettingSideEffect{Key: configModels.KeyEnrollmentBookingsAuthoritative, Handle: func(ctx context.Context, _ int64, value any) (func(), error) {
		authoritative, ok := value.(bool)
		if !ok {
			return nil, nil
		}
		_, err := authority.ApplyBookingAuthoritySetting(ctx, timezone.TodayDate(), authoritative)
		if errors.Is(err, careplan.ErrBookingAuthorityBlocked) {
			return nil, &settingsModule.BookingAuthorityBlockedError{Reason: err}
		}
		return nil, err
	}}
}
