package contract

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmailOutcomeOfNeverCallsAnAcceptedMailDelivered(t *testing.T) {
	t.Parallel()
	state := func(value string) *string { return &value }
	unknown := DispatchOutcomeUnknown
	other := "mailbox full"
	for name, test := range map[string]struct {
		status string
		state  *string
		err    *string
		mark   EmailAddressMark
		want   EmailOutcome
	}{
		"waiting":                   {status: "pending", want: EmailQueued},
		"with the worker":           {status: "claimed", want: EmailQueued},
		"accepted without an event": {status: "sent", want: EmailAccepted},
		"delivered":                 {status: "sent", state: state("delivered"), want: EmailDelivered},
		"delayed":                   {status: "sent", state: state("delayed"), want: EmailDelayed},
		"bounced":                   {status: "sent", state: state("failed"), want: EmailUndeliverable},
		"refused by the server":     {status: "dead_letter", state: state("failed"), want: EmailUndeliverable},
		"held back by the list":     {status: "cancelled", state: state("suppressed"), want: EmailBlocked},
		"held back, event missing":  {status: "cancelled", mark: EmailMarkFailed, want: EmailBlocked},
		"withdrawn by the school":   {status: "cancelled", want: EmailCancelled},
		"complained":                {status: "sent", state: state("complained"), want: EmailComplained},
		"given up by moto":          {status: "dead_letter", err: &other, want: EmailSendFailed},
		"worker stopped":            {status: "dead_letter", err: &unknown, want: EmailOutcomeUnknown},
		"back on the fallback way":  {status: "pending", want: EmailQueued},
		"left over the fallback":    {status: "sent", want: EmailAccepted},
		"accepted, address marked":  {status: "sent", mark: EmailMarkFailed, want: EmailAccepted},
	} {
		assert.Equal(t, test.want, EmailOutcomeOf(test.status, test.state, test.err, test.mark), name)
	}
}

func TestEmailSendBlockOf(t *testing.T) {
	t.Parallel()
	assert.Equal(t, EmailSendBlockAddress, EmailSendBlockOf(EmailMarkFailed, EmailDelivered))
	assert.Equal(t, EmailSendBlockAddress, EmailSendBlockOf(EmailMarkComplained, EmailDelayed), "a mark outranks a delay")
	assert.Equal(t, EmailSendBlockDelayed, EmailSendBlockOf("", EmailDelayed))
	assert.Empty(t, EmailSendBlockOf("", EmailBlocked), "a lifted address can be mailed again")
	assert.Empty(t, EmailSendBlockOf("", ""))
}
