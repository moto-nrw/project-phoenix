package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The DATEV booking batch of a released run (#4694) through the frame of the
// finance exports (#4693). The file holds the documents of the run with an
// amount, the same documents a Sollstellung of the run writes, each with one
// line per revenue account and cost centers of its positions (#4821,
// booking_lines.go): an invoice charges the debtor of the family (Soll), its
// Storno and a credit relieve it (Haben), against the revenue account, KOST1
// and KOST2 its release copied onto the positions. The debtor is the number
// the release copied onto the invoice; the Belegfeld 1 is the document
// number.
//
// Invariants:
//  1. No file while the check of the settings names a setting
//     (DatevNotReadyError); the check is the one the settings page shows.
//  2. A document DATEV would refuse refuses the whole file and is named
//     (DatevBookingsError); nothing is left out silently.
//  3. Both refusals come before the file number is reserved, so a refused
//     download takes no number and leaves no protocol entry.
//  4. Every document of the file is linked to its protocol entry, so the
//     history of each invoice names the file.
//  5. The standard file holds each booking at most once across the DATEV
//     files of the Träger (#6433, datev_run_selection.go): it leaves out
//     what an earlier file held, and the store refuses a booking another
//     download recorded at the same time (ErrDatevBookingsTaken), so the
//     transaction rolls back with its number. "All again" repeats bookings
//     on purpose and the protocol marks the file.
//  6. A run without a new booking makes no file and takes no number
//     (ErrDatevNoNewBookings).
//  7. Every line has a valid revenue account (#4821); a missing or too long
//     one, and a missing KOST1 while the Träger requires one, refuse the
//     file like a refused debtor (invariant 2).

// datevContentType is the character set of the batch (Windows-1252).
const datevContentType = "text/csv; charset=windows-1252"

// DatevFile renders the DATEV booking batch of a released run, gives it the
// next file number of the Träger and records it.
func (f *FinanceExports) DatevFile(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
	options parentbilling.DatevFileOptions,
) (parentbilling.ExportedFile, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportedFile{}, err
	}
	if runKind != parentbilling.DebitRunCare && runKind != parentbilling.DebitRunHoliday || runID <= 0 {
		return parentbilling.ExportedFile{}, parentbilling.ErrExportRunNotFound
	}
	// The values the check passed: the settings of the Träger in the same
	// Träger transaction.
	settings, readiness, err := f.datev.settingsAndReadiness(ctx, reviewer)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	if !readiness.Ready {
		return parentbilling.ExportedFile{}, &parentbilling.DatevNotReadyError{Readiness: readiness}
	}
	content, err := f.store.ExportRunContent(ctx, reviewer.OrganizationID, runKind, runID)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	assignments, err := f.store.BookingAssignments(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	batch, err := datevRunBatch(content.Invoices, settings, options, assignments)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	header := batch.header(settings, runKind, f.datev.now())
	bookingContent, err := datevBookingsContent(header, batch, settings.CostCenterRequired)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	file := exportFile{format: parentbilling.ExportFormatDatev, runKind: runKind, runID: runID,
		totalCents: batch.totalCents, careInvoices: batch.careInvoices, documents: batch.documents,
		documentCount: len(batch.refs), bookingContent: bookingContent,
		datevAll: options.All, datevBookings: batch.refs, resolvedPositions: batch.resolved, costCenterFallbacks: batch.fallbacks,
		fileName: datevFileName,
		render: func(number int64) ([]byte, error) {
			numbered := header
			numbered.FileNumber, numbered.Label = int(number), datevBatchLabel(runKind, number)
			out, err := FormatDatevBookingBatch(numbered, batch.bookings)
			return out.Content, err
		}}
	entry, data, err := recordExportFile(ctx, f.store, reviewer, file)
	if err != nil {
		return parentbilling.ExportedFile{}, err
	}
	entry.RunLabel = content.Run.Label
	return parentbilling.ExportedFile{Entry: entry, ContentType: datevContentType, Data: data}, nil
}

// datevFileName is the name FormatDatevBookingBatch gives the file.
func datevFileName(number int64) string {
	return "EXTF_Buchungsstapel_" + strconv.FormatInt(number, 10) + ".csv"
}

// datevBatchLabel is the Bezeichnung of the batch (header field 17): at most
// 30 of [A-Za-z0-9_./ ], with the file number so the tax office finds the
// file of the protocol.
func datevBatchLabel(runKind string, number int64) string {
	if runKind == parentbilling.DebitRunHoliday {
		return "Ferien Datei " + strconv.FormatInt(number, 10)
	}
	return "Beitraege Datei " + strconv.FormatInt(number, 10)
}

// datevBatch is the bookings of a run, the documents behind them and the
// document dates.
type datevBatch struct {
	bookings []DatevBooking
	// numbers are the document numbers of the bookings, in their order.
	numbers []string
	// refs name the booking of each line for the record of what went to
	// DATEV.
	refs         []parentbilling.DatevBookingRef
	careInvoices []int64
	documents    []int64
	totalCents   int64
	first, last  calendar.Date
	// resolved counts the positions booked on the values of the day of the
	// export (#4821).
	resolved int
	// fallbacks are the holiday positions booked on the KOST1 of the
	// holiday care (#6624).
	fallbacks []parentbilling.CostCenterFallback
}

// datevRunBatch books the documents the selection of the options takes
// (datev_run_selection.go). A run without any booking is ErrExportRunEmpty;
// a standard file whose bookings all went to DATEV before is
// ErrDatevNoNewBookings, as is a chosen fiscal year without a booking.
func datevRunBatch(invoices []parentbilling.ExportInvoice, settings parentbilling.DatevSettings, options parentbilling.DatevFileOptions,
	assignments parentbilling.BookingAssignments,
) (datevBatch, error) {
	validDebtor, month := datevSelectionRules(settings)
	selection := selectDatevDocuments(invoices, validDebtor, month, options.All)
	if !selection.anyBooking {
		return datevBatch{}, parentbilling.ErrExportRunEmpty
	}
	chosen, _ := selection.inFiscalYear(options.FiscalYearStart, month)
	if len(chosen) == 0 {
		return datevBatch{}, parentbilling.ErrDatevNoNewBookings
	}
	resolver := newBookingResolver(assignments, strings.TrimSpace(settings.RevenueAccount.Value))
	batch := datevBatch{}
	for _, invoice := range chosen {
		batch.add(invoice, resolver)
	}
	return batch, nil
}

// add books one document, one line per where its positions book: a charge
// of the debtor, a negative line (Storno, credit) relieves it.
func (b *datevBatch) add(invoice parentbilling.ExportInvoice, resolver bookingResolver) {
	lines, resolved := resolver.documentLines(invoice)
	b.resolved += resolved
	b.fallbacks = append(b.fallbacks, costCenterFallbacks(invoice, lines)...)
	for _, line := range lines {
		booking := DatevBooking{AmountCents: line.amountCents, Side: DatevDebit, Account: invoice.DebtorNumber,
			CounterAccount: line.accounts.RevenueAccount, CostCenter1: line.accounts.CostCenter, CostCenter2: line.accounts.CostCenter2,
			DocumentDate: invoice.IssuedOn, DocumentNumber: invoice.Number, Text: datevBookingText(invoice)}
		if line.amountCents < 0 {
			booking.AmountCents, booking.Side = -line.amountCents, DatevCredit
		}
		b.bookings = append(b.bookings, booking)
		b.numbers = append(b.numbers, invoice.Number)
	}
	ref := parentbilling.DatevBookingRef{DocumentID: invoice.DocumentID, CareCancellationID: invoice.CareCancellationID}
	if invoice.CareCancellationID == 0 {
		ref.CareInvoiceID = invoice.CareInvoiceID
	}
	b.refs = append(b.refs, ref)
	b.totalCents += invoice.TotalCents
	if invoice.CareInvoiceID > 0 && !slices.Contains(b.careInvoices, invoice.CareInvoiceID) {
		b.careInvoices = append(b.careInvoices, invoice.CareInvoiceID)
	}
	if invoice.DocumentID > 0 {
		b.documents = append(b.documents, invoice.DocumentID)
	}
	if b.first.IsZero() || invoice.IssuedOn.Before(b.first) {
		b.first = invoice.IssuedOn
	}
	if b.last.IsZero() || invoice.IssuedOn.After(b.last) {
		b.last = invoice.IssuedOn
	}
}

// datevBookingText names the document and the child; datevPersonText
// brings it onto Windows-1252 and 60 characters.
func datevBookingText(invoice parentbilling.ExportInvoice) string {
	kind := "Beitrag"
	switch invoice.Kind {
	case "cancellation":
		kind = "Storno"
	case "credit":
		kind = "Gutschrift"
	}
	text := kind + " " + invoice.ServiceFrom.Format("01/2006")
	if invoice.ChildName != "" {
		text += " " + invoice.ChildName
	}
	return text
}

// header is the header of the batch: the settings, which the check found
// valid, and the period. The period runs from the first day of the month of
// the earliest document to the last day of the month of the latest one,
// inside the fiscal year of the earliest: DATEV takes the year of every
// document date from the fiscal year, so a document of a later fiscal year
// is named, not booked into the wrong year.
func (b datevBatch) header(settings parentbilling.DatevSettings, runKind string, now time.Time) DatevBatchHeader {
	number := func(setting parentbilling.DatevSetting) int {
		value, _ := strconv.Atoi(setting.Value) // the check accepted every number
		return value
	}
	start, end := datevFiscalYear(b.first, time.Month(number(settings.FiscalYearStart)))
	to := calendar.NewDate(b.last.Year(), b.last.Month()+1, 1).AddDays(-1)
	if to.After(end) {
		to = end
	}
	return DatevBatchHeader{
		ConsultantNumber: number(settings.ConsultantNumber), ClientNumber: number(settings.ClientNumber),
		AccountLength: number(settings.AccountLength), ChartOfAccounts: settings.ChartOfAccounts.Value,
		FiscalYearStart: start, From: calendar.NewDate(b.first.Year(), b.first.Month(), 1), To: to,
		Label: datevBatchLabel(runKind, 1), FileNumber: 1, CreatedAt: now, Locked: settings.Locked,
	}
}

// addDatevProblem names a problem of a document once, however many of its
// lines have it.
func addDatevProblem(refused *parentbilling.DatevBookingsError, number, field, reason string) {
	problem := parentbilling.DatevBookingProblem{DocumentNumber: number, Field: field, Reason: reason}
	if !slices.Contains(refused.Problems, problem) {
		refused.Problems = append(refused.Problems, problem)
	}
}

// datevBookingsContent validates before a number is taken and renders the
// bookings with their accounting settings. File metadata is fixed for the
// comparison hash; the store also hashes the actual downloaded file.
func datevBookingsContent(header DatevBatchHeader, batch datevBatch, costCenterRequired bool) ([]byte, error) {
	header.CreatedAt = time.Date(2000, time.January, 1, 0, 0, 0, 0, calendar.Berlin)
	header.FileNumber, header.Label = 1, ""
	refused := &parentbilling.DatevBookingsError{}
	if costCenterRequired {
		for i, booking := range batch.bookings {
			if booking.CostCenter1 == "" {
				addDatevProblem(refused, batch.numbers[i], parentbilling.DatevBookingCostCenter, parentbilling.DatevProblemMissing)
			}
		}
	}
	out, err := FormatDatevBookingBatch(header, batch.bookings)
	if err == nil && len(refused.Problems) == 0 {
		return out.Content, nil
	}
	var formatErr *DatevFormatError
	if err != nil && !errors.As(err, &formatErr) {
		return nil, err
	}
	if formatErr == nil {
		return nil, refused
	}
	fields := map[string]string{
		DatevFieldAccount: parentbilling.DatevBookingAccount, DatevFieldDocumentDate: parentbilling.DatevBookingDate,
		DatevFieldDocumentNumber: parentbilling.DatevBookingNumber, DatevFieldAmount: parentbilling.DatevBookingAmount,
		DatevFieldCounterAccount: parentbilling.DatevBookingRevenueAccount, DatevFieldCostCenter1: parentbilling.DatevBookingCostCenter,
		DatevFieldCostCenter2: parentbilling.DatevBookingCostCenter2,
	}
	reasons := map[string]string{
		DatevMissing: parentbilling.DatevProblemMissing, DatevInvalid: parentbilling.DatevProblemInvalid,
		DatevOutOfPeriod: parentbilling.DatevProblemOutOfPeriod,
	}
	for _, problem := range formatErr.Problems {
		field, known := fields[problem.Field]
		if problem.Line < 1 || problem.Line > len(batch.numbers) || !known {
			return nil, fmt.Errorf("parent billing: the DATEV batch of a ready Träger: %w", err)
		}
		addDatevProblem(refused, batch.numbers[problem.Line-1], field, reasons[problem.Reason])
	}
	return nil, refused
}
