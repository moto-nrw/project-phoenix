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

// The list of a monthly run for one cost bearer (#4687, ADR 0169): one row
// per invoice the cost bearer pays a part of, with the child, the school,
// the contribution, the share and what the cost bearer pays, then the sum.
// The Träger sends it to the cost bearer to bill its part separately. A
// calculated run lists its preview without the excluded invoices and gives
// no number; a released run lists its issued invoices and marks the ones a
// Storno took back, outside the sum. It names no reference to a proof and
// nothing of the decision: the cost bearer knows its own decision.

// careCostShareColumns are the columns of the list.
var careCostShareColumns = []ExportColumn{
	{ID: "number", Label: "Rechnungsnummer"},
	{ID: "school", Label: "Schule"},
	{ID: "child", Label: "Kind"},
	{ID: "kind", Label: "Art"},
	{ID: "contribution", Label: "Berechnungsgrundlage in Euro"},
	{ID: "share", Label: "Anteil"},
	{ID: "amount", Label: "Übernommener Betrag in Euro"},
	{ID: "status", Label: "Status"},
}

// costShareKindLabels are the kinds as the lists and letters name them.
var costShareKindLabels = map[string]string{
	parentbilling.CostShareEducationParticipation: "Bildung und Teilhabe",
	parentbilling.CostShareYouthOffice:            "Jugendamt",
	parentbilling.CostShareOtherSubsidy:           "Sonstiger Zuschuss",
}

// ExportCareCostShares renders the list of a run for one cost bearer of the
// Träger and records the download with the contracts it names; the caller
// rolls back when that fails.
func (c *CareBilling) ExportCareCostShares(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, costBearerID int64, format string) (parentbilling.ExportFile, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.ExportFile{}, err
	}
	switch format {
	case parentbilling.ExportPDF, parentbilling.ExportXLSX, parentbilling.ExportCSV:
	default:
		return parentbilling.ExportFile{}, validation.Errors{"format": errors.New("must be pdf, xlsx or csv")}
	}
	if runID <= 0 {
		return parentbilling.ExportFile{}, parentbilling.ErrCareRunNotFound
	}
	if costBearerID <= 0 {
		return parentbilling.ExportFile{}, parentbilling.ErrCostBearerNotFound
	}
	bearer, err := c.store.CostBearer(ctx, reviewer.OrganizationID, costBearerID)
	if err != nil {
		return parentbilling.ExportFile{}, err
	}
	detail, err := c.CareRun(ctx, reviewer, runID)
	if err != nil {
		return parentbilling.ExportFile{}, err
	}
	if detail.Status == parentbilling.RunDiscarded {
		return parentbilling.ExportFile{}, parentbilling.ErrBillingRunDiscarded
	}
	cancelled := map[int64]bool{}
	if detail.Status == parentbilling.RunReleased {
		cancellations, err := c.store.CareRunCancellations(ctx, reviewer.OrganizationID, detail.ID)
		if err != nil {
			return parentbilling.ExportFile{}, err
		}
		cancelled = cancelledInvoices(cancellations)
	}
	table, contracts := careCostShareTable(detail, bearer, cancelled)
	file, err := c.renderer.RenderTable(table, format,
		"kostentraeger-"+strconv.FormatInt(bearer.ID, 10)+"-"+detail.BillingMonth.Format("2006-01"))
	if err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: render the cost bearer list: %w", err)
	}
	if err := c.store.InsertCareRunEvent(ctx, reviewer.OrganizationID, detail.ID, reviewer.AccountID, runEventExport,
		map[string]any{"format": format, "list": "cost_share", "cost_bearer_id": bearer.ID, "contract_ids": contracts}); err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: record the cost bearer list: %w", err)
	}
	return file, nil
}

// careCostShareTable lays out the list of a run for one cost bearer and
// names the contracts it lists.
func careCostShareTable(detail parentbilling.CareRunDetail, bearer parentbilling.CostBearer, cancelled map[int64]bool) (ExportTable, []int64) {
	rows := []map[string]string{}
	contracts := []int64{}
	var total int64
	for _, invoice := range detail.Invoices {
		if invoice.Exclusion != nil {
			continue
		}
		for _, share := range invoice.CostShares {
			if share.CostBearerID != bearer.ID {
				continue
			}
			status := "Ausgestellt"
			switch {
			case detail.Status != parentbilling.RunReleased:
				status = "Vorschau"
				total += share.AmountCents
			case cancelled[invoice.ID]:
				status = "Storniert, nicht in der Summe"
			default:
				total += share.AmountCents
			}
			contracts = append(contracts, invoice.ContractID)
			rows = append(rows, map[string]string{
				"number": invoice.Number, "school": invoice.SchoolName,
				"child": strings.TrimSpace(invoice.ChildFirstName + " " + invoice.ChildLastName),
				"kind":  costShareKindLabels[share.Kind], "contribution": formatAmount(costShareBaseCents(invoice, share.Scope)),
				"share": costShareText(share), "amount": formatAmount(share.AmountCents), "status": status,
			})
		}
	}
	rows = append(rows, map[string]string{"child": "Summe", "amount": formatAmount(total)})
	lines := []string{}
	if address := strings.TrimSpace(bearer.Address); address != "" {
		lines = append(lines, "An: "+bearer.Name+", "+strings.Join(textLines(address), ", "))
	}
	if reference := strings.TrimSpace(bearer.FileReference); reference != "" {
		lines = append(lines, "Aktenzeichen: "+reference)
	}
	if detail.Status != parentbilling.RunReleased {
		lines = append(lines, "Vorschau ohne Rechnungsnummern. Die Nummern gibt erst die Ausstellung.")
	}
	return ExportTable{
		Title: "Kostenträger-Anteile " + bearer.Name, Subtitle: parentbilling.CareMonthName(detail.BillingMonth),
		Lines: lines, Columns: careCostShareColumns, Rows: rows,
	}, contracts
}

// costShareText writes a share: "100 %" or "45,00 € im Monat".
func costShareText(share parentbilling.CareInvoiceCostShare) string {
	text := ""
	switch {
	case share.ShareBasisPoints != nil:
		text = strings.ReplaceAll(strconv.FormatFloat(float64(*share.ShareBasisPoints)/100, 'f', -1, 64), ".", ",") + " %"
	case share.ShareCents != nil:
		text = formatEuro(*share.ShareCents) + " im Monat"
	default:
		return ""
	}
	return text + costShareScopeTexts[share.Scope]
}

// costShareScopeTexts name what a share covers (#4686); a share of the care
// says nothing, as before the Mensa.
var costShareScopeTexts = map[string]string{
	parentbilling.CostShareScopeMeal: " für die Mensa",
	parentbilling.CostShareScopeBoth: " für Betreuung und Mensa",
}

// costShareBaseCents is what a share of the scope took its part of on the
// invoice (#4686): the Mensa, the contribution of the modules, or both. The
// list of a cost bearer that covers the Mensa so shows the Mensa, not the
// care it does not pay for.
func costShareBaseCents(invoice parentbilling.CareInvoice, scope string) int64 {
	var meals int64
	for _, position := range invoice.MealPositions {
		meals += position.AmountCents
	}
	switch scope {
	case parentbilling.CostShareScopeMeal:
		return meals
	case parentbilling.CostShareScopeBoth:
		return invoice.ContributionCents()
	}
	return invoice.ContributionCents() - meals
}
