package care_test

import (
	"context"
	"errors"

	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
)

// unconfiguredRequestSharer is the package care_test copy of the internal
// test stub. Types in package care _test.go files are not part of the
// importable care package, so the today-status and weekly-plan builders
// cannot use that type.
type unconfiguredRequestSharer struct{}

func (unconfiguredRequestSharer) ShareRequestInTx(
	_ context.Context, _, _ int64, _ string, _ int64, recipientProfileIDs []int64,
) error {
	if len(recipientProfileIDs) == 0 {
		return nil
	}
	return errors.New("parent: request sharing service is not configured")
}

func (unconfiguredRequestSharer) LoadRequestShareVisibility(context.Context, int64) (care.RequestShareVisibility, error) {
	return submitterOnlyVisibility{}, nil
}

type submitterOnlyVisibility struct{}

func (submitterOnlyVisibility) Allows(_ string, _, accountID, submittedBy int64) bool {
	return submittedBy == accountID
}

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
