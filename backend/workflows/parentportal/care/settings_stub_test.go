package care_test

import (
	"context"

	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// parentSettingsStub answers ResolveBoolForTenant / ResolveStringForTenant
// from maps; every other SettingsService method panics via the embedded nil
// interface, keeping the tests honest about what the services touch. Test-only
// copy of the services/parent stub for the relocated care-schedule tests.
type parentSettingsStub struct {
	configService.SettingsService
	boolValues   map[string]bool
	stringValues map[string]string
}

func (s parentSettingsStub) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	return s.boolValues[key], nil
}

func (s parentSettingsStub) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	return s.stringValues[key], nil
}
