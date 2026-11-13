package application

import (
	"fmt"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
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

// cancellationLetter writes the letter of a cancellation; a fee notice is
// lifted by an Aufhebungsbescheid (#4688).
func cancellationLetter(content letterContent) Letter {
	if content.document.DocumentKind == parentbilling.DocumentKindFeeNotice {
		return noticeCancellationLetter(content)
	}
	letter := correctionLetterFrame(content, "Storno")
	letter.Blocks = append([]LetterBlock{
		{Text: fmt.Sprintf("wir stornieren die Rechnung %s für „%s“. Grund: %s",
			invoiceReference(content), content.run.PeriodName, sentence(content.document.Reason))},
		{Facts: correctionFacts(content.document, "Storno gesamt")},
		{Text: "Diese Rechnung gilt nicht mehr. Sie müssen sie nicht bezahlen. Haben Sie schon bezahlt? Dann melden Sie sich bitte bei uns.", Bold: true},
	}, letter.Blocks...)
	return letter
}

// creditLetter writes the letter of a credit; a fee notice is lowered by an
// Änderungsbescheid (#4688).
func creditLetter(content letterContent) Letter {
	if content.document.DocumentKind == parentbilling.DocumentKindFeeNotice {
		return noticeCreditLetter(content)
	}
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
	return correctionFrame(content, kind, "Rechnung")
}

// correctionFrame is correctionLetterFrame for a document that refers to an
// invoice or, for a fee notice, to a Bescheid.
func correctionFrame(content letterContent, kind, reference string) Letter {
	document, head := content.document, content.letterhead
	sender := textLines(head.SenderText)
	facts := []LetterFact{{Label: kind + "-Nummer", Value: document.Number}, {Label: "Zum " + reference, Value: document.ReferenceNumber}}
	if reference == "Rechnung" {
		facts[1].Label = "Zur Rechnung"
	}
	for _, fact := range letterFacts(content) {
		if fact.Label != numberLabel(document.DocumentKind) && fact.Label != "Zahlen bis" {
			facts = append(facts, fact)
		}
	}
	link := " zum "
	if reference == "Rechnung" {
		link = " zur "
	}
	letter := Letter{
		Title:       kind + " " + document.Number + link + reference + " " + document.ReferenceNumber,
		AccentColor: head.AccentColor,
		Logo:        head.Logo,
		Sender:      sender,
		Recipient:   recipientLines(document),
		Facts:       facts,
		Subject:     kind + link + reference + " " + document.ReferenceNumber + ": " + content.run.PeriodName,
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

// noticeCancellationLetter writes the Aufhebungsbescheid of a fee notice:
// a Storno of a fee notice lifts the notice as a whole (#4688, ADR 0119
// addendum 2026-11-08). It keeps the number and the minus positions of the
// Storno and prints the Träger's legal remedy when the template has one.
func noticeCancellationLetter(content letterContent) Letter {
	letter := correctionFrame(content, "Aufhebungsbescheid", "Bescheid")
	letter.Blocks = append([]LetterBlock{
		{Text: fmt.Sprintf("wir heben den Gebührenbescheid %s für „%s“ auf. Grund: %s",
			invoiceReference(content), content.run.PeriodName, sentence(content.document.Reason))},
		{Facts: correctionFacts(content.document, "Aufgehoben gesamt")},
		{Text: "Der Bescheid gilt nicht mehr. Sie müssen den Betrag nicht bezahlen. Haben Sie schon bezahlt? Dann melden Sie sich bitte bei uns.", Bold: true},
	}, letter.Blocks...)
	letter.Blocks = append(letter.Blocks, correctionLegalRemedy(content)...)
	return letter
}

// noticeCreditLetter writes the Änderungsbescheid of a fee notice: a
// Gutschrift lowers the fee the notice set (#4688).
func noticeCreditLetter(content letterContent) Letter {
	letter := correctionFrame(content, "Änderungsbescheid", "Bescheid")
	left := fmt.Sprintf("Nach dem Bescheid %s müssen Sie jetzt noch %s bezahlen.", content.document.ReferenceNumber, formatEuro(content.leftCents))
	if content.leftCents <= 0 {
		left = fmt.Sprintf("Nach dem Bescheid %s müssen Sie nichts mehr bezahlen.", content.document.ReferenceNumber)
	}
	letter.Blocks = append([]LetterBlock{
		{Text: fmt.Sprintf("wir ändern den Gebührenbescheid %s für „%s“ und setzen den Beitrag um %s niedriger fest. Grund: %s",
			invoiceReference(content), content.run.PeriodName, formatEuro(-content.document.TotalCents), sentence(content.document.Reason))},
		{Facts: correctionFacts(content.document, "Weniger gesamt")},
		{Text: left + " Haben Sie schon mehr bezahlt? Dann melden Sie sich bitte bei uns.", Bold: true},
	}, letter.Blocks...)
	letter.Blocks = append(letter.Blocks, correctionLegalRemedy(content)...)
	return letter
}

// correctionLegalRemedy prints the legal remedy of the template of the fee
// notice when it has one. A notice that only lowers the fee needs none, so
// a missing one does not hold the Storno back.
func correctionLegalRemedy(content letterContent) []LetterBlock {
	template := content.letterhead.Template.LegalRemedy
	if strings.TrimSpace(template) == "" {
		return nil
	}
	values := letterValues(content)
	return legalRemedyBlocks(templates.Fill(templates.Template{LegalRemedy: template}, values).LegalRemedy)
}
