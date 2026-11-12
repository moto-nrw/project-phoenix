package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The SEPA direct debit file of an issued run (#4697, ADR 0181). moto
// collects no money: the office downloads one PAIN.008 file per run
// (pain.008.001.08, the version of the Deutsche Kreditwirtschaft) and
// submits it to its bank.
//
// The release writes one debit per invoice by direct debit with the account
// of the mandate as it is then (sealed, never opened); the file uses that
// account, never a later change. A download changes nothing and the same
// debits give the same file with the same message ID, so a bank refuses a
// second submission of it. Only the office's confirmation that it submitted
// the file collects: it marks the debits submitted, sets the last collection
// of each mandate and turns its next sequence from FRST to RCUR, in one
// transaction. A mandate that is revoked, given to another creditor
// identifier or expired (36 months without a collection) leaves its debit
// out of the file; so does a cancelled invoice.

// Kinds of run a debit file belongs to.
const (
	DebitRunCare    = "care"
	DebitRunHoliday = "holiday"
	// DebitRunNotice is the file of the installments of the Jahresbescheide
	// due in one month; its ID is the month as YYYYMM (#4697, ADR 0182).
	DebitRunNotice = "notice"
)

// States of a debit.
const (
	DebitPending   = "pending"
	DebitSubmitted = "submitted"
)

// Why a pending debit stays out of the file.
const (
	// DebitLeftNoMandate is an installment of a family without a valid
	// mandate in force.
	DebitLeftNoMandate       = "mandate_missing"
	DebitLeftOneOffUsed      = "one_off_used"
	DebitLeftOneOffMultiple  = "one_off_multiple"
	DebitLeftCancelled       = "cancelled"
	DebitLeftRevoked         = "revoked"
	DebitLeftExpired         = "expired"
	DebitLeftCreditorChanged = "creditor_changed"
)

// MandateExpiryMonths is how long a mandate holds without a collection
// (SEPA Core Rulebook, section 4.5): 36 months after the last one, or after
// the signature when there was none.
const MandateExpiryMonths = 36

// DebitGapExpired is a mandate that expired before the debit (#4697).
const DebitGapExpired = "mandate_expired"

var (
	// ErrSEPAEndToEndDuplicate reports a file in which two debits would
	// carry the same end-to-end ID; the bank could not tell them apart.
	ErrSEPAEndToEndDuplicate = errors.New("two SEPA debits of the file would carry the same end-to-end ID")
	// ErrSEPAAccountHolderUnsupported reports an empty name after DK conversion.
	ErrSEPAAccountHolderUnsupported = errors.New("the account holder name needs Latin letters for the SEPA file")
	// ErrSEPADebitRunNotFound reports a run of another Träger, none, or one
	// that is not released.
	ErrSEPADebitRunNotFound = errors.New("no released run with SEPA debits")
	// ErrSEPADebitsEmpty reports a run without a debit for the file.
	ErrSEPADebitsEmpty = errors.New("the run has no SEPA debit to collect")
	// ErrSEPADebitsChanged reports a confirmation of a file that is no
	// longer the file of the run: a debit left it or came back.
	ErrSEPADebitsChanged = errors.New("the SEPA debit file of the run changed")
	// ErrSEPACreditorAccountMissing reports a Träger without a default bank
	// account: the debits are credited to it.
	ErrSEPACreditorAccountMissing = errors.New("the Träger has no bank account for the SEPA debits")
)

// MandateExpiresOn is the first day a mandate no longer holds: 36 months
// after its last collection, or after its signature without one.
func MandateExpiresOn(signedOn calendar.Date, lastCollectedOn *calendar.Date) calendar.Date {
	from := signedOn
	if lastCollectedOn != nil && lastCollectedOn.After(from) {
		from = *lastCollectedOn
	}
	if from.IsZero() {
		return from
	}
	year, month, day := from.Year(), from.Month(), from.Day()
	shifted := time.Date(year, month+MandateExpiryMonths, 1, 0, 0, 0, 0, time.UTC)
	// The same day of the month, the month's last when it is shorter.
	last := time.Date(shifted.Year(), shifted.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	return calendar.NewDate(shifted.Year(), shifted.Month(), min(day, last))
}

// MandateUsedOn is the day a mandate's 36 months run from besides its
// signature: its last collection in moto, or, for a mandate that was
// already used elsewhere (sequence recurring, taken over by the office or
// the import, #6198) and not yet collected in moto, the day moto took it
// over. Nil for a mandate never used.
func MandateUsedOn(sequence string, lastCollectedOn *calendar.Date, takenOverOn calendar.Date) *calendar.Date {
	if lastCollectedOn != nil {
		return lastCollectedOn
	}
	if sequence == SequenceRecurring && !takenOverOn.IsZero() {
		return &takenOverOn
	}
	return nil
}

// SEPADebit is one debit of a run as the office sees it: the invoice, the
// family, the amount, the due date, the sequence type and the mandate with
// the masked IBAN. Left names why a pending debit stays out of the file.
type SEPADebit struct {
	ID               int64         `json:"id"`
	MandateID        int64         `json:"mandate_id"`
	ContractID       int64         `json:"contract_id"`
	RunKind          string        `json:"run_kind"`
	RunID            int64         `json:"run_id"`
	InvoiceNumber    string        `json:"invoice_number"`
	RecipientName    string        `json:"recipient_name"`
	AmountCents      int64         `json:"amount_cents"`
	DueOn            calendar.Date `json:"due_on"`
	CollectOn        calendar.Date `json:"collect_on,omitempty"`
	SequenceType     string        `json:"sequence_type"`
	MandateReference string        `json:"mandate_reference"`
	IBANMasked       string        `json:"iban_masked"`
	Status           string        `json:"status"`
	Left             string        `json:"left,omitempty"`
	SubmittedAt      *time.Time    `json:"submitted_at"`
	// NoticeID and NoticeMonth name the installment of a Jahresbescheid the
	// debit collects instead of an invoice (ADR 0182); InvoiceNumber is
	// then the number of the notice.
	NoticeID    int64         `json:"notice_id,omitempty"`
	NoticeMonth calendar.Date `json:"notice_month,omitempty"`
	MessageID   string        `json:"message_id,omitempty"`
}

// SEPADebitGroup is one batch of the file: the debits of one sequence type
// collected on one day.
type SEPADebitGroup struct {
	SequenceType string        `json:"sequence_type"`
	CollectOn    calendar.Date `json:"collect_on"`
	Count        int           `json:"count"`
	TotalCents   int64         `json:"total_cents"`
}

// SEPADebitRun is the debit file of a run: what goes into it, what stays
// out and why, and whether the office submitted it. MessageID names the
// file as it is now; a confirmation carries it.
type SEPADebitRun struct {
	RunKind        string           `json:"run_kind"`
	RunID          int64            `json:"run_id"`
	CreditorID     string           `json:"creditor_id"`
	CreditorName   string           `json:"creditor_name"`
	CreditorIBAN   string           `json:"creditor_iban_masked"`
	AccountMissing bool             `json:"account_missing"`
	MessageID      string           `json:"message_id"`
	Count          int              `json:"count"`
	TotalCents     int64            `json:"total_cents"`
	Groups         []SEPADebitGroup `json:"groups"`
	PastDue        int              `json:"past_due"`
	Debits         []SEPADebit      `json:"debits"`
	Submitted      []SEPADebit      `json:"submitted"`
	Left           []SEPADebit      `json:"left"`
	SubmittedAt    *time.Time       `json:"submitted_at"`
}

// SEPADebitFile is the PAIN.008 file of a run.
type SEPADebitFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

// SEPADebits lets the Abrechnung of a Träger download the PAIN.008 file of
// an issued run and confirm its submission to the bank. Every operation runs
// in the Träger transaction of the reviewer's Träger, which the caller
// opens; the download records the reveal of each mandate's account.
type SEPADebits interface {
	// SEPADebitRun answers the debit file of a run without bank data.
	SEPADebitRun(ctx context.Context, reviewer HolidayReviewer, runKind string, runID int64) (SEPADebitRun, error)
	// SEPADebitFile renders the PAIN.008 file of the pending debits of a
	// run. It changes no debit.
	SEPADebitFile(ctx context.Context, reviewer HolidayReviewer, runKind string, runID int64) (SEPADebitFile, error)
	// SubmitSEPADebits confirms that the file messageID was submitted to the
	// bank: its debits become submitted and their mandates collected.
	SubmitSEPADebits(ctx context.Context, reviewer HolidayReviewer, runKind string, runID int64, messageID string) (SEPADebitRun, error)
}

// SealedSEPADebit is a debit as the store of the owner reads it: with the
// sealed account the release copied, the state of its mandate and of its
// invoice now, and the Träger's creditor identifier now. Only the owner
// opens the account.
type SealedSEPADebit struct {
	Debit           SEPADebit
	CreditorID      string
	MandateSignedOn calendar.Date
	BIC             string
	// The mandate now: revoked, its kind, signature, sequence, last
	// collection and the day moto took it over.
	MandateRevoked         bool
	MandateKind            string
	MandateSigned          calendar.Date
	MandateSequence        string
	MandateLastCollectedOn *calendar.Date
	MandateCreatedOn       calendar.Date
	// CreditorNow is the Träger's creditor identifier now, empty without one.
	CreditorNow string
	// Cancelled is an invoice cancelled since; CreditedCents the credits
	// (negative) a holiday invoice got since.
	Cancelled     bool
	CreditedCents int64
	// OneOffShared is a one-off mandate another uncancelled debit of any
	// run also claims: the authorization covers one collection only.
	OneOffShared bool
	// MandateUpdatedAt is the last change of the mandate an installment of
	// a Jahresbescheid is planned from; its account is fixed only by the
	// confirmation, so a change of it changes the file.
	MandateUpdatedAt time.Time
	Holder           []byte
	IBAN             []byte
}

// SEPANoticeRate is what the file needs of an installment of a
// Jahresbescheid besides its due: the recipient of the notice, the mandate
// in force of the family (nil without), the creditor identifier now and, for
// the file, the sealed account. OneOffClaimed is a one-off mandate another
// debit already claims.
type SEPANoticeRate struct {
	RecipientName string
	Mandate       *SEPAMandate
	CreditorNow   string
	OneOffClaimed bool
	Holder        []byte
	IBAN          []byte
}
