package application

import (
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The PDF of a Jahresbescheid (#6347). It prints page 1 as the page of the
// office shows it (ADR 0182 point 8): head with number, date and contact,
// the child, I. Berechnung, II. Abrechnung with the installments and the
// payment method, and the legal remedy the notice copied at its release.
// Nothing of the missing page 2 of MK-007 is printed: no twelve-rate
// schedule, no legal basis, no account balance.

// feeNoticeLetter writes the letter of one issued Jahresbescheid on the
// letterhead the Träger has when the original is stored.
func feeNoticeLetter(notice parentbilling.CareNotice, head parentbilling.Letterhead) Letter {
	change := notice.Kind == parentbilling.NoticeKindChange
	title := "Gebührenbescheid " + notice.Number
	if change {
		title = "Änderungsbescheid " + notice.Number
	}
	sender := textLines(head.SenderText)
	letter := Letter{
		Title:       title,
		AccentColor: head.AccentColor,
		Logo:        head.Logo,
		Sender:      sender,
		Recipient: billingRecipientLines(notice.Recipient.Kind, notice.Recipient.FirstName, notice.Recipient.LastName,
			notice.Recipient.SecondName, notice.Recipient.Address),
		Facts:      careNoticeFacts(notice, head),
		Subject:    fmt.Sprintf("%s für die Betreuung %d", strings.Fields(title)[0], notice.Year),
		Salutation: billingSalutation(notice.Recipient.Kind, notice.Recipient.FirstName, notice.Recipient.LastName),
		Closing:    nonBlank("Mit freundlichen Grüßen", firstLine(sender)),
		Footer:     nonBlank(append(textLines(head.FooterText), bankLine(head.BankAccount))...),
	}
	letter.Blocks = careNoticeBlocks(notice, head, change)
	return letter
}

func careNoticeFacts(notice parentbilling.CareNotice, head parentbilling.Letterhead) []LetterFact {
	facts := []LetterFact{{Label: "Bescheid-Nr.", Value: notice.Number}}
	if notice.IssuedOn != nil {
		facts = append(facts, LetterFact{Label: "Datum", Value: germanDate(*notice.IssuedOn)})
	}
	facts = append(facts,
		LetterFact{Label: "Kind", Value: fullName(notice.ChildFirstName, notice.ChildLastName)},
		LetterFact{Label: "Zeitraum", Value: noticePeriodText(notice.PeriodFrom, notice.PeriodUntil)})
	if notice.Amends != nil {
		facts = append(facts, LetterFact{Label: "Ändert Bescheid", Value: noticeReferenceText(*notice.Amends)})
	}
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

func careNoticeBlocks(notice parentbilling.CareNotice, head parentbilling.Letterhead, change bool) []LetterBlock {
	child, school := fullName(notice.ChildFirstName, notice.ChildLastName), strings.TrimSpace(notice.SchoolName)
	intro := fmt.Sprintf("für die Betreuung von %s an der %s setzen wir den Beitrag für %s fest.",
		child, school, noticePeriodText(notice.PeriodFrom, notice.PeriodUntil))
	if change && notice.Amends != nil {
		intro = fmt.Sprintf("für die Betreuung von %s an der %s ändern wir den Bescheid %s. Dieser Bescheid gilt ab jetzt für die genannten Monate. Die Spalte „bisher“ zeigt, was der Bescheid %s festgesetzt hat.",
			child, school, noticeReferenceText(*notice.Amends), notice.Amends.Number)
	}
	totals := []LetterFact{}
	if change {
		totals = append(totals, LetterFact{Label: "Bisher festgesetzt", Value: formatEuro(notice.PreviousCents)},
			LetterFact{Label: "Zu-/Abgang", Value: formatSignedEuro(notice.DeltaCents)})
	}
	totals = append(totals, LetterFact{Label: "Gesamtbetrag", Value: formatEuro(notice.TotalCents), Bold: true})
	blocks := []LetterBlock{
		{Text: intro},
		{Heading: "I. Berechnung", Text: "Je Berechnungsgrundlage und Zeitraum mit gleichem Monatsbetrag eine Zeile.",
			Facts: append(careNoticeLineFacts(notice.Lines, change), totals...)},
		{Heading: "II. Abrechnung", Text: "Der Gesamtbetrag ist in Monatsraten zum Monatsersten fällig. Raten für Monate, die schon begonnen haben, sind sofort fällig. " +
			careNoticePaymentText(notice), Facts: careNoticeInstallmentFacts(notice.Installments, change)},
	}
	if question := questionText(head); question != "" {
		blocks = append(blocks, LetterBlock{Text: question})
	}
	if remedy := strings.TrimSpace(filledLegalRemedy(notice, head)); remedy != "" {
		blocks = append(blocks, LetterBlock{Heading: legalRemedyHeading, Text: remedy})
	}
	return blocks
}

// careNoticeLineFacts prints one line per basis and span: label, detail,
// span and monthly amount; the value is the amount of the span, on a change
// notice with bisher and Zu-/Abgang.
func careNoticeLineFacts(lines []parentbilling.CareNoticeLine, change bool) []LetterFact {
	facts := make([]LetterFact, 0, len(lines))
	for _, line := range lines {
		label := strings.TrimSpace(line.Label)
		if detail := strings.TrimSpace(line.Detail); detail != "" {
			label += " (" + detail + ")"
		}
		label += fmt.Sprintf(", %s, %d × %s", noticePeriodText(line.PeriodFrom, line.PeriodUntil), line.Months,
			formatEuro(line.MonthlyCents))
		value := formatEuro(line.AnnualCents)
		if change {
			label += fmt.Sprintf(", bisher %d × %s", line.Months, formatEuro(line.MonthlyPreviousCents))
			value = fmt.Sprintf("bisher %s, neu %s, %s", formatEuro(line.AnnualPreviousCents), value, formatSignedEuro(line.DeltaCents))
		}
		facts = append(facts, LetterFact{Label: label, Value: value})
	}
	return facts
}

func careNoticeInstallmentFacts(installments []parentbilling.CareNoticeInstallment, change bool) []LetterFact {
	facts := make([]LetterFact, 0, len(installments))
	for _, rate := range installments {
		due := "fällig am " + germanDate(rate.DueOn)
		if rate.ImmediatelyDue {
			due = "sofort fällig (" + germanDate(rate.DueOn) + ")"
		}
		label := "Rate " + parentbilling.CareMonthName(rate.Month) + ", " + due
		value := formatEuro(rate.AmountCents)
		if change {
			value = fmt.Sprintf("bisher %s, neu %s", formatEuro(rate.PreviousCents), value)
		}
		facts = append(facts, LetterFact{Label: label, Value: value})
	}
	return facts
}

// careNoticePaymentText says what the family does with the rates.
func careNoticePaymentText(notice parentbilling.CareNotice) string {
	if notice.PaymentMethod == parentbilling.PaymentDirectDebit {
		return "Die fälligen Raten werden per SEPA-Lastschrift eingezogen. Bitte nicht überweisen."
	}
	return "Bitte überweisen Sie jede Rate bis zum Fälligkeitstag auf das Konto unten. Geben Sie dabei die Bescheid-Nr. " +
		notice.Number + " an."
}

func noticePeriodText(from, until calendar.Date) string {
	return germanDate(from) + " bis " + germanDate(until)
}

func noticeReferenceText(reference parentbilling.CareNoticeReference) string {
	if reference.IssuedOn == nil {
		return reference.Number
	}
	return reference.Number + " vom " + germanDate(*reference.IssuedOn)
}

// formatSignedEuro prints a Zu- or Abgang with its sign: "+32,00 €".
func formatSignedEuro(cents int64) string {
	if cents > 0 {
		return "+" + formatEuro(cents)
	}
	return formatEuro(cents)
}

// careNoticeFilename names the PDF by kind and number.
func careNoticeFilename(notice parentbilling.CareNotice) string {
	if notice.Kind == parentbilling.NoticeKindChange {
		return "aenderungsbescheid-" + notice.Number
	}
	return "gebuehrenbescheid-" + notice.Number
}
