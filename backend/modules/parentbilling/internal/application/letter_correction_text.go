package application

import (
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The letters of a cancellation (Storno) and a credit (Gutschrift) of an
// issued invoice (#4690). They keep the layout, the address field and the
// contact of the confirmation letter, name the invoice they belong to and
// the reason, and list their positions with a minus. Neither asks for a
// payment; the credit says what the invoice still asks for. Who has paid
// more already contacts the Träger, because moto does not know the
// payments.

// runLetterContent is what the letter of one document of a run prints. The
// letter of a cancellation or a credit finds its invoice among the
// documents of the run by its reference and counts what the invoice asks
// for after this document and the ones issued before it. Issuance order is
// the order of the IDs, not of the numbers: a pattern without a year sorts
// a later credit before its invoice.
func runLetterContent(run parentbilling.BillingRun, documents []parentbilling.BillingDocument, document parentbilling.BillingDocument,
	input parentbilling.BillingLetterInput, head parentbilling.Letterhead,
) letterContent {
	content := letterContent{run: run, document: document, input: input, letterhead: head}
	if document.Kind == parentbilling.DocumentInvoice || document.ReferenceDocumentID == nil {
		return content
	}
	invoice, ok := findDocument(documents, *document.ReferenceDocumentID)
	if !ok {
		return content
	}
	issuedUpTo := []parentbilling.BillingDocument{document}
	for _, other := range documents {
		if other.ID < document.ID {
			issuedUpTo = append(issuedUpTo, other)
		}
	}
	content.invoice, content.leftCents = &invoice, leftOf(invoice, issuedUpTo)
	return content
}

// cancellationLetter writes the letter of a cancellation.
func cancellationLetter(content letterContent) Letter {
	letter := correctionLetterFrame(content, "Storno")
	letter.Blocks = append([]LetterBlock{
		{Text: fmt.Sprintf("wir stornieren die Rechnung %s für „%s“. Grund: %s",
			invoiceReference(content), content.run.PeriodName, sentence(content.document.Reason))},
		{Facts: correctionFacts(content.document, "Storno gesamt")},
		{Text: "Diese Rechnung gilt nicht mehr. Sie müssen sie nicht bezahlen. Haben Sie schon bezahlt? Dann melden Sie sich bitte bei uns.", Bold: true},
	}, letter.Blocks...)
	return letter
}

// creditLetter writes the letter of a credit.
func creditLetter(content letterContent) Letter {
	letter := correctionLetterFrame(content, "Gutschrift")
	left := fmt.Sprintf("Für die Rechnung %s müssen Sie jetzt noch %s bezahlen.", content.document.ReferenceNumber, formatEuro(content.leftCents))
	if content.leftCents <= 0 {
		left = fmt.Sprintf("Für die Rechnung %s müssen Sie nichts mehr bezahlen.", content.document.ReferenceNumber)
	}
	letter.Blocks = append([]LetterBlock{
		{Text: fmt.Sprintf("zur Rechnung %s für „%s“ schreiben wir Ihnen %s gut. Grund: %s",
			invoiceReference(content), content.run.PeriodName, formatEuro(-content.document.TotalCents), sentence(content.document.Reason))},
		{Facts: correctionFacts(content.document, "Gutschrift gesamt")},
		{Text: left + " Haben Sie schon mehr bezahlt? Dann melden Sie sich bitte bei uns.", Bold: true},
	}, letter.Blocks...)
	return letter
}

// correctionLetterFrame is the letter of a cancellation or a credit
// without its body: the layout of the Träger, the address field, the facts
// and the question block at the end.
func correctionLetterFrame(content letterContent, kind string) Letter {
	document, head := content.document, content.letterhead
	sender := textLines(head.SenderText)
	facts := []LetterFact{{Label: kind + "-Nummer", Value: document.Number}, {Label: "Zur Rechnung", Value: document.ReferenceNumber}}
	for _, fact := range letterFacts(content) {
		if fact.Label != "Rechnungsnummer" && fact.Label != "Zahlen bis" {
			facts = append(facts, fact)
		}
	}
	letter := Letter{
		Title:       kind + " " + document.Number + " zur Rechnung " + document.ReferenceNumber,
		AccentColor: head.AccentColor,
		Logo:        head.Logo,
		Sender:      sender,
		Recipient:   recipientLines(document),
		Facts:       facts,
		Subject:     kind + " zur Rechnung " + document.ReferenceNumber + ": " + content.run.PeriodName,
		Salutation:  "Sehr geehrte Familie " + strings.TrimSpace(document.RecipientLastName) + ",",
		Closing:     nonBlank("Mit freundlichen Grüßen", firstLine(sender)),
		Footer:      nonBlank(append(textLines(head.FooterText), bankLine(head.BankAccount))...),
	}
	if question := questionText(head); question != "" {
		letter.Blocks = []LetterBlock{{Text: question}}
	}
	return letter
}

// invoiceReference names the invoice with its date when the letter knows
// it: "FE-2026-0001 vom 01.12.2026".
func invoiceReference(content letterContent) string {
	if content.invoice != nil && content.invoice.IssuedOn != nil {
		return content.document.ReferenceNumber + " vom " + germanDate(*content.invoice.IssuedOn)
	}
	return content.document.ReferenceNumber
}

// correctionFacts lists every position by child with its minus and closes
// with the total.
func correctionFacts(document parentbilling.BillingDocument, totalLabel string) []LetterFact {
	facts := []LetterFact{}
	for _, child := range document.Children {
		for _, position := range child.Positions {
			label := positionLabel(position)
			if position.Kind == parentbilling.LineCredit {
				label = "Gutschrift"
			}
			facts = append(facts, LetterFact{Label: strings.TrimSpace(child.FirstName) + ": " + label, Value: formatEuro(position.AmountCents)})
		}
	}
	return append(facts, LetterFact{Label: totalLabel, Value: formatEuro(document.TotalCents), Bold: true})
}

// sentence ends a reason with a full stop unless it has its own mark.
func sentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || strings.ContainsAny(text[len(text)-1:], ".!?") {
		return text
	}
	return text + "."
}
