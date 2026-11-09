package parentbilling

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The finance exports of the Elternabrechnung (#4693): the frame every file
// for the finance system of a Träger goes through. Each download of a file
// gets the next file number of the Träger (one counter across the formats
// of the Elternabrechnung, number_range_counters service "export"), carries
// it in its name and leaves one entry in the export protocol with the
// invoices it holds, in the transaction that answers the file. A rollback
// returns the number, so the numbers of the files that left moto have no gap
// and name one file each. A repeated download is a new file with a new
// number; the content hash or, for PAIN.008, its stable collection ID
// identifies an earlier export of the same content. The Sollstellung is the general CSV whose columns the Träger maps;
// the PAIN.008 file of the direct debits records itself here too (ADR 0181).
// Further formats (DATEV #4694, CIP/SIMBA #4698) add a format constant and
// record their file through FinanceExportLog.

// Formats of the export protocol.
const (
	ExportFormatSollstellung = "sollstellung"
	ExportFormatPain008      = "pain008"
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
	// SollFieldConstant writes the column's fixed value into every row, for
	// example an account or a booking key the Kämmerei asks for.
	SollFieldConstant = "constant"
)

// SollstellungFields are the fields in the order the settings offer them.
var SollstellungFields = []string{
	SollFieldInvoiceNumber, SollFieldInvoiceDate, SollFieldDueOn, SollFieldAmount, SollFieldDebtorNumber,
	SollFieldRecipientName, SollFieldRecipientStreet, SollFieldRecipientPostal, SollFieldRecipientCity,
	SollFieldChildName, SollFieldSchoolName, SollFieldServiceFrom, SollFieldServiceUntil, SollFieldPaymentMethod,
	SollFieldMandateReference, SollFieldDocumentKind, SollFieldFileNumber, SollFieldConstant,
}

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
)

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

// ExportSettings is what the export settings page shows.
type ExportSettings struct {
	Sollstellung SollstellungSettings `json:"sollstellung"`
	FileNumbers  ExportFileNumbers    `json:"file_numbers"`
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
	// ExportedBy is the account that downloaded the file, nil after its
	// erasure; ExportedByYou marks the reader's own downloads.
	ExportedBy    *int64     `json:"exported_by_account_id"`
	ExportedByYou bool       `json:"exported_by_you"`
	ExportedAt    time.Time  `json:"exported_at"`
	SubmittedAt   *time.Time `json:"submitted_at"`
}

// ExportRun is a released run as the export page lists it: what it holds
// and its last export of each format.
type ExportRun struct {
	RunKind      string        `json:"run_kind"`
	RunID        int64         `json:"run_id"`
	Label        string        `json:"label"`
	IssuedOn     calendar.Date `json:"issued_on"`
	InvoiceCount int           `json:"invoice_count"`
	TotalCents   int64         `json:"total_cents"`
	// LastSollstellung is the newest Sollstellung of the run, nil before
	// the first; Exports counts every file of the run.
	LastSollstellung *ExportProtocolEntry `json:"last_sollstellung"`
	Exports          int                  `json:"exports"`
}

// ExportInvoice is an issued invoice as the Sollstellung writes it.
type ExportInvoice struct {
	CareInvoiceID    int64
	DocumentID       int64
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
	Content        []byte
	TotalCents     int64
	Reference      string
	CareInvoiceIDs []int64
	DocumentIDs    []int64
	CareNoticeIDs  []int64
	// SEPADebitIDs are debits of a PAIN.008 file; the store records the
	// invoice or holiday document each collects.
	SEPADebitIDs []int64
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
	Kind      string               `json:"kind"`
	At        time.Time            `json:"at"`
	AccountID *int64               `json:"account_id"`
	ByYou     bool                 `json:"by_you"`
	Positions int                  `json:"positions,omitempty"`
	File      *ExportProtocolEntry `json:"file,omitempty"`
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
// runs with their exports and the protocol, and download the Sollstellung of
// a run; with the right "Abrechnung" it reads the history of an invoice.
// Every operation runs in the Träger transaction of the reviewer's Träger,
// which the caller opens and rolls back when an operation fails.
type FinanceExports interface {
	ExportSettings(ctx context.Context, reviewer HolidayReviewer) (ExportSettings, error)
	SaveSollstellungMapping(ctx context.Context, reviewer HolidayReviewer, mapping SollstellungMapping) (ExportSettings, error)
	ResetSollstellungMapping(ctx context.Context, reviewer HolidayReviewer) (ExportSettings, error)
	SetExportFileNumberStart(ctx context.Context, reviewer HolidayReviewer, start int64) (ExportSettings, error)
	ExportRuns(ctx context.Context, reviewer HolidayReviewer) ([]ExportRun, error)
	ExportProtocol(ctx context.Context, reviewer HolidayReviewer, runKind string, runID int64) ([]ExportProtocolEntry, error)
	SollstellungFile(ctx context.Context, reviewer HolidayReviewer, runKind string, runID int64) (ExportedFile, error)
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

// InvoiceHistoryPost reads the existing Delivery states of a run in the caller's Träger transaction.
type InvoiceHistoryPost interface {
	LetterMailStates(ctx context.Context, organizationID, runID int64) ([]BillingLetterMailState, error)
	CareLetterMailStates(ctx context.Context, organizationID, runID int64) ([]BillingLetterMailState, error)
}
