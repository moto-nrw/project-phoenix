package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Beleg types of a Träger (#4720 slice 2, ADR 0184). Each Beleg of the Träger
// has one retention record of its type. A new type is code.
const (
	// BelegHolidayDocument is a document of a holiday billing run: an
	// invoice, a cancellation or a credit note with its positions, letter
	// and mailings.
	BelegHolidayDocument = "holiday_document"
	// BelegCareInvoice is a monthly invoice with its positions, letters,
	// mailings, corrections and its cancellation.
	BelegCareInvoice = "care_invoice"
	// BelegCareNotice is a Jahresbescheid (fee notice) with its months.
	BelegCareNotice = "care_notice"
	// BelegCareContract is a concluded Betreuungsvertrag with its versions,
	// parties, PDFs, signatures and mailings. Drafts are no Beleg.
	BelegCareContract = "care_contract"
	// BelegSEPAMandate is a SEPA mandate with its events.
	BelegSEPAMandate = "sepa_mandate"
	// BelegSEPADebit is a submitted SEPA direct debit.
	BelegSEPADebit = "sepa_debit"
)

// BelegFamilyDebtorNumber names, in the deletion log only, the rows of the
// families without a care contract whose debtor number went after its own
// period (#6692). It is no Beleg type: the instruction and the deletion
// stop of the Träger for its holiday documents govern it.
const BelegFamilyDebtorNumber = "family_debtor_number"

// BelegTypes lists every Beleg type in display order.
var BelegTypes = []string{BelegHolidayDocument, BelegCareInvoice, BelegCareNotice, BelegCareContract, BelegSEPAMandate, BelegSEPADebit}

// Events that start a retention period.
const (
	// TriggerIssued is the issue of the Beleg: the issue day of an invoice,
	// cancellation, credit note or notice, the latest one of an invoice
	// and its cancellation, the submission day of a debit.
	TriggerIssued = "issued"
	// TriggerContractConcluded is the conclusion of a contract.
	TriggerContractConcluded = "contract_concluded"
	// TriggerContractEnded is the end of a contract.
	TriggerContractEnded = "contract_ended"
	// TriggerLastCollection is the submission day of the last collected
	// debit of an expired SEPA mandate.
	TriggerLastCollection = "last_collection"
)

// Limits of an instruction.
const (
	// MinRetentionYears and MaxRetentionYears bound a period in years.
	MinRetentionYears = 1
	MaxRetentionYears = 30
	// MinMandateRetentionMonths is the minimum after the expiry of a SEPA
	// mandate: 14 months from the last collected debit (Inkassovereinbarung).
	MinMandateRetentionMonths = 14
	// MaxMandateRetentionMonths bounds the months of a mandate.
	MaxMandateRetentionMonths = 360
	// ProposedBookingRetentionYears is the period the Löschkonzept proposes
	// for invoices, cancellations, credit notes and debits (§ 147 AO,
	// § 257 HGB, § 14b UStG).
	ProposedBookingRetentionYears = 8
	// ClarificationReviewDays is the internal deadline to clarify a Beleg
	// with the Träger after it was listed.
	ClarificationReviewDays = 30
	// MaxInstructionTextLength bounds the text fields of an instruction.
	MaxInstructionTextLength = 500
	// MaxJustificationLength bounds a written justification, the note of a
	// review and the reason of an escalation.
	MaxJustificationLength = 1000
	// MinBookingVoucherYears is the least period of a booking voucher under
	// § 147 AO, § 257 HGB and § 14b UStG; MinBusinessLetterYears the least
	// of a business letter (Löschkonzept section 2).
	MinBookingVoucherYears = 8
	MinBusinessLetterYears = 6
	// MaxClarificationReviewDays bounds the next review of a clarification
	// entry: an open entry is reviewed at least once a year.
	MaxClarificationReviewDays = 365
)

// Bases a Weisung names (Löschkonzept section 2).
const (
	// BasisBookingVoucher: a booking voucher under § 147 AO, § 257 HGB or
	// § 14b UStG, kept at least eight years after the end of its year.
	BasisBookingVoucher = "booking_voucher"
	// BasisBusinessLetter: a contract or notice the Träger classifies as a
	// business letter, kept at least six years.
	BasisBusinessLetter = "business_letter"
	// BasisCollectionAgreement: a SEPA mandate under the Inkassovereinbarung,
	// kept at least 14 months after its last collection.
	BasisCollectionAgreement = "collection_agreement"
	// BasisNotApplicable: the Träger states in writing why the statutes do
	// not apply; only then may a Beleg go before their periods.
	BasisNotApplicable = "not_applicable"
)

// ClarificationOwner is responsible at moto for clarifying a Beleg with its
// Träger and receives the escalation (Löschkonzept section 3).
const ClarificationOwner = "Florian (Abstimmung mit dem Träger)"

// Retention states of a Beleg.
const (
	// RetentionScheduled has an instruction and an end date.
	RetentionScheduled = "scheduled"
	// RetentionRunning has an instruction whose event has not happened yet:
	// a contract in force, a mandate still valid.
	RetentionRunning = "running"
	// RetentionClarification has no instruction or no date to compute from;
	// it waits on the clarification list and is never deleted.
	RetentionClarification = "clarification"
	// RetentionSuspended is held back by a suspension.
	RetentionSuspended = "suspended"
)

// Reasons a Beleg is on the clarification list.
const (
	// ClarifyNoInstruction: the Träger has given no instruction for the type.
	ClarifyNoInstruction = "no_instruction"
	// ClarifyNoCollection: an expired mandate without a collected debit.
	ClarifyNoCollection = "no_collection"
	// ClarifyNoTriggerDate: the instruction names an event the Beleg has
	// no date for, although it should have happened.
	ClarifyNoTriggerDate = "no_trigger_date"
)

var (
	// ErrBelegRetentionInvalid reports an instruction or suspension that
	// breaks a rule of its type.
	ErrBelegRetentionInvalid = errors.New("invalid beleg retention instruction")
	// ErrBelegBasisCitesStatute reports a basis "the statutes do not apply"
	// that the database's citation heuristic finds naming § 147 AO, § 257
	// HGB or § 14b UStG. Only the database decides it.
	ErrBelegBasisCitesStatute = errors.New("a basis without statutes cites § 147 AO, § 257 HGB or § 14b UStG")
	// ErrBelegSuspensionNotFound reports a suspension that is not active
	// for the Träger.
	ErrBelegSuspensionNotFound = errors.New("beleg retention suspension not found")
	// ErrBelegTraegerNotFound reports a Träger that does not exist.
	ErrBelegTraegerNotFound = errors.New("traeger not found")
	// ErrBelegClarificationNotFound reports a type and reason with no Beleg
	// on the clarification list of the Träger.
	ErrBelegClarificationNotFound = errors.New("beleg clarification entry not found")
)

// RetentionInstruction is the documented instruction of a Träger for one
// Beleg type. Period is years after the end of the year of the event, for
// a SEPA mandate months after the event. A basis "the statutes do not
// apply" carries the structured confirmation of the Träger and when the
// moto team recorded it.
type RetentionInstruction struct {
	ID                   int64         `json:"id"`
	OrganizationID       int64         `json:"organization_id"`
	BelegType            string        `json:"beleg_type"`
	TriggerEvent         string        `json:"trigger_event"`
	Period               int           `json:"period"`
	Basis                string        `json:"basis"`
	LegalBasis           string        `json:"legal_basis"`
	BasisJustification   string        `json:"basis_justification,omitempty"`
	InstructedBy         string        `json:"instructed_by"`
	InstructedOn         calendar.Date `json:"instructed_on"`
	InstructionReference string        `json:"instruction_reference"`
	// StatutesNotApplicableConfirmed: the Träger confirmed in writing that
	// § 147 AO, § 257 HGB and § 14b UStG do not apply.
	StatutesNotApplicableConfirmed bool       `json:"statutes_not_applicable_confirmed"`
	ConfirmedAt                    *time.Time `json:"confirmed_at,omitempty"`
	RecordedAt                     time.Time  `json:"recorded_at"`
}

// RetentionSuspension holds back the deletion of one Beleg or of every
// Beleg of a type, with reason, scope, responsible person and next review.
// A zero BelegID covers the whole type.
type RetentionSuspension struct {
	ID             int64         `json:"id"`
	OrganizationID int64         `json:"organization_id"`
	BelegType      string        `json:"beleg_type"`
	BelegID        int64         `json:"beleg_id,omitempty"`
	Reason         string        `json:"reason"`
	Scope          string        `json:"scope"`
	Responsible    string        `json:"responsible"`
	ReviewOn       calendar.Date `json:"review_on"`
	CreatedAt      time.Time     `json:"created_at"`
}

// BelegRetentionSummary counts the Belege of one type by state.
type BelegRetentionSummary struct {
	BelegType     string         `json:"beleg_type"`
	Scheduled     int            `json:"scheduled"`
	Running       int            `json:"running"`
	Clarification int            `json:"clarification"`
	Suspended     int            `json:"suspended"`
	NextEnd       *calendar.Date `json:"next_end,omitempty"`
}

// BelegClarification is one entry of the clarification list: the Belege of
// a type listed for the same reason, with the earliest listing and review
// and the latest review the moto team recorded.
type BelegClarification struct {
	BelegType   string                    `json:"beleg_type"`
	Reason      string                    `json:"reason"`
	Count       int                       `json:"count"`
	Overdue     int                       `json:"overdue"`
	ListedOn    calendar.Date             `json:"listed_on"`
	ReviewOn    calendar.Date             `json:"review_on"`
	Responsible string                    `json:"responsible"`
	LastReview  *BelegClarificationReview `json:"last_review,omitempty"`
}

// BelegClarificationReview is a review of a clarification entry
// (Löschkonzept section 3): what was clarified with the Träger, whether it
// went to Florian and why, and the next review.
type BelegClarificationReview struct {
	ID               int64         `json:"id"`
	BelegType        string        `json:"beleg_type"`
	Reason           string        `json:"reason"`
	Note             string        `json:"note"`
	Escalated        bool          `json:"escalated"`
	EscalationReason string        `json:"escalation_reason,omitempty"`
	ReviewedOn       calendar.Date `json:"reviewed_on"`
	NextReviewOn     calendar.Date `json:"next_review_on"`
	ReviewedAt       time.Time     `json:"reviewed_at"`
}

// NewBelegClarificationReview is a review the moto team records for the
// entry of a type and reason.
type NewBelegClarificationReview struct {
	BelegType        string
	Reason           string
	Note             string
	Escalated        bool
	EscalationReason string
	NextReviewOn     calendar.Date
}

// BelegDeletion is one entry of the deletion log: how many Belege of a type
// the retention deleted on a day, without content.
type BelegDeletion struct {
	BelegType string        `json:"beleg_type"`
	DeletedOn calendar.Date `json:"deleted_on"`
	Count     int           `json:"count"`
}

// BelegRetentionOverview is what the moto team sees per Träger.
type BelegRetentionOverview struct {
	Instructions   []RetentionInstruction  `json:"instructions"`
	Suspensions    []RetentionSuspension   `json:"suspensions"`
	Summary        []BelegRetentionSummary `json:"summary"`
	Clarifications []BelegClarification    `json:"clarifications"`
	Deletions      []BelegDeletion         `json:"deletions"`
	EvaluatedOn    *calendar.Date          `json:"evaluated_on,omitempty"`
}

// NewRetentionInstruction is what the moto team records from the
// documented instruction of a Träger.
type NewRetentionInstruction struct {
	BelegType            string
	TriggerEvent         string
	Period               int
	Basis                string
	LegalBasis           string
	BasisJustification   string
	InstructedBy         string
	InstructedOn         calendar.Date
	InstructionReference string
	// StatutesNotApplicableConfirmed is the explicit confirmation a basis
	// "the statutes do not apply" needs and every other basis lacks.
	StatutesNotApplicableConfirmed bool
}

// NewRetentionSuspension is a suspension the moto team records.
type NewRetentionSuspension struct {
	BelegType   string
	BelegID     int64
	Reason      string
	Scope       string
	Responsible string
	ReviewOn    calendar.Date
}

// BelegRetention is the retention of the Belege of every Träger: the
// instructions, suspensions and the clarification list the moto team keeps
// in the Operator portal, and the hourly evaluation and deletion. Every
// method runs in the caller's administrative transaction; the operator
// actor is recorded by the caller's audit.
type BelegRetention interface {
	Overview(ctx context.Context, organizationID int64) (BelegRetentionOverview, error)
	RecordInstruction(ctx context.Context, organizationID, operatorID int64, instruction NewRetentionInstruction) (RetentionInstruction, error)
	RecordSuspension(ctx context.Context, organizationID, operatorID int64, suspension NewRetentionSuspension) (RetentionSuspension, error)
	LiftSuspension(ctx context.Context, organizationID, suspensionID, operatorID int64) error
	// RecordClarificationReview records the review of the clarification
	// entry of a type and reason and moves its next review.
	RecordClarificationReview(ctx context.Context, organizationID, operatorID int64, review NewBelegClarificationReview) (BelegClarificationReview, error)
	// ApplyBelegRetention evaluates every Beleg of every Träger once per
	// day and deletes the Belege whose instructed period ended. It answers
	// the number of deleted Belege.
	ApplyBelegRetention(ctx context.Context) (int, error)
}

// RetainedContracts is what the retention needs of the Betreuungsvertrag:
// the facts of the concluded contracts and their deletion. The root binds
// it to Care Contract; both calls join the caller's administrative
// transaction.
type RetainedContracts interface {
	// ContractRetentionFacts lists the contracts past their draft of one
	// Träger or, for organizationID 0, of every Träger.
	ContractRetentionFacts(ctx context.Context, organizationID int64) ([]ContractRetentionFact, error)
	// DeleteEndedContracts deletes the given contracts of the Träger that
	// ended by today and whose event (TriggerContractEnded or
	// TriggerContractConcluded) lies on or before lastEventOn, with
	// everything that goes with them, and answers the IDs that went.
	DeleteEndedContracts(ctx context.Context, organizationID int64, contractIDs []int64, today calendar.Date, event string, lastEventOn calendar.Date) ([]int64, error)
}

// ContractRetentionFact is one contract past its draft: when it was
// concluded and whether and on which day it ended. Ended without EndedOn
// is a contract terminated or withdrawn without a known last day.
type ContractRetentionFact struct {
	OrganizationID int64
	ContractID     int64
	ConcludedOn    *calendar.Date
	Ended          bool
	EndedOn        *calendar.Date
}

// BelegFact is what the retention knows of one Beleg: its type and the days
// its events happened on. Issued is the issue of an invoice, cancellation,
// credit note or notice and the submission of a debit; a contract has
// ConcludedOn and, once Ended (terminated, withdrawn or ended), the day
// EndedOn it ended; a mandate has SignedOn, RevokedOn and the submission
// day of its last collected debit. CollectionTrigger is that day as far as
// a debit recorded it; the record keeps it once known.
type BelegFact struct {
	OrganizationID    int64
	BelegType         string
	BelegID           int64
	Issued            *calendar.Date
	ConcludedOn       *calendar.Date
	EndedOn           *calendar.Date
	Ended             bool
	SignedOn          *calendar.Date
	RevokedOn         *calendar.Date
	LastCollection    *calendar.Date
	CollectionTrigger *calendar.Date
}

// BelegRecord is the retention record of one Beleg on the day of its
// evaluation.
type BelegRecord struct {
	OrganizationID    int64
	BelegType         string
	BelegID           int64
	State             string
	Reason            string
	TriggerEvent      string
	TriggerDate       *calendar.Date
	DeletableOn       *calendar.Date
	ProposedOn        *calendar.Date
	InstructionID     int64
	SuspensionID      int64
	CollectionTrigger *calendar.Date
}

// DueContract is a contract whose instructed period ended: Care Contract
// deletes it when its event still lies on or before LastEventOn.
type DueContract struct {
	OrganizationID int64
	ContractID     int64
	TriggerEvent   string
	LastEventOn    calendar.Date
}
