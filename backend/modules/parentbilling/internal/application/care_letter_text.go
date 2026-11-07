package application

import (
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The letters of a monthly invoice and of its Storno (#4684, ADR 0165). They
// keep the layout, the address field, the contact and the bank line of the
// holiday letters (ADR 0121) and list one line per booked module with its
// weekdays, never one per calendar day (ADR 0158 point 2). The invoice asks
// for payment by the date of the run; the Storno asks for nothing.

// careLetterContent is everything one monthly letter prints.
type careLetterContent struct {
	run          parentbilling.CareRun
	invoice      parentbilling.CareInvoice
	cancellation *parentbilling.CareCancellation
	input        parentbilling.CareLetterInput
	letterhead   parentbilling.Letterhead
}

var germanWeekdays = map[string]string{"mon": "Mo", "tue": "Di", "wed": "Mi", "thu": "Do", "fri": "Fr", "sat": "Sa", "sun": "So"}

func careMonthName(month calendar.Date) string { return parentbilling.CareMonthName(month) }

func carePeriodName(month calendar.Date) string { return parentbilling.CarePeriodName(month) }

// careInvoiceLetter writes the letter of one issued monthly invoice.
func careInvoiceLetter(content careLetterContent) Letter {
	invoice, head := content.invoice, content.letterhead
	letter := careLetterFrame(content, "Rechnung "+invoice.Number,
		"Rechnung "+invoice.Number+": "+carePeriodName(content.run.BillingMonth))
	letter.Facts = careLetterFacts(content, []LetterFact{{Label: "Rechnungsnummer", Value: invoice.Number}})
	blocks := []LetterBlock{
		{Text: fmt.Sprintf("für die Betreuung von %s an der %s berechnen wir Ihnen den Monatsbeitrag für %s. Der Betrag richtet sich nach Ihrem Betreuungsvertrag.",
			fullName(invoice.ChildFirstName, invoice.ChildLastName), strings.TrimSpace(invoice.SchoolName), careMonthName(content.run.BillingMonth))},
		{Heading: "Ihre Betreuung", Facts: append(carePositionFacts(invoice.Positions, 1),
			LetterFact{Label: "Summe", Value: formatEuro(invoice.TotalCents), Bold: true})},
		{Text: carePaymentText(content), Bold: true},
	}
	if notes := strings.TrimSpace(content.input.Notes); notes != "" {
		blocks = append(blocks, LetterBlock{Heading: "Hinweise", Text: notes})
	}
	if question := questionText(head); question != "" {
		blocks = append(blocks, LetterBlock{Text: question})
	}
	letter.Blocks = blocks
	return letter
}

// careCancellationLetter writes the letter of the Storno of a sent monthly
// invoice. It names the invoice with its date and the reason and lists every
// position with a minus.
func careCancellationLetter(content careLetterContent) Letter {
	invoice, cancellation, head := content.invoice, *content.cancellation, content.letterhead
	letter := careLetterFrame(content, "Storno "+cancellation.Number+" zur Rechnung "+invoice.Number,
		"Storno zur Rechnung "+invoice.Number+": "+carePeriodName(content.run.BillingMonth))
	letter.Facts = careLetterFacts(content, []LetterFact{
		{Label: "Storno-Nummer", Value: cancellation.Number}, {Label: "Zur Rechnung", Value: invoice.Number},
	})
	reference := invoice.Number
	if invoice.IssuedOn != nil {
		reference += " vom " + germanDate(*invoice.IssuedOn)
	}
	letter.Blocks = []LetterBlock{
		{Text: fmt.Sprintf("wir stornieren die Rechnung %s für %s. Grund: %s",
			reference, carePeriodName(content.run.BillingMonth), sentence(cancellation.Reason))},
		{Facts: append(carePositionFacts(invoice.Positions, -1),
			LetterFact{Label: "Storno gesamt", Value: formatEuro(cancellation.TotalCents), Bold: true})},
		{Text: "Diese Rechnung gilt nicht mehr. Sie müssen sie nicht bezahlen. Haben Sie schon bezahlt? Dann melden Sie sich bitte bei uns.", Bold: true},
	}
	if question := questionText(head); question != "" {
		letter.Blocks = append(letter.Blocks, LetterBlock{Text: question})
	}
	return letter
}

// careLetterFrame is a monthly letter without its facts and body.
func careLetterFrame(content careLetterContent, title, subject string) Letter {
	invoice, head := content.invoice, content.letterhead
	sender := textLines(head.SenderText)
	return Letter{
		Title:       title,
		AccentColor: head.AccentColor,
		Logo:        head.Logo,
		Sender:      sender,
		Recipient: nonBlank("Familie "+strings.TrimSpace(invoice.Recipient.LastName),
			fullName(invoice.Recipient.FirstName, invoice.Recipient.LastName)),
		Subject:    subject,
		Salutation: "Sehr geehrte Familie " + strings.TrimSpace(invoice.Recipient.LastName) + ",",
		Closing:    nonBlank("Mit freundlichen Grüßen", firstLine(sender)),
		Footer:     nonBlank(append(textLines(head.FooterText), bankLine(head.BankAccount))...),
	}
}

// careLetterFacts are the facts beside the address field: the numbers, the
// month, the date, the payment date of an invoice and the contact person.
func careLetterFacts(content careLetterContent, numbers []LetterFact) []LetterFact {
	facts := append(numbers, LetterFact{Label: "Kind", Value: fullName(content.invoice.ChildFirstName, content.invoice.ChildLastName)},
		LetterFact{Label: "Monat", Value: careMonthName(content.run.BillingMonth)})
	switch {
	case content.cancellation != nil:
		facts = append(facts, LetterFact{Label: "Datum", Value: germanDate(content.cancellation.IssuedOn)})
	case content.invoice.IssuedOn != nil:
		facts = append(facts, LetterFact{Label: "Datum", Value: germanDate(*content.invoice.IssuedOn)})
	}
	if content.cancellation == nil && content.invoice.TotalCents > 0 {
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

// carePositionFacts prints one line per position: the module, its booked
// weekdays and, when the position bills only a part of the month, its days.
// sign -1 prints the amounts of a Storno.
func carePositionFacts(positions []parentbilling.CarePosition, sign int64) []LetterFact {
	facts := make([]LetterFact, 0, len(positions))
	for _, position := range positions {
		weekdays := make([]string, 0, len(position.Weekdays))
		for _, weekday := range position.Weekdays {
			weekdays = append(weekdays, germanWeekdays[weekday.Weekday])
		}
		label := strings.TrimSpace(position.ModuleName)
		if len(weekdays) > 0 {
			label += " (" + strings.Join(nonBlank(weekdays...), ", ") + ")"
		}
		if !position.ServiceFrom.IsZero() && position.ServiceFrom.Day() != 1 || !isMonthEnd(position.ServiceUntil) {
			label += fmt.Sprintf(", %s bis %s", position.ServiceFrom.Format("02.01."), germanDate(position.ServiceUntil))
		}
		facts = append(facts, LetterFact{Label: label, Value: formatEuro(sign * position.AmountCents)})
	}
	return facts
}

func isMonthEnd(date calendar.Date) bool {
	return date.IsZero() || date.AddDays(1).Day() == 1
}

// carePaymentText asks for the transfer. The Träger's own payment text fills
// in amount, date and number; a monthly invoice has no debtor number yet
// (#4689), so a text that names one falls back to the built-in sentence,
// which never prints a gap where the reference belongs.
func carePaymentText(content careLetterContent) string {
	invoice := content.invoice
	if invoice.TotalCents == 0 {
		return "Sie müssen für diesen Monat nichts bezahlen."
	}
	if text := strings.TrimSpace(content.letterhead.PaymentText); text != "" && !strings.Contains(text, placeholderDebtorNumber) {
		return strings.NewReplacer(
			placeholderAmount, formatEuro(invoice.TotalCents),
			placeholderPaymentDueOn, germanDate(content.input.PaymentDueOn),
			placeholderInvoiceNumber, invoice.Number,
		).Replace(text)
	}
	return fmt.Sprintf("Bitte überweisen Sie %s bis zum %s auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer %s an.",
		formatEuro(invoice.TotalCents), germanDate(content.input.PaymentDueOn), invoice.Number)
}
