package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The finance exports of a Träger (#4693): settings, runs, protocol,
// Sollstellung and the history of an invoice. Every file leaves moto
// through recordExportFile, which takes the next file number of the Träger
// and writes the protocol entry in the caller's Träger transaction.

// ExportFileStore records the files of the finance exports. Every method
// runs in the Träger transaction of organizationID.
type ExportFileStore interface {
	// ReserveExportFileNumber takes the next file number and locks the
	// counter of the Träger until the transaction ends.
	ReserveExportFileNumber(ctx context.Context, organizationID int64) (int64, error)
	// InsertExportFile records a file and the invoices it holds.
	InsertExportFile(ctx context.Context, organizationID, accountID int64, record parentbilling.ExportFileRecord) (parentbilling.ExportProtocolEntry, error)
	// MarkExportFilesSubmitted notes the confirmed submission of a PAIN.008
	// file on its protocol entries.
	MarkExportFilesSubmitted(ctx context.Context, organizationID, accountID int64, runKind string, runID int64, messageID string) error
}

// FinanceExportStore persists the settings, runs and protocol.
type FinanceExportStore interface {
	ExportFileStore
	ExportFileNumbers(ctx context.Context, organizationID int64) (parentbilling.ExportFileNumbers, error)
	StartExportFileNumbers(ctx context.Context, organizationID, start int64) error
	ExportMapping(ctx context.Context, organizationID int64, format string) (json.RawMessage, *time.Time, error)
	SaveExportMapping(ctx context.Context, organizationID, accountID int64, format string, settings json.RawMessage) error
	DeleteExportMapping(ctx context.Context, organizationID int64, format string) error
	ExportFiles(ctx context.Context, organizationID int64, runKind string, runID int64) ([]parentbilling.ExportProtocolEntry, error)
	ExportRuns(ctx context.Context, organizationID int64) ([]parentbilling.ExportRun, error)
	ExportRunContent(ctx context.Context, organizationID int64, runKind string, runID int64) (parentbilling.ExportRunContent, error)
	InvoiceHistory(ctx context.Context, organizationID int64, kind string, id int64) (string, []parentbilling.InvoiceHistoryEntry, []parentbilling.ExportProtocolEntry, error)
}

// FinanceExports serves parentbilling.FinanceExports.
type FinanceExports struct {
	store FinanceExportStore
}

var _ parentbilling.FinanceExports = (*FinanceExports)(nil)

// NewFinanceExports builds the finance exports.
func NewFinanceExports(store FinanceExportStore) *FinanceExports {
	if store == nil {
		panic("parent billing: the export store is required")
	}
	return &FinanceExports{store: store}
}

// exportFile is a file about to leave moto: its content and the invoices.
type exportFile struct {
	format, runKind string
	runID           int64
	data            []byte
	totalCents      int64
	reference       string
	careInvoices    []int64
	documents       []int64
	notices         []int64
	// debits are SEPA debits whose invoice or holiday document the file
	// collects.
	debits []int64
	// fileName writes the reserved number into the name of the file.
	fileName func(number int64) string
	// render renders the content with the reserved number, nil when the
	// content does not carry it.
	render func(number int64) ([]byte, error)
}

// recordExportFile takes the next file number of the Träger, renders the
// file with it when needed and records it with its fingerprint. The caller
// answers the file only after this returns, in the same transaction.
func recordExportFile(ctx context.Context, store ExportFileStore, reviewer parentbilling.HolidayReviewer, file exportFile,
) (parentbilling.ExportProtocolEntry, []byte, error) {
	number, err := store.ReserveExportFileNumber(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportProtocolEntry{}, nil, fmt.Errorf("parent billing: reserve the export file number: %w", err)
	}
	data := file.data
	if file.render != nil {
		if data, err = file.render(number); err != nil {
			return parentbilling.ExportProtocolEntry{}, nil, err
		}
	}
	sum := sha256.Sum256(data)
	entry, err := store.InsertExportFile(ctx, reviewer.OrganizationID, reviewer.AccountID, parentbilling.ExportFileRecord{
		Format: file.format, RunKind: file.runKind, RunID: file.runID, FileNumber: number, FileName: file.fileName(number),
		Fingerprint: hex.EncodeToString(sum[:]), TotalCents: file.totalCents, Reference: file.reference,
		CareInvoiceIDs: file.careInvoices, DocumentIDs: file.documents, CareNoticeIDs: file.notices, SEPADebitIDs: file.debits,
	})
	if err != nil {
		return parentbilling.ExportProtocolEntry{}, nil, fmt.Errorf("parent billing: record the export file: %w", err)
	}
	entry.ExportedByYou = true
	return entry, data, nil
}

// ExportSettings answers the mapping and the file numbers of the Träger.
func (f *FinanceExports) ExportSettings(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.ExportSettings, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportSettings{}, err
	}
	soll, err := f.sollstellung(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	numbers, err := f.store.ExportFileNumbers(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	return parentbilling.ExportSettings{Sollstellung: soll, FileNumbers: numbers}, nil
}

// sollstellung reads the stored mapping, the default without one.
func (f *FinanceExports) sollstellung(ctx context.Context, organizationID int64) (parentbilling.SollstellungSettings, error) {
	stored, updatedAt, err := f.store.ExportMapping(ctx, organizationID, parentbilling.ExportFormatSollstellung)
	if err != nil {
		return parentbilling.SollstellungSettings{}, err
	}
	if stored == nil {
		return parentbilling.SollstellungSettings{Mapping: parentbilling.DefaultSollstellungMapping()}, nil
	}
	var mapping parentbilling.SollstellungMapping
	if err := json.Unmarshal(stored, &mapping); err != nil {
		return parentbilling.SollstellungSettings{}, fmt.Errorf("parent billing: read the Sollstellung mapping: %w", err)
	}
	return parentbilling.SollstellungSettings{Mapping: mapping, Custom: true, UpdatedAt: updatedAt}, nil
}

// SaveSollstellungMapping checks and stores the mapping of the Träger.
func (f *FinanceExports) SaveSollstellungMapping(ctx context.Context, reviewer parentbilling.HolidayReviewer, mapping parentbilling.SollstellungMapping,
) (parentbilling.ExportSettings, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportSettings{}, err
	}
	mapping, err := normalizeSollMapping(mapping)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	settings, err := json.Marshal(mapping)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	if err := f.store.SaveExportMapping(ctx, reviewer.OrganizationID, reviewer.AccountID, parentbilling.ExportFormatSollstellung, settings); err != nil {
		return parentbilling.ExportSettings{}, err
	}
	return f.ExportSettings(ctx, reviewer)
}

// ResetSollstellungMapping returns the Träger to the default mapping.
func (f *FinanceExports) ResetSollstellungMapping(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.ExportSettings, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportSettings{}, err
	}
	if err := f.store.DeleteExportMapping(ctx, reviewer.OrganizationID, parentbilling.ExportFormatSollstellung); err != nil {
		return parentbilling.ExportSettings{}, err
	}
	return f.ExportSettings(ctx, reviewer)
}

// SetExportFileNumberStart sets the number of the first file once, so the
// numbers continue those of the Träger's former system.
func (f *FinanceExports) SetExportFileNumberStart(ctx context.Context, reviewer parentbilling.HolidayReviewer, start int64,
) (parentbilling.ExportSettings, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportSettings{}, err
	}
	if start < 1 || start > parentbilling.ExportFileNumberStartMax {
		return parentbilling.ExportSettings{}, validation.Errors{"start": errors.New("must be between 1 and 999999999")}
	}
	if err := f.store.StartExportFileNumbers(ctx, reviewer.OrganizationID, start); err != nil {
		return parentbilling.ExportSettings{}, err
	}
	return f.ExportSettings(ctx, reviewer)
}

// ExportRuns lists the released runs with their exports.
func (f *FinanceExports) ExportRuns(ctx context.Context, reviewer parentbilling.HolidayReviewer) ([]parentbilling.ExportRun, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	runs, err := f.store.ExportRuns(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		if last := runs[i].LastSollstellung; last != nil {
			last.ExportedByYou = byAccount(last.ExportedBy, reviewer.AccountID)
		}
	}
	return runs, nil
}

// ExportProtocol lists the protocol, newest first; of one run when
// runKind is set.
func (f *FinanceExports) ExportProtocol(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
) ([]parentbilling.ExportProtocolEntry, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	if runKind != "" && (validRunKind(runKind) != nil || runID <= 0) {
		return nil, parentbilling.ErrExportRunNotFound
	}
	entries, err := f.store.ExportFiles(ctx, reviewer.OrganizationID, runKind, runID)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		entries[i].ExportedByYou = byAccount(entries[i].ExportedBy, reviewer.AccountID)
	}
	return entries, nil
}

// SollstellungFile renders the Sollstellung of a released run with the
// mapping of the Träger, gives it the next file number and records it.
func (f *FinanceExports) SollstellungFile(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
) (parentbilling.ExportedFile, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportedFile{}, err
	}
	if runKind != parentbilling.DebitRunCare && runKind != parentbilling.DebitRunHoliday || runID <= 0 {
		return parentbilling.ExportedFile{}, parentbilling.ErrExportRunNotFound
	}
	soll, err := f.sollstellung(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	content, err := f.store.ExportRunContent(ctx, reviewer.OrganizationID, runKind, runID)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	if len(content.Invoices) == 0 {
		return parentbilling.ExportedFile{}, parentbilling.ErrExportRunEmpty
	}
	file := exportFile{format: parentbilling.ExportFormatSollstellung, runKind: runKind, runID: runID,
		fileName: func(number int64) string { return sollFileName(soll.Mapping.FileNamePattern, number) },
		render: func(number int64) ([]byte, error) {
			return renderSollstellung(soll.Mapping, content.Invoices, number)
		}}
	for _, invoice := range content.Invoices {
		file.totalCents += invoice.TotalCents
		if invoice.CareInvoiceID > 0 && !slices.Contains(file.careInvoices, invoice.CareInvoiceID) {
			file.careInvoices = append(file.careInvoices, invoice.CareInvoiceID)
		}
		if invoice.DocumentID > 0 {
			file.documents = append(file.documents, invoice.DocumentID)
		}
	}
	entry, data, err := recordExportFile(ctx, f.store, reviewer, file)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	entry.RunLabel = content.Run.Label
	return parentbilling.ExportedFile{Entry: entry, ContentType: sollContentType(soll.Mapping.Encoding), Data: data}, nil
}

func sollContentType(encoding string) string {
	if encoding == parentbilling.SollEncodingWindows1252 {
		return "text/csv; charset=windows-1252"
	}
	return "text/csv; charset=utf-8"
}

// InvoiceHistory answers the steps of an invoice, oldest first.
func (f *FinanceExports) InvoiceHistory(ctx context.Context, reviewer parentbilling.HolidayReviewer, kind string, id int64,
) (parentbilling.InvoiceHistory, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.InvoiceHistory{}, err
	}
	if id <= 0 {
		return parentbilling.InvoiceHistory{}, parentbilling.ErrExportInvoiceNotFound
	}
	number, steps, files, err := f.store.InvoiceHistory(ctx, reviewer.OrganizationID, kind, id)
	if err != nil {
		return parentbilling.InvoiceHistory{}, err
	}
	for i := range files {
		file := files[i]
		file.ExportedByYou = byAccount(file.ExportedBy, reviewer.AccountID)
		steps = append(steps, parentbilling.InvoiceHistoryEntry{Kind: parentbilling.HistoryExported, At: file.ExportedAt,
			AccountID: file.ExportedBy, File: &file})
	}
	for i := range steps {
		steps[i].ByYou = byAccount(steps[i].AccountID, reviewer.AccountID)
	}
	slices.SortStableFunc(steps, func(a, b parentbilling.InvoiceHistoryEntry) int { return a.At.Compare(b.At) })
	return parentbilling.InvoiceHistory{Kind: kind, ID: id, Number: number, Entries: steps}, nil
}

func byAccount(account *int64, reviewer int64) bool {
	return account != nil && *account == reviewer
}
