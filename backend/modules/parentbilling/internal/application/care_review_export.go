package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
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
		return parentbilling.ExportFile{}, parentbilling.ErrBillingRunReleased
	}
	detail := parentbilling.CareRunDetail{CareRun: run}
	if err := c.reviewDetail(ctx, reviewer.OrganizationID, &detail); err != nil {
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
	rows := []map[string]string{}
	var total int64
	for _, invoice := range detail.Invoices {
		status := reviewStatusIssue
		if invoice.Exclusion != nil {
			status = reviewStatusExcluded + invoice.Exclusion.Reason
		} else {
			total += invoice.TotalCents
		}
		base := map[string]string{
			"invoice_date": invoiceDate, "school": invoice.SchoolName,
			"child":     strings.TrimSpace(invoice.ChildFirstName + " " + invoice.ChildLastName),
			"recipient": strings.TrimSpace(invoice.Recipient.FirstName + " " + invoice.Recipient.LastName),
			"email":     invoice.Recipient.Email, "status": status,
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
	}
	for _, unpriced := range detail.Unpriced {
		rows = append(rows, map[string]string{
			"invoice_date": invoiceDate, "school": unpriced.SchoolName,
			"child":    strings.TrimSpace(unpriced.ChildFirstName + " " + unpriced.ChildLastName),
			"position": unpriced.ModuleName, "weekdays": germanWeekdays[unpriced.Weekday], "status": reviewStatusUnpriced,
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

// reviewWeekdays writes the booked weekdays of a position: "Mo, Di".
func reviewWeekdays(weekdays []parentbilling.CareWeekdayPrice) string {
	names := make([]string, len(weekdays))
	for i, weekday := range weekdays {
		names[i] = germanWeekdays[weekday.Weekday]
	}
	return strings.Join(names, ", ")
}
