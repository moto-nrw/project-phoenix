package settings

import (
	"context"
	"errors"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// ModuleKey identifies one additional module of the module catalog (#4812).
// moto Basis is always included and has no key.
type ModuleKey = configModel.ModuleKey

// ModuleDefinition is one entry of the central module list.
type ModuleDefinition = configModel.ModuleDefinition

// The additional modules a school can book.
const (
	ModuleNFCAnwesenheit      = configModel.ModuleNFCAnwesenheit
	ModuleMittagessen         = configModel.ModuleMittagessen
	ModuleElternApp           = configModel.ModuleElternApp
	ModuleOnlineAnmeldung     = configModel.ModuleOnlineAnmeldung
	ModuleUnterrichtVormittag = configModel.ModuleUnterrichtVormittag
	ModuleDienstplan          = configModel.ModuleDienstplan
	ModuleZeiterfassung       = configModel.ModuleZeiterfassung
	ModulePersonalakte        = configModel.ModulePersonalakte
	ModuleTraegerUebersicht   = configModel.ModuleTraegerUebersicht
	ModuleFerienbetreuung     = configModel.ModuleFerienbetreuung
	ModuleVertraege           = configModel.ModuleVertraege
	ModuleElternbeitraege     = configModel.ModuleElternbeitraege
)

// Modules returns the central module list in catalog order.
func Modules() []ModuleDefinition { return configModel.Modules() }

// ModuleSettingKeys lists the setting key of every module switch, for a
// caller that batch-resolves them (PrefetchSettings, ResolveManyForTenant).
func ModuleSettingKeys() []string {
	modules := configModel.Modules()
	keys := make([]string, len(modules))
	for i, module := range modules {
		keys[i] = module.SettingKey
	}
	return keys
}

// ModuleDependencyError rejects a module switch that breaks a prerequisite.
type ModuleDependencyError = configModel.ModuleDependencyError

// ModuleNotBookedError reports that a school has not booked a module.
type ModuleNotBookedError = configModel.ModuleNotBookedError

// ModuleBookingReader is the settings access ModuleBooking needs: the
// request tenant's value inside tenant middleware, an explicit tenant's
// value outside it (scheduler, device auth, login). Both read through the
// settings service and its request cache, never an environment variable.
type ModuleBookingReader interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
}

// ModuleBooking answers whether a school booked a module. Routes, jobs and
// portals use it to treat a module that is not booked as removed (#4893,
// #4894, #4895). A failed read is an error, never "booked" or "not booked".
type ModuleBooking struct {
	reader ModuleBookingReader
}

// NewModuleBooking binds the helper to the settings service. A missing
// reader is a composition error.
func NewModuleBooking(reader ModuleBookingReader) *ModuleBooking {
	if reader == nil {
		panic("settings: ModuleBooking requires a settings reader")
	}
	return &ModuleBooking{reader: reader}
}

// IsBooked reports whether the school of the request tenant booked module.
func (b *ModuleBooking) IsBooked(ctx context.Context, module ModuleKey) (bool, error) {
	return b.booked(ctx, nil, module)
}

// IsBookedForTenant reports whether school tenantID booked module.
func (b *ModuleBooking) IsBookedForTenant(ctx context.Context, tenantID int64, module ModuleKey) (bool, error) {
	return b.booked(ctx, &tenantID, module)
}

// booked resolves the switch of module for the request tenant (tenantID nil)
// or for an explicit school.
func (b *ModuleBooking) booked(ctx context.Context, tenantID *int64, module ModuleKey) (bool, error) {
	definition, ok := configModel.LookupModule(module)
	if !ok {
		return false, fmt.Errorf("%w: %q", ErrUnknownModule, module)
	}
	if tenantID == nil {
		return b.reader.ResolveBool(ctx, definition.SettingKey)
	}
	return b.reader.ResolveBoolForTenant(ctx, *tenantID, definition.SettingKey)
}

// Require returns a *ModuleNotBookedError for the first of modules the
// request tenant's school has not booked. tenantID names the school in the
// error; read errors pass through.
func (b *ModuleBooking) Require(ctx context.Context, tenantID int64, modules ...ModuleKey) error {
	return requireModules(tenantID, modules, func(module ModuleKey) (bool, error) {
		return b.IsBooked(ctx, module)
	})
}

// RequireForTenant is Require outside tenant middleware.
func (b *ModuleBooking) RequireForTenant(ctx context.Context, tenantID int64, modules ...ModuleKey) error {
	return requireModules(tenantID, modules, func(module ModuleKey) (bool, error) {
		return b.IsBookedForTenant(ctx, tenantID, module)
	})
}

func requireModules(tenantID int64, modules []ModuleKey, booked func(ModuleKey) (bool, error)) error {
	for _, module := range modules {
		ok, err := booked(module)
		if err != nil {
			return err
		}
		if !ok {
			return &ModuleNotBookedError{TenantID: tenantID, Module: module}
		}
	}
	return nil
}

// BookedModules lists the booked modules in catalog order from one batch
// of resolved settings, for a reader that already holds a snapshot.
func BookedModules(readBool func(key string) (bool, error)) ([]ModuleKey, error) {
	booked := []ModuleKey{}
	for _, module := range configModel.Modules() {
		ok, err := readBool(module.SettingKey)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", module.SettingKey, err)
		}
		if ok {
			booked = append(booked, module.Key)
		}
	}
	return booked, nil
}

// ErrUnknownModule reports a module key that is not in the module list.
var ErrUnknownModule = errors.New("unknown module")
