package parentbilling

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The finance exports of the Elternabrechnung (#4693): the frame every file
// for the finance system of a Träger goes through. Each download of a file
// gets the next file number of the Träger (one counter across the formats
// of the Elternabrechnung, number_range_counters service "export"), carries
// it in its name and leaves one entry in the export protocol with the
// invoices it holds, in the transaction that answers the file. A rollback
// returns the number, so the numbers of the files that left moto have no gap
// and name one file each. A repeated download is a new file with a new
// number; the content hash, DATEV booking hash or stable PAIN.008 collection
// ID identifies an earlier export of the same content. The Sollstellung is
// the general CSV whose columns the Träger maps;
// the PAIN.008 file of the direct debits records itself here too (ADR 0181),
// and so does the DATEV booking batch of a run (#4694). Further formats
// (CIP/SIMBA #4698) add a format constant and record their file the same way.

// Formats of the export protocol.
const (
	ExportFormatSollstellung = "sollstellung"
	ExportFormatPain008      = "pain008"
	// ExportFormatDatev is the DATEV booking batch (EXTF Buchungsstapel).
	ExportFormatDatev = "datev"
)

// Fields a column of the Sollstellung can take.
const (
	SollFieldInvoiceNumber    = "invoice_number"
	SollFieldInvoiceDate      = "invoice_date"
	SollFieldDueOn            = "due_on"
	SollFieldAmount           = "amount"
	SollFieldDebtorNumber     = "debtor_number"
	SollFieldRecipientName    = "recipient_name"
	SollFieldRecipientStreet  = "recipient_street"
	SollFieldRecipientPostal  = "recipient_postal_code"
	SollFieldRecipientCity    = "recipient_city"
	SollFieldChildName        = "child_name"
	SollFieldSchoolName       = "school_name"
	SollFieldServiceFrom      = "service_from"
	SollFieldServiceUntil     = "service_until"
	SollFieldPaymentMethod    = "payment_method"
	SollFieldMandateReference = "mandate_reference"
	SollFieldDocumentKind     = "document_kind"
	SollFieldFileNumber       = "file_number"
	// SollFieldRevenueAccount, SollFieldCostCenter and SollFieldCostCenter2
	// are where a line books (#4821). A mapping with one of them writes one
	// row per revenue account and cost centers of a document, the same
	// lines as the DATEV file; without them one row per document.
	SollFieldRevenueAccount = "revenue_account"
	SollFieldCostCenter     = "cost_center"
	SollFieldCostCenter2    = "cost_center_2"
	// SollFieldConstant writes the column's fixed value into every row, for
	// example an account or a booking key the Kämmerei asks for.
	SollFieldConstant = "constant"
)

// SollstellungFields are the fields in the order the settings offer them.
var SollstellungFields = []string{
	SollFieldInvoiceNumber, SollFieldInvoiceDate, SollFieldDueOn, SollFieldAmount, SollFieldDebtorNumber,
	SollFieldRecipientName, SollFieldRecipientStreet, SollFieldRecipientPostal, SollFieldRecipientCity,
	SollFieldChildName, SollFieldSchoolName, SollFieldServiceFrom, SollFieldServiceUntil, SollFieldPaymentMethod,
	SollFieldMandateReference, SollFieldDocumentKind, SollFieldFileNumber,
	SollFieldRevenueAccount, SollFieldCostCenter, SollFieldCostCenter2, SollFieldConstant,
}

// SollBookingFields are the fields that split a row by where it books.
var SollBookingFields = []string{SollFieldRevenueAccount, SollFieldCostCenter, SollFieldCostCenter2}

// Options of the Sollstellung file.
const (
	SollDateGerman = "dd.mm.yyyy"
	SollDateISO    = "yyyy-mm-dd"
	// SollEncodingUTF8BOM is UTF-8 with a byte order mark, which Excel reads.
	SollEncodingUTF8BOM = "utf-8-bom"
	SollEncodingUTF8    = "utf-8"
	// SollEncodingWindows1252 is the code page older finance systems read.
	SollEncodingWindows1252 = "windows-1252"
	// SollFileNumberToken is where the file number goes in the file name.
	SollFileNumberToken = "{NR}"
)

// Limits of a mapping.
const (
	MaxSollColumns        = 40
	MaxSollHeaderLength   = 60
	MaxSollConstantLength = 60
	MaxSollFileNameLength = 120
)

// ExportFileNumberStartMax bounds the start value of the file numbers.
const ExportFileNumberStartMax = 999_999_999

var (
	// ErrExportRunNotFound reports a run of another Träger, none, or one that
	// is not released.
	ErrExportRunNotFound = errors.New("no released run to export")
	// ErrExportRunEmpty reports a released run without an invoice.
	ErrExportRunEmpty = errors.New("the run has no invoice to export")
	// ErrExportFileNumberStartLocked reports a start value after the first
	// file or after a start value was set: the numbers run on without a gap.
	ErrExportFileNumberStartLocked = errors.New("the file numbers of the Träger already started")
	// ErrExportInvoiceNotFound reports an invoice of another Träger or none.
	ErrExportInvoiceNotFound = errors.New("no invoice of the Träger")
	// ErrDatevNoNewBookings reports a standard DATEV download of a run whose
	// bookings are all in an earlier DATEV file (#6433): no file, no number.
	ErrDatevNoNewBookings = errors.New("every booking of the run is in a DATEV file")
	// ErrDatevBookingsTaken reports a standard DATEV download whose bookings
	// another download put into its file at the same time; it rolls back.
	ErrDatevBookingsTaken = errors.New("another DATEV download took the bookings")
)

// DatevFileOptions chooses what a DATEV file holds (#6433). The standard
// download holds the bookings no DATEV file of the Träger held yet. All
// repeats every booking of the run on purpose, for a file the tax office
// lost; the protocol marks it. FiscalYearStart picks the fiscal year of the
// file, the earliest one with bookings when zero: one DATEV file holds one
// fiscal year.
type DatevFileOptions struct {
	All             bool
	FiscalYearStart calendar.Date
}

// DatevNotReadyError refuses a DATEV file while the settings of the Träger
// make no valid batch; it names the keys, never their values.
type DatevNotReadyError struct {
	Readiness DatevReadiness
}

func (e *DatevNotReadyError) Error() string {
	return "the DATEV settings of the Träger are incomplete"
}

// DatevBookingProblem is a document of the run DATEV would refuse: its
// number, the field (DatevBookingAccount, DatevBookingDate, …) and why.
type DatevBookingProblem struct {
	DocumentNumber string `json:"document_number"`
	Field          string `json:"field"`
	Reason         string `json:"reason"`
}

// Fields of a DatevBookingProblem.
const (
	// DatevBookingAccount is the debtor number of the family.
	DatevBookingAccount = "debtor_number"
	// DatevBookingDate is the date of the document, outside the fiscal
	// year of the batch.
	DatevBookingDate = "document_date"
	// DatevBookingNumber is the number of the document.
	DatevBookingNumber = "document_number"
	// DatevBookingAmount is the amount of the document.
	DatevBookingAmount = "amount"
	// DatevBookingRevenueAccount is the revenue account of a line (#4821):
	// missing, or longer than the Sachkonten.
	DatevBookingRevenueAccount = "revenue_account"
	// DatevBookingCostCenter is the KOST1 of a line, missing while the
	// Träger requires one; DatevBookingCostCenter2 its KOST2.
	DatevBookingCostCenter  = "cost_center"
	DatevBookingCostCenter2 = "cost_center_2"
)

// Reasons of a DatevBookingProblem.
const (
	DatevProblemMissing     = "missing"
	DatevProblemInvalid     = "invalid"
	DatevProblemOutOfPeriod = "out_of_period"
)

// DatevBookingsError refuses a DATEV file with documents DATEV would
// refuse; no file number is taken.
type DatevBookingsError struct {
	Problems []DatevBookingProblem
}

func (e *DatevBookingsError) Error() string {
	return fmt.Sprintf("%d documents of the run make no valid DATEV booking", len(e.Problems))
}

// SollRevenueAccountError refuses a Sollstellung whose mapping writes the
// revenue account while a line of the documents has none (#4821): the
// standard of the DATEV settings is empty. No file number is taken.
type SollRevenueAccountError struct {
	DocumentNumbers []string
}

func (e *SollRevenueAccountError) Error() string {
	return fmt.Sprintf("%d documents of the run have no revenue account", len(e.DocumentNumbers))
}

// SollstellungColumn is one column of the file: the field it takes, its
// header, and the value of a constant column.
type SollstellungColumn struct {
	Field  string `json:"field"`
	Header string `json:"header"`
	Value  string `json:"value,omitempty"`
}

// SollstellungMapping is how the Träger wants its Sollstellung: the columns
// in order, the separators, the date format, the encoding, whether a header
// row comes first, and the file name with SollFileNumberToken.
type SollstellungMapping struct {
	Columns          []SollstellungColumn `json:"columns"`
	Delimiter        string               `json:"delimiter"`
	DecimalSeparator string               `json:"decimal_separator"`
	DateFormat       string               `json:"date_format"`
	Encoding         string               `json:"encoding"`
	HeaderRow        bool                 `json:"header_row"`
	FileNamePattern  string               `json:"file_name_pattern"`
}

// DefaultSollstellungMapping is the mapping of a Träger that set none: the
// fields most finance systems need, German dates and amounts, UTF-8 with a
// byte order mark and the file name of the SJR's export.
func DefaultSollstellungMapping() SollstellungMapping {
	return SollstellungMapping{
		Columns: []SollstellungColumn{
			{Field: SollFieldDebtorNumber, Header: "Debitorennummer"},
			{Field: SollFieldInvoiceNumber, Header: "Rechnungsnummer"},
			{Field: SollFieldInvoiceDate, Header: "Rechnungsdatum"},
			{Field: SollFieldDueOn, Header: "Fällig am"},
			{Field: SollFieldAmount, Header: "Betrag"},
			{Field: SollFieldRecipientName, Header: "Name"},
			{Field: SollFieldChildName, Header: "Kind"},
			{Field: SollFieldServiceFrom, Header: "Leistung von"},
			{Field: SollFieldServiceUntil, Header: "Leistung bis"},
			{Field: SollFieldPaymentMethod, Header: "Zahlungsweg"},
		},
		Delimiter: ";", DecimalSeparator: ",", DateFormat: SollDateGerman, Encoding: SollEncodingUTF8BOM,
		HeaderRow: true, FileNamePattern: "sollstellung-" + SollFileNumberToken + ".csv",
	}
}

// SollstellungSettings is the mapping of the Träger and whether it set one.
type SollstellungSettings struct {
	Mapping   SollstellungMapping `json:"mapping"`
	Custom    bool                `json:"custom"`
	UpdatedAt *time.Time          `json:"updated_at"`
}

// ExportFileNumbers is the state of the file numbers of a Träger: the
// number the next file gets and whether the start is fixed (a start value
// was set or a file was made).
type ExportFileNumbers struct {
	Next    int64 `json:"next"`
	Started bool  `json:"started"`
}

// ExportSettings is what the export settings page shows. Datev says
// whether the DATEV settings of the Träger make a valid batch.
type ExportSettings struct {
	Sollstellung SollstellungSettings `json:"sollstellung"`
	FileNumbers  ExportFileNumbers    `json:"file_numbers"`
	Datev        DatevReadiness       `json:"datev"`
}

// ExportProtocolEntry is one entry of the export protocol.
type ExportProtocolEntry struct {
	ID           int64  `json:"id"`
	FileNumber   int64  `json:"file_number"`
	Format       string `json:"format"`
	RunKind      string `json:"run_kind"`
	RunID        int64  `json:"run_id"`
	RunLabel     string `json:"run_label"`
	FileName     string `json:"file_name"`
	Fingerprint  string `json:"fingerprint"`
	InvoiceCount int    `json:"invoice_count"`
	TotalCents   int64  `json:"total_cents"`
	// Reference is the message ID of a PAIN.008 file.
	Reference string `json:"reference,omitempty"`
	// RepeatOf is the number of the earliest file of the run with the same
	// content, 0 when the content is new.
	RepeatOf int64 `json:"repeat_of,omitempty"`
	// DatevAll marks a DATEV file of "all bookings again" (#6433).
	DatevAll bool `json:"datev_all,omitempty"`
	// ResolvedPositions counts the positions of the file that had no
	// revenue account or cost center of their release: the file booked
	// them on the values of the day of the export (#4821).
	ResolvedPositions int `json:"resolved_positions,omitempty"`
	// CostCenterFallbacks are the holiday weeks of the file that booked on
	// the KOST1 of the holiday care (#6624).
	CostCenterFallbacks []CostCenterFallback `json:"cost_center_fallbacks"`
	// ExportedBy is the account that downloaded the file, nil after its
	// erasure; ExportedByYou marks the reader's own downloads.
	// ExportedByName names a colleague who downloaded it at this Träger
	// (#4693): empty for the reader's own downloads, for a person without a
	// name and after the erasure of the account.
	ExportedBy     *int64     `json:"exported_by_account_id"`
	ExportedByYou  bool       `json:"exported_by_you"`
	ExportedByName string     `json:"exported_by_name"`
	ExportedAt     time.Time  `json:"exported_at"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	// Handover is the handover to the recipient of the Träger (ADR 0218),
	// nil for a download.
	Handover *ExportFileHandover `json:"handover,omitempty"`
}

// ExportRun is a released run as the export page lists it: what it holds
// and its last export of each format.
//
// InvoiceCount and TotalCents count the documents a Sollstellung of the run
// writes now: its invoices with every later Storno and credit, the same
// rows ExportRunContent reads. A file keeps the figures of its own content.
type ExportRun struct {
	RunKind      string        `json:"run_kind"`
	RunID        int64         `json:"run_id"`
	Label        string        `json:"label"`
	IssuedOn     calendar.Date `json:"issued_on"`
	InvoiceCount int           `json:"invoice_count"`
	TotalCents   int64         `json:"total_cents"`
	// LastSollstellung and LastDatev are the newest Sollstellung and DATEV
	// batch of the run, nil before the first; Exports counts every file of
	// the run.
	LastSollstellung *ExportProtocolEntry `json:"last_sollstellung"`
	LastDatev        *ExportProtocolEntry `json:"last_datev"`
	Exports          int                  `json:"exports"`
}

// DatevRunOverview is what the export page shows of the DATEV bookings of a
// run (#6433): NewBookings counts the bookings the next standard DATEV files
// hold, across fiscal years; FiscalYears are the starts of the fiscal years
// the bookings of the run fall in, for the choice of "all again".
type DatevRunOverview struct {
	RunKind     string          `json:"run_kind"`
	RunID       int64           `json:"run_id"`
	NewBookings int             `json:"new_bookings"`
	FiscalYears []calendar.Date `json:"fiscal_years"`
}

// DatevRunOverviews answers the runs of a page of the export page; Runs is
// empty while the DATEV settings of the Träger make no valid file (Ready
// false), because no count could be right.
type DatevRunOverviews struct {
	Ready bool               `json:"ready"`
	Runs  []DatevRunOverview `json:"runs"`
}

// ExportPageSize is the number of runs or protocol entries one page of the
// export page holds. A longer list continues on the next page; nothing is
// cut off without a way to it.
const ExportPageSize = 50

// ExportPage names one page of a list, newest first: Offset entries of the
// list come before it.
type ExportPage struct {
	Offset int
}

// ExportRunPage is one page of the released runs. Total and NotExported
// count every run of the Träger, not only the page; NextOffset names the
// next page, nil on the last.
type ExportRunPage struct {
	Runs        []ExportRun `json:"runs"`
	Total       int         `json:"total"`
	NotExported int         `json:"not_exported"`
	NextOffset  *int        `json:"next_offset"`
}

// ExportProtocolPage is one page of the export protocol; Total counts every
// entry, NextOffset names the next page, nil on the last.
type ExportProtocolPage struct {
	Files      []ExportProtocolEntry `json:"files"`
	Total      int                   `json:"total"`
	NextOffset *int                  `json:"next_offset"`
}

// ExportInvoice is an issued invoice as the Sollstellung writes it.
type ExportInvoice struct {
	CareInvoiceID int64
	// CareCancellationID is the Storno of the monthly invoice CareInvoiceID
	// when the row is that Storno.
	CareCancellationID int64
	DocumentID         int64
	// ReferenceDocumentID is the holiday invoice a Storno or credit refers to.
	ReferenceDocumentID int64
	// DatevBooked says the booking is in a DATEV file of the Träger (#6433).
	DatevBooked      bool
	Number           string
	Kind             string
	IssuedOn         calendar.Date
	DueOn            *calendar.Date
	TotalCents       int64
	DebtorNumber     string
	RecipientName    string
	Street           string
	PostalCode       string
	City             string
	ChildName        string
	SchoolName       string
	ServiceFrom      calendar.Date
	ServiceUntil     calendar.Date
	PaymentMethod    string
	MandateReference string
	// Parts are the positions of the document with where each books
	// (#4821); a monthly Storno has the parts of its invoice.
	Parts []ExportBookingPart
	// CostShares are the issued funding amounts, before review corrections.
	CostShares []CareInvoiceCostShare
}

// Kinds of an ExportBookingPart.
const (
	BookingPartCare    = "care"
	BookingPartMeal    = "meal"
	BookingPartHoliday = "holiday"
)

// ExportBookingPart is one position of an exported document: its payable
// amount after position-specific reductions and where its release booked it.
// A position issued before #4821 has no frozen accounts; its kind, school,
// Angebot and day let the export resolve them.
type ExportBookingPart struct {
	Kind           string
	ChildID        int64 // The holiday child whose reductions apply to this part.
	SchoolID       int64
	CareOfferingID int64
	Day            calendar.Date
	AmountCents    int64
	Accounts       PositionAccounts
	// Removed is an applied position-removal correction of the issued invoice.
	Removed bool
	// CostCenterFallback, WeekStartsOn and WeekEndsOn name why a holiday
	// position books on the KOST1 of the holiday care (#6624).
	CostCenterFallback string
	WeekStartsOn       *calendar.Date
	WeekEndsOn         *calendar.Date
}

// CostCenterFallback is a week of a holiday document whose positions a file
// booked on the KOST1 of the holiday care instead of the host school's
// (#6624): the document, the week (nil for lines without one), the reason
// (CostCenterFallbackNoHostSchool and siblings) and the KOST1 it booked on,
// empty when the holiday care has none either.
type CostCenterFallback struct {
	Number       string         `json:"number"`
	WeekStartsOn *calendar.Date `json:"week_starts_on"`
	WeekEndsOn   *calendar.Date `json:"week_ends_on"`
	Reason       string         `json:"reason"`
	CostCenter   string         `json:"cost_center"`
}

// ExportRunContent is a released run with its invoices.
type ExportRunContent struct {
	Run      ExportRun
	Invoices []ExportInvoice
}

// ExportedFile is a file of a finance export as it leaves moto.
type ExportedFile struct {
	Entry       ExportProtocolEntry
	ContentType string
	Data        []byte
}

// ExportFileRecord is what an export writes into the protocol: the file and
// the invoices, holiday documents and Jahresbescheide it holds.
type ExportFileRecord struct {
	Format     string
	RunKind    string
	RunID      int64
	FileNumber int64
	FileName   string
	// DocumentCount is the number of documents (rows) the file holds when
	// it differs from its links: an invoice and its Storno share one link.
	DocumentCount int
	// Content is the file; the store keeps only its SHA-256 fingerprint.
	Content []byte
	// BookingContent is the content with fixed file metadata for repeat
	// detection: DATEV, and a Sollstellung without its file number (ADR
	// 0218); the store keeps its SHA-256 separately from the file hash.
	BookingContent []byte
	TotalCents     int64
	Reference      string
	CareInvoiceIDs []int64
	DocumentIDs    []int64
	CareNoticeIDs  []int64
	// SEPADebitIDs are debits of a PAIN.008 file; the store records the
	// invoice or holiday document each collects.
	SEPADebitIDs []int64
	// DatevAll marks a DATEV file of "all bookings again". DatevBookings are
	// the bookings of a DATEV file; the store records the first file of
	// each, and a standard file fails with ErrDatevBookingsTaken when a
	// booking is already recorded.
	DatevAll      bool
	DatevBookings []DatevBookingRef
	// ResolvedPositions counts the positions booked on the values of the
	// day of the export (#4821).
	ResolvedPositions int
	// CostCenterFallbacks are the holiday positions booked on the KOST1 of
	// the holiday care (#6624).
	CostCenterFallbacks []CostCenterFallback
}

// ExportRunRef names a run of the export page.
type ExportRunRef struct {
	RunKind string
	RunID   int64
}

// DatevBookingRef names one booking of a DATEV file: a monthly invoice, its
// Storno, or a holiday document.
type DatevBookingRef struct {
	CareInvoiceID      int64
	CareCancellationID int64
	DocumentID         int64
}

// Kinds of an invoice the history is asked for.
const (
	HistoryCareInvoice     = "care_invoice"
	HistoryHolidayDocument = "holiday_document"
)

// Entries of the history of an invoice.
const (
	HistoryIssued        = "issued"
	HistoryMailRequested = "mail_requested"
	HistorySent          = "sent"
	HistoryExported      = "exported"
	HistorySubmitted     = "submitted"
	HistoryCancelled     = "cancelled"
)

// InvoiceHistoryEntry is one step of an invoice: when, what, by whom and,
// for an export, the file. Positions counts the positions of the issue.
type InvoiceHistoryEntry struct {
	Kind      string    `json:"kind"`
	At        time.Time `json:"at"`
	AccountID *int64    `json:"account_id"`
	ByYou     bool      `json:"by_you"`
	// AccountName names the colleague of the step like ExportedByName
	// (#4693).
	AccountName string               `json:"account_name"`
	Positions   int                  `json:"positions,omitempty"`
	File        *ExportProtocolEntry `json:"file,omitempty"`
	// Note is the number of the cancellation, the message ID of a debit
	// or the recipient of a mail, as the step names it.
	Note string `json:"note,omitempty"`
	// RunID and DeliveryID resolve a mailing through Delivery without exposing internal references.
	RunID      int64 `json:"-"`
	DeliveryID int64 `json:"-"`
}

// InvoiceHistory is the history of one invoice, oldest first.
type InvoiceHistory struct {
	Kind    string                `json:"kind"`
	ID      int64                 `json:"id"`
	Number  string                `json:"number"`
	Entries []InvoiceHistoryEntry `json:"entries"`
}

// FinanceExports lets a person with the Träger right "Finanz-Export" set
// the field mapping and the start of the file numbers, list the released
// runs with their exports and the protocol, and download the Sollstellung or
// the DATEV booking batch of a run; with the right "Abrechnung" it reads the
// history of an invoice.
// Every operation runs in the Träger transaction of the reviewer's Träger,
// which the caller opens and rolls back when an operation fails.
type FinanceExports interface {
	ExportSettings(ctx context.Context, reviewer HolidayReviewer) (ExportSettings, error)
	SaveSollstellungMapping(ctx context.Context, reviewer HolidayReviewer, mapping SollstellungMapping) (ExportSettings, error)
	ResetSollstellungMapping(ctx context.Context, reviewer HolidayReviewer) (ExportSettings, error)
	SetExportFileNumberStart(ctx context.Context, reviewer HolidayReviewer, start int64) (ExportSettings, error)
	ExportRuns(ctx context.Context, reviewer HolidayReviewer, page ExportPage) (ExportRunPage, error)
	ExportProtocol(ctx context.Context, reviewer HolidayReviewer, runKind string, runID int64, page ExportPage) (ExportProtocolPage, error)
	SollstellungFile(ctx context.Context, reviewer HolidayReviewer, runKind string, runID int64) (ExportedFile, error)
	DatevFile(ctx context.Context, reviewer HolidayReviewer, runKind string, runID int64, options DatevFileOptions) (ExportedFile, error)
	DatevRunOverviews(ctx context.Context, reviewer HolidayReviewer, runs []ExportRunRef) (DatevRunOverviews, error)
	InvoiceHistory(ctx context.Context, reviewer HolidayReviewer, kind string, id int64) (InvoiceHistory, error)
}

// ExportRunLabel names a run as the export page and protocol show it: the
// month of a monthly run, the period of a holiday run, the month of the
// installments of the Jahresbescheide.
func ExportRunLabel(runKind string, careMonth, noticeMonth *calendar.Date, periodName string) string {
	monthOf := func(date *calendar.Date) string {
		if date == nil {
			return ""
		}
		return CareMonthName(*date)
	}
	switch runKind {
	case DebitRunCare:
		return "Monatsbeiträge " + monthOf(careMonth)
	case DebitRunHoliday:
		return "Ferien: " + periodName
	case DebitRunNotice:
		return "Bescheidraten " + monthOf(noticeMonth)
	}
	return runKind
}

// ExportActorNames names the people of the Träger-Büro who exported a file
// or acted on an invoice (#4693): the name each carries at the Träger, from
// memberships in any state, so a person who left still names their past
// steps. An account without a name is missing from the answer. It reads in
// the caller's Träger transaction, never the address or any other account
// data, and only for the Träger it is given.
type ExportActorNames interface {
	TraegerActorNames(ctx context.Context, organizationID int64, accountIDs []int64) (map[int64]string, error)
}

// InvoiceHistoryPost reads the existing Delivery states of a run in the caller's Träger transaction.
type InvoiceHistoryPost interface {
	LetterMailStates(ctx context.Context, organizationID, runID int64) ([]BillingLetterMailState, error)
	CareLetterMailStates(ctx context.Context, organizationID, runID int64) ([]BillingLetterMailState, error)
}
