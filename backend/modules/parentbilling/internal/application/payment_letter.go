package application

import (
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// How a letter names the payment of an invoice (#4689, ADR 0167). An invoice
// issued since #4689 carries its own payment method and due date, which
// ResolvePayment decided; an older one takes the date the letters of its run
// set.

// paymentMethodLabels name the payment methods on a letter and in the export.
var paymentMethodLabels = map[string]string{
	parentbilling.PaymentTransfer:    "Überweisung",
	parentbilling.PaymentDirectDebit: "Lastschrift",
}

// paymentMethodLabel is the name of a payment method, empty for none.
func paymentMethodLabel(method string) string { return paymentMethodLabels[method] }

// dueOnOf is the due date of an invoice: its own, else the date of the
// letters of its run.
func dueOnOf(payment *parentbilling.InvoicePayment, letters calendar.Date) calendar.Date {
	if payment != nil && !payment.DueOn.IsZero() {
		return payment.DueOn
	}
	return letters
}

// isDirectDebit tells whether the Träger collects the amount.
func isDirectDebit(payment *parentbilling.InvoicePayment) bool {
	return payment != nil && payment.Method == parentbilling.PaymentDirectDebit
}

// paymentFacts are the facts of the payment beside the address field: the
// payment method of an invoice issued since #4689 and the date it is paid
// or collected.
func paymentFacts(payment *parentbilling.InvoicePayment, letters calendar.Date) []LetterFact {
	due := germanDate(dueOnOf(payment, letters))
	if payment == nil {
		return []LetterFact{{Label: "Zahlen bis", Value: due}}
	}
	if isDirectDebit(payment) {
		facts := []LetterFact{{Label: "Zahlungsweg", Value: paymentMethodLabel(payment.Method)}, {Label: "Abbuchung am", Value: due}}
		if mandate := payment.Mandate; mandate != nil {
			// The pre-notification of the SEPA direct debit (#4695, ADR 0171):
			// the account only as its mask.
			facts = append(facts, LetterFact{Label: "Konto", Value: mandate.IBANMasked},
				LetterFact{Label: "Mandatsreferenz", Value: mandate.Reference},
				LetterFact{Label: "Gläubiger-ID", Value: mandate.CreditorID})
		}
		return facts
	}
	return []LetterFact{{Label: "Zahlungsweg", Value: paymentMethodLabel(payment.Method)}, {Label: "Zahlen bis", Value: due}}
}

// directDebitText says when the Träger collects the amount; the payment
// text of the Träger asks for a transfer, so a direct debit never uses it.
// With its mandate the sentence is the pre-notification of the SEPA direct
// debit (#4695, ADR 0171): amount, date, the masked account, the creditor
// identifier and the mandate reference.
func directDebitText(amountCents int64, due calendar.Date, reference string, mandate *parentbilling.InvoiceMandate) string {
	if mandate == nil {
		return fmt.Sprintf("Wir buchen %s am %s von Ihrem Konto ab. Verwendungszweck: %s. Sie müssen nichts überweisen.",
			formatEuro(amountCents), germanDate(due), reference)
	}
	return fmt.Sprintf("Wir buchen %s am %s per SEPA-Lastschrift von Ihrem Konto %s ab. "+
		"Gläubiger-ID: %s. Mandatsreferenz: %s. Verwendungszweck: %s. "+
		"Sie müssen nichts überweisen. Bitte achten Sie darauf, dass das Konto an diesem Tag gedeckt ist.",
		formatEuro(amountCents), germanDate(due), mandate.IBANMasked, mandate.CreditorID, mandate.Reference, reference)
}

// paymentMandate is the mandate of a payment, nil without one.
func paymentMandate(payment *parentbilling.InvoicePayment) *parentbilling.InvoiceMandate {
	if payment == nil {
		return nil
	}
	return payment.Mandate
}

// billingRecipientLines are the lines of the address field: another payer
// by name with its address, both parents by both names, the family
// otherwise, then the address when the invoice has one.
func billingRecipientLines(kind, firstName, lastName, secondName string, address *parentbilling.PostalAddress) []string {
	var lines []string
	switch kind {
	case parentbilling.RecipientPayer:
		lines = []string{fullName(firstName, lastName)}
	case parentbilling.RecipientBoth:
		lines = []string{"Familie " + strings.TrimSpace(lastName), fullName(firstName, lastName) + " und " + strings.TrimSpace(secondName)}
	default:
		lines = []string{"Familie " + strings.TrimSpace(lastName), fullName(firstName, lastName)}
	}
	if address != nil {
		lines = append(lines, address.Street, strings.TrimSpace(address.PostalCode)+" "+strings.TrimSpace(address.City))
	}
	return nonBlank(lines...)
}

// billingSalutation greets the family, or another payer by name.
func billingSalutation(kind, firstName, lastName string) string {
	if kind == parentbilling.RecipientPayer {
		return "Guten Tag " + fullName(firstName, lastName) + ","
	}
	return "Sehr geehrte Familie " + strings.TrimSpace(lastName) + ","
}
