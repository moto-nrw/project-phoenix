package owners

import (
	"context"
	"testing"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type intentCapture struct {
	delivery.Capability
	intent deliveryContract.EmailIntent
}

func (c *intentCapture) EnqueueEmail(_ context.Context, intent deliveryContract.EmailIntent) (deliveryContract.Enqueued, error) {
	c.intent = intent
	return deliveryContract.Enqueued{ID: 1}, nil
}

func (c *intentCapture) EnqueueSystemEmail(ctx context.Context, intent deliveryContract.EmailIntent) (deliveryContract.Enqueued, error) {
	return c.EnqueueEmail(ctx, intent)
}

// The mails someone waits for to get in go first; a notice about a change does
// not (#5932).
func TestTransactionalMailMarksSecurityMail(t *testing.T) {
	t.Parallel()
	for mailType, security := range map[string]bool{
		"mfa_email_code":                  true,
		"operator_mfa_email_code":         true,
		"password_reset":                  true,
		"invitation":                      true,
		"operator_invitation":             true,
		"traeger_invitation":              true,
		"operator_email_change_verify":    true,
		"mfa_trusted_device_added":        false,
		"operator_email_change_confirmed": false,
	} {
		capture := &intentCapture{}
		outbox := OutboxEnqueuer{Runtime: &deliveryCompose.Runtime{Module: capture}}

		_, err := outbox.EnqueueMailIntent(context.Background(), DurableMail{
			Type: mailType, IdempotencyKey: mailType, TenantID: 7,
			Message: email.Message{To: email.NewEmail("", "person@example.com")},
		})

		require.NoError(t, err, mailType)
		assert.Equal(t, security, capture.intent.Security, mailType)
	}
}
