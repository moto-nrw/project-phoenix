package parentbilling

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Sending and printing the confirmation letters (spec #4611 user stories
// 94, 95, 97 and 100, #4670, ADR 0123). The Abrechnung mails the stored
// letter of one family or of every family of a released run. Each mail is
// a mail of the Träger on Delivery's outbox, enqueued in the Träger
// transaction that records the send, with the PDF attached. The run shows
// the state of the last mail of every family, and a failed or delivered
// mail can be sent again. All letters of a run print as one PDF for the
// families without internet.

// MailKindBillingLetter is the outbox kind of the mail that carries a
// confirmation letter.
const MailKindBillingLetter = "parent_billing_letter"

// MailRelatedBillingRun is the related entity type of the mails of a run.
const MailRelatedBillingRun = "parent_billing_run"

// States of the mail of a family.
const (
	// LetterMailNotSent names a family whose letter was never mailed.
	LetterMailNotSent = "not_sent"
	// LetterMailQueued names a mail that waits for its first or a further
	// attempt.
	LetterMailQueued = "queued"
	// LetterMailSent names a mail the mail server accepted.
	LetterMailSent = "sent"
	// LetterMailFailed names a mail Delivery gave up on.
	LetterMailFailed = "failed"
	// LetterMailUnknown names a mail whose result Delivery no longer
	// keeps.
	LetterMailUnknown = "unknown"
)

var (
	// ErrBillingLettersMissing reports a send or print before the letters
	// of the run are created.
	ErrBillingLettersMissing = errors.New("the letters of the billing run are not created yet")
	// ErrLetterMailPending reports a send while the last mail of the family
	// still waits.
	ErrLetterMailPending = errors.New("the last mail of this letter is still being sent")
	// ErrRecipientEmailMissing reports a family without an e-mail address.
	ErrRecipientEmailMissing = errors.New("the family has no e-mail address")
)

// BillingLetterMailing is the mail state of the letter of one family.
// RequestedAt and SentAt are nil before the first mail; Sends counts every
// mail of the letter.
type BillingLetterMailing struct {
	DocumentID         int64      `json:"document_id"`
	Number             string     `json:"number"`
	RecipientFirstName string     `json:"recipient_first_name"`
	RecipientLastName  string     `json:"recipient_last_name"`
	RecipientEmail     string     `json:"recipient_email"`
	Status             string     `json:"status"`
	Sends              int        `json:"sends"`
	Attempts           int        `json:"attempts"`
	RequestedAt        *time.Time `json:"requested_at"`
	SentAt             *time.Time `json:"sent_at"`
}

// BillingLetterMailings are the mail states of every family of a run.
type BillingLetterMailings struct {
	Documents []BillingLetterMailing `json:"documents"`
}

// BillingLetterSeries reports a send to every family: Queued mails were
// put on the outbox, Already families had a mail before, WithoutEmail have
// no address and need the printed letter.
type BillingLetterSeries struct {
	Queued       int `json:"queued"`
	Already      int `json:"already"`
	WithoutEmail int `json:"without_email"`
}

// BillingLetterMail is the mail of one letter as it waits on the outbox:
// everything the mail says and the PDF, so the mail never changes after the
// send.
type BillingLetterMail struct {
	OrganizationID int64  `json:"-"`
	RunID          int64  `json:"-"`
	DocumentID     int64  `json:"document_id"`
	Recipient      string `json:"-"`
	IdempotencyKey string `json:"-"`
	GuardianName   string `json:"guardian_name"`
	TraegerName    string `json:"traeger_name"`
	PeriodName     string `json:"period_name"`
	Number         string `json:"number"`
	Filename       string `json:"filename"`
	PDF            []byte `json:"pdf"`
}

// BillingLetterSend is one stored send of a letter: the outbox entry of
// Delivery, the address it went to and when.
type BillingLetterSend struct {
	DocumentID     int64
	DeliveryID     int64
	RecipientEmail string
	CreatedAt      time.Time
}

// BillingDocumentLetter is the stored letter of one invoice.
type BillingDocumentLetter struct {
	DocumentID int64
	File       BillingLetterFile
}

// BillingLetterMailState is the state of one mail on the outbox.
type BillingLetterMailState struct {
	DeliveryID int64
	Status     string
	Attempts   int
	SentAt     *time.Time
}

// BillingLetterPost is Delivery and the letterhead of the Träger as the
// letters are mailed. The root binds it; every operation joins the caller's
// Träger transaction.
type BillingLetterPost interface {
	// SenderText is the letterhead text of the Träger; its first line names
	// the Träger, as in the closing of the letter.
	SenderText(ctx context.Context, organizationID int64) (string, error)
	// EnqueueLetterMail puts the mail of a Träger on Delivery's outbox with
	// the run as its related entity and returns the outbox entry.
	EnqueueLetterMail(ctx context.Context, mail BillingLetterMail) (int64, error)
	// LetterMailStates reads the mails of a run Delivery still keeps.
	LetterMailStates(ctx context.Context, organizationID, runID int64) ([]BillingLetterMailState, error)
}

// RenderedLetterMail is the mail of a letter as the Delivery worker sends
// it.
type RenderedLetterMail struct {
	SenderName string
	Subject    string
	Template   string
	Content    LetterMailContent
	Filename   string
	PDF        []byte
}

// LetterMailContent is what the text of the mail names.
type LetterMailContent struct {
	GuardianName string
	TraegerName  string
	PeriodName   string
	Number       string
}

// LetterMailRenderer turns the payload of a claimed letter mail into the
// mail the worker sends.
type LetterMailRenderer func(ctx context.Context, payload json.RawMessage) (RenderedLetterMail, error)

// BillingLetterMailService lets the Abrechnung of a Träger mail and print
// the letters of a released run. Every operation runs in the Träger
// transaction of the session's Träger, which the caller opens; every send
// is recorded with the account and the address, every read in the run's
// trail.
type BillingLetterMailService interface {
	// LetterMailings reads the mail state of every family of the run.
	LetterMailings(ctx context.Context, reviewer HolidayReviewer, runID int64) (BillingLetterMailings, error)
	// MailBillingLetters mails the letter of every family that has an
	// address and no mail yet. A second call sends nothing twice.
	MailBillingLetters(ctx context.Context, reviewer HolidayReviewer, runID int64) (BillingLetterSeries, error)
	// MailBillingLetter mails the letter of one family, first or again,
	// unless its last mail still waits.
	MailBillingLetter(ctx context.Context, reviewer HolidayReviewer, runID, documentID int64) (BillingLetterMailing, error)
	// PrintBillingLetters combines the letters of every family into one
	// PDF.
	PrintBillingLetters(ctx context.Context, reviewer HolidayReviewer, runID int64) (BillingLetterFile, error)
}
