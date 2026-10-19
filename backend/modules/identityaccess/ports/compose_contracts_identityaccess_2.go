package ports

import (
	"context"
)

// PrecedingMailState is the delivery state of the mail a guardian welcome
// follows, in Identity's terms. The root reports the delivery owner's state
// through PrecedingMails; Identity decides what it means for the welcome
// (#4006).
type PrecedingMailState int

// PrecedingMails reports the delivery state of a queued mail of the school.
// found is false when the mail does not exist (any more).
type PrecedingMails interface {
	PrecedingMailState(ctx context.Context, tenantID, outboxID int64) (state PrecedingMailState, found bool, err error)
}
