package compose

import (
	"context"

	applicationBindings "github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	identityBindingPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
)

const (
	// PrecedingMailQueued has not been accepted by the mail transport yet.
	PrecedingMailQueued = identityBindingPorts.PrecedingMailState(domain.PrecedingMailQueued)
	// PrecedingMailSent was accepted by the mail transport.
	PrecedingMailSent = identityBindingPorts.PrecedingMailState(domain.PrecedingMailSent)
	// PrecedingMailCancelled was withdrawn before it was sent.
	PrecedingMailCancelled = identityBindingPorts.PrecedingMailState(domain.PrecedingMailCancelled)
	// PrecedingMailUndeliverable exhausted its delivery attempts.
	PrecedingMailUndeliverable = identityBindingPorts.PrecedingMailState(domain.PrecedingMailUndeliverable)
)

// NewGuardianWelcomeOrder answers whether the guardian welcome that follows
// the queued mail outboxID may go out now (sent: the preceding mail was
// accepted) or never (terminal). The school is the tenant in ctx.
func NewGuardianWelcomeOrder(mails identityBindingPorts.PrecedingMails) func(ctx context.Context, outboxID int64) (sent, terminal bool, err error) {
	return applicationBindings.NewGuardianWelcomePredecessor(mails)
}
