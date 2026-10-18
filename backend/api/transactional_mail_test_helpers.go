//go:build testhooks

package api

import (
	"context"
	"log/slog"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/email"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// fixtureMailOutbox drives each committed fixture intent through the actual
// leased worker without introducing a persistent test service graph.
type fixtureMailOutbox func(context.Context, DurableMail) error

func (enqueue fixtureMailOutbox) EnqueueTransactionalMail(ctx context.Context, mail DurableMail) error {
	return enqueue(ctx, mail)
}

func newAuthFixtureOutbox(outbox OutboxEnqueuer, runtime *deliveryCompose.Runtime, mailer email.Mailer, logger *slog.Logger, identity func() *identityaccess.Module) DurableMailOutbox {
	return fixtureMailOutbox(func(ctx context.Context, mail DurableMail) error {
		queued, err := outbox.enqueueTransactionalMail(ctx, mail)
		if err != nil {
			return err
		}
		tenant.RegisterAfterCommit(ctx, func() {
			sendCtx, cancel := context.WithTimeout(identityaccessFixtureContext(ctx), 15*time.Second)
			defer cancel()
			provider := &deliveryCompose.MailPushProvider{
				Registry: deliveryCompose.NewEmailTemplateRegistry(map[string]deliveryCompose.MailRenderer{
					transactionalMailKind: transactionalMailRenderer{identity: identity(), logger: logger},
				}), Mailer: mailer, Logger: logger, Tenants: deliveryCompose.NewTenantTransactions(nil),
			}
			if err := deliveryCompose.DrainEmailForTest(sendCtx, runtime, provider, mail.TenantID, queued.ID); err != nil {
				logger.Error("fixture outbox worker failed", "error", err.Error())
			}
			if mail.TenantID == 0 {
				if err := deliveryCompose.DeleteSystemEmailForTest(sendCtx, runtime, queued.ID); err != nil {
					logger.Error("fixture system mail cleanup failed", "error", err.Error())
				}
			}
		})
		return nil
	})
}

func identityaccessFixtureContext(ctx context.Context) context.Context {
	return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(context.WithoutCancel(ctx)))
}
