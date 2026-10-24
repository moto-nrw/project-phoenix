package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// FindAnalyticsConsent reads the account holder's own answer to the usage
// analytics (#4901). An account without an answer reports a nil Granted. Like
// the password change, it is the holder's business on the global account row
// and runs on the global identity unit.
func (a *AccountAdministration) FindAnalyticsConsent(ctx context.Context, accountID int64) (identityaccess.AnalyticsConsent, error) {
	if a == nil {
		return identityaccess.AnalyticsConsent{}, errAccountAdministrationUnavailable
	}
	ctx = bindRuntime(a.auth.attach, ctx)
	var result domain.AnalyticsConsent
	err := globalIdentityRead(ctx, a.runtime, func(txCtx context.Context) error {
		value, found, _, err := a.store.FindAnalyticsConsent(txCtx, accountID)
		result = value
		return stateChange(found, err, domain.ErrAccountNotFound)
	})
	if err != nil {
		return identityaccess.AnalyticsConsent{}, administrationError(err)
	}
	return identityaccess.AnalyticsConsent(result), nil
}

// DecideAnalyticsConsent records the holder's answer with the time it was
// given and returns the stored answer, read back in the same unit, so the
// caller confirms what was persisted. Repeating an answer renews its time:
// the latest decision is the evidence.
func (a *AccountAdministration) DecideAnalyticsConsent(ctx context.Context, accountID int64, granted bool) (identityaccess.AnalyticsConsent, error) {
	if a == nil {
		return identityaccess.AnalyticsConsent{}, errAccountAdministrationUnavailable
	}
	ctx = bindRuntime(a.auth.attach, ctx)
	var result domain.AnalyticsConsent
	err := globalIdentityRead(ctx, a.runtime, func(txCtx context.Context) error {
		written, _, err := a.store.SetAnalyticsConsent(txCtx, accountID, granted)
		if err = stateChange(written, err, domain.ErrAccountNotFound); err != nil {
			return err
		}
		value, found, _, err := a.store.FindAnalyticsConsent(txCtx, accountID)
		result = value
		return stateChange(found, err, domain.ErrAccountNotFound)
	})
	if err != nil {
		return identityaccess.AnalyticsConsent{}, administrationError(err)
	}
	return identityaccess.AnalyticsConsent(result), nil
}
