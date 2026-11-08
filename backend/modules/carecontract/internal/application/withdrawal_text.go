package application

import (
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The withdrawal notice of a distance contract (#4682, § 312g, § 356 BGB,
// Art. 246a § 1 Abs. 2 EGBGB with the model of its Anlage 1) and the
// parents' wish that care begins within the withdrawal period (§ 356 Abs.
// 4, § 357a Abs. 2 BGB). Both stand in the enrollment form, the school
// copies them there from the conditions page, and in every contract PDF.

var withdrawalDays = strconv.Itoa(carecontract.WithdrawalDays)

// EarlyStartText is the block the parents tick in the enrollment form when
// care shall begin within the withdrawal period. The conclusion at the
// acceptance finds it by this exact text.
const EarlyStartText = "Betreuung soll vor Ende der Widerrufsfrist beginnen\n\n" +
	"Ich verlange ausdrücklich, dass die Betreuung schon vor Ende der Widerrufsfrist beginnt. " +
	"Widerrufe ich den Vertrag, zahle ich für die Betreuung bis zum Widerruf einen angemessenen Betrag."

// WithdrawalNoticeText is the withdrawal notice of a contract with the
// Träger named partner.
func WithdrawalNoticeText(partner string) string {
	partner = strings.TrimSpace(partner)
	if partner == "" {
		partner = "dem Träger"
	}
	return "Widerrufsbelehrung\n\n" +
		"Widerrufsrecht: Sie haben das Recht, binnen " + withdrawalDays + " Tagen ohne Angabe von Gründen diesen Vertrag zu widerrufen. " +
		"Die Widerrufsfrist beträgt " + withdrawalDays + " Tage ab dem Tag des Vertragsabschlusses. " +
		"Die Frist beginnt erst, wenn Sie diese Belehrung erhalten haben.\n\n" +
		"Um Ihr Widerrufsrecht auszuüben, müssen Sie uns (" + partner + ") mittels einer eindeutigen Erklärung " +
		"(zum Beispiel per E-Mail, per Brief oder mit dem Knopf „Vertrag widerrufen“ in der moto Eltern-App) " +
		"über Ihren Entschluss, diesen Vertrag zu widerrufen, informieren. " +
		"Zur Wahrung der Widerrufsfrist reicht es aus, dass Sie die Mitteilung vor Ablauf der Widerrufsfrist absenden.\n\n" +
		"Folgen des Widerrufs: Wenn Sie diesen Vertrag widerrufen, haben wir Ihnen alle Zahlungen, die wir von Ihnen " +
		"erhalten haben, unverzüglich und spätestens binnen 14 Tagen ab dem Tag zurückzuzahlen, an dem die Mitteilung " +
		"über Ihren Widerruf bei uns eingegangen ist. Haben Sie verlangt, dass die Betreuung während der Widerrufsfrist " +
		"beginnen soll, so haben Sie uns einen angemessenen Betrag zu zahlen, der dem Anteil der bis zum Widerruf " +
		"erbrachten Betreuung im Vergleich zum Gesamtumfang des Vertrags entspricht."
}

// withdrawalBlocks are the withdrawal notice and the parents' choice on an
// early start, as the contract PDF prints them.
func withdrawalBlocks(record ContractRecord, partner string) []LetterBlock {
	notice := strings.SplitN(WithdrawalNoticeText(partner), "\n\n", 2)
	early := "Sie haben nicht verlangt, dass die Betreuung vor Ende der Widerrufsfrist beginnt."
	if record.EarlyStart {
		early = "Sie haben im Anmeldeformular ausdrücklich verlangt, dass die Betreuung vor Ende der Widerrufsfrist beginnt."
	}
	return []LetterBlock{
		{Heading: notice[0], Text: notice[1]},
		{Heading: "Beginn vor Ende der Widerrufsfrist", Text: early},
	}
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

// withdrawalTexts are the texts the school copies into the enrollment form.
func withdrawalTexts(terms carecontract.TemplateInput) (string, string) {
	if strings.TrimSpace(terms.ContractPartner) == "" {
		return "", ""
	}
	return WithdrawalNoticeText(terms.ContractPartner), EarlyStartText
}
