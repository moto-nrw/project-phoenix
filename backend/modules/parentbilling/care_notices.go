package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Jahresbescheid of the monthly care contribution (#6219, ADR 0182,
// pattern Gemeinde Hardt MK-007). A notice run assesses the months of a
// calendar year from its first month to December for the schools it names:
// per contract one fee notice with a line per module and rule, the monthly
// amount and the proportional annual amount, and one installment per month
// due on its first day; a month already past on the notice date is due
// immediately. The months come from the monthly computation (ADR 0158,
// ADR 0164, ADR 0169), so the notice bills what the monthly run would bill.
//
// Invariants:
//   - No month is billed twice: a month a monthly invoice bills is not
//     assessed, and a month an effective notice assesses is billed by no
//     monthly run (it counts as billed there).
//   - An issued notice never changes, nor do its lines and installments. A
//     change of the contract is a change notice that names the notice it
//     amends, shows "bisher / neu / Zu- bzw. Abgang" and from then on is the
//     effective notice of its months; a notice is amended at most once.
//   - A contract has at most one first notice per year.
//   - The release refuses when count, total or fingerprint differ from the
//     review, or when the template of the fee notice has no legal remedy.

// Kinds of a fee notice.
const (
	// NoticeKindInitial assesses months nothing assessed before.
	NoticeKindInitial = "initial"
	// NoticeKindChange amends the effective notice of the contract.
	NoticeKindChange = "change"
)

// Kinds of a line of a fee notice.
const (
	NoticeLineModule    = "module"
	NoticeLineReduction = "reduction"
	NoticeLineCostShare = "cost_share"
)

var (
	// ErrCareNoticeRunNotFound reports a notice run that does not exist for
	// the Träger of the session.
	ErrCareNoticeRunNotFound = errors.New("fee notice run not found")
	// ErrCareNoticeRunOpen reports a second calculated notice run of a year.
	ErrCareNoticeRunOpen = errors.New("the year already has a fee notice run that is not released")
	// ErrCareNoticeNotFound reports a notice the run does not hold.
	ErrCareNoticeNotFound = errors.New("fee notice not found")
)

// CareNoticeRunStart starts a notice run: the year, the first month it
// assesses (the run ends with December), its schools (none takes every
// school of the Träger) and an optional notice date.
type CareNoticeRunStart struct {
	Year       int
	FromMonth  calendar.Date
	SchoolIDs  []int64
	NoticeDate *calendar.Date
}

// CareNoticeRun is a notice run as the list shows it. NoticeCount and
// TotalCents are set once it is released.
type CareNoticeRun struct {
	ID          int64          `json:"id"`
	Year        int            `json:"year"`
	PeriodFrom  calendar.Date  `json:"period_from"`
	PeriodUntil calendar.Date  `json:"period_until"`
	Status      string         `json:"status"`
	SchoolIDs   []int64        `json:"school_ids"`
	NoticeDate  *calendar.Date `json:"notice_date"`
	CreatedAt   time.Time      `json:"created_at"`
	ReleasedAt  *time.Time     `json:"released_at"`
	IssuedOn    *calendar.Date `json:"issued_on"`
	NoticeCount *int           `json:"notice_count"`
	TotalCents  *int64         `json:"total_cents"`
}

// CareNoticeRunList is the list of the notice runs, the latest first, with
// the year and month a new run would start with.
type CareNoticeRunList struct {
	Runs           []CareNoticeRun `json:"runs"`
	SuggestedYear  int             `json:"suggested_year"`
	SuggestedMonth calendar.Date   `json:"suggested_month"`
}

// CareNoticeLine is a line of a fee notice: a module, a reduction or a cost
// bearer share over a span of months with one monthly amount. Previous is
// what the amended notice assessed for the span, zero on a first notice;
// DeltaCents is the Zu- bzw. Abgang of the annual amount. Reductions and
// shares are negative: they lower what the family pays.
type CareNoticeLine struct {
	Kind                 string        `json:"kind"`
	Label                string        `json:"label"`
	Detail               string        `json:"detail"`
	CareOfferingID       *int64        `json:"care_offering_id,omitempty"`
	PeriodFrom           calendar.Date `json:"period_from"`
	PeriodUntil          calendar.Date `json:"period_until"`
	Months               int           `json:"months"`
	Quantity             int           `json:"quantity"`
	MonthlyPreviousCents int64         `json:"monthly_previous_cents"`
	MonthlyCents         int64         `json:"monthly_cents"`
	AnnualPreviousCents  int64         `json:"annual_previous_cents"`
	AnnualCents          int64         `json:"annual_cents"`
	DeltaCents           int64         `json:"delta_cents"`
}

// CareNoticeInstallment is the rate of one month: due on the first of the
// month, or on the notice date when that day is past (ImmediatelyDue).
type CareNoticeInstallment struct {
	Month          calendar.Date `json:"month"`
	DueOn          calendar.Date `json:"due_on"`
	ImmediatelyDue bool          `json:"immediately_due"`
	PreviousCents  int64         `json:"previous_cents"`
	AmountCents    int64         `json:"amount_cents"`
	DeltaCents     int64         `json:"delta_cents"`
}

// CareNoticeReference names another notice of the same contract.
type CareNoticeReference struct {
	ID       int64          `json:"id"`
	Number   string         `json:"number"`
	IssuedOn *calendar.Date `json:"issued_on"`
}

// CareNotice is the fee notice of one contract: a draft without a number in
// a calculated run, the issued notice in a released one. TotalCents is the
// annual amount the family pays, PreviousCents what the amended notice
// assessed, DeltaCents the difference.
type CareNotice struct {
	ID             int64          `json:"id"`
	Number         string         `json:"number"`
	IssuedOn       *calendar.Date `json:"issued_on"`
	Kind           string         `json:"kind"`
	ContractID     int64          `json:"contract_id"`
	SchoolID       int64          `json:"school_id"`
	SchoolName     string         `json:"school_name"`
	ChildFirstName string         `json:"child_first_name"`
	ChildLastName  string         `json:"child_last_name"`
	// StudentID is the child record the contract names, nil without one.
	StudentID *int64        `json:"-"`
	Recipient CareRecipient `json:"recipient"`
	// FamilyEmail is the family of the contract party, whoever pays
	// (#4689); the sibling rules and caps count by it.
	FamilyEmail   string                  `json:"-"`
	PaymentMethod string                  `json:"payment_method"`
	Year          int                     `json:"year"`
	PeriodFrom    calendar.Date           `json:"period_from"`
	PeriodUntil   calendar.Date           `json:"period_until"`
	Lines         []CareNoticeLine        `json:"lines"`
	Installments  []CareNoticeInstallment `json:"installments"`
	PreviousCents int64                   `json:"previous_cents"`
	TotalCents    int64                   `json:"total_cents"`
	DeltaCents    int64                   `json:"delta_cents"`
	Amends        *CareNoticeReference    `json:"amends"`
	AmendedBy     *CareNoticeReference    `json:"amended_by"`
	// LegalRemedy is the Rechtsbehelfsbelehrung the release copied from the
	// template of the fee notice (ADR 0170); a draft shows the current one.
	LegalRemedy string `json:"legal_remedy"`
	// Months are what the notice assesses per month, as the monthly
	// computation billed it; the monthly run and the rule chain read them.
	Months []CareInvoice `json:"-"`
}

// CareNoticeRunDetail is a notice run with its notices. A calculated run
// shows the preview of today; Unchanged counts the contracts whose
// effective notice still holds, Billed the contracts a monthly invoice
// bills in a month of the period, Unpriced and Duplicates the bookings the
// months leave out as in the monthly run.
type CareNoticeRunDetail struct {
	CareNoticeRun
	Notices            []CareNotice    `json:"notices"`
	Unchanged          int             `json:"unchanged"`
	Billed             int             `json:"billed"`
	Unpriced           []CareUnpriced  `json:"unpriced"`
	Duplicates         []CareDuplicate `json:"duplicates"`
	PreviewCount       int             `json:"preview_count"`
	PreviewCents       int64           `json:"preview_cents"`
	PreviewDeltaCents  int64           `json:"preview_delta_cents"`
	Fingerprint        string          `json:"fingerprint"`
	NumberPattern      string          `json:"number_pattern"`
	NumberRangeSet     bool            `json:"number_range_set"`
	LegalRemedyMissing bool            `json:"legal_remedy_missing"`
}

// CareNoticeLetter is a fee notice with the letterhead of the Träger, as
// page 1 of the Bescheid shows it.
type CareNoticeLetter struct {
	Notice       CareNotice `json:"notice"`
	SenderText   string     `json:"sender_text"`
	FooterText   string     `json:"footer_text"`
	ContactName  string     `json:"contact_name"`
	ContactPhone string     `json:"contact_phone"`
	ContactEmail string     `json:"contact_email"`
	AccentColor  string     `json:"accent_color"`
	// LogoDataURL is the logo of the Träger as a data URL, empty without.
	LogoDataURL string `json:"logo_data_url"`
	// BankAccount is the default bank account of the Träger, nil without;
	// a notice paid by transfer names it.
	BankAccount *CareNoticeBankAccount `json:"bank_account"`
}

// CareNoticeBankAccount is the bank account a notice names.
type CareNoticeBankAccount struct {
	AccountHolder string `json:"account_holder"`
	IBAN          string `json:"iban"`
	BIC           string `json:"bic"`
	BankName      string `json:"bank_name"`
}

// CareFeeNotices lets the Abrechnung of a Träger assess the months of a
// year in a fee notice per contract. Every operation names the person of
// the session and runs in the Träger transaction of its Träger; every read
// of personal data and every change of a run is recorded in its trail.
type CareFeeNotices interface {
	ListCareNoticeRuns(ctx context.Context, organizationID int64) (CareNoticeRunList, error)
	StartCareNoticeRun(ctx context.Context, reviewer HolidayReviewer, start CareNoticeRunStart) (CareNoticeRun, error)
	CareNoticeRun(ctx context.Context, reviewer HolidayReviewer, runID int64) (CareNoticeRunDetail, error)
	ReleaseCareNoticeRun(ctx context.Context, reviewer HolidayReviewer, runID int64, release CareRelease) (CareNoticeRunDetail, error)
	CareNoticeLetter(ctx context.Context, reviewer HolidayReviewer, runID, noticeID int64) (CareNoticeLetter, error)
	// EffectiveNoticeDues reads per contract the installment of the month
	// the effective notice sets: of the latest change notice, never of a
	// notice a later one amended. One entry per contract; the Träger's own
	// rows only. It names no person but the family key.
	EffectiveNoticeDues(ctx context.Context, organizationID int64, month calendar.Date) ([]CareNoticeDue, error)
}

// CareFeeNoticeHold is a contract the monthly run would bill in a month
// that an amended Jahresbescheid set to nothing (ADR 0182): the month stays
// assessed, so no monthly invoice bills it. A new notice of the year bills
// it again.
type CareFeeNoticeHold struct {
	ContractID     int64  `json:"contract_id"`
	SchoolID       int64  `json:"school_id"`
	ChildFirstName string `json:"child_first_name"`
	ChildLastName  string `json:"child_last_name"`
	NoticeNumber   string `json:"notice_number"`
}

// CareNoticeDue is the installment of a month that the effective notice of
// a contract sets (ADR 0182): the rate, its due date and the notice. The
// SEPA collection (#4697) reads it instead of resolving the change notices
// itself.
type CareNoticeDue struct {
	ContractID     int64         `json:"contract_id"`
	SchoolID       int64         `json:"school_id"`
	FamilyEmail    string        `json:"family_email"`
	NoticeID       int64         `json:"notice_id"`
	NoticeNumber   string        `json:"notice_number"`
	PaymentMethod  string        `json:"payment_method"`
	Month          calendar.Date `json:"month"`
	DueOn          calendar.Date `json:"due_on"`
	ImmediatelyDue bool          `json:"immediately_due"`
	AmountCents    int64         `json:"amount_cents"`
}
