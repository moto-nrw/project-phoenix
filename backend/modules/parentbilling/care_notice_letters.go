package parentbilling

import (
	"context"
	"time"
)

// The stored original of a Jahresbescheid and its delivery (#6347, decision
// A, ADR 0182 point 8 and ADR 0165). A released notice run gets the steps of
// the monthly letters: the Abrechnung renders the PDF of every issued notice
// once and stores it; from then on the letterhead, the template, the address
// or the contract of today never reach it. Only an explicit send of the
// office mails exactly that PDF and, with the first send, puts it in the
// inbox of the parents (ADR 0126 point 1). Neither the mail nor the inbox is
// a formal delivery (Bekanntgabe) of the notice; nothing in moto calls it
// one.
//
// Invariants:
//   - One stored PDF per issued notice (uq_care_notice_letters_notice); the
//     row never changes. A change notice is a notice of its own with its own
//     PDF that names the notice it amends.
//   - The PDF of the download, the attachment of every mail and the inbox are
//     the same stored bytes.
//   - Every send is one outbox entry with a key per letter and send, stored
//     in the Träger transaction that records it under the lock of the run; a
//     send while the last mail still waits is refused, a series sends nothing
//     twice, and only the first send announces the letter in the inbox.
//   - A send goes only to the address the notice copied, and only while that
//     address is still the effective billing address of the contract's
//     family (a deviating payer included); a notice without an address is
//     printed, never mailed.
//   - The inbox shows a sent notice only to the account with the copied
//     address that holds parent_portal.enrollments.view on the child of the
//     notice at its school; membership alone opens nothing.

// MailRelatedCareNoticeRun is the related entity type of the mails of a
// notice run.
const MailRelatedCareNoticeRun = "parent_billing_care_notice_run"

// The mail kinds of the letters of a notice; they travel as
// MailKindBillingLetter like every letter of the Träger.
const (
	MailDocumentCareFeeNotice    = "care_fee_notice"
	MailDocumentCareChangeNotice = "care_change_notice"
)

// The kinds the parents inbox shows for a Jahresbescheid.
const (
	ParentDocumentKindCareFeeNotice    = "care_fee_notice"
	ParentDocumentKindCareChangeNotice = "care_change_notice"
)

// CareNoticeLetters are the stored originals of a released notice run and
// the state of every notice.
type CareNoticeLetters struct {
	Notices []CareNoticeMailing     `json:"notices"`
	Summary CareNoticeLetterSummary `json:"summary"`
}

// CareNoticeMailing is one issued notice with its stored PDF and the state of
// its last mail; Status is LetterMailNotSent before the first mail.
type CareNoticeMailing struct {
	NoticeID       int64  `json:"notice_id"`
	ContractID     int64  `json:"contract_id"`
	Kind           string `json:"kind"`
	Number         string `json:"number"`
	AmendsNumber   string `json:"amends_number,omitempty"`
	AmendedBy      string `json:"amended_by_number,omitempty"`
	ChildFirstName string `json:"child_first_name"`
	ChildLastName  string `json:"child_last_name"`
	SchoolName     string `json:"school_name"`
	// Recipient is the party the notice copied at its release.
	RecipientFirstName string `json:"recipient_first_name"`
	RecipientLastName  string `json:"recipient_last_name"`
	RecipientEmail     string `json:"recipient_email"`
	TotalCents         int64  `json:"total_cents"`
	HasLetter          bool   `json:"has_letter"`
	// SHA256 identifies the stored PDF; every download and every mail
	// carries these bytes.
	SHA256 string `json:"sha256,omitempty"`
	// InInbox tells whether a sent notice lands in the parents inbox too:
	// only when the notice names the child's record.
	InInbox       bool       `json:"in_inbox"`
	Status        string     `json:"status"`
	Sends         int        `json:"sends"`
	Attempts      int        `json:"attempts"`
	CreatedAt     *time.Time `json:"created_at"`
	RequestedAt   *time.Time `json:"requested_at"`
	SentAt        *time.Time `json:"sent_at"`
	MailOutcome   string     `json:"mail_outcome,omitempty"`
	ProviderBlock string     `json:"provider_block,omitempty"`
}

// CareNoticeLetterSummary counts the notices of a run by their letter and
// the state of the last mail, like CareLetterSummary.
type CareNoticeLetterSummary struct {
	Notices      int `json:"notices"`
	Letters      int `json:"letters"`
	Sent         int `json:"sent"`
	Queued       int `json:"queued"`
	Failed       int `json:"failed"`
	NotSent      int `json:"not_sent"`
	WithoutEmail int `json:"without_email"`
	WithoutInbox int `json:"without_inbox"`
}

// CareNoticeLetterRecord is the stored PDF of one notice; File is nil where
// the reader asked without it.
type CareNoticeLetterRecord struct {
	ID        int64
	NoticeID  int64
	SHA256    string
	CreatedAt time.Time
	File      *BillingLetterFile
}

// CareNoticeLetterSend is one stored send of the PDF of a notice.
type CareNoticeLetterSend struct {
	LetterID       int64
	DeliveryID     int64
	RecipientEmail string
	CreatedAt      time.Time
}

// CareNoticeLetterPost is Delivery as the notices of a run are mailed. The
// root binds it next to CareLetterPost; both join the caller's Träger
// transaction.
type CareNoticeLetterPost interface {
	// EnqueueCareNoticeLetterMail puts the mail of the notice on Delivery's
	// outbox with the notice run as its related entity.
	EnqueueCareNoticeLetterMail(ctx context.Context, mail BillingLetterMail) (int64, error)
	// CareNoticeLetterMailStates reads the mails of a notice run Delivery
	// still keeps.
	CareNoticeLetterMailStates(ctx context.Context, organizationID, runID int64) ([]BillingLetterMailState, error)
}

// CareNoticeLetterService lets the Abrechnung of a Träger store, send and
// print the originals of the Jahresbescheide of a released run. Every
// operation names the person of the session and runs in the Träger
// transaction of its Träger, which the caller opens; every read and change is
// recorded in the run's trail.
type CareNoticeLetterService interface {
	// CareNoticeLetters reads the state of every issued notice of the run.
	CareNoticeLetters(ctx context.Context, reviewer HolidayReviewer, runID int64) (CareNoticeLetters, error)
	// CreateCareNoticeLetters renders and stores the PDF of every issued
	// notice of the run that has none yet; a stored PDF never changes.
	CreateCareNoticeLetters(ctx context.Context, reviewer HolidayReviewer, runID int64) (CareNoticeLetters, error)
	// CareNoticePDF reads the stored PDF of the notice of a contract.
	CareNoticePDF(ctx context.Context, reviewer HolidayReviewer, runID, contractID int64) (BillingLetterFile, error)
	// MailCareNoticeLetters mails every stored notice with an address and no
	// mail yet.
	MailCareNoticeLetters(ctx context.Context, reviewer HolidayReviewer, runID int64) (BillingLetterSeries, error)
	// MailCareNoticeLetter mails the notice of a contract, first or again,
	// unless its last mail still waits.
	MailCareNoticeLetter(ctx context.Context, reviewer HolidayReviewer, runID, contractID int64) (CareNoticeMailing, error)
	// PrintCareNoticeLetters combines the stored notices of the run into one
	// PDF; CareLetterPrintWithoutEmail keeps only those without an address.
	PrintCareNoticeLetters(ctx context.Context, reviewer HolidayReviewer, runID int64, only string) (BillingLetterFile, error)
}
