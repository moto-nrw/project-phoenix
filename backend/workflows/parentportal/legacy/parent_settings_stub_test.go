package legacy_test

import (
	"context"
	"strconv"

	configModels "github.com/moto-nrw/project-phoenix/models/config"

	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// parentSettingsStub answers ResolveBoolForTenant / ResolveStringForTenant
// from maps; every other SettingsService method panics via the embedded nil
// interface, keeping the tests honest about what the services touch.
type parentSettingsStub struct {
	configService.SettingsService
	boolValues               map[string]bool
	boolDefault              bool  // returned for keys not in boolValues
	boolErr                  error // returned by every bool lookup when set
	stringValues             map[string]string
	stringErr                error
	stringInTxFn             func(string) (string, error)
	lockPickupChangePolicyFn func(context.Context, int64) error
}

func (s parentSettingsStub) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	if s.boolErr != nil {
		return false, s.boolErr
	}
	if v, ok := s.boolValues[key]; ok {
		return v, nil
	}
	if key == configModels.KeyParentSickReportsEnabled || key == configModels.KeyParentExcusedReportsEnabled {
		return true, nil
	}
	return s.boolDefault, nil
}

func (s parentSettingsStub) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if s.stringErr != nil {
		return "", s.stringErr
	}
	return s.stringValues[key], nil
}

func (s parentSettingsStub) ResolveStringForTenantInTx(_ context.Context, _ int64, key string) (string, error) {
	if s.stringInTxFn != nil {
		return s.stringInTxFn(key)
	}
	if s.stringErr != nil {
		return "", s.stringErr
	}
	if value, ok := s.boolValues[key]; ok {
		return strconv.FormatBool(value), nil
	}
	return s.stringValues[key], nil
}

func (s parentSettingsStub) LockParentPickupChangePolicySharedForTenant(ctx context.Context, tenantID int64) error {
	if s.lockPickupChangePolicyFn != nil {
		return s.lockPickupChangePolicyFn(ctx, tenantID)
	}
	return nil
}
