package carecontract

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Withdrawal and termination of a concluded contract (#4682, ADR 0088
// section 2). The parents end a contract in the parents portal, with the
// button of § 312k BGB, or by mail or letter to the Träger-Büro, which
// records it for them: a termination needs no stricter form than Textform
// (§ 309 Nr. 13 BGB). Every declaration gets a confirmation in Textform,
// by mail and on the screen, after the Träger's template.
//
// Invariants (each has a test):
//   - A termination ends the contract on its notice date: status
//     gekuendigt, the last day ends_on. VersionsInForce answers no day after
//     it, so the monthly contribution (#4684) stops there.
//   - A withdrawal undoes the contract from its start: status widerrufen,
//     and VersionsInForce answers no day at all. It is open only within the
//     withdrawal period: 14 days from the conclusion when the parents got
//     the withdrawal notice with the contract, else 12 months and 14 days
//     (§ 356 Abs. 3 BGB). A contract signed on paper at the office has none.
//   - A contract leaves aktiv once: each declaration takes the row lock of
//     the contract first and then reads its versions, the order of the
//     conclusion (#4680) and the amendment (#4681). A concurrent termination
//     or amendment waits and then sees the ended contract.
//   - The end of an extraordinary termination is decided once, under the
//     contract's row lock: the Träger-Büro sets it or does not accept the
//     reason, never both. Either way the parents get one notice in
//     Textform (mail and parents portal), keyed to the declaration.
//   - The parents act only for a child whose relationship grants the
//     enrollment submission (parent_portal.enrollment.submit), on a contract
//     sent to their own address; every declaration and every read of the
//     notice page lands in the trail.

// The kinds of a declaration.
const (
	NoticeTermination = "kuendigung"
	NoticeWithdrawal  = "widerruf"
)

// The kinds of a termination: to the next date the terms allow, the
// special termination to the half year when the template provides one, or
// the extraordinary termination for a reason the parents name (§ 312k
// Abs. 2 BGB). An extraordinary termination is received as declared, and
// the Träger-Büro checks the reason (#4682, decision of 07.11.2026): it
// accepts it and sets the last day (SetNoticeEnd), or it does not accept
// it, and the declaration holds as an ordinary termination to the next
// notice date (RejectNoticeReason, § 140 BGB). Until then the contract
// holds.
const (
	TerminationOrdinary      = "ordentlich"
	TerminationSpecial       = "sonderkuendigung"
	TerminationExtraordinary = "ausserordentlich"
)

// The decisions on an extraordinary termination: the Träger-Büro accepted
// the reason and set the end, or it did not accept it and the declaration
// holds as an ordinary termination (Umdeutung, § 140 BGB).
const (
	EndDecisionSet           = "festgelegt"
	EndDecisionReinterpreted = "umgedeutet"
)

// GuardianPermissionEnrollmentSubmit is the relationship right of the
// parents portal that lets a guardian act for a child's contract: sign,
// terminate, and receive the notice of a decided end.
const GuardianPermissionEnrollmentSubmit = "parent_portal.enrollment.submit"

// The templates of the notices the parents get in Textform once the end of
// an extraordinary termination is decided; NoticeTemplates reads them by
// these kinds.
const (
	NoticeTemplateEndSet         = "ende_festgelegt"
	NoticeTemplateReasonRejected = "grund_abgelehnt"
)

// ReasonMaxLength bounds the reason of an extraordinary termination.
const ReasonMaxLength = 2000

// Who made the declaration: the parents with the button in the parents
// portal, or the Träger-Büro for a declaration it received in Textform.
const (
	NoticeChannelParents = "eltern_app"
	NoticeChannelOffice  = "traeger_buero"
)

// WithdrawalDays is the withdrawal period of a distance contract (§ 355
// Abs. 2 BGB). The law fixes it; no setting shortens it.
const WithdrawalDays = 14

// MailKindNotice is the outbox kind of the confirmation of a declaration.
const MailKindNotice = "care_contract_notice"

var (
	// ErrContractNotEndable reports a declaration on a contract that is not
	// aktiv: not concluded yet, or terminated or withdrawn already.
	ErrContractNotEndable = errors.New("the contract is not active")
	// ErrWithdrawalClosed reports a withdrawal after its period, or of a
	// contract without a right of withdrawal.
	ErrWithdrawalClosed = errors.New("the contract can no longer be withdrawn")
	// ErrSpecialTerminationClosed reports a special termination the terms of
	// the contract do not provide.
	ErrSpecialTerminationClosed = errors.New("the contract has no special termination")
	// ErrNoticeTemplateMissing reports that the Träger has no confirmation
	// template; its setting has an empty value.
	ErrNoticeTemplateMissing = errors.New("the Träger has no confirmation template")
	// ErrNoticeEndNotOpen reports SetNoticeEnd or RejectNoticeReason on a
	// contract without an extraordinary termination whose end is still
	// open: the end is decided once, by one of the two.
	ErrNoticeEndNotOpen = errors.New("the contract has no termination with an open end")
)

// NoticeInput is a declaration. Kind is NoticeTermination or
// NoticeWithdrawal; a termination names its TerminationKind. ReceivedOn is
// the day the Träger-Büro received a declaration by mail or letter; the
// parents portal sets today.
type NoticeInput struct {
	// DispatchedOn is the postal dispatch day. Omission conservatively uses receipt.
	DispatchedOn calendar.Date `json:"dispatched_on"`
	// ExpectedEndsOn rejects a termination date changed since the displayed choice.
	ExpectedEndsOn  calendar.Date `json:"expected_ends_on"`
	Kind            string        `json:"kind"`
	TerminationKind string        `json:"termination_kind"`
	// Reason is the parents' reason of an extraordinary termination; no
	// other declaration carries one.
	Reason     string        `json:"reason"`
	ReceivedOn calendar.Date `json:"received_on"`
}

// Notice is a declaration as recorded, with its confirmation.
type Notice struct {
	DispatchedOn    *calendar.Date `json:"dispatched_on,omitempty"`
	Kind            string         `json:"kind"`
	TerminationKind string         `json:"termination_kind,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	Channel         string         `json:"channel"`
	DeclaredAt      time.Time      `json:"declared_at"`
	ReceivedOn      calendar.Date  `json:"received_on"`
	// EndsOn is nil for a withdrawal and for an extraordinary termination
	// the Träger has not checked yet.
	EndsOn           *calendar.Date `json:"ends_on"`
	Recipient        string         `json:"recipient"`
	ConfirmationText string         `json:"confirmation_text"`
	// EndDecision is how the Träger-Büro decided the end of an
	// extraordinary termination (EndDecisionSet, EndDecisionReinterpreted),
	// empty while it is open and for every other declaration. EndNoticeText
	// is the notice the parents got with it, as sent.
	EndDecision   string     `json:"end_decision,omitempty"`
	EndDecidedAt  *time.Time `json:"end_decided_at,omitempty"`
	EndNoticeText string     `json:"end_notice_text,omitempty"`
	// EndNoticeSent tells that the notice went out by mail. It stays false
	// when no address of the contract still belongs to a guardian whose
	// relationship to the child grants GuardianPermissionEnrollmentSubmit:
	// the office then tells the parents another way.
	EndNoticeSent bool `json:"end_notice_sent,omitempty"`
}

// NoticeOptions is what the parents (or the office for them) can still
// declare on a contract today, or the declaration made.
type NoticeOptions struct {
	ContractID int64  `json:"contract_id"`
	Status     string `json:"status"`
	ChildName  string `json:"child_name"`
	Modules    string `json:"modules"`
	SchoolName string `json:"school_name"`
	// Terms is how the contract may be terminated, as its PDF says.
	Terms string `json:"terms"`
	// OrdinaryEndsOn is the last day of the contract after a termination
	// today; SpecialEndsOn the same for the special termination, nil when
	// the terms provide none.
	OrdinaryEndsOn *calendar.Date `json:"ordinary_ends_on"`
	SpecialEndsOn  *calendar.Date `json:"special_ends_on"`
	// WithdrawalUntil is the last day of the withdrawal period, nil when
	// the contract has no right of withdrawal. Parents see nil after expiry;
	// the office retains the deadline for timely postal dispatches.
	WithdrawalUntil *calendar.Date `json:"withdrawal_until"`
	// ReinterpretedEndsOn is, for the office and an extraordinary
	// termination whose end is open, the last day if the Träger-Büro does
	// not accept the reason: the ordinary notice date from the receipt.
	ReinterpretedEndsOn *calendar.Date `json:"reinterpreted_ends_on,omitempty"`
	// EarlyStart tells that the parents asked for care within the period.
	EarlyStart bool    `json:"early_start"`
	Notice     *Notice `json:"notice"`
}

// NoticeMail is the payload of the confirmation mail: everything the worker
// needs, so it needs no Träger transaction (ADR 0123 point 2).
type NoticeMail struct {
	OrganizationID int64  `json:"organization_id"`
	ContractID     int64  `json:"contract_id"`
	Recipient      string `json:"recipient"`
	GuardianName   string `json:"guardian_name"`
	TraegerName    string `json:"traeger_name"`
	Kind           string `json:"kind"`
	// Decision is set on the notice of a decided end (EndDecisionSet,
	// EndDecisionReinterpreted); empty on a confirmation.
	Decision         string `json:"decision,omitempty"`
	ConfirmationText string `json:"confirmation_text"`
	IdempotencyKey   string `json:"idempotency_key"`
}

// RenderedNoticeMail is the confirmation the worker sends.
type RenderedNoticeMail struct {
	SenderName   string
	Subject      string
	Template     string
	GuardianName string
	TraegerName  string
	Withdrawal   bool
	// Heading is the heading of a notice of a decided end; empty on a
	// confirmation, whose heading follows Withdrawal.
	Heading    string
	Paragraphs []string
}

// NoticeMailRenderer renders the payload of a claimed confirmation mail.
type NoticeMailRenderer func(ctx context.Context, payload []byte) (RenderedNoticeMail, error)

// NoticeTemplates reads the Träger's confirmation templates in the
// caller's Träger transaction (setting care_contract.*_confirmation, ADR
// 0086): never an environment value. kind is NoticeTermination,
// NoticeWithdrawal, TerminationExtraordinary, NoticeTemplateEndSet or
// NoticeTemplateReasonRejected.
type NoticeTemplates interface {
	NoticeTemplate(ctx context.Context, organizationID int64, kind string) (string, error)
}

// NoticeAddressees answers, in the caller's tenant transaction of the
// child's school, the mail addresses of those of accountIDs (the contract
// party's and the declaring parent's account) whose guardian relationship
// to the child grants GuardianPermissionEnrollmentSubmit: the guardian
// profile address and the account address. The notice of a decided end
// goes only to such an address (#4682,
// .claude/rules/guardian-parent-permissions.md).
type NoticeAddressees interface {
	PermittedAddresses(ctx context.Context, studentID int64, accountIDs []int64) ([]string, error)
}

// NoticeDelivery is evidence that the provider delivered a contract mail.
type NoticeDelivery struct {
	OutboxID   int64
	ProvidedAt time.Time
}

// NoticePost enqueues confirmations and reads positive delivery evidence
// in the caller's Träger transaction.
type NoticePost interface {
	NoticeDeliveries(ctx context.Context, organizationID, contractID int64) ([]NoticeDelivery, error)
	EnqueueNoticeMail(ctx context.Context, mail NoticeMail) (int64, error)
}

// Notices serves withdrawal and termination. The office operations run in
// the Träger transaction of the reviewer, which the caller opens; the
// parents' in the Träger transaction of the contract's Träger, after the
// parents portal checked the relationships (reader.Children).
type Notices interface {
	NoticeOptions(ctx context.Context, reviewer Reviewer, contractID int64, receivedOn calendar.Date) (NoticeOptions, error)
	DeclareNotice(ctx context.Context, reviewer Reviewer, contractID int64, input NoticeInput) (NoticeOptions, error)
	ParentNoticeOptions(ctx context.Context, organizationID, contractID int64, reader InboxReader) (NoticeOptions, error)
	// ParentDeclareNotice confirms in the name traeger, the Träger the
	// parents portal resolved from the relationships.
	ParentDeclareNotice(ctx context.Context, organizationID, contractID int64, reader InboxReader, traeger string, input NoticeInput) (NoticeOptions, error)
	// SetNoticeEnd sets the last day of a contract the parents terminated
	// extraordinarily, after the Träger checked their reason; it takes the
	// contract's row lock first, as every declaration does.
	// It tells the parents the last day in Textform.
	SetNoticeEnd(ctx context.Context, reviewer Reviewer, contractID int64, endsOn calendar.Date) (NoticeOptions, error)
	// RejectNoticeReason records that the Träger-Büro does not accept the
	// reason of an extraordinary termination: the declaration holds as an
	// ordinary termination to the next notice date from its receipt
	// (§ 140 BGB), and the parents get the new end in Textform.
	// expectedEndsOn, when set, refuses an end other than the one shown.
	RejectNoticeReason(ctx context.Context, reviewer Reviewer, contractID int64, expectedEndsOn calendar.Date) (NoticeOptions, error)
}
