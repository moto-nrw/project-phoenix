package contract

import (
	"testing"
	"time"

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
	assert.Equal(t, EmailSendBlockStopped, EmailSendBlockOf(EmailMarkFailed, true, EmailDelivered, ""), "the school's decision comes first")
	assert.Equal(t, EmailSendBlockAddress, EmailSendBlockOf(EmailMarkFailed, false, EmailDelivered, ""))
	assert.Equal(t, EmailSendBlockAddress, EmailSendBlockOf(EmailMarkComplained, false, EmailDelayed, ""), "a mark outranks a delay")
	assert.Equal(t, EmailSendBlockDelayed, EmailSendBlockOf("", false, EmailDelayed, ""))
	assert.Empty(t, EmailSendBlockOf("", false, EmailOverdue, ""), "after the wait a new mail is allowed")
	assert.Empty(t, EmailSendBlockOf("", false, EmailBlocked, ""), "a lifted address can be mailed again")
	assert.Empty(t, EmailSendBlockOf("", false, "", ""))
}

func TestEmailOutcomeAtEndsTheWaitForADelayedMail(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	justBefore := now.Add(-EmailDelayLimit + time.Minute)
	justAfter := now.Add(-EmailDelayLimit - time.Minute)
	assert.Equal(t, EmailDelayed, EmailOutcomeAt(EmailDelayed, &justBefore, now))
	assert.Equal(t, EmailOverdue, EmailOutcomeAt(EmailDelayed, &justAfter, now))
	assert.Equal(t, EmailDelayed, EmailOutcomeAt(EmailDelayed, nil, now))
	assert.Equal(t, EmailDelivered, EmailOutcomeAt(EmailDelivered, &justAfter, now), "only a delay ends")
}

func TestUnsubscribedIsNoBrokenAddress(t *testing.T) {
	t.Parallel()
	state := "unsubscribed"
	assert.Equal(t, EmailUnsubscribed, EmailOutcomeOf("sent", &state, nil, ""))
}

// The calendar portal's test holds these names on its side.
func TestAppointmentMailsAreParentMails(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"appointment_published", "appointment_updated", "appointment_cancelled", "appointment_reminder"} {
		assert.True(t, IsParentMail(kind), kind)
	}
}
