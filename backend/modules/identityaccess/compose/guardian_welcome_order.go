package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// PrecedingMailState is the delivery state of the mail a guardian welcome
// follows, in Identity's terms. The root reports the delivery owner's state
// through PrecedingMails; Identity decides what it means for the welcome
// (#4006).
type PrecedingMailState int

const (
	// PrecedingMailQueued has not been accepted by the mail transport yet.
	PrecedingMailQueued = PrecedingMailState(domain.PrecedingMailQueued)
	// PrecedingMailSent was accepted by the mail transport.
	PrecedingMailSent = PrecedingMailState(domain.PrecedingMailSent)
	// PrecedingMailCancelled was withdrawn before it was sent.
	PrecedingMailCancelled = PrecedingMailState(domain.PrecedingMailCancelled)
	// PrecedingMailUndeliverable exhausted its delivery attempts.
	PrecedingMailUndeliverable = PrecedingMailState(domain.PrecedingMailUndeliverable)
)

// PrecedingMails reports the delivery state of a queued mail of the school.
// found is false when the mail does not exist (any more).
type PrecedingMails interface {
	PrecedingMailState(ctx context.Context, tenantID, outboxID int64) (state PrecedingMailState, found bool, err error)
}

// NewGuardianWelcomeOrder answers whether the guardian welcome that follows
// the queued mail outboxID may go out now (sent: the preceding mail was
// accepted) or never (terminal). The school is the tenant in ctx.
func NewGuardianWelcomeOrder(mails PrecedingMails) func(ctx context.Context, outboxID int64) (sent, terminal bool, err error) {
	return func(ctx context.Context, outboxID int64) (bool, bool, error) {
		if mails == nil {
			return false, false, fmt.Errorf("guardian welcome predecessor: delivery state is not composed")
		}
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return false, false, fmt.Errorf("guardian welcome predecessor: tenant is required: %w", err)
		}
		state, found, err := mails.PrecedingMailState(ctx, tenantID.Int64(), outboxID)
		if err != nil {
			return false, false, err
		}
		sent, terminal := domain.GuardianWelcomeOrder(domain.PrecedingMailState(state), found)
		return sent, terminal, nil
	}
}
