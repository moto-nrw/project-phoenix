package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextEmailDeliveryState(t *testing.T) {
	t.Parallel()

	final := func(events ...EmailDeliveryState) EmailDeliveryState {
		state := EmailDeliveryState("")
		for _, event := range events {
			state = NextEmailDeliveryState(state, event)
		}
		return state
	}

	assert.Equal(t, EmailDeliveryDelayed, final(EmailDeliveryDelayed, EmailDeliveryDelayed), "repeated soft bounces stay one delay")
	assert.Equal(t, EmailDeliveryDelivered, final(EmailDeliveryDelayed, EmailDeliveryDelivered), "a delay can still end in delivery")
	assert.Equal(t, EmailDeliveryComplained, final(EmailDeliveryDelivered, EmailDeliveryComplained), "a complaint arrives after delivery")
	assert.Equal(t, EmailDeliveryFailed, final(EmailDeliveryDelivered, EmailDeliveryFailed), "a bounce can arrive after delivery")
	assert.Equal(t, EmailDeliveryDelivered, final(EmailDeliveryDelivered, EmailDeliveryDelayed), "a late delay does not undo delivery")
	assert.Equal(t, EmailDeliveryFailed, final(EmailDeliveryFailed, EmailDeliveryDelivered), "a late delivery does not undo a failure")
	assert.Equal(t, EmailDeliveryComplained, final(EmailDeliveryComplained, EmailDeliveryFailed), "a late failure does not undo a complaint")
	assert.Equal(t, EmailDeliveryFailed, final(EmailDeliverySuppressed, EmailDeliveryFailed))
	assert.Equal(t, EmailDeliveryFailed, final(EmailDeliveryFailed, EmailDeliverySuppressed))
}

func TestRedactDiagnostic(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "550 5.1.1 <[address]>: User unknown", RedactDiagnostic("550 5.1.1 <jane.doe@example.com>: User unknown"))
	assert.Equal(t, "452 4.2.2 mailbox full", RedactDiagnostic("452 4.2.2 mailbox full"))
	assert.Equal(t, "unknown [address] here", RedactDiagnostic("unknown müller@example.com here"))
	assert.Equal(t, "unknown [address] here", RedactDiagnostic("unknown max@müller-familie.de here"))
	assert.Equal(t, "unknown <[address]>", RedactDiagnostic(`unknown <"max mustermann"@example.com>`))
}
