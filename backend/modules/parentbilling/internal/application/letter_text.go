package application

import (
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The text of the confirmation letter (#4669, ADR 0121). It follows the
// Bissendorf letter field by field (GH-003): the family in the address
// field, the date, the subject, the salutation, the confirmation naming
// the children, the total, the reference to pay with, the payment date,
// the bank account of the Träger, the free text of the run (packing list,
// rule on illness), the emergency numbers of the hosting OGS and whom to
// ask. The address field names the family and the person who registered,
// then the postal address the invoice copied from the registration when the
// form asked for it (ADR 0121 addendum, #5625). The reference to pay with is
// the debtor number of the family the invoice copied from the registration,
// the Bissendorf address number, and without one the invoice number (ADR
// 0121 addendum, #5621).

// Letter is a confirmation letter as the Elternabrechnung writes it: every
// text, in the parts of a letter. The LetterRenderer lays it out in the
// layout of the Träger.
type Letter struct {
	Title       string
	AccentColor string
	Logo        []byte
	Sender      []string
	Recipient   []string
	Facts       []LetterFact
	Subject     string
	Salutation  string
	Blocks      []LetterBlock
	Closing     []string
	Footer      []string
}

// LetterFact is a label with its value; Bold marks a total.
type LetterFact struct {
	Label string
	Value string
	Bold  bool
}

// LetterBlock is a paragraph of the body with an optional heading and an
// optional two-column table after it.
type LetterBlock struct {
	Heading string
	Text    string
	Bold    bool
	Facts   []LetterFact
}

// LetterRenderer lays out a letter as a PDF. The adapter binds it to
// Document Rendering; it stores nothing.
type LetterRenderer interface {
	RenderLetter(letter Letter, filenameBase string) (parentbilling.BillingLetterFile, error)
}

// letterContent is everything one letter prints. The letter of a
// cancellation or a credit also names its invoice and what the invoice
// still asks for after it (#4690).
type letterContent struct {
	run        parentbilling.BillingRun
	document   parentbilling.BillingDocument
	input      parentbilling.BillingLetterInput
	letterhead parentbilling.Letterhead
	invoice    *parentbilling.BillingDocument
	leftCents  int64
}

// confirmationLetter writes the letter of one issued invoice in the kind of
// document its release froze (#4688): the address field, the facts and the
// footer come from the invoice and the letterhead; subject, salutation, body
// and closing come from the Träger's template of that kind, filled with the
// values of this invoice (#4677, ADR 0152). Without a template of its own
// the Träger writes with the standard template; the standard confirmation
// letter is the letter of ADR 0121. A fee notice prints the legal remedy of
// its template after the body, so no text of the template leaves it out.
func confirmationLetter(content letterContent) Letter {
	document, head := content.document, content.letterhead
	kind := templateKindOf(document.DocumentKind)
	filled := templates.Fill(letterTemplate(head.Template, kind), letterValues(content))
	return Letter{
		Title:       documentTitle(document.DocumentKind) + " " + document.Number,
		AccentColor: head.AccentColor,
		Logo:        head.Logo,
		Sender:      textLines(head.SenderText),
		Recipient:   recipientLines(document),
		Facts:       letterFacts(content),
		Subject:     filled.Subject,
		Salutation:  filled.Salutation,
		Blocks:      append(letterBlocks(filled.Blocks), legalRemedyBlocks(filled.LegalRemedy)...),
		Closing:     filled.Closing,
		Footer:      nonBlank(append(textLines(head.FooterText), bankLine(head.BankAccount))...),
	}
}

// templateKindOf is the kind of template a kind of document is written
// with; an invoice issued before #4688 is a confirmation letter.
func templateKindOf(documentKind string) templates.Kind {
	switch documentKind {
	case parentbilling.DocumentKindInvoice:
		return templates.KindInvoice
	case parentbilling.DocumentKindFeeNotice:
		return templates.KindFeeNotice
	}
	return templates.KindConfirmation
}

// documentTitle names a kind of document in the title of its letter.
func documentTitle(documentKind string) string {
	switch documentKind {
	case parentbilling.DocumentKindInvoice:
		return "Rechnung"
	case parentbilling.DocumentKindFeeNotice:
		return "Gebührenbescheid"
	}
	return "Bestätigung und Rechnung"
}

// legalRemedyHeading heads the legal remedy when the Träger's text starts
// without a heading of its own.
const legalRemedyHeading = "Rechtsbehelfsbelehrung"

// legalRemedyBlocks prints the legal remedy of a fee notice under its
// heading (#4688).
func legalRemedyBlocks(blocks []templates.Block) []LetterBlock {
	result := letterBlocks(blocks)
	if len(result) > 0 && strings.TrimSpace(result[0].Heading) == "" {
		result[0].Heading = legalRemedyHeading
	}
	return result
}

// letterTemplate is the Träger's template, or the standard one of kind
// while the letterhead carries none.
func letterTemplate(template parentbilling.LetterTemplate, kind templates.Kind) templates.Template {
	if strings.TrimSpace(template.Body) == "" {
		standard := templates.Default(kind)
		standard.LegalRemedy = template.LegalRemedy
		return standard
	}
	return templates.Template{
		Subject: template.Subject, Salutation: template.Salutation, Body: template.Body, Closing: template.Closing,
		Invoice: invoiceLayout(template), LegalRemedy: template.LegalRemedy,
	}
}

func invoiceLayout(template parentbilling.LetterTemplate) templates.InvoiceLayout {
	return templates.InvoiceLayout{GroupByChild: template.GroupByChild, HideTimeModels: template.HideTimeModels}
}

// letterValues are the values of every placeholder of the confirmation
// letter for one invoice, with the table of amounts in the layout of the
// Träger's template (#5691).
func letterValues(content letterContent) map[string]templates.Value {
	document, head := content.document, content.letterhead
	layout := invoiceLayout(head.Template)
	text := func(value string) templates.Value { return templates.Value{Text: strings.TrimSpace(value)} }
	values := map[string]templates.Value{
		"familienname":    text(document.RecipientLastName),
		"anmelder":        text(fullName(document.RecipientFirstName, document.RecipientLastName)),
		"kinder":          text(childNames(document.Children)),
		"traeger":         text(firstLine(textLines(head.SenderText))),
		"ansprechperson":  text(head.ContactName),
		"rueckfragen":     text(questionText(head)),
		"zeitraum":        text(content.run.PeriodName),
		"rechnungsnummer": text(document.Number),
		"bescheidnummer":  text(document.Number),
		"debitorennummer": text(document.DebtorNumber),
		"positionen":      {Rows: templates.InvoiceRows(layout, invoiceChildren(document), formatEuro(document.TotalCents))},
		"summe":           text(formatEuro(document.TotalCents)),
		"zahlungshinweis": text(paymentText(content)),
		"hinweise":        text(content.input.Notes),
		"notfallnummern":  {Rows: emergencyRows(content.input.EmergencyContacts)},
	}
	values["datum"], values["zahlen_bis"] = templates.Value{}, templates.Value{}
	if document.IssuedOn != nil {
		values["datum"] = text(germanDate(*document.IssuedOn))
	}
	if document.TotalCents > 0 {
		values["zahlen_bis"] = text(germanDate(dueOnOf(document.Payment, content.input.PaymentDueOn)))
	}
	return values
}

// missingLetterValues names the placeholders of the confirmation letter the
// letter writes no value for. Every placeholder its kind offers the Träger
// must have one, even an empty one, or a paragraph of the Träger's template
// would vanish without a reason.
func missingLetterValues(values map[string]templates.Value) []string {
	return missingLetterValuesOf(templates.KindConfirmation, values)
}

// missingLetterValuesOf names the placeholders of a kind of template the
// letter writes no value for (#4688).
func missingLetterValuesOf(kind templates.Kind, values map[string]templates.Value) []string {
	var missing []string
	for _, placeholder := range templates.Placeholders(kind) {
		if _, ok := values[placeholder.Name]; !ok {
			missing = append(missing, placeholder.Name)
		}
	}
	return missing
}

func emergencyRows(contacts []parentbilling.BillingEmergencyContact) []templates.Row {
	result := make([]templates.Row, len(contacts))
	for i, contact := range contacts {
		result[i] = templates.Row{Label: contact.Name, Value: contact.Phone}
	}
	return result
}

func letterBlocks(blocks []templates.Block) []LetterBlock {
	result := make([]LetterBlock, len(blocks))
	for i, block := range blocks {
		var facts []LetterFact
		for _, row := range block.Rows {
			facts = append(facts, LetterFact(row))
		}
		result[i] = LetterBlock{Heading: block.Heading, Text: block.Text, Bold: block.Bold, Facts: facts}
	}
	return result
}

// recipientLines are the lines of the address field: the family, the
// person who registered and, when the invoice has one, the street and the
// postal code with the city.
func recipientLines(document parentbilling.BillingDocument) []string {
	return billingRecipientLines(document.RecipientKind, document.RecipientFirstName, document.RecipientLastName,
		document.RecipientSecondName, document.RecipientAddress)
}

func letterFacts(content letterContent) []LetterFact {
	kind := templateKindOf(content.document.DocumentKind)
	facts := []LetterFact{{Label: numberLabel(content.document.DocumentKind), Value: content.document.Number}}
	if number := strings.TrimSpace(content.document.DebtorNumber); number != "" {
		facts = append(facts, LetterFact{Label: templates.DebtorNumberLabel(kind), Value: number})
	}
	if content.document.IssuedOn != nil {
		facts = append(facts, LetterFact{Label: "Datum", Value: germanDate(*content.document.IssuedOn)})
	}
	if content.document.TotalCents > 0 {
		facts = append(facts, paymentFacts(content.document.Payment, content.input.PaymentDueOn)...)
	}
	head := content.letterhead
	for _, fact := range []LetterFact{
		{Label: "Ansprechperson", Value: head.ContactName},
		{Label: "Telefon", Value: head.ContactPhone},
		{Label: "E-Mail", Value: head.ContactEmail},
	} {
		if strings.TrimSpace(fact.Value) != "" {
			facts = append(facts, fact)
		}
	}
	return facts
}

// numberLabel names the number of a document beside the address field.
func numberLabel(documentKind string) string {
	if documentKind == parentbilling.DocumentKindFeeNotice {
		return "Bescheidnummer"
	}
	return "Rechnungsnummer"
}

// invoiceChildren are the positions of the invoice by child with the
// child's subtotal; the template's layout prints them (#5691).
func invoiceChildren(document parentbilling.BillingDocument) []templates.InvoiceChild {
	children := make([]templates.InvoiceChild, 0, len(document.Children))
	for _, child := range document.Children {
		lines := make([]templates.InvoiceLine, 0, len(child.Positions))
		var subtotal int64
		for _, position := range child.Positions {
			line := invoiceLine(position)
			line.Amount = formatEuro(position.AmountCents)
			lines = append(lines, line)
			subtotal += position.AmountCents
		}
		children = append(children, templates.InvoiceChild{Name: strings.TrimSpace(child.FirstName), Lines: lines, Subtotal: formatEuro(subtotal)})
	}
	return children
}

// invoiceLine names a position: a booking by its week and time model,
// every other position by a text every layout prints.
func invoiceLine(position parentbilling.BillingPosition) templates.InvoiceLine {
	switch position.Kind {
	case parentbilling.LineSibling:
		return templates.InvoiceLine{Text: "Ermäßigung für Geschwister"}
	case parentbilling.LineBenefitsExemption:
		return templates.InvoiceLine{Text: "Befreiung bei Leistungsbezug"}
	case parentbilling.LineCostTakeover:
		return templates.InvoiceLine{Text: "Kosten übernimmt: " + strings.TrimSpace(position.CostBearerName)}
	case parentbilling.LineCorrection:
		return templates.InvoiceLine{Text: "Korrektur: " + strings.TrimSpace(position.Note)}
	}
	line := templates.InvoiceLine{TimeModel: strings.TrimSpace(position.TimeModelName)}
	if position.WeekStartsOn != nil && position.WeekEndsOn != nil {
		line.Week = fmt.Sprintf("Woche %s bis %s", position.WeekStartsOn.Format("02.01."), germanDate(*position.WeekEndsOn))
	}
	return line
}

// positionLabel names a position in one line, as the letters of a
// cancellation or a credit list it: the text of the position or its week
// with its time model.
func positionLabel(position parentbilling.BillingPosition) string {
	line := invoiceLine(position)
	if line.Text != "" {
		return line.Text
	}
	return strings.Join(nonBlank(line.Week, line.TimeModel), ", ")
}

// The placeholders of the payment text of a Träger (#5622). The settings
// registry admits exactly these four (parent_billing.payment_text).
const (
	placeholderAmount        = "{Betrag}"
	placeholderPaymentDueOn  = "{Zahlungsfrist}"
	placeholderInvoiceNumber = "{Rechnungsnummer}"
	placeholderDebtorNumber  = "{Debitorennummer}"
)

// paymentText asks for the transfer. A Träger's own payment text replaces
// the built-in sentence with its placeholders filled in (ADR 0121 addendum,
// #5622); an invoice of 0 € keeps the fixed sentence, which names the cost
// bearer when one takes the costs over (#5627). The reference to pay with of
// the built-in sentence is the debtor number of the family when the invoice
// has one (ADR 0121 addendum, #5621), otherwise the invoice number.
func paymentText(content letterContent) string {
	if content.document.TotalCents == 0 {
		if bearer := costBearerName(content.document); bearer != "" {
			return "Sie müssen nichts bezahlen. Die Kosten übernimmt: " + bearer + "."
		}
		return "Sie müssen für diese Betreuung nichts bezahlen."
	}
	due := dueOnOf(content.document.Payment, content.input.PaymentDueOn)
	reference := "die Rechnungsnummer " + content.document.Number
	if number := strings.TrimSpace(content.document.DebtorNumber); number != "" {
		reference = "Ihre Debitorennummer " + number
	}
	if isDirectDebit(content.document.Payment) {
		return directDebitText(content.document.TotalCents, due, strings.TrimPrefix(strings.TrimPrefix(reference, "die "), "Ihre "))
	}
	if text := strings.TrimSpace(content.letterhead.PaymentText); text != "" {
		return strings.NewReplacer(
			placeholderAmount, formatEuro(content.document.TotalCents),
			placeholderPaymentDueOn, germanDate(due),
			placeholderInvoiceNumber, content.document.Number,
			placeholderDebtorNumber, strings.TrimSpace(content.document.DebtorNumber),
		).Replace(text)
	}
	return fmt.Sprintf("Bitte überweisen Sie %s bis zum %s auf das Konto unten. Geben Sie als Verwendungszweck %s an.",
		formatEuro(content.document.TotalCents), germanDate(due), reference)
}

// costBearerName is the name of the cost bearer of the first cost takeover
// of the invoice, or empty without one.
func costBearerName(document parentbilling.BillingDocument) string {
	for _, child := range document.Children {
		for _, position := range child.Positions {
			if position.Kind == parentbilling.LineCostTakeover {
				return strings.TrimSpace(position.CostBearerName)
			}
		}
	}
	return ""
}

func questionText(head parentbilling.Letterhead) string {
	name := strings.TrimSpace(head.ContactName)
	ways := nonBlank(head.ContactPhone, head.ContactEmail)
	switch {
	case name == "" && len(ways) == 0:
		return ""
	case name == "":
		return "Bei Fragen erreichen Sie uns unter " + strings.Join(ways, " oder ") + "."
	case len(ways) == 0:
		return "Bei Fragen hilft Ihnen " + name + "."
	}
	return "Bei Fragen erreichen Sie " + name + " unter " + strings.Join(ways, " oder ") + "."
}

func bankLine(account *parentbilling.LetterheadBankAccount) string {
	if account == nil {
		return ""
	}
	parts := nonBlank(account.AccountHolder, "IBAN "+groupIBAN(account.IBAN))
	if bic := strings.TrimSpace(account.BIC); bic != "" {
		parts = append(parts, "BIC "+bic)
	}
	return "Bankverbindung: " + strings.Join(append(parts, nonBlank(account.BankName)...), " · ")
}

// groupIBAN prints an IBAN in groups of four, as banks print it.
func groupIBAN(iban string) string {
	compact := strings.ToUpper(strings.ReplaceAll(iban, " ", ""))
	var groups []string
	for len(compact) > 4 {
		groups = append(groups, compact[:4])
		compact = compact[4:]
	}
	return strings.Join(append(groups, compact), " ")
}

// childNames joins the first names: "Lina", "Lina und Ben", "Lina, Ben
// und Mia".
func childNames(children []parentbilling.BillingChild) string {
	names := make([]string, 0, len(children))
	for _, child := range children {
		names = append(names, strings.TrimSpace(child.FirstName))
	}
	names = nonBlank(names...)
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " und " + names[len(names)-1]
}

// formatEuro prints cents as "1.234,50 €"; a discount keeps its minus.
func formatEuro(cents int64) string {
	return formatAmount(cents) + " €"
}

func germanDate(date calendar.Date) string {
	if date.IsZero() {
		return ""
	}
	return date.Format("02.01.2006")
}

func fullName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

func textLines(text string) []string {
	return nonBlank(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")...)
}

func firstLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func nonBlank(values ...string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			kept = append(kept, value)
		}
	}
	return kept
}
