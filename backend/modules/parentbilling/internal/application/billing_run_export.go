package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// runEventExport records the download of the overview of a run.
const runEventExport = "export"

// ExportColumn is one column of an exported table.
type ExportColumn struct {
	ID    string
	Label string
}

// ExportTable is a list the Elternabrechnung exports: a title, the lines
// under it, the columns and one map of values per row.
type ExportTable struct {
	Title    string
	Subtitle string
	Lines    []string
	Columns  []ExportColumn
	Rows     []map[string]string
	Footer   string
}

// TableRenderer renders an exported table as a PDF, an Excel or a CSV
// file. The adapter binds it to Document Rendering's list renderer; it
// neither stores the file nor decides who may read it.
type TableRenderer interface {
	RenderTable(table ExportTable, format, filenameBase string) (parentbilling.ExportFile, error)
}

// ExportBillingRun renders the overview of a released run for the Kämmerei
// (#4672, ADR 0122): every issued invoice with its positions per family, the
// sum of every invoice and of the run. It records the download in the run's
// trail; the caller rolls back when that fails.
func (b *BillingRuns) ExportBillingRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID int64, format string) (parentbilling.ExportFile, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportFile{}, err
	}
	switch format {
	case parentbilling.ExportPDF, parentbilling.ExportXLSX, parentbilling.ExportCSV:
	default:
		return parentbilling.ExportFile{}, validation.Errors{"format": errors.New("must be pdf, xlsx or csv")}
	}
	if runID <= 0 {
		return parentbilling.ExportFile{}, parentbilling.ErrBillingRunNotFound
	}
	run, err := b.store.BillingRun(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.ExportFile{}, err
	}
	if run.Status != parentbilling.RunReleased {
		return parentbilling.ExportFile{}, parentbilling.ErrBillingRunNotReleased
	}
	documents, err := b.store.RunDocuments(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: read invoices: %w", err)
	}
	file, err := b.renderer.RenderTable(billingRunOverview(run, documents), format, "abrechnungsanordnung-"+run.PeriodName)
	if err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: render billing run overview: %w", err)
	}
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventExport,
		map[string]any{"format": format, "document_count": len(documents)}); err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: record billing run export: %w", err)
	}
	return file, nil
}

// billingRunOverviewColumns are the columns of the overview: one row per
// position, then the sum of the invoice; the last row is the sum of the run.
var billingRunOverviewColumns = []ExportColumn{
	{ID: "number", Label: "Rechnungsnummer"},
	{ID: "recipient", Label: "Empfänger"},
	{ID: "child", Label: "Kind"},
	{ID: "position", Label: "Position"},
	{ID: "amount", Label: "Betrag in Euro"},
}

// billingRunOverview is the general layout of the overview of a released
// run. The recipient's mail address stays out; the Kämmerei needs who pays
// what, not how to reach them.
func billingRunOverview(run parentbilling.BillingRun, documents []parentbilling.BillingDocument) ExportTable {
	rows := []map[string]string{}
	var total int64
	for _, document := range documents {
		recipient := strings.TrimSpace(document.RecipientLastName + ", " + document.RecipientFirstName)
		for _, child := range document.Children {
			name := strings.TrimSpace(child.FirstName + " " + child.LastName)
			for _, position := range child.Positions {
				rows = append(rows, map[string]string{
					"number": document.Number, "recipient": recipient, "child": name,
					"position": overviewPositionLabel(position), "amount": formatAmount(position.AmountCents),
				})
			}
		}
		rows = append(rows, map[string]string{
			"number": document.Number, "recipient": recipient, "position": "Summe der Rechnung",
			"amount": formatAmount(document.TotalCents),
		})
		total += document.TotalCents
	}
	rows = append(rows, map[string]string{"position": "Gesamtsumme des Laufs", "amount": formatAmount(total)})
	return ExportTable{
		Title: "Abrechnungsanordnung", Subtitle: run.PeriodName,
		Lines: overviewLines(run, documents, total), Columns: billingRunOverviewColumns, Rows: rows,
		Footer: "Vertraulich. Enthält personenbezogene Daten.",
	}
}

// overviewLines are the facts of the run under the title.
func overviewLines(run parentbilling.BillingRun, documents []parentbilling.BillingDocument, total int64) []string {
	lines := []string{"Leistungszeitraum: " + run.ServiceFrom.Format("02.01.2006") + " bis " + run.ServiceUntil.Format("02.01.2006")}
	if run.IssuedOn != nil {
		lines = append(lines, "Rechnungsdatum: "+run.IssuedOn.Format("02.01.2006"))
	}
	count := strconv.Itoa(len(documents)) + " Rechnungen"
	switch len(documents) {
	case 0:
	case 1:
		count = "1 Rechnung: " + documents[0].Number
	default:
		count += ", Nummern " + documents[0].Number + " bis " + documents[len(documents)-1].Number
	}
	return append(lines, count, "Gesamtsumme: "+formatAmount(total)+" Euro")
}

// overviewPositionLabel names what a position stands for, as the run page
// does; the letters name it shorter (positionLabel).
func overviewPositionLabel(position parentbilling.BillingPosition) string {
	switch position.Kind {
	case parentbilling.LineBase:
		if position.WeekStartsOn != nil && position.WeekEndsOn != nil {
			return "Woche " + position.WeekStartsOn.Format("02.01.2006") + " bis " + position.WeekEndsOn.Format("02.01.2006") +
				", " + position.TimeModelName
		}
		return position.TimeModelName
	case parentbilling.LineSibling:
		label := "Geschwister"
		if position.ChildRank != nil {
			label += ": " + strconv.Itoa(*position.ChildRank) + ". Kind"
		}
		switch {
		case position.DiscountBasisPoints != nil:
			label += ", " + formatShare(*position.DiscountBasisPoints) + " weniger"
		case position.DiscountCents != nil:
			label += ", " + formatAmount(*position.DiscountCents) + " Euro weniger"
		}
		return label
	case parentbilling.LineBenefitsExemption:
		return "Befreiung bei Leistungsbezug: " + position.Note
	case parentbilling.LineCorrection:
		return "Korrektur: " + position.Note
	}
	return position.Kind
}

// formatAmount writes cents the German way without a currency sign:
// 123456 → "1.234,56", -500 → "-5,00".
func formatAmount(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	euros := strconv.FormatInt(cents/100, 10)
	for i := len(euros) - 3; i > 0; i -= 3 {
		euros = euros[:i] + "." + euros[i:]
	}
	return fmt.Sprintf("%s%s,%02d", sign, euros, cents%100)
}

// formatShare writes basis points as a percentage: 5000 → "50 %",
// 1250 → "12,5 %".
func formatShare(basisPoints int) string {
	share := strings.TrimRight(strings.TrimRight(formatAmount(int64(basisPoints)), "0"), ",")
	return share + " %"
}
