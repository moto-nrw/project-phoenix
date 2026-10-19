// Package contract publishes Delivery's native intent and provider values.
package contract

import (
	"context"
) // Retention deletes the delivery data of the school in context after the
// fixed window of the retention list (#3955): mail and push intents and the
// mail delivery logs 30 days after the intent was sent, finally failed or
// was cancelled. Intents still being tried stay. A guardian welcome keeps
// only its idempotency key, so nobody is welcomed twice.
type Retention interface {
	// DeleteExpiredDeliveries reports the rows it deleted or cleared and the
	// window in days.
	DeleteExpiredDeliveries(ctx context.Context) (deleted, retentionDays int, err error)
}
