package application

import (
	"slices"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Which documents of a run a DATEV file books (#6433). One rule serves the
// download and the count of new bookings on the export page, so the page
// never announces a booking the download leaves out.
//
//  1. A document of 0 € is no booking: it moves no money and DATEV refuses
//     an amount of 0.
//  2. An invoice whose copied debtor number DATEV refuses, cancelled by a
//     Storno, is left out together with that Storno while neither was in a
//     DATEV file: the two cancel each other and never reached the books. The
//     correction (Storno, right number at the family, reissue in a later
//     run) frees the run instead of locking it for good; the issued
//     documents stay as they are.
//  3. The standard file leaves out every booking an earlier DATEV file of
//     the Träger held; "all again" keeps them.
//  4. One file holds one fiscal year (DATEV takes the year of every
//     document date from the fiscal year of the header). The file takes the
//     chosen fiscal year, else the earliest; a booking of a later fiscal
//     year waits for the next file.

// datevSelection is what a DATEV file of a run books.
type datevSelection struct {
	// bookable are the documents of rules 1 to 3, in their order.
	bookable []parentbilling.ExportInvoice
	// fiscalYears are the starts of the fiscal years of every booking of
	// the run (rules 1 and 2, booked or not), in order.
	fiscalYears []calendar.Date
	// anyBooking says the run has a booking at all (rules 1 and 2).
	anyBooking bool
}

// selectDatevDocuments applies rules 1 to 3. validDebtor is the debtor rule
// of the Träger's account length; fiscalYearMonth starts its fiscal year.
func selectDatevDocuments(invoices []parentbilling.ExportInvoice, validDebtor func(string) bool, fiscalYearMonth time.Month, all bool,
) datevSelection {
	settled := datevSettledPairs(invoices, validDebtor)
	selection := datevSelection{}
	for i, invoice := range invoices {
		if invoice.TotalCents == 0 || settled[i] {
			continue
		}
		selection.anyBooking = true
		start, _ := datevFiscalYear(invoice.IssuedOn, fiscalYearMonth)
		if !slices.Contains(selection.fiscalYears, start) {
			selection.fiscalYears = append(selection.fiscalYears, start)
		}
		if all || !invoice.DatevBooked {
			selection.bookable = append(selection.bookable, invoice)
		}
	}
	slices.SortFunc(selection.fiscalYears, func(a, b calendar.Date) int { return a.Compare(b) })
	return selection
}

// inFiscalYear applies rule 4: the bookings of the fiscal year starting on
// start, or of the earliest one when start is zero, with that start.
func (s datevSelection) inFiscalYear(start calendar.Date, fiscalYearMonth time.Month) ([]parentbilling.ExportInvoice, calendar.Date) {
	if start.IsZero() {
		for _, invoice := range s.bookable {
			if year, _ := datevFiscalYear(invoice.IssuedOn, fiscalYearMonth); start.IsZero() || year.Before(start) {
				start = year
			}
		}
	}
	var chosen []parentbilling.ExportInvoice
	for _, invoice := range s.bookable {
		if year, _ := datevFiscalYear(invoice.IssuedOn, fiscalYearMonth); year == start {
			chosen = append(chosen, invoice)
		}
	}
	return chosen, start
}

// datevSettledPairs marks rule 2: the invoice with a debtor DATEV refuses
// and its Storno, while neither is in a DATEV file. A monthly Storno shares
// the invoice's CareInvoiceID; a holiday Storno refers to its invoice.
func datevSettledPairs(invoices []parentbilling.ExportInvoice, validDebtor func(string) bool) map[int]bool {
	original := func(cancellation parentbilling.ExportInvoice) int {
		for i, invoice := range invoices {
			if invoice.Kind != "invoice" {
				continue
			}
			if cancellation.CareCancellationID > 0 && invoice.CareInvoiceID == cancellation.CareInvoiceID ||
				cancellation.ReferenceDocumentID > 0 && invoice.DocumentID == cancellation.ReferenceDocumentID {
				return i
			}
		}
		return -1
	}
	settled := map[int]bool{}
	for i, cancellation := range invoices {
		if cancellation.Kind != "cancellation" || cancellation.DatevBooked {
			continue
		}
		o := original(cancellation)
		if o < 0 || invoices[o].DatevBooked || validDebtor(invoices[o].DebtorNumber) {
			continue
		}
		settled[i], settled[o] = true, true
	}
	return settled
}

// datevSelectionRules are the parts of the DATEV settings the selection
// needs: the debtor rule and the first month of the fiscal year. The check
// of the settings accepted both numbers when the file is made.
func datevSelectionRules(settings parentbilling.DatevSettings) (func(string) bool, time.Month) {
	length, _ := strconv.Atoi(settings.AccountLength.Value)
	month, err := strconv.Atoi(settings.FiscalYearStart.Value)
	if err != nil || month < 1 || month > 12 {
		month = 1
	}
	return datevDebtorAccount(length), time.Month(month)
}

// datevRunOverview counts the new bookings of a run and names the fiscal
// years of its bookings, by the same rules as the file.
func datevRunOverview(run parentbilling.ExportRunRef, invoices []parentbilling.ExportInvoice, settings parentbilling.DatevSettings,
) parentbilling.DatevRunOverview {
	validDebtor, month := datevSelectionRules(settings)
	selection := selectDatevDocuments(invoices, validDebtor, month, false)
	years := selection.fiscalYears
	if years == nil {
		years = []calendar.Date{}
	}
	return parentbilling.DatevRunOverview{RunKind: run.RunKind, RunID: run.RunID, NewBookings: len(selection.bookable), FiscalYears: years}
}
