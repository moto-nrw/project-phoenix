package application

import (
	"fmt"
	"strconv"
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
	period := carePeriodName(content.run.BillingMonth)
	if invoice.Service == parentbilling.ServiceMeal {
		period = "Mensa " + careMonthName(content.run.BillingMonth)
	}
	letter := careLetterFrame(content, "Rechnung "+invoice.Number, "Rechnung "+invoice.Number+": "+period)
	letter.Facts = careLetterFacts(content, []LetterFact{{Label: "Rechnungsnummer", Value: invoice.Number}})
	intro, heading := careInvoiceIntro(content)
	blocks := []LetterBlock{
		{Text: intro},
		// A table follows a paragraph: the letter layout pulls it up by one
		// paragraph spacing, so a heading alone would overlap its first row.
		{Heading: heading, Text: careBasisText(invoice)},
		{Facts: append(append(append(append(carePositionFacts(invoice.Positions, 1), careReductionFacts(invoice.Reductions, 1)...),
			careMealFacts(invoice.MealPositions, 1)...), careCostShareFacts(invoice, 1)...),
			LetterFact{Label: careTotalLabel(invoice), Value: formatEuro(invoice.TotalCents), Bold: true})},
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

// careInvoiceIntro is the first sentence and the heading of an invoice
// letter: the monthly contribution of the care, or the Mensa on an invoice
// of its own (#4686, ADR 0167).
func careInvoiceIntro(content careLetterContent) (string, string) {
	invoice := content.invoice
	child, school := fullName(invoice.ChildFirstName, invoice.ChildLastName), strings.TrimSpace(invoice.SchoolName)
	if invoice.Service == parentbilling.ServiceMeal {
		return fmt.Sprintf("für das Mittagessen von %s an der %s berechnen wir Ihnen die Mensa. Die Rechnung gehört zu %s.",
			child, school, careMonthName(content.run.BillingMonth)), "Ihr Mittagessen"
	}
	return fmt.Sprintf("für die Betreuung von %s an der %s berechnen wir Ihnen den Monatsbeitrag für %s. Der Betrag richtet sich nach Ihrem Betreuungsvertrag.",
		child, school, careMonthName(content.run.BillingMonth)), "Ihre Betreuung"
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
		{Facts: append(append(append(append(carePositionFacts(invoice.Positions, -1), careReductionFacts(invoice.Reductions, -1)...),
			careMealFacts(invoice.MealPositions, -1)...), careCostShareFacts(invoice, -1)...),
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
		Recipient: billingRecipientLines(invoice.Recipient.Kind, invoice.Recipient.FirstName, invoice.Recipient.LastName,
			invoice.Recipient.SecondName, invoice.Recipient.Address),
		Subject:    subject,
		Salutation: billingSalutation(invoice.Recipient.Kind, invoice.Recipient.FirstName, invoice.Recipient.LastName),
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
		facts = append(facts, paymentFacts(content.invoice.Payment, content.input.PaymentDueOn)...)
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
		// A module priced by a scale names its step (#6196).
		if position.DaysPerWeek != nil {
			label += ", " + daysPerWeekLabel(*position.DaysPerWeek)
		}
		if position.MealExtra {
			label += ", ohne Essen"
		}
		if !position.ServiceFrom.IsZero() && position.ServiceFrom.Day() != 1 || !isMonthEnd(position.ServiceUntil) {
			label += fmt.Sprintf(", %s bis %s", position.ServiceFrom.Format("02.01."), germanDate(position.ServiceUntil))
		}
		facts = append(facts, LetterFact{Label: label, Value: formatEuro(sign * position.AmountCents)})
	}
	return facts
}

// daysPerWeekLabel names a scale step: „1 Tag je Woche“, „3 Tage je Woche“.
func daysPerWeekLabel(days int) string {
	if days == 1 {
		return "1 Tag je Woche"
	}
	return fmt.Sprintf("%d Tage je Woche", days)
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
	if invoice.TotalCents == 0 && len(invoice.CostShares) > 0 {
		return "Beitragsfrei: " + costBearerNames(invoice.CostShares) + " übernimmt Ihren Beitrag. Sie müssen für diesen Monat nichts bezahlen."
	}
	if invoice.TotalCents == 0 {
		return "Sie müssen für diesen Monat nichts bezahlen."
	}
	due := dueOnOf(invoice.Payment, content.input.PaymentDueOn)
	if isDirectDebit(invoice.Payment) {
		return directDebitText(invoice.TotalCents, due, "Rechnungsnummer "+invoice.Number, paymentMandate(invoice.Payment))
	}
	if text := strings.TrimSpace(content.letterhead.PaymentText); text != "" && !strings.Contains(text, placeholderDebtorNumber) {
		return strings.NewReplacer(
			placeholderAmount, formatEuro(invoice.TotalCents),
			placeholderPaymentDueOn, germanDate(due),
			placeholderInvoiceNumber, invoice.Number,
		).Replace(text)
	}
	return fmt.Sprintf("Bitte überweisen Sie %s bis zum %s auf das Konto unten. Geben Sie als Verwendungszweck die Rechnungsnummer %s an.",
		formatEuro(invoice.TotalCents), germanDate(due), invoice.Number)
}

// careReductionFacts prints one line per reduction of the chain (#4685, ADR
// 0164): its name and why it applies, with its minus; sign -1 prints the
// amounts of a Storno.
func careReductionFacts(reductions []parentbilling.CareReduction, sign int64) []LetterFact {
	facts := make([]LetterFact, 0, len(reductions))
	for _, reduction := range reductions {
		label := strings.TrimSpace(reduction.Name)
		if why := careReductionReason(reduction); why != "" {
			label += " (" + why + ")"
		}
		facts = append(facts, LetterFact{Label: label, Value: formatEuro(sign * reduction.AmountCents)})
	}
	return facts
}

// careReductionReason says in a few words why a reduction applies, as the
// run shows it.
func careReductionReason(reduction parentbilling.CareReduction) string {
	switch reduction.Kind {
	case "sibling":
		if reduction.ChildRank == nil || reduction.FamilyChildren == nil {
			return careRuleEffect(reduction)
		}
		return fmt.Sprintf("%d. Kind von %d, %s; zuerst zählt das Kind mit dem höchsten Beitrag",
			*reduction.ChildRank, *reduction.FamilyChildren, careRuleEffect(reduction))
	case "cap":
		if reduction.CapCents != nil {
			return "Höchstbetrag " + formatEuro(*reduction.CapCents) + " je Familie und Monat erreicht"
		}
	case "income_level":
		return "Einkommensstufe, " + careRuleEffect(reduction)
	case "single_parent":
		return "Alleinerziehend, " + careRuleEffect(reduction)
	}
	return ""
}

func careRuleEffect(reduction parentbilling.CareReduction) string {
	switch {
	case reduction.TariffSiblingPrice:
		return "Geschwisterpreis laut Preisliste"
	case reduction.PriceCents != nil:
		return "Preis " + formatEuro(*reduction.PriceCents)
	case reduction.DiscountBasisPoints != nil:
		return strings.ReplaceAll(strconv.FormatFloat(float64(*reduction.DiscountBasisPoints)/100, 'f', -1, 64), ".", ",") + " % weniger"
	case reduction.DiscountCents != nil:
		return formatEuro(*reduction.DiscountCents) + " weniger"
	}
	return ""
}

// careCostShareFacts prints the subtotal of the contribution and one line
// per cost bearer share with its minus (#4687, ADR 0169); sign -1 prints the
// amounts of a Storno. An invoice without a share prints nothing.
func careCostShareFacts(invoice parentbilling.CareInvoice, sign int64) []LetterFact {
	if len(invoice.CostShares) == 0 {
		return nil
	}
	label := "Monatsbeitrag"
	if len(invoice.MealPositions) > 0 {
		// With the Mensa (#4686) the sum is more than the contribution.
		label = "Zwischensumme"
	}
	facts := []LetterFact{{Label: label, Value: formatEuro(sign * invoice.ContributionCents())}}
	for _, share := range invoice.CostShares {
		facts = append(facts, LetterFact{
			Label: fmt.Sprintf("Übernimmt %s (%s, %s)", strings.TrimSpace(share.CostBearerName),
				costShareKindLabels[share.Kind], costShareText(share)),
			Value: formatEuro(-sign * share.AmountCents),
		})
	}
	return facts
}

// careTotalLabel names the sum of an invoice: what the family pays, which
// is only a rest when a cost bearer pays a part.
func careTotalLabel(invoice parentbilling.CareInvoice) string {
	if len(invoice.CostShares) > 0 {
		return "Ihr Anteil"
	}
	return "Summe"
}

// costBearerNames names the cost bearers of the shares: "A" or "A und B".
func costBearerNames(shares []parentbilling.CareInvoiceCostShare) string {
	names := []string{}
	for _, share := range shares {
		names = append(names, strings.TrimSpace(share.CostBearerName))
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " und " + names[len(names)-1]
}
