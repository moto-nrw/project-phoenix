package ports

import (
	"context"
	"time"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/domain"
)

// EmailDeliveryEventStore keeps what happens to a mail after the mail server
// accepted it, and the suppression list that follows from it (#5932). Both
// statements cross schools, because a mail is found by its identity alone.
type EmailDeliveryEventStore interface {
	// RecordEmailDeliveryEvent stores the event of the matching mail, moves the
	// mail's state and extends the school's suppression list. An event without a
	// match or one already stored changes nothing.
	RecordEmailDeliveryEvent(context.Context, domain.EmailDeliveryEvent) (domain.EmailDeliveryEventResult, error)
	// SchoolEmailHolds and TraegerEmailHolds report what holds a mail to the
	// address back: the list, or a stop on every person who holds the address. A
	// Träger reads the holds of its schools. A school also reads whether the
	// families holding the address have left it (#5960).
	SchoolEmailHolds(ctx context.Context, tenantID int64, address string) (domain.SchoolMailHolds, error)
	TraegerEmailHolds(ctx context.Context, organizationID int64, address string) (listed, stopped bool, err error)
}

// EmailReconciliationStore finds the mails the reconciliation asks the provider
// about, across schools like the event store, and frees corrected addresses in
// the transaction of the school they name.
type EmailReconciliationStore interface {
	OpenProviderMails(context.Context, domain.ProviderMailQuery) ([]domain.ProviderMail, error)
	MarkEmailDeliveryChecked(ctx context.Context, id int64, at time.Time) error
	EmailSuppressionReason(ctx context.Context, tenantID int64, address string) (domain.EmailSuppressionReason, bool, error)
	DeleteEmailSuppression(ctx context.Context, tenantID int64, address string) error
}

// EmailProviderAPI is the mail provider's own record of its mails. Without a
// provider that keeps one, the reconciliation does not run.
type EmailProviderAPI interface {
	MessageEvents(ctx context.Context, providerMessageID string) ([]domain.EmailDeliveryEvent, error)
	WebhookEnabled(context.Context) (bool, error)
	LiftSuppression(ctx context.Context, address string) error
	// SuppressionActive reports whether the provider holds any block of exactly
	// this address, for whichever reason.
	SuppressionActive(ctx context.Context, address string) (bool, error)
}

// EmailWebhookProvider verifies and reads one provider's webhook deliveries.
type EmailWebhookProvider interface {
	Name() string
	Configured() bool
	Verify(header func(string) string, body []byte, now time.Time) error
	Parse(body []byte) (domain.ReportedEmailEvent, error)
}

// EmailOutcomeStore reads what became of a school's mails in the school's tenant
// transaction (#5934).
type EmailOutcomeStore interface {
	EmailOutcomes(context.Context, int64, deliveryContract.EmailOutcomeQuery) ([]deliveryContract.EmailOutcomeReport, error)
	LatestEmailOutcomes(ctx context.Context, tenantID int64, kinds, addresses []string) (map[string]deliveryContract.EmailOutcomeReport, error)
	EmailAddressMarks(ctx context.Context, tenantID int64, addresses []string) (map[string]deliveryContract.EmailAddressMark, error)
	StoppedEmailAddresses(ctx context.Context, tenantID int64, addresses []string) (map[string]bool, error)
	TraegerMailHolds(ctx context.Context, organizationID int64, address string) (listed, stopped bool, err error)
	TraegerMailHoldsOf(ctx context.Context, organizationID int64, addresses []string) (listed, stopped map[string]bool, err error)
}
