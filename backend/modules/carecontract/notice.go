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
//   - The parents act only for a child whose relationship grants the
//     enrollment submission (parent_portal.enrollment.submit), on a contract
//     sent to their own address; every declaration and every read of the
//     notice page lands in the trail.

// The states a declaration leaves a contract in.
const (
	StatusTerminated = "gekuendigt"
	StatusWithdrawn  = "widerrufen"
)

// The kinds of a declaration.
const (
	NoticeTermination = "kuendigung"
	NoticeWithdrawal  = "widerruf"
)

// The kinds of a termination: to the next date the terms allow, or the
// special termination to the half year when the template provides one.
const (
	TerminationOrdinary = "ordentlich"
	TerminationSpecial  = "sonderkuendigung"
)

// Who made the declaration: the parents with the button in the parents
// portal, or the Träger-Büro for a declaration it received in Textform.
const (
	ChannelParents = "eltern_app"
	ChannelOffice  = "traeger_buero"
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
)

// NoticeInput is a declaration. Kind is NoticeTermination or
// NoticeWithdrawal; a termination names its TerminationKind. ReceivedOn is
// the day the Träger-Büro received a declaration by mail or letter; the
// parents portal sets today.
type NoticeInput struct {
	Kind            string        `json:"kind"`
	TerminationKind string        `json:"termination_kind"`
	ReceivedOn      calendar.Date `json:"received_on"`
}

// Notice is a declaration as recorded, with its confirmation.
type Notice struct {
	Kind             string         `json:"kind"`
	TerminationKind  string         `json:"termination_kind,omitempty"`
	Channel          string         `json:"channel"`
	DeclaredAt       time.Time      `json:"declared_at"`
	ReceivedOn       calendar.Date  `json:"received_on"`
	EndsOn           *calendar.Date `json:"ends_on"`
	Recipient        string         `json:"recipient"`
	ConfirmationText string         `json:"confirmation_text"`
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
	// the contract has no right of withdrawal or it has passed.
	WithdrawalUntil *calendar.Date `json:"withdrawal_until"`
	// EarlyStart tells that the parents asked for care within the period.
	EarlyStart bool    `json:"early_start"`
	Notice     *Notice `json:"notice"`
}

// NoticeMail is the payload of the confirmation mail: everything the worker
// needs, so it needs no Träger transaction (ADR 0123 point 2).
type NoticeMail struct {
	OrganizationID   int64  `json:"organization_id"`
	ContractID       int64  `json:"contract_id"`
	Recipient        string `json:"recipient"`
	GuardianName     string `json:"guardian_name"`
	TraegerName      string `json:"traeger_name"`
	Kind             string `json:"kind"`
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
	Paragraphs   []string
}

// NoticeMailRenderer renders the payload of a claimed confirmation mail.
type NoticeMailRenderer func(ctx context.Context, payload []byte) (RenderedNoticeMail, error)

// NoticeTemplates reads the Träger's confirmation templates in the
// caller's Träger transaction (setting care_contract.notice_confirmation_*,
// ADR 0086): never an environment value.
type NoticeTemplates interface {
	NoticeTemplate(ctx context.Context, organizationID int64, kind string) (string, error)
}

// NoticePost enqueues the confirmation mail in the caller's Träger
// transaction.
type NoticePost interface {
	EnqueueNoticeMail(ctx context.Context, mail NoticeMail) (int64, error)
}

// Notices serves withdrawal and termination. The office operations run in
// the Träger transaction of the reviewer, which the caller opens; the
// parents' in the Träger transaction of the contract's Träger, after the
// parents portal checked the relationships (reader.Children).
type Notices interface {
	NoticeOptions(ctx context.Context, reviewer Reviewer, contractID int64) (NoticeOptions, error)
	DeclareNotice(ctx context.Context, reviewer Reviewer, contractID int64, input NoticeInput) (NoticeOptions, error)
	ParentNoticeOptions(ctx context.Context, organizationID, contractID int64, reader InboxReader) (NoticeOptions, error)
	// ParentDeclareNotice confirms in the name traeger, the Träger the
	// parents portal resolved from the relationships.
	ParentDeclareNotice(ctx context.Context, organizationID, contractID int64, reader InboxReader, traeger string, input NoticeInput) (NoticeOptions, error)
}
