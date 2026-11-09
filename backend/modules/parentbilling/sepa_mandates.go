package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SEPA direct debit of the families (#4695, spec #4611 user stories 82, 84,
// 85, ADR 0171). moto collects no money: the Träger collects with the
// mandate a payer gave, and moto keeps the mandate, the creditor identifier
// of the Träger and the pre-notification on the invoice.
//
// A mandate belongs to the payer of a family. The family is the mail
// address of the contract party (FamilyKey, ADR 0167); the mandate hangs on
// the contract it was given for and goes with it. At most one mandate of a
// family is in force. A revocation ends it; the revoked mandate stays as
// history and never changes again. The IBAN is sealed at rest with the field
// cipher; the Abrechnung only ever sees its mask, and SEPAMandateDebits opens
// it for the debit file (#4697). Every read and change is recorded in the
// mandate's trail, without names or bank data.
//
// SEPAMandates is the one owner of the mandates: the import of existing
// mandates (#6198, Source MandateFromImport), the digital mandate of the
// parents (#4696, MandateFromParent) and the debit file (#4697) write and
// read through it.

// Mandate kinds.
const (
	// MandateRecurring collects every month (Dauermandat); the first
	// collection is FRST, every further one RCUR.
	MandateRecurring = "recurring"
	// MandateOneOff collects once (Einzelmandat, OOFF).
	MandateOneOff = "one_off"
)

// Sequence of the next collection of a recurring mandate.
const (
	// SequenceFirst is a mandate not yet collected with.
	SequenceFirst = "first"
	// SequenceRecurring is a mandate already collected with, also one taken
	// over from another system.
	SequenceRecurring = "recurring"
)

// SEPA sequence types as the debit file names them.
const (
	SequenceTypeFirst     = "FRST"
	SequenceTypeRecurring = "RCUR"
	SequenceTypeOneOff    = "OOFF"
)

// Where a mandate came from.
const (
	MandateFromOffice = "office"
	MandateFromImport = "import"
	MandateFromParent = "parent"
)

// What an invoice of a family that pays by direct debit does without a
// valid mandate or without the creditor identifier of the Träger.
const (
	// MissingMandateTransfer issues the invoice as a transfer and the
	// review shows the family as a hint; the default.
	MissingMandateTransfer = "transfer"
	// MissingMandateBlock refuses the release while such an invoice is in it.
	MissingMandateBlock = "block"
)

// Why an invoice of a family that pays by direct debit is not collected.
const (
	DebitGapMandate  = "mandate_missing"
	DebitGapCreditor = "creditor_missing"
	// DebitGapCreditorChanged is a mandate given to an earlier creditor
	// identifier of the Träger: it needs a new mandate.
	DebitGapCreditorChanged = "creditor_changed"
)

// Limits of the direct debit.
const (
	// DefaultPrenotificationDays is the lead of the pre-notification the
	// SEPA rulebook sets unless creditor and debtor agree otherwise.
	DefaultPrenotificationDays = 14
	// MinPrenotificationDays and MaxPrenotificationDays bound a shorter or
	// longer agreed lead.
	MinPrenotificationDays = 1
	MaxPrenotificationDays = 30
	// MaxAccountHolderLength is the longest debtor name of a SEPA message.
	MaxAccountHolderLength = 70
)

var (
	// ErrSEPAMandateNotFound reports a mandate that does not exist for the
	// Träger of the session.
	ErrSEPAMandateNotFound = errors.New("SEPA mandate not found")
	// ErrSEPAMandateInForce reports a family that already has a mandate in
	// force; it has to be revoked first.
	ErrSEPAMandateInForce = errors.New("the family already has a SEPA mandate in force")
	// ErrSEPAMandateReferenceTaken reports a reference the Träger already
	// gave another mandate.
	ErrSEPAMandateReferenceTaken = errors.New("the Träger already has a SEPA mandate with this reference")
	// ErrSEPACreditorMissing reports a mandate for a Träger without a
	// creditor identifier: a mandate is given to one.
	ErrSEPACreditorMissing = errors.New("the Träger has no SEPA creditor identifier")
	// ErrSEPAMandateRevoked reports a change of a revoked mandate.
	ErrSEPAMandateRevoked = errors.New("a revoked SEPA mandate never changes")
	// ErrDirectDebitWithoutMandate reports a release that would issue an
	// invoice by direct debit without a valid mandate or creditor
	// identifier while the Träger chose MissingMandateBlock.
	ErrDirectDebitWithoutMandate = errors.New("an invoice by direct debit has no valid SEPA mandate")
)

// SEPACreditor is how a Träger collects: its creditor identifier
// (Gläubiger-Identifikationsnummer, empty until it sets one), the days the
// pre-notification goes out before the debit and what an invoice without
// a valid mandate does.
type SEPACreditor struct {
	CreditorID          string     `json:"creditor_id"`
	PrenotificationDays int        `json:"prenotification_days"`
	MissingMandate      string     `json:"missing_mandate"`
	UpdatedAt           *time.Time `json:"updated_at"`
}

// DefaultSEPACreditor is the creditor of a Träger that set nothing.
func DefaultSEPACreditor() SEPACreditor {
	return SEPACreditor{PrenotificationDays: DefaultPrenotificationDays, MissingMandate: MissingMandateTransfer}
}

// SEPAMandate is a mandate as the Abrechnung sees it: the IBAN only as its
// mask. PartyFirstName and PartyLastName name the contract party of the
// family. CreditorID is the creditor identifier the mandate was given to;
// CreditorCurrent tells whether it is still the Träger's. LastCollectedOn is
// the day of the last collection (#4697 sets it; FRST becomes RCUR, and 36
// months without one end the mandate).
type SEPAMandate struct {
	ID              int64          `json:"id"`
	ContractID      int64          `json:"contract_id"`
	FamilyEmail     string         `json:"family_email"`
	PartyFirstName  string         `json:"party_first_name"`
	PartyLastName   string         `json:"party_last_name"`
	Reference       string         `json:"reference"`
	SignedOn        calendar.Date  `json:"signed_on"`
	Kind            string         `json:"kind"`
	Sequence        string         `json:"sequence"`
	AccountHolder   string         `json:"account_holder"`
	CreditorID      string         `json:"creditor_id"`
	CreditorCurrent bool           `json:"creditor_current"`
	LastCollectedOn *calendar.Date `json:"last_collected_on"`
	// ExpiresOn is the first day the mandate no longer holds without a
	// further collection (#4697, 36 months).
	ExpiresOn  calendar.Date  `json:"expires_on"`
	IBANMasked string         `json:"iban_masked"`
	BIC        string         `json:"bic"`
	Source     string         `json:"source"`
	RevokedOn  *calendar.Date `json:"revoked_on"`
	RevokedAt  *time.Time     `json:"revoked_at"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	// HolderSealed is the sealed account holder as the store reads it; the
	// owner opens it into AccountHolder and never answers it.
	HolderSealed []byte `json:"-"`
}

// InForce tells whether the mandate is not revoked.
func (m SEPAMandate) InForce() bool { return m.RevokedOn == nil }

// ExpiryDay is the first day the mandate no longer holds: 36 months after
// its signature or its last use (#4697, MandateUsedOn).
func (m SEPAMandate) ExpiryDay() calendar.Date {
	return MandateExpiresOn(m.SignedOn, MandateUsedOn(m.Sequence, m.LastCollectedOn, calendar.DateFromTime(m.CreatedAt)))
}

// Expired tells whether the mandate no longer holds on the day: 36 months
// passed without a collection (#4697).
func (m SEPAMandate) Expired(on calendar.Date) bool { return !m.ExpiryDay().After(on) }

// SequenceType is the SEPA sequence type of the next collection.
func (m SEPAMandate) SequenceType() string {
	switch {
	case m.Kind == MandateOneOff:
		return SequenceTypeOneOff
	case m.Sequence == SequenceRecurring:
		return SequenceTypeRecurring
	default:
		return SequenceTypeFirst
	}
}

// SEPAMandateInput is a new mandate for the family of a contract of the
// Träger. An empty Reference takes one moto forms; an empty Source is
// MandateFromOffice. IBAN and BIC may carry spaces and lower case. The
// mandate is given to the Träger's creditor identifier; CreditorID, when
// set (an imported mandate, #6198), must be that one.
type SEPAMandateInput struct {
	ContractID    int64
	CreditorID    string
	Reference     string
	SignedOn      calendar.Date
	Kind          string
	Sequence      string
	AccountHolder string
	IBAN          string
	BIC           string
	Source        string
}

// SEPAMandateChange changes the account of a mandate in force: the holder,
// the IBAN (empty keeps it), the BIC and the sequence.
type SEPAMandateChange struct {
	AccountHolder string
	IBAN          string
	BIC           string
	Sequence      string
}

// SEPAMandateEvent is one entry of the trail of a mandate: what happened,
// from where, which fields a change touched, who and when. It holds no
// value.
type SEPAMandateEvent struct {
	ID         int64     `json:"id"`
	Action     string    `json:"action"`
	Source     string    `json:"source"`
	Fields     []string  `json:"fields"`
	AccountID  *int64    `json:"account_id"`
	OccurredAt time.Time `json:"occurred_at"`
}

// SEPAMandateDebit is a mandate with its open IBAN for the debit file
// (#4697).
type SEPAMandateDebit struct {
	MandateID     int64         `json:"mandate_id"`
	Reference     string        `json:"reference"`
	SignedOn      calendar.Date `json:"signed_on"`
	SequenceType  string        `json:"sequence_type"`
	AccountHolder string        `json:"account_holder"`
	IBAN          string        `json:"-"`
	BIC           string        `json:"bic"`
}

// SealedSEPAMandate is a mandate with its sealed IBAN as the store of the
// owner keeps it; only the owner opens it.
type SealedSEPAMandate struct {
	Mandate SEPAMandate
	IBAN    []byte
	Holder  []byte
}

// InvoiceMandate is the mandate an invoice by direct debit is collected
// with, as the release copies it onto the invoice and the pre-notification
// prints it (ADR 0119 point 7).
type InvoiceMandate struct {
	MandateID    int64         `json:"mandate_id"`
	Reference    string        `json:"reference"`
	SignedOn     calendar.Date `json:"signed_on"`
	CreditorID   string        `json:"creditor_id"`
	IBANMasked   string        `json:"iban_masked"`
	SequenceType string        `json:"sequence_type"`
	// NoticeShort tells the review that the debit comes sooner after the
	// invoice date than the pre-notification days of the Träger; it is not
	// copied onto the invoice.
	NoticeShort bool `json:"notice_short,omitempty"`
}

// ResolveDebit completes the payment ResolvePayment decided (ADR 0167) for
// a direct debit (ADR 0171): with the creditor identifier and a mandate in
// force signed by the issue date and given to that creditor identifier the
// invoice carries the mandate. Without
// one it carries the gap; under MissingMandateTransfer it is paid by
// transfer, with the due date ResolvePayment gives a transfer, and under
// MissingMandateBlock it stays a direct debit the release refuses. Any other
// payment passes unchanged.
func ResolveDebit(payment InvoicePayment, terms ServiceTerms, run RunDueDates, family *FamilyBilling, issuedOn calendar.Date,
	creditor SEPACreditor, mandate *SEPAMandate,
) InvoicePayment {
	if payment.Method != PaymentDirectDebit {
		return payment
	}
	gap := ""
	switch {
	case creditor.CreditorID == "":
		gap = DebitGapCreditor
	case mandate == nil || !mandate.InForce() || mandate.SignedOn.After(issuedOn):
		gap = DebitGapMandate
	case mandate.Kind == MandateOneOff && mandate.LastCollectedOn != nil:
		// A one-off mandate authorizes one collection; once collected it is
		// no mandate for another invoice (#4697).
		gap = DebitGapMandate
	case mandate.CreditorID != creditor.CreditorID:
		// A mandate binds the creditor it was given to; after a change of
		// the creditor identifier it is no proof for the new one.
		gap = DebitGapCreditorChanged
	case mandate.Expired(issuedOn):
		// 36 months without a collection end the mandate (#4697); the file
		// checks the day of the collection again.
		gap = DebitGapExpired
	}
	if gap == "" {
		payment.Mandate = &InvoiceMandate{MandateID: mandate.ID, Reference: mandate.Reference, SignedOn: mandate.SignedOn,
			CreditorID: mandate.CreditorID, IBANMasked: mandate.IBANMasked, SequenceType: mandate.SequenceType(),
			NoticeShort: payment.DueOn.Before(issuedOn.AddDays(creditor.PrenotificationDays))}
		return payment
	}
	if creditor.MissingMandate == MissingMandateBlock {
		payment.DebitGap = gap
		return payment
	}
	transfer := terms
	transfer.PaymentMethod = PaymentTransfer
	var asTransfer *FamilyBilling
	if family != nil {
		copied := *family
		copied.PaymentMethod = PaymentTransfer
		asTransfer = &copied
	}
	fallback := ResolvePayment(transfer, run, asTransfer, issuedOn)
	fallback.DebitGap = gap
	return fallback
}

// BankDataCipher seals and opens single database values bound to their
// context. The root binds Security Runtime's field cipher (#4722), the same
// key ring that seals the health information (ADR 0171).
type BankDataCipher interface {
	Seal(plaintext, context []byte) ([]byte, error)
	Open(sealed, context []byte) ([]byte, error)
}

// SEPAMandates lets the Abrechnung of a Träger keep the creditor identifier
// and the SEPA mandates of its families. Every operation names the person of
// the session and runs in the Träger transaction of its Träger, which the
// caller opens. Every read and change of a mandate is recorded in its trail.
type SEPAMandates interface {
	// SEPACreditor answers the creditor of the Träger, the defaults where it
	// set nothing.
	SEPACreditor(ctx context.Context, organizationID int64) (SEPACreditor, error)
	// SaveSEPACreditor sets the creditor identifier, the pre-notification
	// days and the choice for a direct debit without a mandate.
	SaveSEPACreditor(ctx context.Context, reviewer HolidayReviewer, creditor SEPACreditor) (SEPACreditor, error)
	// ListSEPAMandates lists the mandates, newest first; contractID > 0
	// keeps those of the family of that contract.
	ListSEPAMandates(ctx context.Context, reviewer HolidayReviewer, contractID int64) ([]SEPAMandate, error)
	// CreateSEPAMandate stores the mandate of the family of a contract.
	CreateSEPAMandate(ctx context.Context, reviewer HolidayReviewer, input SEPAMandateInput) (SEPAMandate, error)
	// ChangeSEPAMandate changes the account of a mandate in force.
	ChangeSEPAMandate(ctx context.Context, reviewer HolidayReviewer, id int64, change SEPAMandateChange) (SEPAMandate, error)
	// RevokeSEPAMandate ends a mandate in force on the day the revocation
	// arrived, at most today.
	RevokeSEPAMandate(ctx context.Context, reviewer HolidayReviewer, id int64, revokedOn calendar.Date) (SEPAMandate, error)
	// SEPAMandateHistory reads the trail of a mandate, oldest first.
	SEPAMandateHistory(ctx context.Context, reviewer HolidayReviewer, id int64) ([]SEPAMandateEvent, error)
	// SEPAMandateDebits opens the IBAN of mandates for the debit file
	// (#4697) and records the reveal of each.
	SEPAMandateDebits(ctx context.Context, reviewer HolidayReviewer, ids []int64) ([]SEPAMandateDebit, error)
	// SEPAMandateCollections lists the debits of a mandate, newest first:
	// the collection history per mandate and invoice (#4697). It names no
	// bank data.
	SEPAMandateCollections(ctx context.Context, reviewer HolidayReviewer, id int64) ([]SEPADebit, error)
}
