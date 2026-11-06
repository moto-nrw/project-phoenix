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

// EmailSuppressions changes a school's list. Each method opens its own tenant
// transaction: lifting calls the provider, which a request's transaction must
// not wait for.
type EmailSuppressions interface {
	// LiftEmailSuppression frees an address someone at the school checked, on
	// the school's list and at the provider. A complaint's block stays: the
	// failure is ErrEmailSuppressionNeedsReview of the contract.
	LiftEmailSuppression(ctx context.Context, tenantID int64, address string) error
	// StopEmails puts an address on the list on the school's own decision. The
	// mark of a complaint stays.
	StopEmails(ctx context.Context, tenantID int64, address string) error
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
