package delivery

import (
	"context"
	"errors"
)

// EmailWebhooks takes in one signed delivery of a mail provider. The failures
// are the ErrEmailWebhook* values of the contract. It is a capability of its own
// because the webhook needs nothing of Capability.
type EmailWebhooks interface {
	IngestEmailWebhook(ctx context.Context, provider string, header func(string) string, body []byte) error
}

// EmailSuppressions frees an address after a correction, on the school's list
// and at the provider. A complaint's block stays: the failure is
// ErrEmailSuppressionNeedsReview of the contract.
type EmailSuppressions interface {
	LiftEmailSuppression(ctx context.Context, tenantID int64, address string) error
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
