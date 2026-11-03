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
// ask. The registration has no postal address, so the address field names
// the family and the person who registered; the invoice number is the
// reference to pay with, as the Bissendorf address number was.

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

// letterContent is everything one letter prints.
type letterContent struct {
	run        parentbilling.BillingRun
	document   parentbilling.BillingDocument
	input      parentbilling.BillingLetterInput
	letterhead parentbilling.Letterhead
}

// confirmationLetter writes the letter of one issued invoice: the
// address field, the facts and the footer come from the invoice and the
// letterhead; subject, salutation, body and closing come from the Träger's
// template of the confirmation letter, filled with the values of this
// invoice (#4677, ADR 0150). Without a template of its own the Träger
// writes with the standard template, the letter of ADR 0121.
func confirmationLetter(content letterContent) Letter {
	document, head := content.document, content.letterhead
	filled := templates.Fill(letterTemplate(head.Template), letterValues(content))
	return Letter{
		Title:       "Bestätigung und Rechnung " + document.Number,
		AccentColor: head.AccentColor,
		Logo:        head.Logo,
		Sender:      textLines(head.SenderText),
		Recipient:   nonBlank("Familie "+document.RecipientLastName, fullName(document.RecipientFirstName, document.RecipientLastName)),
		Facts:       letterFacts(content),
		Subject:     filled.Subject,
		Salutation:  filled.Salutation,
		Blocks:      letterBlocks(filled.Blocks),
		Closing:     filled.Closing,
		Footer:      nonBlank(append(textLines(head.FooterText), bankLine(head.BankAccount))...),
	}
}

// letterTemplate is the Träger's template, or the standard one while the
// letterhead carries none.
func letterTemplate(template parentbilling.LetterTemplate) templates.Template {
	if strings.TrimSpace(template.Body) == "" {
		return templates.Default(templates.KindConfirmation)
	}
	return templates.Template{Subject: template.Subject, Salutation: template.Salutation, Body: template.Body, Closing: template.Closing}
}

// letterValues are the values of every placeholder of the confirmation
// letter for one invoice.
func letterValues(content letterContent) map[string]templates.Value {
	document, head := content.document, content.letterhead
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
		"positionen":      {Rows: rows(positionFacts(document))},
		"summe":           text(formatEuro(document.TotalCents)),
		"zahlungshinweis": text(paymentText(content)),
		"hinweise":        text(content.input.Notes),
		"notfallnummern":  {Rows: emergencyRows(content.input.EmergencyContacts)},
	}
	if document.IssuedOn != nil {
		values["datum"] = text(germanDate(*document.IssuedOn))
	}
	if document.TotalCents > 0 {
		values["zahlen_bis"] = text(germanDate(content.input.PaymentDueOn))
	}
	return values
}

// unfilledLetterPlaceholders names the placeholders of the confirmation
// letter a value map has no value for. Every placeholder its kind offers the
// Träger must be filled, or a paragraph of the Träger's template vanishes.
func unfilledLetterPlaceholders(values map[string]templates.Value) []string {
	var missing []string
	for _, placeholder := range templates.Placeholders(templates.KindConfirmation) {
		if value := values[placeholder.Name]; value.Text == "" && len(value.Rows) == 0 {
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

func rows(facts []LetterFact) []templates.Row {
	result := make([]templates.Row, len(facts))
	for i, fact := range facts {
		result[i] = templates.Row(fact)
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

func letterFacts(content letterContent) []LetterFact {
	facts := []LetterFact{{Label: "Rechnungsnummer", Value: content.document.Number}}
	if content.document.IssuedOn != nil {
		facts = append(facts, LetterFact{Label: "Datum", Value: germanDate(*content.document.IssuedOn)})
	}
	if content.document.TotalCents > 0 {
		facts = append(facts, LetterFact{Label: "Zahlen bis", Value: germanDate(content.input.PaymentDueOn)})
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

// positionFacts lists every position of the invoice by child and closes
// with the total.
func positionFacts(document parentbilling.BillingDocument) []LetterFact {
	facts := []LetterFact{}
	for _, child := range document.Children {
		for _, position := range child.Positions {
			facts = append(facts, LetterFact{
				Label: strings.TrimSpace(child.FirstName) + ": " + positionLabel(position),
				Value: formatEuro(position.AmountCents),
			})
		}
	}
	return append(facts, LetterFact{Label: "Gesamtbetrag", Value: formatEuro(document.TotalCents), Bold: true})
}

func positionLabel(position parentbilling.BillingPosition) string {
	switch position.Kind {
	case parentbilling.LineSibling:
		return "Ermäßigung für Geschwister"
	case parentbilling.LineBenefitsExemption:
		return "Befreiung bei Leistungsbezug"
	case parentbilling.LineCorrection:
		return "Korrektur: " + strings.TrimSpace(position.Note)
	}
	label := strings.TrimSpace(position.TimeModelName)
	if position.WeekStartsOn != nil && position.WeekEndsOn != nil {
		week := fmt.Sprintf("Woche %s bis %s", position.WeekStartsOn.Format("02.01."), germanDate(*position.WeekEndsOn))
		label = strings.TrimSpace(week + ", " + label)
	}
	return strings.TrimSuffix(label, ",")
}

func paymentText(content letterContent) string {
	if content.document.TotalCents == 0 {
		return "Sie müssen für diese Betreuung nichts bezahlen."
	}
	return fmt.Sprintf("Bitte überweisen Sie %s bis zum %s auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer %s an.",
		formatEuro(content.document.TotalCents), germanDate(content.input.PaymentDueOn), content.document.Number)
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
