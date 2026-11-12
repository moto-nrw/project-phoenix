package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The review list of a calculated monthly run (#5916, SJR "Rechnungs-
// prüfungsexport", columns of E4 for now): one row per position of every
// invoice of the preview, the reductions of the rule chain as rows of
// their own, excluded invoices marked and outside the sum. It needs no
// issued invoice and gives no number.

// careReviewColumns are the columns of the review list.
var careReviewColumns = []ExportColumn{
	{ID: "invoice_date", Label: "Rechnungsdatum"},
	{ID: "school", Label: "Schule"},
	{ID: "child", Label: "Kind"},
	{ID: "recipient", Label: "Empfänger"},
	{ID: "email", Label: "E-Mail"},
	{ID: "payment_method", Label: "Zahlungsweg"},
	{ID: "due_on", Label: "Fällig am"},
	{ID: "position", Label: "Position"},
	{ID: "weekdays", Label: "Wochentage"},
	{ID: "period", Label: "Zeitraum"},
	{ID: "amount", Label: "Betrag in Euro"},
	{ID: "status", Label: "Status"},
}

// Status values of the review list.
const (
	reviewStatusIssue    = "Wird ausgestellt"
	reviewStatusExcluded = "Ausgeschlossen: "
	reviewStatusUnpriced = "Nicht abgerechnet: Preis fehlt"
	reviewStatusDouble   = "Nicht abgerechnet: doppelter Vertrag"
)

// ExportCareRunReview renders the review list of a calculated run as an
// Excel or a CSV file and records the download with the contracts it names.
func (c *CareBilling) ExportCareRunReview(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, format string) (parentbilling.ExportFile, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportFile{}, err
	}
	if format != parentbilling.ExportXLSX && format != parentbilling.ExportCSV {
		return parentbilling.ExportFile{}, validation.Errors{"format": errors.New("must be xlsx or csv")}
	}
	if runID <= 0 {
		return parentbilling.ExportFile{}, parentbilling.ErrCareRunNotFound
	}
	run, err := c.store.CareRun(ctx, reviewer.OrganizationID, runID, false)
	if err != nil {
		return parentbilling.ExportFile{}, err
	}
	if run.Status != parentbilling.RunCalculated {
		return parentbilling.ExportFile{}, closedRunError(run.Status)
	}
	detail := parentbilling.CareRunDetail{CareRun: run}
	if err := c.reviewDetail(ctx, reviewer, &detail); err != nil {
		return parentbilling.ExportFile{}, err
	}
	file, err := c.renderer.RenderTable(careReviewTable(detail), format, "pruefliste-monatsbeitrag-"+run.BillingMonth.Format("2006-01"))
	if err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: render the review list: %w", err)
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventExport,
		map[string]any{"format": format, "contract_ids": shownContracts(detail)}); err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: record the review list: %w", err)
	}
	return file, nil
}

// careReviewTable lays out the review list of a run.
func careReviewTable(detail parentbilling.CareRunDetail) ExportTable {
	invoiceDate := "Tag der Ausstellung"
	if detail.InvoiceDate != nil {
		invoiceDate = detail.InvoiceDate.Format("02.01.2006")
	}
	rows, total := reviewInvoiceRows(detail.Invoices, invoiceDate)
	rows = append(rows, reviewHeldRows(detail, invoiceDate)...)
	rows = append(rows, map[string]string{"position": "Summe, die ausgestellt wird", "amount": formatAmount(total)})
	if detail.ExcludedCount > 0 {
		rows = append(rows, map[string]string{
			"position": "Ausgeschlossen, nicht in der Summe (" + invoiceCount(detail.ExcludedCount) + ")",
			"amount":   formatAmount(detail.ExcludedCents),
		})
	}
	schools := make([]string, len(detail.Schools))
	for i, school := range detail.Schools {
		schools[i] = school.Name
	}
	return ExportTable{
		Title: "Prüfliste Monatsbeitrag", Subtitle: parentbilling.CareMonthName(detail.BillingMonth),
		Lines: []string{
			"Rechnungsdatum: " + invoiceDate,
			"Schulen: " + strings.Join(schools, ", "),
			"Vorschau ohne Rechnungsnummern. Die Nummern gibt erst die Ausstellung.",
		},
		Columns: careReviewColumns, Rows: rows,
	}
}

// reviewInvoiceRows writes one row per position and reduction of every
// invoice of the preview and sums the invoices the release issues.
func reviewInvoiceRows(invoices []parentbilling.CareInvoice, invoiceDate string) ([]map[string]string, int64) {
	rows := []map[string]string{}
	var total int64
	for _, invoice := range invoices {
		status := reviewStatusIssue
		if invoice.Exclusion != nil {
			status = reviewStatusExcluded + invoice.Exclusion.Reason
		} else {
			total += invoice.TotalCents
		}
		recipient := strings.TrimSpace(invoice.Recipient.FirstName + " " + invoice.Recipient.LastName)
		if second := strings.TrimSpace(invoice.Recipient.SecondName); second != "" {
			recipient += " und " + second
		}
		base := map[string]string{
			"invoice_date": invoiceDate, "school": invoice.SchoolName,
			"child":     strings.TrimSpace(invoice.ChildFirstName + " " + invoice.ChildLastName),
			"recipient": recipient, "email": invoice.Recipient.Email, "status": status,
		}
		// The payment the release copies, from the one rule (#4689).
		if payment := invoice.Payment; payment != nil {
			base["payment_method"], base["due_on"] = paymentMethodLabel(payment.Method), payment.DueOn.Format("02.01.2006")
		}
		for _, position := range invoice.Positions {
			rows = append(rows, withValues(base, map[string]string{
				"position": position.ModuleName, "weekdays": reviewWeekdays(position.Weekdays),
				"period": position.ServiceFrom.Format("02.01.") + " bis " + position.ServiceUntil.Format("02.01.2006"),
				"amount": formatAmount(position.AmountCents),
			}))
		}
		for _, reduction := range invoice.Reductions {
			rows = append(rows, withValues(base, map[string]string{
				"position": reduction.Name, "amount": formatAmount(reduction.AmountCents),
			}))
		}
		rows = append(rows, reviewMealAndShareRows(base, invoice)...)
		rows = append(rows, reviewCorrectionRows(base, invoice)...)
	}
	return rows, total
}

// reviewMealAndShareRows writes the Mensa months (#4686), so the rows add up
// to the invoice, and the cost bearer shares that come off what the family
// pays (#4687).
func reviewMealAndShareRows(base map[string]string, invoice parentbilling.CareInvoice) []map[string]string {
	rows := []map[string]string{}
	for _, meal := range invoice.MealPositions {
		rows = append(rows, withValues(base, map[string]string{
			"position": mealPositionLabel(meal), "amount": formatAmount(meal.AmountCents),
		}))
	}
	for _, share := range invoice.CostShares {
		rows = append(rows, withValues(base, map[string]string{
			"position": "Übernimmt " + share.CostBearerName + " (" + costShareText(share) + ")",
			"amount":   formatAmount(-share.AmountCents),
		}))
	}
	return rows
}

// reviewHeldRows writes the bookings the run does not bill: without a price
// or with a duplicate contract.
func reviewHeldRows(detail parentbilling.CareRunDetail, invoiceDate string) []map[string]string {
	rows := []map[string]string{}
	for _, unpriced := range detail.Unpriced {
		module := unpriced.ModuleName
		if unpriced.DaysPerWeek != nil {
			module += ", " + daysPerWeekLabel(*unpriced.DaysPerWeek)
		}
		rows = append(rows, map[string]string{
			"invoice_date": invoiceDate, "school": unpriced.SchoolName,
			"child":    strings.TrimSpace(unpriced.ChildFirstName + " " + unpriced.ChildLastName),
			"position": module, "weekdays": germanWeekdays[unpriced.Weekday], "status": reviewStatusUnpriced,
		})
	}
	for _, duplicate := range detail.Duplicates {
		schools := []string{}
		for _, contract := range duplicate.Contracts {
			schools = append(schools, contract.SchoolName)
		}
		rows = append(rows, map[string]string{
			"invoice_date": invoiceDate, "school": strings.Join(schools, ", "),
			"child":    strings.TrimSpace(duplicate.ChildFirstName + " " + duplicate.ChildLastName),
			"position": duplicate.ModuleName, "weekdays": germanWeekdays[duplicate.Weekday], "status": reviewStatusDouble,
		})
	}
	return rows
}

// reviewWeekdays writes the booked weekdays of a position: "Mo, Di".
func reviewWeekdays(weekdays []parentbilling.CareWeekdayPrice) string {
	names := make([]string, len(weekdays))
	for i, weekday := range weekdays {
		names[i] = germanWeekdays[weekday.Weekday]
	}
	return strings.Join(names, ", ")
}
