package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	// ExportFiles and ExportRuns read limit entries after offset, newest
	// first, and count the whole list.
	ExportFiles(ctx context.Context, organizationID int64, runKind string, runID int64, offset, limit int) (parentbilling.ExportProtocolPage, error)
	ExportRuns(ctx context.Context, organizationID int64, offset, limit int) (parentbilling.ExportRunPage, error)
	ExportRunContent(ctx context.Context, organizationID int64, runKind string, runID int64) (parentbilling.ExportRunContent, error)
	// DatevRunBookings reads the documents of the runs with what the DATEV
	// selection needs, in one statement.
	DatevRunBookings(ctx context.Context, organizationID int64, runs []parentbilling.ExportRunRef) (map[parentbilling.ExportRunRef][]parentbilling.ExportInvoice, error)
	InvoiceHistory(ctx context.Context, organizationID int64, kind string, id int64) (string, []parentbilling.InvoiceHistoryEntry, []parentbilling.ExportProtocolEntry, error)
	// BookingAssignmentReader names where positions without their release's
	// values book (#4821).
	BookingAssignmentReader
	// InsertRunEvents records an action in the trail of holiday and monthly
	// runs, here the read of an invoice's history.
	InsertRunEvents(ctx context.Context, organizationID, accountID int64, action string, holiday, care map[int64]map[string]any) error
}

// FinanceExports serves parentbilling.FinanceExports.
type FinanceExports struct {
	store FinanceExportStore
	post  parentbilling.InvoiceHistoryPost
	datev *datevExport
}

var _ parentbilling.FinanceExports = (*FinanceExports)(nil)

// NewFinanceExports builds the finance exports over the DATEV settings of
// the Träger, which the DATEV booking batch (#4694) and its check read.
func NewFinanceExports(store FinanceExportStore, post parentbilling.InvoiceHistoryPost, datev parentbilling.DatevBillingSettings,
	today func() calendar.Date, now func() time.Time,
) *FinanceExports {
	if store == nil || post == nil {
		panic("parent billing: the export store and mail states are required")
	}
	return &FinanceExports{store: store, post: post, datev: newDatevExport(datev, today, now)}
}

// exportFile is a file about to leave moto: its content and the invoices.
type exportFile struct {
	format, runKind string
	runID           int64
	data            []byte
	totalCents      int64
	reference       string
	bookingContent  []byte
	careInvoices    []int64
	documents       []int64
	notices         []int64
	// documentCount is the number of rows of a Sollstellung, 0 to count
	// the links.
	documentCount int
	// debits are SEPA debits whose invoice or holiday document the file
	// collects.
	debits []int64
	// fileName writes the reserved number into the name of the file.
	fileName func(number int64) string
	// render renders the content with the reserved number, nil when the
	// content does not carry it.
	render func(number int64) ([]byte, error)
	// datevAll and datevBookings are what a DATEV file records (#6433).
	datevAll      bool
	datevBookings []parentbilling.DatevBookingRef
	// resolvedPositions counts the positions booked on the values of the
	// day of the export (#4821).
	resolvedPositions int
	// costCenterFallbacks are the holiday positions booked on the KOST1 of
	// the holiday care (#6624).
	costCenterFallbacks []parentbilling.CostCenterFallback
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
	entry, err := store.InsertExportFile(ctx, reviewer.OrganizationID, reviewer.AccountID, parentbilling.ExportFileRecord{
		Format: file.format, RunKind: file.runKind, RunID: file.runID, FileNumber: number, FileName: file.fileName(number),
		Content: data, TotalCents: file.totalCents, Reference: file.reference, BookingContent: file.bookingContent,
		CareInvoiceIDs: file.careInvoices, DocumentIDs: file.documents, CareNoticeIDs: file.notices, SEPADebitIDs: file.debits,
		DocumentCount: file.documentCount, DatevAll: file.datevAll, DatevBookings: file.datevBookings,
		ResolvedPositions: file.resolvedPositions, CostCenterFallbacks: file.costCenterFallbacks,
	})
	if err != nil {
		return parentbilling.ExportProtocolEntry{}, nil, fmt.Errorf("parent billing: record the export file: %w", err)
	}
	entry.ExportedByYou = true
	return entry, data, nil
}

// ExportSettings answers the mapping, the file numbers of the Träger and
// whether its DATEV settings make a valid batch.
func (f *FinanceExports) ExportSettings(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.ExportSettings, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportSettings{}, err
	}
	stored, updatedAt, err := f.store.ExportMapping(ctx, reviewer.OrganizationID, parentbilling.ExportFormatSollstellung)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	soll, err := decodeSollstellung(stored, updatedAt)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	numbers, err := f.store.ExportFileNumbers(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	datev, err := f.datev.DatevReadiness(ctx, reviewer)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	return parentbilling.ExportSettings{Sollstellung: soll, FileNumbers: numbers, Datev: datev}, nil
}

// sollstellung reads the stored mapping, the default without one.
func (f *FinanceExports) sollstellung(ctx context.Context, organizationID int64) (parentbilling.SollstellungSettings, error) {
	stored, updatedAt, err := f.store.ExportMapping(ctx, organizationID, parentbilling.ExportFormatSollstellung)
	if err != nil {
		return parentbilling.SollstellungSettings{}, err
	}
	return decodeSollstellung(stored, updatedAt)
}

// decodeSollstellung reads a stored mapping, the default without one.
func decodeSollstellung(stored json.RawMessage, updatedAt *time.Time) (parentbilling.SollstellungSettings, error) {
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
	numbers, err := f.store.ExportFileNumbers(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	datev, err := f.datev.DatevReadiness(ctx, reviewer)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	return parentbilling.ExportSettings{Sollstellung: parentbilling.SollstellungSettings{Mapping: parentbilling.DefaultSollstellungMapping()},
		FileNumbers: numbers, Datev: datev}, nil
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
	soll, err := f.sollstellung(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	numbers, err := f.store.ExportFileNumbers(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	datev, err := f.datev.DatevReadiness(ctx, reviewer)
	if err != nil {
		return parentbilling.ExportSettings{}, err
	}
	return parentbilling.ExportSettings{Sollstellung: soll, FileNumbers: numbers, Datev: datev}, nil
}

// ExportRuns reads one page of the released runs with their exports.
func (f *FinanceExports) ExportRuns(ctx context.Context, reviewer parentbilling.HolidayReviewer, page parentbilling.ExportPage,
) (parentbilling.ExportRunPage, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportRunPage{}, err
	}
	offset := max(page.Offset, 0)
	runs, err := f.store.ExportRuns(ctx, reviewer.OrganizationID, offset, parentbilling.ExportPageSize)
	if err != nil {
		return parentbilling.ExportRunPage{}, err
	}
	for i := range runs.Runs {
		for _, last := range []*parentbilling.ExportProtocolEntry{runs.Runs[i].LastSollstellung, runs.Runs[i].LastDatev} {
			if last != nil {
				last.ExportedByYou = byAccount(last.ExportedBy, reviewer.AccountID)
			}
		}
	}
	runs.NextOffset = nextOffset(offset, len(runs.Runs), runs.Total)
	return runs, nil
}

// DatevRunOverviews counts the new DATEV bookings of the runs of a page of
// the export page and names their fiscal years (#6433), by the rules of the
// file, once the DATEV settings make a valid file. Runs of other kinds and
// of other Träger are no part of the answer.
func (f *FinanceExports) DatevRunOverviews(ctx context.Context, reviewer parentbilling.HolidayReviewer, runs []parentbilling.ExportRunRef,
) (parentbilling.DatevRunOverviews, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.DatevRunOverviews{}, err
	}
	settings, readiness, err := f.datev.settingsAndReadiness(ctx, reviewer)
	if err != nil || !readiness.Ready {
		return parentbilling.DatevRunOverviews{Runs: []parentbilling.DatevRunOverview{}}, err
	}
	refs := make([]parentbilling.ExportRunRef, 0, len(runs))
	for _, run := range runs {
		if (run.RunKind == parentbilling.DebitRunCare || run.RunKind == parentbilling.DebitRunHoliday) && run.RunID > 0 &&
			!slices.Contains(refs, run) {
			refs = append(refs, run)
		}
	}
	overviews := parentbilling.DatevRunOverviews{Ready: true, Runs: []parentbilling.DatevRunOverview{}}
	if len(refs) == 0 {
		return overviews, nil
	}
	bookings, err := f.store.DatevRunBookings(ctx, reviewer.OrganizationID, refs)
	if err != nil {
		return parentbilling.DatevRunOverviews{}, err
	}
	for _, ref := range refs {
		if invoices, found := bookings[ref]; found {
			overviews.Runs = append(overviews.Runs, datevRunOverview(ref, invoices, settings))
		}
	}
	return overviews, nil
}

// ExportProtocol reads one page of the protocol, newest first; of one run
// when runKind is set.
func (f *FinanceExports) ExportProtocol(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
	page parentbilling.ExportPage,
) (parentbilling.ExportProtocolPage, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportProtocolPage{}, err
	}
	if runKind != "" && (validRunKind(runKind) != nil || runID <= 0) {
		return parentbilling.ExportProtocolPage{}, parentbilling.ErrExportRunNotFound
	}
	offset := max(page.Offset, 0)
	files, err := f.store.ExportFiles(ctx, reviewer.OrganizationID, runKind, runID, offset, parentbilling.ExportPageSize)
	if err != nil {
		return parentbilling.ExportProtocolPage{}, err
	}
	for i := range files.Files {
		files.Files[i].ExportedByYou = byAccount(files.Files[i].ExportedBy, reviewer.AccountID)
	}
	files.NextOffset = nextOffset(offset, len(files.Files), files.Total)
	return files, nil
}

// nextOffset is the offset of the page after one of read entries, nil when
// the list ends with it.
func nextOffset(offset, read, total int) *int {
	if read == 0 || offset+read >= total {
		return nil
	}
	next := offset + read
	return &next
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
	rows, resolved, err := f.sollstellungRows(ctx, reviewer, soll.Mapping, content.Invoices)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	file := exportFile{format: parentbilling.ExportFormatSollstellung, runKind: runKind, runID: runID, resolvedPositions: resolved,
		fileName: func(number int64) string { return sollFileName(soll.Mapping.FileNamePattern, number) },
		render: func(number int64) ([]byte, error) {
			return renderSollRows(soll.Mapping, rows, number)
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
	file.documentCount = len(content.Invoices)
	file.costCenterFallbacks = sollCostCenterFallbacks(soll.Mapping, rows)
	entry, data, err := recordExportFile(ctx, f.store, reviewer, file)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	entry.RunLabel = content.Run.Label
	return parentbilling.ExportedFile{Entry: entry, ContentType: sollContentType(soll.Mapping.Encoding), Data: data}, nil
}

// sollCostCenterFallbacks names the holiday positions on the KOST1 of the
// holiday care when the file writes the KOST1 (#6624).
func sollCostCenterFallbacks(mapping parentbilling.SollstellungMapping, rows []sollRow) []parentbilling.CostCenterFallback {
	if !slices.ContainsFunc(mapping.Columns, func(column parentbilling.SollstellungColumn) bool {
		return column.Field == parentbilling.SollFieldCostCenter
	}) {
		return nil
	}
	var fallbacks []parentbilling.CostCenterFallback
	for i := 0; i < len(rows); {
		invoice := rows[i].invoice
		var lines []bookingLine
		for ; i < len(rows) && rows[i].invoice.Number == invoice.Number; i++ {
			if rows[i].line != nil {
				lines = append(lines, *rows[i].line)
			}
		}
		fallbacks = append(fallbacks, costCenterFallbacks(invoice, lines)...)
	}
	return fallbacks
}

// sollstellungRows are the rows of the file. A mapping that names where a
// row books reads the revenue accounts and cost centers (#4821); a line
// without a revenue account refuses the file before a number is taken.
func (f *FinanceExports) sollstellungRows(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	mapping parentbilling.SollstellungMapping, invoices []parentbilling.ExportInvoice,
) ([]sollRow, int, error) {
	if !sollSplitsLines(mapping) {
		return invoiceSollRows(invoices), 0, nil
	}
	resolver, err := bookingResolverOf(ctx, f.store, f.datev.settings, reviewer.OrganizationID)
	if err != nil {
		return nil, 0, err
	}
	rows, resolved, missing := sollRows(mapping, invoices, resolver)
	if len(missing) > 0 {
		return nil, 0, &parentbilling.SollRevenueAccountError{DocumentNumbers: missing}
	}
	return rows, resolved, nil
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
	if err := f.recordHistoryRead(ctx, reviewer, kind, id, steps); err != nil {
		return parentbilling.InvoiceHistory{}, err
	}
	steps, err = f.invoiceMailHistory(ctx, reviewer.OrganizationID, kind, steps)
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

// recordHistoryRead records the read of an invoice's history in the trail of
// its run, as the family view records the read of its documents.
func (f *FinanceExports) recordHistoryRead(ctx context.Context, reviewer parentbilling.HolidayReviewer, kind string, id int64,
	steps []parentbilling.InvoiceHistoryEntry,
) error {
	runs := map[int64]map[string]any{}
	for _, step := range steps {
		if step.Kind == parentbilling.HistoryIssued && step.RunID > 0 {
			runs[step.RunID] = map[string]any{"view": "invoice_history", "document_ids": []int64{id}}
		}
	}
	if len(runs) == 0 {
		return fmt.Errorf("parent billing: the history of invoice %d names no run", id)
	}
	holiday, care := runs, map[int64]map[string]any(nil)
	if kind == parentbilling.HistoryCareInvoice {
		holiday, care = nil, runs
	}
	if err := f.store.InsertRunEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, runEventRead, holiday, care); err != nil {
		return fmt.Errorf("parent billing: record the read of the invoice history: %w", err)
	}
	return nil
}

func byAccount(account *int64, reviewer int64) bool {
	return account != nil && *account == reviewer
}

// A mailing records the request; only Delivery's successful send supplies
// a separate sent event and its time.
func (f *FinanceExports) invoiceMailHistory(ctx context.Context, organizationID int64, kind string, steps []parentbilling.InvoiceHistoryEntry) ([]parentbilling.InvoiceHistoryEntry, error) {
	states, err := f.mailStatesOfRuns(ctx, organizationID, kind, steps)
	if err != nil {
		return nil, err
	}
	sent := make(map[int64]time.Time, len(states))
	for _, state := range states {
		if state.Status == parentbilling.LetterMailSent && state.SentAt != nil {
			sent[state.DeliveryID] = *state.SentAt
		}
	}
	for _, step := range steps {
		if at, ok := sent[step.DeliveryID]; step.Kind == parentbilling.HistoryMailRequested && ok {
			steps = append(steps, parentbilling.InvoiceHistoryEntry{Kind: parentbilling.HistorySent, At: at, AccountID: step.AccountID, Note: step.Note})
		}
	}
	return steps, nil
}

// mailStatesOfRuns reads the mail states of every run a mailing of the
// history names: a replacement can be mailed in another run than the invoice.
func (f *FinanceExports) mailStatesOfRuns(ctx context.Context, organizationID int64, kind string, steps []parentbilling.InvoiceHistoryEntry,
) ([]parentbilling.BillingLetterMailState, error) {
	var runs []int64
	for _, step := range steps {
		if step.Kind == parentbilling.HistoryMailRequested && !slices.Contains(runs, step.RunID) {
			runs = append(runs, step.RunID)
		}
	}
	var states []parentbilling.BillingLetterMailState
	for _, runID := range runs {
		var read []parentbilling.BillingLetterMailState
		var err error
		if kind == parentbilling.HistoryCareInvoice {
			read, err = f.post.CareLetterMailStates(ctx, organizationID, runID)
		} else {
			read, err = f.post.LetterMailStates(ctx, organizationID, runID)
		}
		if err != nil {
			return nil, fmt.Errorf("parent billing: read invoice mail states: %w", err)
		}
		states = append(states, read...)
	}
	return states, nil
}
