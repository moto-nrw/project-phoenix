package application

import (
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The text of the SEPA mandate the parents confirm online (#4696), version
// parentbilling.MandateTextVersion. It carries the mandatory content of the
// EPC SDD Core Rulebook (DS-01, 4.7.2): the title, the reference, the
// creditor with address and identifier, the recurring payment type, the
// authorisation and refund text in the wording of the German banks, the
// debtor with address and account, place and date. The IBAN appears as its
// mask; the full IBAN stays sealed with the mandate. The text is rendered
// the same way for the preview, the check of the confirmation and the copy
// the parents read later, so its checksum binds what they saw.

// mandateTextFields is everything the text names.
type mandateTextFields struct {
	CreditorName    string
	CreditorAddress string
	CreditorID      string
	Reference       string
	Holder          string
	Street          string
	PostalCode      string
	City            string
	IBANMasked      string
	// IBAN is the full account; only the sealed proof names it.
	IBAN     string
	BIC      string
	SignedOn calendar.Date
}

// renderMandateText renders the text of version sepa-core-de-1. Changing a
// character changes every checksum: a new wording needs a new version.
func renderMandateText(f mandateTextFields) string {
	lines := []string{
		"SEPA-Lastschriftmandat",
		"",
		"Zahlungsempfänger: " + f.CreditorName,
		"Anschrift: " + f.CreditorAddress,
		"Gläubiger-Identifikationsnummer: " + f.CreditorID,
		"Mandatsreferenz: " + f.Reference,
		"Zahlungsart: Wiederkehrende Zahlung",
		"",
		"Ich ermächtige " + f.CreditorName + ", Zahlungen von meinem Konto mittels Lastschrift einzuziehen. " +
			"Zugleich weise ich mein Kreditinstitut an, die von " + f.CreditorName + " auf mein Konto gezogenen Lastschriften einzulösen.",
		"",
		"Hinweis: Ich kann innerhalb von acht Wochen, beginnend mit dem Belastungsdatum, die Erstattung des belasteten Betrages verlangen. " +
			"Es gelten dabei die mit meinem Kreditinstitut vereinbarten Bedingungen.",
		"",
		"Kontoinhaber: " + f.Holder,
		"Anschrift: " + f.Street + ", " + f.PostalCode + " " + f.City,
		"IBAN: " + f.IBANMasked,
	}
	if f.BIC != "" {
		lines = append(lines, "BIC: "+f.BIC)
	}
	lines = append(lines, "",
		"Ort, Datum: "+f.City+", "+f.SignedOn.Format("02.01.2006"),
		"Bestätigt in der Eltern-App von moto durch den Kontoinhaber.")
	return strings.Join(lines, "\n")
}

// sealedIBANPrefix starts the line the sealed proof adds to the confirmed
// text: the full IBAN the mask stands for, so the proof binds exactly one
// account. The parents never see this line; it never leaves the owner.
const sealedIBANPrefix = "\nIBAN des Kontoinhabers (nur im versiegelten Nachweis): "

// sealedMandateText is the confirmed text with the line of the full IBAN.
func sealedMandateText(f mandateTextFields) string {
	return renderMandateText(f) + sealedIBANPrefix + f.IBAN
}

// confirmedText splits a sealed proof into the text the parents confirmed
// and the full IBAN; ok is false for a proof without that line.
func confirmedText(sealed string) (text, iban string, ok bool) {
	index := strings.LastIndex(sealed, sealedIBANPrefix)
	if index < 0 {
		return "", "", false
	}
	return sealed[:index], sealed[index+len(sealedIBANPrefix):], true
}

// oneLine joins the lines of an address with commas.
func oneLine(text string) string {
	var parts []string
	for _, line := range strings.Split(text, "\n") {
		if line = strings.Join(strings.Fields(line), " "); line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, ", ")
}
