package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The DATEV booking batch of a released run (#4694) through the frame of the
// finance exports (#4693). The file holds one booking per document of the
// run with an amount, the same documents a Sollstellung of the run writes:
// an invoice charges the debtor of the family (Soll), its Storno and a
// credit relieve it (Haben), each against the revenue account of the
// settings. The debtor is the number the release copied onto the invoice;
// the Belegfeld 1 is the document number.
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

// datevContentType is the character set of the batch (Windows-1252).
const datevContentType = "text/csv; charset=windows-1252"

// DatevFile renders the DATEV booking batch of a released run, gives it the
// next file number of the Träger and records it.
func (f *FinanceExports) DatevFile(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
) (parentbilling.ExportedFile, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportedFile{}, err
	}
	if runKind != parentbilling.DebitRunCare && runKind != parentbilling.DebitRunHoliday || runID <= 0 {
		return parentbilling.ExportedFile{}, parentbilling.ErrExportRunNotFound
	}
	settings, err := f.datev.settings.DatevSettings(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.ExportedFile{}, fmt.Errorf("parent billing: read the DATEV settings: %w", err)
	}
	readiness, err := f.datev.readinessOf(settings)
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
	batch := datevRunBatch(content.Invoices, settings)
	if len(batch.bookings) == 0 {
		return parentbilling.ExportedFile{}, parentbilling.ErrExportRunEmpty
	}
	header := batch.header(settings, runKind, f.datev.now())
	if err := datevBookingsProblems(header, batch); err != nil {
		return parentbilling.ExportedFile{}, err
	}
	file := exportFile{format: parentbilling.ExportFormatDatev, runKind: runKind, runID: runID,
		totalCents: batch.totalCents, careInvoices: batch.careInvoices, documents: batch.documents,
		documentCount: len(batch.bookings),
		fileName:      datevFileName,
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
	numbers      []string
	careInvoices []int64
	documents    []int64
	totalCents   int64
	first, last  calendar.Date
}

// datevRunBatch books every document with an amount; a document of 0 €
// (a full exemption) moves no money and DATEV refuses an amount of 0.
func datevRunBatch(invoices []parentbilling.ExportInvoice, settings parentbilling.DatevSettings) datevBatch {
	batch := datevBatch{}
	for _, invoice := range invoices {
		if invoice.TotalCents == 0 {
			continue
		}
		booking := DatevBooking{AmountCents: invoice.TotalCents, Side: DatevDebit, Account: invoice.DebtorNumber,
			CounterAccount: settings.RevenueAccount.Value, DocumentDate: invoice.IssuedOn, DocumentNumber: invoice.Number,
			Text: datevBookingText(invoice)}
		if invoice.TotalCents < 0 {
			booking.AmountCents, booking.Side = -invoice.TotalCents, DatevCredit
		}
		batch.bookings = append(batch.bookings, booking)
		batch.numbers = append(batch.numbers, invoice.Number)
		batch.totalCents += invoice.TotalCents
		if invoice.CareInvoiceID > 0 && !slices.Contains(batch.careInvoices, invoice.CareInvoiceID) {
			batch.careInvoices = append(batch.careInvoices, invoice.CareInvoiceID)
		}
		if invoice.DocumentID > 0 {
			batch.documents = append(batch.documents, invoice.DocumentID)
		}
		if batch.first.IsZero() || invoice.IssuedOn.Before(batch.first) {
			batch.first = invoice.IssuedOn
		}
		if batch.last.IsZero() || invoice.IssuedOn.After(batch.last) {
			batch.last = invoice.IssuedOn
		}
	}
	return batch
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

// datevBookingsProblems formats the batch once before a number is taken and
// names every document DATEV would refuse. A problem of the header follows
// from no setting the check passed and is a fault.
func datevBookingsProblems(header DatevBatchHeader, batch datevBatch) error {
	_, err := FormatDatevBookingBatch(header, batch.bookings)
	var formatErr *DatevFormatError
	if err == nil || !errors.As(err, &formatErr) {
		return err
	}
	fields := map[string]string{
		DatevFieldAccount: parentbilling.DatevBookingAccount, DatevFieldDocumentDate: parentbilling.DatevBookingDate,
		DatevFieldDocumentNumber: parentbilling.DatevBookingNumber, DatevFieldAmount: parentbilling.DatevBookingAmount,
	}
	reasons := map[string]string{
		DatevMissing: parentbilling.DatevProblemMissing, DatevInvalid: parentbilling.DatevProblemInvalid,
		DatevOutOfPeriod: parentbilling.DatevProblemOutOfPeriod,
	}
	refused := &parentbilling.DatevBookingsError{}
	for _, problem := range formatErr.Problems {
		field, known := fields[problem.Field]
		if problem.Line < 1 || problem.Line > len(batch.numbers) || !known {
			return fmt.Errorf("parent billing: the DATEV batch of a ready Träger: %w", err)
		}
		refused.Problems = append(refused.Problems, parentbilling.DatevBookingProblem{
			DocumentNumber: batch.numbers[problem.Line-1], Field: field, Reason: reasons[problem.Reason]})
	}
	return refused
}
