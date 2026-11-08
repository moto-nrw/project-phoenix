package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
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

// billingRunNumberColumn replaces "Rechnungsnummer" when the run has a
// Storno or a Gutschrift, which carry numbers of their own (#4690).
var billingRunNumberColumn = ExportColumn{ID: "number", Label: "Nummer"}

// billingRunCorrectionColumns follow the number when the run has a Storno
// or a Gutschrift, or reissues a cancelled invoice (#4690).
var billingRunCorrectionColumns = []ExportColumn{
	{ID: "kind", Label: "Art"},
	{ID: "reference", Label: "Bezug"},
}

// billingRunDebtorColumn follows the recipient when an invoice of the run
// copied a debtor number (ADR 0122 addendum, #5621).
var billingRunDebtorColumn = ExportColumn{ID: "debtor_number", Label: "Debitorennummer"}

// billingRunAddressColumns follow the recipient when an invoice of the run
// copied a postal address (ADR 0122 addendum, #5625).
// billingRunPaymentColumns name how and by when an invoice is paid (#4689);
// a run whose invoices were issued before #4689 has none.
var billingRunPaymentColumns = []ExportColumn{
	{ID: "payment_method", Label: "Zahlungsweg"},
	{ID: "due_on", Label: "Fällig am"},
}

var billingRunAddressColumns = []ExportColumn{
	{ID: "street", Label: "Straße und Hausnummer"},
	{ID: "postal_code", Label: "PLZ"},
	{ID: "city", Label: "Ort"},
}

// billingRunOverview is the general layout of the overview of a released
// run. The recipient's mail address stays out; the Kämmerei needs who pays
// what, not how to reach them. The debtor number and the postal address
// name the payer; their columns appear only when an invoice of the run has
// one.
func billingRunOverview(run parentbilling.BillingRun, documents []parentbilling.BillingDocument) ExportTable {
	rows := []map[string]string{}
	var total int64
	corrected := hasCorrections(documents)
	for _, document := range documents {
		recipient := overviewRecipient(document)
		if corrected {
			recipient["kind"], recipient["reference"] = overviewKindLabel(document.Kind), overviewReference(document)
		}
		for _, child := range document.Children {
			name := strings.TrimSpace(child.FirstName + " " + child.LastName)
			for _, position := range child.Positions {
				rows = append(rows, withValues(recipient, map[string]string{
					"number": document.Number, "child": name,
					"position": overviewPositionLabel(position), "amount": formatAmount(position.AmountCents),
				}))
			}
		}
		rows = append(rows, withValues(recipient, map[string]string{
			"number": document.Number, "position": overviewSumLabel(document), "amount": formatAmount(document.TotalCents),
		}))
		total += document.TotalCents
	}
	rows = append(rows, map[string]string{"position": "Gesamtsumme des Laufs", "amount": formatAmount(total)})
	for _, takeover := range runCostTakeovers(documents) {
		rows = append(rows, map[string]string{
			"position": "Davon übernimmt " + takeover.name + " (" + invoiceCount(takeover.documents) + ")",
			"amount":   formatAmount(takeover.cents),
		})
	}
	return ExportTable{
		Title: "Abrechnungsanordnung", Subtitle: run.PeriodName,
		Lines: overviewLines(run, documents, total), Columns: overviewColumns(documents), Rows: rows,
		Footer: "Vertraulich. Enthält personenbezogene Daten.",
	}
}

// overviewRecipient is the recipient of an invoice in its rows: the name
// and, when the invoice has them, the debtor number and the postal address.
func overviewRecipient(document parentbilling.BillingDocument) map[string]string {
	recipient := map[string]string{"recipient": strings.TrimSpace(document.RecipientLastName + ", " + document.RecipientFirstName)}
	if number := strings.TrimSpace(document.DebtorNumber); number != "" {
		recipient["debtor_number"] = number
	}
	if address := document.RecipientAddress; address != nil {
		recipient["street"], recipient["postal_code"], recipient["city"] = address.Street, address.PostalCode, address.City
	}
	if payment := document.Payment; payment != nil {
		recipient["payment_method"], recipient["due_on"] = paymentMethodLabel(payment.Method), payment.DueOn.Format("02.01.2006")
	}
	return recipient
}

// withValues adds the values of from to row.
func withValues(from, row map[string]string) map[string]string {
	for id, value := range from {
		row[id] = value
	}
	return row
}

// overviewColumns are the columns of the overview: after the recipient the
// debtor number when an invoice of the run has one, then the address
// columns when an invoice of the run has an address.
func overviewColumns(documents []parentbilling.BillingDocument) []ExportColumn {
	debtor, address, paid, corrected := false, false, false, hasCorrections(documents)
	for _, document := range documents {
		debtor = debtor || strings.TrimSpace(document.DebtorNumber) != ""
		address = address || document.RecipientAddress != nil
		paid = paid || document.Payment != nil
	}
	if !debtor && !address && !paid && !corrected {
		return billingRunOverviewColumns
	}
	columns := append([]ExportColumn{}, billingRunOverviewColumns[:2]...)
	if corrected {
		// A run with a Storno or a Gutschrift names the kind and the
		// reference of every document (#4690).
		columns = append([]ExportColumn{billingRunNumberColumn}, billingRunCorrectionColumns...)
		columns = append(columns, billingRunOverviewColumns[1])
	}
	if debtor {
		columns = append(columns, billingRunDebtorColumn)
	}
	if address {
		columns = append(columns, billingRunAddressColumns...)
	}
	columns = append(columns, billingRunOverviewColumns[2:]...)
	if paid {
		// After the amount, so the columns before keep their places (#4689).
		columns = append(columns, billingRunPaymentColumns...)
	}
	return columns
}

// overviewLines are the facts of the run under the title. The count names
// the invoices; a Storno or a Gutschrift has a line of its own (#4690).
func overviewLines(run parentbilling.BillingRun, all []parentbilling.BillingDocument, total int64) []string {
	lines := []string{"Leistungszeitraum: " + run.ServiceFrom.Format("02.01.2006") + " bis " + run.ServiceUntil.Format("02.01.2006")}
	if run.IssuedOn != nil {
		lines = append(lines, "Rechnungsdatum: "+run.IssuedOn.Format("02.01.2006"))
	}
	documents := documentsOfKind(all, parentbilling.DocumentInvoice)
	corrections := []string{}
	for _, kind := range []struct{ kind, one, many string }{
		{parentbilling.DocumentCancellation, "1 Storno", " Stornos"},
		{parentbilling.DocumentCredit, "1 Gutschrift", " Gutschriften"},
	} {
		switch of := documentsOfKind(all, kind.kind); len(of) {
		case 0:
		case 1:
			corrections = append(corrections, kind.one+": "+of[0].Number)
		default:
			corrections = append(corrections, strconv.Itoa(len(of))+kind.many+": "+documentNumbers(of))
		}
	}
	count := strconv.Itoa(len(documents)) + " Rechnungen"
	switch len(documents) {
	case 0:
	case 1:
		count = "1 Rechnung: " + documents[0].Number
	default:
		count += ", Nummern " + documents[0].Number + " bis " + documents[len(documents)-1].Number
	}
	lines = append(append(lines, count), corrections...)
	lines = append(lines, "Gesamtsumme: "+formatAmount(total)+" Euro")
	for _, takeover := range runCostTakeovers(all) {
		lines = append(lines, "Davon übernimmt "+takeover.name+": "+formatAmount(takeover.cents)+" Euro ("+invoiceCount(takeover.documents)+")")
	}
	return lines
}

// hasCorrections reports a run with a Storno or a Gutschrift, or with an
// invoice that replaces a cancelled one; its overview names the kind and
// the reference of every document (#4690).
func hasCorrections(documents []parentbilling.BillingDocument) bool {
	for _, document := range documents {
		if (document.Kind != "" && document.Kind != parentbilling.DocumentInvoice) || document.ReferenceNumber != "" {
			return true
		}
	}
	return false
}

func documentsOfKind(documents []parentbilling.BillingDocument, kind string) []parentbilling.BillingDocument {
	of := []parentbilling.BillingDocument{}
	for _, document := range documents {
		if document.Kind == kind || (kind == parentbilling.DocumentInvoice && document.Kind == "") {
			of = append(of, document)
		}
	}
	return of
}

func documentNumbers(documents []parentbilling.BillingDocument) string {
	numbers := make([]string, len(documents))
	for i, document := range documents {
		numbers[i] = document.Number
	}
	return strings.Join(numbers, ", ")
}

// overviewKindLabel names the kind of a document in the overview.
func overviewKindLabel(kind string) string {
	switch kind {
	case parentbilling.DocumentCancellation:
		return "Storno"
	case parentbilling.DocumentCredit:
		return "Gutschrift"
	}
	return "Rechnung"
}

// overviewReference names the invoice a Storno or a Gutschrift belongs to,
// and the cancelled invoice an invoice replaces.
func overviewReference(document parentbilling.BillingDocument) string {
	switch {
	case document.ReferenceNumber == "":
		return ""
	case document.Kind == parentbilling.DocumentInvoice || document.Kind == "":
		return "ersetzt " + document.ReferenceNumber
	}
	return "zu " + document.ReferenceNumber
}

// overviewSumLabel is the label of the sum row of a document.
func overviewSumLabel(document parentbilling.BillingDocument) string {
	switch document.Kind {
	case parentbilling.DocumentCancellation:
		return "Summe des Stornos"
	case parentbilling.DocumentCredit:
		return "Summe der Gutschrift"
	}
	return "Summe der Rechnung"
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
	case parentbilling.LineCostTakeover:
		return "Kostenübernahme: " + position.CostBearerName
	case parentbilling.LineCorrection:
		return "Korrektur: " + position.Note
	case parentbilling.LineCredit:
		return "Gutschrift: " + position.Note
	}
	return position.Kind
}

// costTakeover is what one cost bearer pays in a run: the sum and the
// number of invoices, under the name the first invoice copied.
type costTakeover struct {
	id        int64
	name      string
	cents     int64
	documents int
}

// runCostTakeovers sums the cost takeovers of the documents per cost
// bearer, in the order the cost bearers first appear (#5627). A Storno
// negates every position of its invoice (#4690), the takeover line too: its
// amount reverses the invoice's and it takes the invoice out of the count.
// A cost bearer whose invoices are all cancelled pays nothing and is left
// out.
func runCostTakeovers(documents []parentbilling.BillingDocument) []costTakeover {
	var takeovers []costTakeover
	index := map[int64]int{}
	for _, document := range documents {
		count := 1
		if document.Kind == parentbilling.DocumentCancellation {
			count = -1
		}
		for _, paid := range documentCostTakeovers(document) {
			at, ok := index[paid.id]
			if !ok {
				at = len(takeovers)
				index[paid.id] = at
				takeovers = append(takeovers, costTakeover{id: paid.id, name: paid.name})
			}
			takeovers[at].cents += paid.cents
			takeovers[at].documents += count
		}
	}
	paying := takeovers[:0]
	for _, takeover := range takeovers {
		if takeover.documents > 0 || takeover.cents != 0 {
			paying = append(paying, takeover)
		}
	}
	return paying
}

// documentCostTakeovers is what each cost bearer pays of one invoice. A
// takeover line is negative; the sum is positive.
func documentCostTakeovers(document parentbilling.BillingDocument) []costTakeover {
	var paid []costTakeover
	for _, child := range document.Children {
		for _, position := range child.Positions {
			if position.Kind != parentbilling.LineCostTakeover || position.CostBearerID == nil {
				continue
			}
			at := slices.IndexFunc(paid, func(takeover costTakeover) bool { return takeover.id == *position.CostBearerID })
			if at < 0 {
				at = len(paid)
				paid = append(paid, costTakeover{id: *position.CostBearerID, name: position.CostBearerName})
			}
			paid[at].cents -= position.AmountCents
		}
	}
	return paid
}

// invoiceCount writes "1 Rechnung" or "3 Rechnungen".
func invoiceCount(count int) string {
	if count == 1 {
		return "1 Rechnung"
	}
	return strconv.Itoa(count) + " Rechnungen"
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
