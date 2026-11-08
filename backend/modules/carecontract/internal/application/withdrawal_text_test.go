package application

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The parents read one withdrawal notice (#6105): the signing page, the
// confirmation mail and the contract PDF carry the same paragraphs, in the
// wording of the statutory model with the withdrawal function of § 356a BGB.
func TestOneWithdrawalNoticeOnEverySurface(t *testing.T) {
	t.Parallel()
	head := carecontract.Letterhead{SenderText: "Träger Muster e. V.\nHauptstraße 1\n12345 Musterstadt"}
	contact := letterContact(carecontract.TemplateInput{ContractPartner: "Träger Muster e. V."}, head)
	signing := signingContact("Träger Muster e. V.", head.SenderText)

	assert.Equal(t, withdrawalNoticeParagraphs(signing), withdrawalNoticeParagraphs(contact),
		"signing page and mail print what the PDF prints")
	blocks := withdrawalBlocks(ContractRecord{Summary: carecontract.ContractSummary{ID: 42}}, contact)
	require.Len(t, blocks, 3)
	assert.Equal(t, "Widerrufsbelehrung", blocks[0].Heading)
	assert.Equal(t, strings.Join(withdrawalNoticeParagraphs(contact), "\n\n"), blocks[0].Text)

	text := strings.Join(withdrawalNoticeParagraphs(contact), "\n")
	assert.Contains(t, text, "Die Widerrufsfrist beträgt 14 Tage ab dem Tag des Vertragsabschlusses.")
	assert.Contains(t, text, "(Träger Muster e. V., Hauptstraße 1, 12345 Musterstadt)", "the model names the addressee with the address")
	assert.Contains(t, text, "Sie können Ihr Widerrufsrecht auch online in der moto Eltern-App beim Vertrag mit der Schaltfläche „Vertrag widerrufen“ ausüben.",
		"the notice names the statutory label of the withdrawal function, not a screen path")
	assert.Contains(t, text, "Eingangsbestätigung mit Informationen zum Inhalt der Widerrufserklärung sowie dem Datum und der Uhrzeit ihres Eingangs")
	assert.NotContains(t, text, "an dem Sie den Vertrag unterschreiben", "no own wording beside the model")
	assert.NotContains(t, text, "Die Frist beginnt erst", "no own wording beside the model")
}

func TestContractPDFCarriesTheModelWithdrawalForm(t *testing.T) {
	t.Parallel()
	head := carecontract.Letterhead{SenderText: "Träger Muster e. V.\nHauptstraße 1\n12345 Musterstadt",
		ContactPhone: "0123 456", ContactEmail: "vertrag@example.org"}
	blocks := withdrawalBlocks(ContractRecord{Summary: carecontract.ContractSummary{ID: 42}}, letterContact(carecontract.TemplateInput{}, head))
	form := blocks[2]
	assert.Equal(t, "Muster-Widerrufsformular", form.Heading)
	assert.Contains(t, form.Text, "An Träger Muster e. V., Hauptstraße 1, 12345 Musterstadt, E-Mail: vertrag@example.org:")
	assert.Contains(t, form.Text, "die Erbringung der folgenden Dienstleistung (*): Betreuungsvertrag Nr. 42")
	assert.Contains(t, form.Text, "Unterschrift des/der Verbraucher(s) (nur bei Mitteilung auf Papier)")
	notice := blocks[0].Text
	assert.Contains(t, notice, "Telefon: 0123 456", "the notice names the phone the model asks for")
	assert.Contains(t, notice, "Sie können dafür das beigefügte Muster-Widerrufsformular verwenden")
}

func TestEnrollmentFormCopiesTheSameNotice(t *testing.T) {
	t.Parallel()
	notice, early := withdrawalTexts(carecontract.TemplateInput{ContractPartner: "Träger Muster e. V."})
	assert.Equal(t, EarlyStartText, early)
	assert.Equal(t, "Widerrufsbelehrung\n\n"+strings.Join(withdrawalNoticeParagraphs(withdrawalContact{Partner: "Träger Muster e. V."}), "\n\n"), notice)
}

// The inbox shows „Vertrag widerrufen“ while the period may run (#6105).
func TestInboxShowsWithdrawalWhileThePeriodMayRun(t *testing.T) {
	t.Parallel()
	signed := time.Date(2026, 11, 2, 10, 0, 0, 0, calendar.Berlin) // Monday
	online := &carecontract.Concluded{Method: carecontract.MethodElectronic, At: signed}
	assert.True(t, withdrawalMayRun(online, &signed, calendar.NewDate(2026, 11, 16)), "the 14th day after signing")
	assert.False(t, withdrawalMayRun(online, &signed, calendar.NewDate(2026, 11, 17)), "the period is over")
	assert.False(t, withdrawalMayRun(nil, nil, calendar.NewDate(2026, 11, 3)), "no conclusion, no withdrawal")

	paper := &carecontract.Concluded{Method: carecontract.MethodPaper, At: signed}
	assert.True(t, withdrawalMayRun(paper, nil, calendar.NewDate(2027, 11, 16)),
		"without a recorded notice the function stays until the right expires")
	assert.False(t, withdrawalMayRun(paper, nil, calendar.NewDate(2027, 11, 17)), "the right expired")
}
