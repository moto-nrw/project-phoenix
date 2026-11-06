// Package carecontract is the public capability of the Betreuungsvertrag
// (care-contract, ADR 0088, ADR 0153, #4678). The Träger-Büro keeps a
// contract template for every care offering of its schools, creates one
// contract from the accepted application of a child (Weg a: the draft is
// sent afterwards) over every module the child books, per weekday and sends it to the parents by mail and into the inbox of
// the parents portal. Contracts, versions and their trail live at the Träger,
// in tables with organization_id and the Träger policy (ADR 0086 section 5).
//
// The owner imports no other domain owner. The root binds its ports: the
// accepted applications and the offerings of a school (Enrollment and Care
// Plan, read in that school's tenant transaction), the schools of the Träger,
// the letterhead and the template of the contract letter (Organisation &
// Tenancy, Settings), and Delivery's outbox.
package carecontract

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The states of a contract (ADR 0088 section 2). The Träger-Büro reaches
// StatusDraft and StatusSent; the conclusion (conclusion.go: at the
// acceptance, #4680, or with the signature, #4679) reaches StatusActive;
// termination (#4682) moves it further.
const (
	StatusDraft  = "entwurf"
	StatusSent   = "versendet"
	StatusSigned = "unterschrieben"
	StatusActive = "aktiv"
	StatusEnded  = "beendet"
)

// The states of a version (ADR 0088 section 2). A draft's version waits as
// VersionRequested until a later step makes it effective.
const (
	VersionRequested  = "beantragt"
	VersionEffective  = "wirksam"
	VersionSuperseded = "abgeloest"
	VersionRejected   = "abgelehnt"
)

// The term of a contract template.
const (
	TermOpenEnded   = "open_ended"
	TermSchoolYear  = "school_year"
	TermFixedMonths = "fixed_months"
)

// The date a notice takes effect at.
const (
	NoticeMonthEnd      = "month_end"
	NoticeHalfYear      = "half_year"
	NoticeSchoolYearEnd = "school_year_end"
)

// MailKindContract is the outbox kind of the mail that carries a contract;
// MailRelatedContract its related entity type (ADR 0110, ADR 0123).
const (
	MailKindContract    = "care_contract"
	MailRelatedContract = "care_contract"
)

// InboxKindContract is the kind of a contract in the parents' inbox.
const InboxKindContract = "care_contract"

var (
	// ErrContractNotFound reports a contract that is not one of the Träger.
	ErrContractNotFound = errors.New("care contract not found")
	// ErrApplicationNotFound reports an application child that is unknown,
	// not accepted, or books no care offering at a school of the Träger.
	ErrApplicationNotFound = errors.New("accepted application not found")
	// ErrSchoolOutsideTraeger reports a school of another Träger.
	ErrSchoolOutsideTraeger = errors.New("the school does not belong to the Träger")
	// ErrOfferingNotFound reports an offering the school does not have.
	ErrOfferingNotFound = errors.New("care offering not found")
	// ErrContractExists reports an application child that has a contract
	// already.
	ErrContractExists = errors.New("the application has a contract")
	// ErrContractNotDraft reports a write that only a draft allows.
	ErrContractNotDraft = errors.New("the contract is no draft")
	// ErrContractNotSendable reports a contract that cannot be sent
	// (signed, ended, or active without its frozen document).
	ErrContractNotSendable = errors.New("the contract can no longer be sent")
	// ErrRecipientMissing reports a party without an address.
	ErrRecipientMissing = errors.New("the contract party has no e-mail address")
	// ErrMailPending reports a send while the last mail still waits.
	ErrMailPending = errors.New("the last mail of the contract still waits")
	// ErrLetterheadTooLong reports a letterhead or footer that leaves too
	// little room for the contract.
	ErrLetterheadTooLong = errors.New("the letterhead leaves too little room for the contract")
	// ErrInboxDocumentNotFound reports a contract outside the reader's inbox.
	ErrInboxDocumentNotFound = errors.New("contract not found in the inbox")
)

// ValidationError names the refused fields of an input.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "the contract template is invalid" }

// Reviewer is the account of the Träger-Büro that acts, with its Träger.
type Reviewer struct {
	OrganizationID int64
	AccountID      int64
}

// TemplateInput is what the Träger-Büro writes into a contract template.
type TemplateInput struct {
	TermKind               string `json:"term_kind"`
	TermMonths             *int   `json:"term_months"`
	NoticeMonths           int    `json:"notice_months"`
	NoticeTo               string `json:"notice_to"`
	SpecialTermination     bool   `json:"special_termination"`
	SpecialTerminationNote string `json:"special_termination_note"`
	StatuteRequired        bool   `json:"statute_required"`
	StatuteTitle           string `json:"statute_title"`
	StatuteURL             string `json:"statute_url"`
	ContractPartner        string `json:"contract_partner"`
}

// Template is the contract template of one offering.
type Template struct {
	ID int64 `json:"id"`
	TemplateInput
	UpdatedAt time.Time `json:"updated_at"`
}

// OfferingTemplate is one care offering of a school with its template, or
// none yet.
type OfferingTemplate struct {
	SchoolID     int64     `json:"school_id"`
	SchoolName   string    `json:"school_name"`
	OfferingID   int64     `json:"offering_id"`
	OfferingName string    `json:"offering_name"`
	PhaseName    string    `json:"phase_name"`
	PriceCents   *int      `json:"price_cents"`
	Active       bool      `json:"active"`
	Template     *Template `json:"template"`
}

// CandidateKey names one accepted application child.
type CandidateKey struct {
	SchoolID       int64 `json:"school_id"`
	RequestChildID int64 `json:"request_child_id"`
}

// Service is one booked service of a contract: on a weekday ("mon" to
// "sun") the child books a module (a care offering of the school) and ends
// at its end time ("HH:MM", empty when the module has none). A weekday may
// carry several modules, early and afternoon care for instance; a weekday
// without a service has no care.
type Service struct {
	Weekday        string `json:"weekday"`
	CareOfferingID int64  `json:"care_offering_id"`
	ModuleName     string `json:"module_name"`
	EndsAt         string `json:"ends_at"`
	// LastDay is the last day the module is booked, set only when it ends
	// before the contract version does: the contract never promises a
	// module beyond its booking. Nil means the module runs as long as the
	// version.
	LastDay *calendar.Date `json:"last_day"`
}

// Candidate is an accepted application child with booked care and no
// contract yet. TemplateIssue names why no draft can be made: a module
// without conditions, or modules whose conditions differ.
type Candidate struct {
	CandidateKey
	// AcceptanceIssue tells why the application gives no proof of the
	// parents' agreement for a conclusion at the acceptance (Weg b): empty,
	// SkipAcceptanceMissing or SkipAcceptanceChanged.
	AcceptanceIssue string        `json:"acceptance_issue"`
	SchoolName      string        `json:"school_name"`
	ChildFirstName  string        `json:"child_first_name"`
	ChildLastName   string        `json:"child_last_name"`
	GuardianName    string        `json:"guardian_name"`
	GuardianEmail   string        `json:"guardian_email"`
	PhaseName       string        `json:"phase_name"`
	Services        []Service     `json:"services"`
	EffectiveFrom   calendar.Date `json:"effective_from"`
	TemplateIssue   string        `json:"template_issue"`
}

// CreateResult answers a creation of several drafts: the created contracts
// and, per refused key, why.
type CreateResult struct {
	Created []ContractSummary `json:"created"`
	Skipped []CreateSkip      `json:"skipped"`
}

// CreateSkip is a key the creation left out, with the error code of its
// reason.
type CreateSkip struct {
	CandidateKey
	Reason string `json:"reason"`
}

// The reasons of a CreateSkip; the first two are also the TemplateIssue
// of a Candidate.
const (
	SkipTemplateMissing     = "template_missing"
	SkipTemplatesDiffer     = "templates_differ"
	SkipContractExists      = "contract_exists"
	SkipNotFound            = "application_not_found"
	SkipConclusionAutomatic = "conclusion_automatic"
	SkipAcceptanceMissing   = "acceptance_missing"
	SkipAcceptanceChanged   = "acceptance_changed"
)

// ContractFilter selects the contracts of the list.
type ContractFilter struct {
	Status   string
	SchoolID int64
	Search   string
	Offset   int
	Limit    int
}

// ContractSummary is one contract in the list.
type ContractSummary struct {
	ID             int64         `json:"id"`
	Status         string        `json:"status"`
	SchoolID       int64         `json:"school_id"`
	SchoolName     string        `json:"school_name"`
	ChildFirstName string        `json:"child_first_name"`
	ChildLastName  string        `json:"child_last_name"`
	ModuleNames    []string      `json:"module_names"`
	PartyName      string        `json:"party_name"`
	PartyEmail     string        `json:"party_email"`
	EffectiveFrom  calendar.Date `json:"effective_from"`
	CreatedAt      time.Time     `json:"created_at"`
	SentAt         *time.Time    `json:"sent_at"`
	// Concluded is how and when the contract was concluded; nil before.
	Concluded *Concluded `json:"concluded"`
}

// ContractList is the filtered list with the count of every status over all
// contracts of the Träger.
type ContractList struct {
	Contracts []ContractSummary `json:"contracts"`
	Counts    map[string]int    `json:"counts"`
	HasMore   bool              `json:"has_more"`
}

// Version is the frozen content of a contract version: the child, the
// booked services per weekday and module, the terms and the effective date.
type Version struct {
	ID             int64          `json:"id"`
	Number         int            `json:"number"`
	Status         string         `json:"status"`
	EffectiveFrom  calendar.Date  `json:"effective_from"`
	EndsOn         *calendar.Date `json:"ends_on"`
	ChildFirstName string         `json:"child_first_name"`
	ChildLastName  string         `json:"child_last_name"`
	ChildBirthDate *calendar.Date `json:"child_birth_date"`
	SchoolName     string         `json:"school_name"`
	PhaseName      string         `json:"phase_name"`
	Services       []Service      `json:"services"`
	// PriceCents is the monthly price when the version was made: the price
	// of its only module, nil when it has none or the contract has several
	// modules (ADR 0088 section 2). The tariffs per weekday and module of
	// #4684 price the rest.
	PriceCents *int64        `json:"price_cents"`
	Terms      TemplateInput `json:"terms"`
	CreatedAt  time.Time     `json:"created_at"`
}

// Party is a party to the contract.
type Party struct {
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Email      string `json:"email"`
	HasAccount bool   `json:"has_account"`
}

// Mailing is one send of the contract with the state of its mail.
type Mailing struct {
	Recipient string    `json:"recipient"`
	SentAt    time.Time `json:"sent_at"`
	State     string    `json:"state"`
	// OutboxID is the mail in Delivery's outbox; the wire never shows it.
	OutboxID int64 `json:"-"`
}

// The states of a Mailing, as ADR 0123 names them.
const (
	MailQueued  = "queued"
	MailSent    = "sent"
	MailFailed  = "failed"
	MailUnknown = "unknown"
)

// ContractDetail is one contract with its version, parties and sends.
type ContractDetail struct {
	ContractSummary
	Version  Version   `json:"version"`
	Parties  []Party   `json:"parties"`
	Mailings []Mailing `json:"mailings"`
	// DocumentFrozen tells that the PDF was rendered at the first send and
	// no longer follows the template.
	DocumentFrozen bool `json:"document_frozen"`
}

// File is a rendered PDF.
type File struct {
	Filename    string
	ContentType string
	Data        []byte
}

// SendResult answers a send of several contracts.
type SendResult struct {
	Sent    []int64    `json:"sent"`
	Skipped []SendSkip `json:"skipped"`
}

// SendSkip is a contract the send left out, with its reason.
type SendSkip struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// The reasons of a SendSkip.
const (
	SkipRecipientMissing = "recipient_missing"
	SkipPartnerMissing   = "partner_missing"
	SkipMailPending      = "mail_pending"
	SkipNotSendable      = "not_sendable"
)

// Contracts serves the contracts of a Träger to its Träger-Büro. Every
// operation lands in the trail. The writes run in the Träger transaction of
// the reviewer's Träger, which the caller opens. The reads (ListContracts,
// Contract, ContractDocument) open their own units, so the caller holds no
// transaction: the Träger transaction, then the tenant transaction of each
// school whose contracts they showed, for its access log (ADR 0088 section 7).
type Contracts interface {
	// ListContracts lists the contracts of the filter, newest first.
	ListContracts(ctx context.Context, reviewer Reviewer, filter ContractFilter) (ContractList, error)
	// Contract reads one contract.
	Contract(ctx context.Context, reviewer Reviewer, contractID int64) (ContractDetail, error)
	// ContractDocument answers the PDF: the stored one once sent, before
	// that a preview with the current template, which stores nothing.
	ContractDocument(ctx context.Context, reviewer Reviewer, contractID int64) (File, error)
	// SendContracts sends each contract by mail and into the inbox: a draft
	// gets its PDF and becomes versendet, a sent or active one is sent again with the
	// same PDF.
	SendContracts(ctx context.Context, reviewer Reviewer, contractIDs []int64) (SendResult, error)
	// ResendContract corrects only the delivery address of a failed send,
	// preserving the contract party, frozen PDF and previous mailings.
	ResendContract(ctx context.Context, reviewer Reviewer, contractID int64, email string) (SendResult, error)
	// DiscardContract removes a draft; a sent contract stays.
	DiscardContract(ctx context.Context, reviewer Reviewer, contractID int64) error
}

// Intake creates contracts from the accepted applications and keeps the
// templates of the offerings. It reads each school in that school's tenant
// transaction and writes in the Träger transaction, one after the other, so
// it opens its own units and must be called outside any transaction.
type Intake interface {
	// Candidates lists the accepted application children with booked care
	// and no contract, over all schools of the Träger.
	Candidates(ctx context.Context, reviewer Reviewer, schoolID int64) ([]Candidate, error)
	// CreateContracts creates a draft for every key over all modules the
	// child books; the keys that have a contract already, or whose modules
	// lack conditions or differ in them, are left out with their reason.
	CreateContracts(ctx context.Context, reviewer Reviewer, keys []CandidateKey) (CreateResult, error)
	// Templates lists every offering of the Träger's schools with its
	// template.
	Templates(ctx context.Context, reviewer Reviewer) ([]OfferingTemplate, error)
	// SaveTemplate creates or replaces the template of one offering.
	SaveTemplate(ctx context.Context, reviewer Reviewer, schoolID, offeringID int64, input TemplateInput) (OfferingTemplate, error)
	// ConclusionModes tells for every school of the Träger how its contracts
	// come about: ModeSendLater (Weg a) or ModeOnAcceptance (Weg b).
	ConclusionModes(ctx context.Context, reviewer Reviewer) ([]SchoolConclusion, error)
}

// InboxReader is the parents' account that reads its inbox, with its
// current address and the children the workflow authorized.
type InboxReader struct {
	AccountID int64
	Email     string
	// Children contains only permitted student IDs, grouped by school.
	Children map[int64][]int64
}

// InboxContract is one sent contract in the inbox of the parents portal.
type InboxContract struct {
	ID          int64         `json:"id"`
	ChildNames  []string      `json:"child_names"`
	ModuleNames []string      `json:"module_names"`
	SchoolName  string        `json:"school_name"`
	SentAt      time.Time     `json:"sent_at"`
	EffectiveOn calendar.Date `json:"effective_on"`
}

// Inbox serves the contracts of one Träger in the parents' inbox (ADR 0088
// section 7, ADR 0126): a contract shows once it was sent to the address of
// the account and the matching child relationship permits it. Both operations
// run in the Träger transaction the caller
// opens and record the read with the account.
type Inbox interface {
	InboxContracts(ctx context.Context, organizationID int64, reader InboxReader) ([]InboxContract, error)
	InboxContractDocument(ctx context.Context, organizationID, contractID int64, reader InboxReader) (File, error)
}

// ContractMail is the payload of the mail of a contract: everything the
// worker needs, so it needs no Träger transaction (ADR 0123 point 2).
type ContractMail struct {
	OrganizationID int64  `json:"organization_id"`
	ContractID     int64  `json:"contract_id"`
	Recipient      string `json:"recipient"`
	GuardianName   string `json:"guardian_name"`
	TraegerName    string `json:"traeger_name"`
	ChildName      string `json:"child_name"`
	Modules        string `json:"modules"`
	Filename       string `json:"filename"`
	PDF            []byte `json:"pdf"`
	IdempotencyKey string `json:"idempotency_key"`
	// Concluded tells that the contract is concluded already (Weg b): the
	// parents receive it for their records and sign nothing.
	Concluded bool `json:"concluded,omitempty"`
}

// RenderedContractMail is the mail the worker sends.
type RenderedContractMail struct {
	SenderName string
	Subject    string
	Template   string
	Content    ContractMailContent
	Filename   string
	PDF        []byte
}

// ContractMailContent is what the mail template prints.
type ContractMailContent struct {
	GuardianName string
	TraegerName  string
	ChildName    string
	Modules      string
	Concluded    bool
}

// ContractMailRenderer renders the payload of a claimed contract mail.
type ContractMailRenderer func(ctx context.Context, payload []byte) (RenderedContractMail, error)
