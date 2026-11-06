package delivery

import (
	"context"
	"errors"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

// EmailWebhooks takes in one signed delivery of a mail provider. The failures
// are the ErrEmailWebhook* values of the contract. It is a capability of its own
// because the webhook needs nothing of Capability.
type EmailWebhooks interface {
	IngestEmailWebhook(ctx context.Context, provider string, header func(string) string, body []byte) error
}

// EmailSuppressions frees an address after a correction, on the school's list
// and at the provider. nil means it is free as far as moto knows. Any error means
// it stays blocked and the list is unchanged:
//
//   - ErrEmailSuppressionNotFound: not on the school's list. Nothing was asked of
//     the provider. A caller that only wants the address free can treat this as done.
//   - ErrEmailSuppressionNeedsReview: a complaint on the school's own list.
//   - ErrEmailSuppressionStaysAtProvider: a block at the provider the school cannot
//     lift. It does not say why: the reason may belong to another school.
//   - anything else: a failure worth another try.
//
// Plain SMTP has no provider, so only the school's list counts there. Each method
// opens its own tenant transaction: lifting calls the provider, which a request's
// transaction must not wait for.
type EmailSuppressions interface {
	// LiftEmailSuppression frees an address someone at the school checked, on
	// the school's list and at the provider. A complaint's block stays: the
	// failure is ErrEmailSuppressionNeedsReview of the contract.
	LiftEmailSuppression(ctx context.Context, tenantID int64, address string) error
}

// EmailOutcomes reads what became of a school's mails, in the caller's tenant
// transaction. A Träger reads its mails through Capability.TraegerEmailStatuses.
type EmailOutcomes interface {
	// EmailOutcomes reports the mails of the related entities.
	EmailOutcomes(ctx context.Context, tenantID int64, query deliveryContract.EmailOutcomeQuery) ([]deliveryContract.EmailOutcomeReport, error)
	// LatestEmailOutcomes reports the newest mail of the kinds to each address.
	// The key is the normalized address; an address without a mail has no key.
	LatestEmailOutcomes(ctx context.Context, tenantID int64, kinds, addresses []string) (map[string]deliveryContract.EmailOutcomeReport, error)
	// EmailAddressMarks reports the marked addresses among the given ones. The
	// key is the normalized address.
	EmailAddressMarks(ctx context.Context, tenantID int64, addresses []string) (map[string]deliveryContract.EmailAddressMark, error)
	// StoppedEmailAddresses reports the addresses among the given ones whose
	// holders all carry the school's stop. The key is the normalized address.
	StoppedEmailAddresses(ctx context.Context, tenantID int64, addresses []string) (map[string]bool, error)
	// TraegerMailHolds reports, in the caller's Träger transaction, whether a school
	// of the Träger lists the address and whether all its holders there carry a stop.
	TraegerMailHolds(ctx context.Context, organizationID int64, address string) (listed, stopped bool, err error)
	// ProviderEmailBlocks reads the provider's current block of each address, exactly
	// for that address, keyed by normalized address. It is the one read here that
	// waits for the provider: within a small budget and only for addresses the
	// caller's own mail ran into. Ask for nothing else.
	ProviderEmailBlocks(ctx context.Context, addresses []string) map[string]deliveryContract.EmailProviderBlock
}

// ReconcileEmailDelivery compares open mails with the provider's record. Its
// error is the run's alarm. A worker without a provider's record does nothing.
func (w *Worker) ReconcileEmailDelivery(ctx context.Context) error {
	engine, ok := w.engine.(interface {
		ReconcileEmailDelivery(context.Context) error
	})
	if !ok {
		return errors.New("delivery worker: email reconciliation is not configured")
	}
	return engine.ReconcileEmailDelivery(ctx)
}
