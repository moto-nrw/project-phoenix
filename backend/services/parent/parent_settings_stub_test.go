package parent_test

import (
	"context"

	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// parentSettingsStub answers ResolveBoolForTenant / ResolveStringForTenant
// from maps; every other SettingsService method panics via the embedded nil
// interface, keeping the tests honest about what the services touch.
type parentSettingsStub struct {
	configService.SettingsService
	boolValues   map[string]bool
	boolDefault  bool  // returned for keys not in boolValues
	boolErr      error // returned by every bool lookup when set
	stringValues map[string]string
	stringErr    error
}

func (s parentSettingsStub) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	if s.boolErr != nil {
		return false, s.boolErr
	}
	if v, ok := s.boolValues[key]; ok {
		return v, nil
	}
	return s.boolDefault, nil
}

func (s parentSettingsStub) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if s.stringErr != nil {
		return "", s.stringErr
	}
	return s.stringValues[key], nil
}
