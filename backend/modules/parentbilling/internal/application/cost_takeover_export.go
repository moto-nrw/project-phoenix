package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The list for the reimbursement by a cost bearer (#5627): every child of a
// released run whose invoice the cost bearer pays, with its weeks, the base
// amount, the sibling discount and the amount the cost bearer pays, whether
// the family brought a proof and when the Träger-Büro confirmed the
// registration. The Träger sends it to the cost bearer to get the amounts
// back. It uses the renderer of the overview for the Kämmerei.

// costTakeoverColumns are the columns of the list: one row per child, then
// the sum.
var costTakeoverColumns = []ExportColumn{
	{ID: "number", Label: "Rechnungsnummer"},
	{ID: "child", Label: "Kind"},
	{ID: "weeks", Label: "Wochen und Zeitmodell"},
	{ID: "base", Label: "Grundbetrag in Euro"},
	{ID: "sibling", Label: "Geschwisterermäßigung in Euro"},
	{ID: "takeover", Label: "Übernommener Betrag in Euro"},
	{ID: "proof", Label: "Nachweis"},
	{ID: "confirmed", Label: "Bestätigt am"},
}

// ExportCostTakeovers renders the list of a released run for one cost
// bearer of the Träger and records the download in the run's trail; the
// caller rolls back when that fails.
func (b *BillingRuns) ExportCostTakeovers(ctx context.Context, reviewer parentbilling.HolidayReviewer, runID, costBearerID int64, format string) (parentbilling.ExportFile, error) {
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
	if costBearerID <= 0 {
		return parentbilling.ExportFile{}, parentbilling.ErrCostBearerNotFound
	}
	run, err := b.store.BillingRun(ctx, reviewer.OrganizationID, runID)
	if err != nil {
		return parentbilling.ExportFile{}, err
	}
	if run.Status != parentbilling.RunReleased {
		return parentbilling.ExportFile{}, parentbilling.ErrBillingRunNotReleased
	}
	bearer, err := b.store.CostBearer(ctx, reviewer.OrganizationID, costBearerID)
	if err != nil {
		return parentbilling.ExportFile{}, err
	}
	in, err := b.costTakeoverInput(ctx, reviewer, run, bearer)
	if err != nil {
		return parentbilling.ExportFile{}, err
	}
	table, documents := costTakeoverList(in)
	file, err := b.renderer.RenderTable(table, format, "kostenuebernahme-"+run.PeriodName)
	if err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: render cost takeover list: %w", err)
	}
	if err := b.store.InsertRunEvent(ctx, reviewer.OrganizationID, run.ID, reviewer.AccountID, runEventExport,
		map[string]any{"format": format, "list": parentbilling.LineCostTakeover, "cost_bearer_id": bearer.ID, "document_count": documents}); err != nil {
		return parentbilling.ExportFile{}, fmt.Errorf("parent billing: record cost takeover export: %w", err)
	}
	return file, nil
}

// costTakeoverInput is what the list of one cost bearer reads.
type costTakeoverInput struct {
	run       parentbilling.BillingRun
	bearer    parentbilling.CostBearer
	documents []parentbilling.BillingDocument
	proofs    map[int64]bool
	confirmed map[int64]time.Time
}

func (b *BillingRuns) costTakeoverInput(ctx context.Context, reviewer parentbilling.HolidayReviewer, run parentbilling.BillingRun, bearer parentbilling.CostBearer) (costTakeoverInput, error) {
	documents, err := b.store.RunDocuments(ctx, reviewer.OrganizationID, run.ID)
	if err != nil {
		return costTakeoverInput{}, fmt.Errorf("parent billing: read invoices: %w", err)
	}
	registrations, err := b.registrations.ConfirmedHolidayRegistrations(ctx, reviewer, run.HolidayPeriodID)
	if err != nil {
		return costTakeoverInput{}, fmt.Errorf("parent billing: read proofs: %w", err)
	}
	proofs := make(map[int64]bool, len(registrations))
	for _, registration := range registrations {
		proofs[registration.ID] = registration.HasBenefitsProof
	}
	confirmed, err := b.store.RegistrationConfirmations(ctx, reviewer.OrganizationID, run.HolidayPeriodID)
	if err != nil {
		return costTakeoverInput{}, fmt.Errorf("parent billing: read confirmations: %w", err)
	}
	return costTakeoverInput{run: run, bearer: bearer, documents: documents, proofs: proofs, confirmed: confirmed}, nil
}

// costTakeoverList is the list of one cost bearer and the number of its
// invoices. A registration that is gone after 92 days (ADR 0118) leaves the
// proof unknown and the confirmation day empty; the invoice keeps the rest.
func costTakeoverList(in costTakeoverInput) (ExportTable, int) {
	rows := []map[string]string{}
	var total int64
	documents, children := 0, 0
	for _, document := range in.documents {
		// A Storno negates its invoice, the takeover too (#4690): its rows
		// reduce the sum and it takes the invoice out of the counts.
		sign := 1
		if document.Kind == parentbilling.DocumentCancellation {
			sign = -1
		}
		billed := false
		for _, child := range document.Children {
			row, takeover, ok := costTakeoverRow(child, in.bearer.ID)
			if !ok {
				continue
			}
			billed = true
			children += sign
			total += takeover
			row["number"] = takeoverDocumentNumber(document)
			row["proof"], row["confirmed"] = registrationFacts(in, document.RegistrationID)
			rows = append(rows, row)
		}
		if billed {
			documents += sign
		}
	}
	rows = append(rows, map[string]string{"weeks": "Summe", "takeover": formatAmount(total)})
	rows = append(rows, costBearerRows(in.bearer)...)
	return ExportTable{
		Title: "Liste für die Erstattung", Subtitle: in.run.PeriodName,
		Lines: costTakeoverLines(in, children, documents, total), Columns: costTakeoverColumns, Rows: rows,
		Footer: "Vertraulich. Enthält personenbezogene Daten.",
	}, documents
}

// takeoverDocumentNumber names a document in the list: an invoice by its
// number, a Storno by its number and the invoice it cancels.
func takeoverDocumentNumber(document parentbilling.BillingDocument) string {
	if document.Kind == parentbilling.DocumentCancellation && document.ReferenceNumber != "" {
		return document.Number + " (Storno zu " + document.ReferenceNumber + ")"
	}
	return document.Number
}

// proofGone is what the list says about the proof of a registration that
// is gone after 92 days (ADR 0118): its proofs went with it, so neither
// "Ja" nor "Nein" would be true.
const proofGone = "Nicht mehr verfügbar"

// registrationFacts are the proof and the confirmation day of the
// registration an invoice bills. The frozen contribution goes with the
// registration, so a registration without a confirmation day is gone: then
// the proof is no longer known and the day stays empty.
func registrationFacts(in costTakeoverInput, registrationID int64) (proof, confirmed string) {
	confirmedAt, found := in.confirmed[registrationID]
	if !found {
		return proofGone, ""
	}
	proof = "Nein"
	if in.proofs[registrationID] {
		proof = "Ja"
	}
	return proof, calendar.DateFromTime(confirmedAt).Format("02.01.2006")
}

// costTakeoverRow is the row of one child the cost bearer pays for: its
// weeks, the base amount, the sibling discount and the amount taken over.
func costTakeoverRow(child parentbilling.BillingChild, costBearerID int64) (map[string]string, int64, bool) {
	var base, sibling, takeover int64
	found := false
	weeks := []string{}
	for _, position := range child.Positions {
		switch position.Kind {
		case parentbilling.LineBase:
			base += position.AmountCents
			weeks = append(weeks, takeoverWeek(position))
		case parentbilling.LineSibling:
			sibling += position.AmountCents
		case parentbilling.LineCostTakeover:
			if position.CostBearerID != nil && *position.CostBearerID == costBearerID {
				found = true
				takeover -= position.AmountCents
			}
		}
	}
	if !found {
		return nil, 0, false
	}
	return map[string]string{
		"child": strings.TrimSpace(child.FirstName + " " + child.LastName), "weeks": strings.Join(weeks, "; "),
		"base": formatAmount(base), "sibling": formatAmount(sibling), "takeover": formatAmount(takeover),
	}, takeover, true
}

func takeoverWeek(position parentbilling.BillingPosition) string {
	if position.WeekStartsOn == nil || position.WeekEndsOn == nil {
		return position.TimeModelName
	}
	return position.WeekStartsOn.Format("02.01.") + " bis " + position.WeekEndsOn.Format("02.01.2006") + ", " + position.TimeModelName
}

// costBearerRows close the table with the name, the address and the file
// reference of the cost bearer, so the CSV and the Excel file carry them
// too; the PDF prints them under the title as well.
func costBearerRows(bearer parentbilling.CostBearer) []map[string]string {
	rows := []map[string]string{{"child": "Kostenträger", "weeks": bearer.Name}}
	if address := strings.Join(textLines(bearer.Address), ", "); address != "" {
		rows = append(rows, map[string]string{"child": "Anschrift", "weeks": address})
	}
	if reference := strings.TrimSpace(bearer.FileReference); reference != "" {
		rows = append(rows, map[string]string{"child": "Aktenzeichen", "weeks": reference})
	}
	return rows
}

// costTakeoverLines name the cost bearer, the run and the sum under the
// title.
func costTakeoverLines(in costTakeoverInput, children, documents int, total int64) []string {
	lines := []string{"Kostenträger: " + in.bearer.Name}
	if address := strings.Join(textLines(in.bearer.Address), ", "); address != "" {
		lines = append(lines, "Anschrift: "+address)
	}
	if reference := strings.TrimSpace(in.bearer.FileReference); reference != "" {
		lines = append(lines, "Aktenzeichen: "+reference)
	}
	lines = append(lines, "Leistungszeitraum: "+in.run.ServiceFrom.Format("02.01.2006")+" bis "+in.run.ServiceUntil.Format("02.01.2006"))
	if in.run.IssuedOn != nil {
		lines = append(lines, "Rechnungsdatum: "+in.run.IssuedOn.Format("02.01.2006"))
	}
	childCount := strconv.Itoa(children) + " Kinder"
	if children == 1 {
		childCount = "1 Kind"
	}
	return append(lines, childCount+", "+invoiceCount(documents), "Summe: "+formatAmount(total)+" Euro")
}
