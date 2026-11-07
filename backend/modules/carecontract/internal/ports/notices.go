package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The actions of the trail for a declaration (#4682).
const (
	ActionTerminate = "terminate"
	ActionWithdraw  = "withdraw"
	// ActionSetEnd is the Träger's end of an extraordinary termination.
	ActionSetEnd = "set_end"
	// ActionRejectReason is the Träger's refusal of the reason of an
	// extraordinary termination, which then holds as an ordinary one.
	ActionRejectReason = "reject_reason"
)

// NoticeState is what a declaration on a contract needs beyond its record:
// the start of its first effective version, the end of its newest one, when
// the parents got the withdrawal notice, their wish for an early start, and
// the declaration made.
type NoticeState struct {
	FirstFrom   *calendar.Date
	ContractEnd *calendar.Date
	InformedAt  *time.Time
	EarlyStart  bool
	Notice      *carecontract.Notice
	// NoticeID identifies Notice; zero without one.
	NoticeID int64
}

// NoticeEnd is the decided end of an extraordinary termination: the last
// day, how the Träger-Büro decided it, who and when, and the notice the
// parents get.
type NoticeEnd struct {
	EndsOn    calendar.Date
	Decision  string
	AccountID int64
	DecidedAt time.Time
	Text      string
}

// NewNotice is a declaration to store.
type NewNotice struct {
	DispatchedOn     *calendar.Date
	ContractID       int64
	Kind             string
	TerminationKind  string
	Reason           string
	Channel          string
	AccountID        int64
	DeclaredAt       time.Time
	ReceivedOn       calendar.Date
	EndsOn           *calendar.Date
	Recipient        string
	ConfirmationText string
}

// NoticeStore persists the declarations of a Träger in its Träger
// transaction. The postgres Store implements it next to Store.
type NoticeStore interface {
	// NoticeState reads the state of a contract for a declaration; the
	// caller holds the contract's row lock when it writes afterwards.
	NoticeState(ctx context.Context, organizationID, contractID int64) (NoticeState, error)
	// InsertNotice stores a declaration and moves the contract to its
	// status: gekuendigt with its last day, or widerrufen.
	InsertNotice(ctx context.Context, organizationID int64, notice NewNotice) (int64, error)
	// SetNoticeEnd decides the end of a contract whose extraordinary
	// termination has no end yet, on the declaration and the contract; it
	// answers carecontract.ErrNoticeEndNotOpen when there is none. The caller
	// holds the contract's row lock.
	SetNoticeEnd(ctx context.Context, organizationID, contractID int64, end NoticeEnd) error
	// SetEndNoticeOutbox records the mail that told the parents the end.
	SetEndNoticeOutbox(ctx context.Context, organizationID, noticeID, outboxID int64) error
	// SetNoticeOutbox records the confirmation mail of a declaration.
	SetNoticeOutbox(ctx context.Context, organizationID, noticeID, outboxID int64) error
	// InboxContractID tells whether the contract was sent to the reader's
	// address and its child lies in the reader's permitted children.
	InboxContractID(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (bool, error)
}
