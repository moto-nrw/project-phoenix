package application

import (
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The withdrawal notice of a distance contract (#4682, § 312g, § 356 BGB,
// Art. 246a § 1 Abs. 2 EGBGB) and the parents' wish that care begins within
// the withdrawal period (§ 356 Abs. 4, § 357a Abs. 2 BGB).
//
// There is one notice (#6105): the model of Anlage 1 zu Art. 246a § 1 Abs.
// 2 Satz 2 EGBGB for services, with the note on the withdrawal function of
// § 356a BGB (Gestaltungshinweis 3, since 19 June 2026). The enrollment
// form, the contract PDF, the signing page and the confirmation mail all
// print withdrawalNoticeParagraphs; only the addressee's contact details
// differ with what the surface knows. Federal law, so the text is the same
// in every Land.
//
// The notice names the withdrawal function by its statutory label „Vertrag
// widerrufen“ (§ 356a Abs. 1 Satz 2 BGB), not by a screen path, so a renamed
// menu never makes a frozen PDF wrong.

// withdrawalDays is carecontract.WithdrawalDays as the model spells it;
// TestWithdrawalDaysMatchTheModelWording ties both together.
const withdrawalDays = "vierzehn"

// EarlyStartText is the block the parents tick in the enrollment form when
// care shall begin within the withdrawal period. The conclusion at the
// acceptance finds it by this exact text.
const EarlyStartText = "Betreuung soll vor Ende der Widerrufsfrist beginnen\n\n" +
	"Ich verlange ausdrücklich, dass die Betreuung schon vor Ende der Widerrufsfrist beginnt. " +
	"Widerrufe ich den Vertrag, zahle ich für die Betreuung bis zum Widerruf einen angemessenen Betrag."

// withdrawalContact is the addressee of a withdrawal: the contract partner
// and, where known, the address lines, phone and e-mail the model asks for.
type withdrawalContact struct {
	Partner string
	Lines   []string
	Phone   string
	Email   string
}

// letterContact is the addressee the contract PDF names: the partner of the
// terms, else the letterhead's first line, with the letterhead's address.
func letterContact(terms carecontract.TemplateInput, head carecontract.Letterhead) withdrawalContact {
	contact := signingContact(contractPartner(terms, head), head.SenderText)
	contact.Phone, contact.Email = strings.TrimSpace(head.ContactPhone), strings.TrimSpace(head.ContactEmail)
	return contact
}

// signingContact is the addressee the signing page and the mail name; they
// know the letterhead's text only.
func signingContact(partner, letterhead string) withdrawalContact {
	contact := withdrawalContact{Partner: strings.TrimSpace(partner)}
	for _, line := range textLines(letterhead) {
		if line != contact.Partner {
			contact.Lines = append(contact.Lines, line)
		}
	}
	return contact
}

// details is the addressee in one line; withPhone adds the phone, which the
// withdrawal form does not ask for.
func (c withdrawalContact) details(withPhone bool) string {
	partner := c.Partner
	if partner == "" {
		partner = "dem Träger"
	}
	parts := append([]string{partner}, c.Lines...)
	if withPhone && c.Phone != "" {
		parts = append(parts, "Telefon: "+c.Phone)
	}
	if c.Email != "" {
		parts = append(parts, "E-Mail: "+c.Email)
	}
	return strings.Join(parts, ", ")
}

// withdrawalNoticeParagraphs is the model notice for a service contract.
func withdrawalNoticeParagraphs(contact withdrawalContact) []string {
	return []string{
		"Widerrufsrecht: Sie haben das Recht, binnen " + withdrawalDays + " Tagen ohne Angabe von Gründen diesen Vertrag zu widerrufen. " +
			"Die Widerrufsfrist beträgt " + withdrawalDays + " Tage ab dem Tag des Vertragsabschlusses.",
		"Um Ihr Widerrufsrecht auszuüben, müssen Sie uns (" + contact.details(true) + ") mittels einer eindeutigen Erklärung " +
			"(z. B. ein mit der Post versandter Brief oder E-Mail) über Ihren Entschluss, diesen Vertrag zu widerrufen, informieren. " +
			"Sie können dafür das beigefügte Muster-Widerrufsformular verwenden, das jedoch nicht vorgeschrieben ist.",
		"Sie können Ihr Widerrufsrecht auch online in der moto Eltern-App beim Vertrag mit der Schaltfläche „Vertrag widerrufen“ ausüben. " +
			"Wenn Sie diese Online-Funktion nutzen, übermitteln wir Ihnen auf einem dauerhaften Datenträger (z. B. durch eine E-Mail) " +
			"unverzüglich eine Eingangsbestätigung mit Informationen zum Inhalt der Widerrufserklärung sowie dem Datum und der Uhrzeit ihres Eingangs.",
		"Zur Wahrung der Widerrufsfrist reicht es aus, dass Sie die Mitteilung über die Ausübung des Widerrufsrechts vor Ablauf der Widerrufsfrist absenden.",
		"Folgen des Widerrufs: Wenn Sie diesen Vertrag widerrufen, haben wir Ihnen alle Zahlungen, die wir von Ihnen erhalten haben, " +
			"einschließlich der Lieferkosten (mit Ausnahme der zusätzlichen Kosten, die sich daraus ergeben, dass Sie eine andere Art der " +
			"Lieferung als die von uns angebotene, günstigste Standardlieferung gewählt haben), " +
			"unverzüglich und spätestens binnen " + withdrawalDays + " Tagen ab dem Tag zurückzuzahlen, an dem die Mitteilung über Ihren Widerruf " +
			"dieses Vertrags bei uns eingegangen ist. Für diese Rückzahlung verwenden wir dasselbe Zahlungsmittel, das Sie bei der ursprünglichen " +
			"Transaktion eingesetzt haben, es sei denn, mit Ihnen wurde ausdrücklich etwas anderes vereinbart; in keinem Fall werden Ihnen " +
			"wegen dieser Rückzahlung Entgelte berechnet.",
		"Haben Sie verlangt, dass die Dienstleistungen während der Widerrufsfrist beginnen sollen, so haben Sie uns einen angemessenen " +
			"Betrag zu zahlen, der dem Anteil der bis zu dem Zeitpunkt, zu dem Sie uns von der Ausübung des Widerrufsrechts hinsichtlich " +
			"dieses Vertrags unterrichten, bereits erbrachten Dienstleistungen im Vergleich zum Gesamtumfang der im Vertrag vorgesehenen " +
			"Dienstleistungen entspricht.",
	}
}

// withdrawalBlocks are the withdrawal notice, the parents' choice on an
// early start and the model withdrawal form (Anlage 2 zu Art. 246a § 1 Abs.
// 2 Satz 1 Nr. 1 EGBGB), as the contract PDF prints them.
func withdrawalBlocks(record ContractRecord, contact withdrawalContact) []LetterBlock {
	early := "Sie haben nicht verlangt, dass die Betreuung vor Ende der Widerrufsfrist beginnt."
	if record.EarlyStart {
		early = "Sie haben im Anmeldeformular ausdrücklich verlangt, dass die Betreuung vor Ende der Widerrufsfrist beginnt."
	}
	return []LetterBlock{
		{Heading: "Widerrufsbelehrung", Text: strings.Join(withdrawalNoticeParagraphs(contact), "\n\n")},
		{Heading: "Beginn vor Ende der Widerrufsfrist", Text: early},
		{Heading: "Muster-Widerrufsformular", Text: withdrawalForm("Betreuungsvertrag Nr. "+strconv.FormatInt(record.Summary.ID, 10), contact)},
	}
}

// withdrawalForm is the model form of Anlage 2 for a service, with the
// addressee and the service filled in.
func withdrawalForm(service string, contact withdrawalContact) string {
	return strings.Join([]string{
		"(Wenn Sie den Vertrag widerrufen wollen, dann füllen Sie bitte dieses Formular aus und senden Sie es zurück.)",
		"– An " + contact.details(false) + ":",
		"– Hiermit widerrufe(n) ich/wir (*) den von mir/uns (*) abgeschlossenen Vertrag über die Erbringung der folgenden Dienstleistung (*): " + service,
		"– Bestellt am (*)/erhalten am (*):",
		"– Name des/der Verbraucher(s):",
		"– Anschrift des/der Verbraucher(s):",
		"– Unterschrift des/der Verbraucher(s) (nur bei Mitteilung auf Papier):",
		"– Datum:",
		"(*) Unzutreffendes streichen.",
	}, "\n")
}

// earlyStartRequested tells whether the parents ticked the early start in
// the application, submitted before the school's acceptance.
func earlyStartRequested(draft NewContract) bool {
	for _, proof := range draft.Acceptances {
		if submittedAcceptance(draft, proof) && strings.TrimSpace(proof.Text) == EarlyStartText {
			return true
		}
	}
	return false
}

// withdrawalTexts are the texts the school copies into the enrollment form:
// the notice with the Träger's address from its letterhead and the model
// form it calls attached, and the early start.
func withdrawalTexts(terms carecontract.TemplateInput, head carecontract.Letterhead, offering string) (string, string) {
	if strings.TrimSpace(terms.ContractPartner) == "" {
		return "", ""
	}
	contact := letterContact(terms, head)
	notice := "Widerrufsbelehrung\n\n" + strings.Join(withdrawalNoticeParagraphs(contact), "\n\n") +
		"\n\nMuster-Widerrufsformular\n\n" + withdrawalForm("Betreuung im Angebot „"+strings.TrimSpace(offering)+"“", contact)
	return notice, EarlyStartText
}
