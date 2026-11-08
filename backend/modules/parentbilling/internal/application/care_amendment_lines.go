package application

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The correction lines of the changes of the review (#6197) as the letter
// and the review list show them: after the cost bearer shares, so the lines
// above add up to the total. A recipient or address change adds no line.

// careCorrectionLine names a correction line: an amount change by its
// reason like the holiday corrections, a position by its label.
func careCorrectionLine(amendment parentbilling.CareAmendment) string {
	if amendment.Kind == parentbilling.AmendAmount {
		return "Korrektur: " + strings.TrimSpace(amendment.Reason)
	}
	return strings.TrimSpace(amendment.Label)
}

// careCorrectionLines are the applied changes of an invoice that add a line.
func careCorrectionLines(invoice parentbilling.CareInvoice) []parentbilling.CareAmendment {
	lines := []parentbilling.CareAmendment{}
	for _, amendment := range invoice.Amendments {
		switch amendment.Kind {
		case parentbilling.AmendAmount, parentbilling.AmendAddPosition, parentbilling.AmendRemovePosition:
			if amendment.Applied() {
				lines = append(lines, amendment)
			}
		}
	}
	return lines
}

// careCorrectionFacts are the correction lines of a letter; sign -1 lists
// them with a minus on a Storno.
func careCorrectionFacts(invoice parentbilling.CareInvoice, sign int64) []LetterFact {
	facts := []LetterFact{}
	for _, line := range careCorrectionLines(invoice) {
		facts = append(facts, LetterFact{Label: careCorrectionLine(line), Value: formatEuro(sign * line.AmountCents)})
	}
	return facts
}

// reviewCorrectionRows are the correction lines of an invoice in the review
// list, one row each, with the reason.
func reviewCorrectionRows(base map[string]string, invoice parentbilling.CareInvoice) []map[string]string {
	rows := []map[string]string{}
	for _, line := range careCorrectionLines(invoice) {
		position := careCorrectionLine(line)
		if line.Kind != parentbilling.AmendAmount {
			// The review list carries every reason; the letter keeps the label.
			position += " (Grund: " + strings.TrimSpace(line.Reason) + ")"
		}
		rows = append(rows, withValues(base, map[string]string{
			"position": position, "amount": formatAmount(line.AmountCents),
		}))
	}
	return rows
}
