package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

// EmailDeliveryEventStore keeps what happens to a mail after the mail server
// accepted it, and the suppression list that follows from it (#5932). Both
// statements cross schools, because a mail is found by its identity alone.
type EmailDeliveryEventStore interface {
	// RecordEmailDeliveryEvent stores the event of the matching mail, moves the
	// mail's state and extends the school's suppression list. An event without a
	// match or one already stored changes nothing.
	RecordEmailDeliveryEvent(context.Context, domain.EmailDeliveryEvent) (domain.EmailDeliveryEventResult, error)
	EmailAddressSuppressed(ctx context.Context, tenantID int64, address string) (bool, error)
}
