package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// hardtLegalRemedy is the legal remedy a Träger writes into its fee notice,
// after the notice of the Gemeinde Hardt (MK-007, #4688). moto writes no
// legal text of its own; the test only stands in for the Träger.
const hardtLegalRemedy = "Gegen diesen Bescheid kann innerhalb eines Monats nach Bekanntgabe Widerspruch erhoben werden.\n\n" +
	"Der Widerspruch ist bei {{traeger}} einzulegen. Die Frist ist auch gewahrt, wenn der Widerspruch beim Landratsamt Rottweil eingelegt wird."

// hardtNoticeTemplate is a fee notice template of the Träger in the order of
// page 1 of MK-007: what the notice sets, the table of amounts, the
// Buchungszeichen to pay with and the legal remedy.
func hardtNoticeTemplate() parentbilling.LetterTemplate {
	return parentbilling.LetterTemplate{
		Version: 2, Subject: "Gebührenbescheid {{bescheidnummer}} über die Betreuungsgebühren",
		Salutation: "Sehr geehrte Familie {{familienname}},",
		Body: "für die Betreuung von {{kinder}} setzen wir für {{zeitraum}} folgende Gebühren fest:\n{{positionen}}\n\n" +
			"**{{zahlungshinweis}}**\n\nBitte geben Sie bei allen Zahlungen das Buchungszeichen {{debitorennummer}} an.",
		Closing:     "Mit freundlichen Grüßen\n{{traeger}}",
		LegalRemedy: hardtLegalRemedy,
	}
}

func noticeContent(t *testing.T) letterContent {
	t.Helper()
	content := goldenLetterContent(t)
	content.document.DocumentKind = parentbilling.DocumentKindFeeNotice
	content.document.Number = "GB-2026-0007"
	content.document.DebtorNumber = "5.0311.000123.4"
	content.letterhead.SenderText = "Gemeinde Hardt\nHauptstraße 1\n78739 Hardt"
	content.letterhead.Template = hardtNoticeTemplate()
	return content
}

func blockText(letter Letter) string {
	parts := []string{letter.Title, letter.Subject, letter.Salutation}
	for _, fact := range letter.Facts {
		parts = append(parts, fact.Label+": "+fact.Value)
	}
	for _, block := range letter.Blocks {
		parts = append(parts, "# "+block.Heading, block.Text)
		for _, fact := range block.Facts {
			parts = append(parts, fact.Label+": "+fact.Value)
		}
	}
	return strings.Join(append(parts, letter.Closing...), "\n")
}

// TestFeeNoticeFollowsPageOneOfTheHardtNotice is the golden test of a holiday
// fee notice against page 1 of MK-007 (#4688): the title and the number of
// a notice, the Buchungszeichen beside the address field, the positions and
// the total, the payment, and the legal remedy of the Träger last, under its
// heading, after every text of the template.
func TestFeeNoticeFollowsPageOneOfTheHardtNotice(t *testing.T) {
	t.Parallel()
	letter := confirmationLetter(noticeContent(t))
	assert.Equal(t, "Gebührenbescheid GB-2026-0007", letter.Title)
	assert.Equal(t, "Gebührenbescheid GB-2026-0007 über die Betreuungsgebühren", letter.Subject)
	assert.Equal(t, []LetterFact{
		{Label: "Bescheidnummer", Value: "GB-2026-0007"}, {Label: "Buchungszeichen", Value: "5.0311.000123.4"},
		{Label: "Datum", Value: "01.12.2026"}, {Label: "Zahlen bis", Value: "15.12.2026"},
		{Label: "Ansprechperson", Value: "Paula Kern"}, {Label: "Telefon", Value: "05402 1234-0"},
		{Label: "E-Mail", Value: "ferien@ganztag.example"},
	}, letter.Facts)
	require.GreaterOrEqual(t, len(letter.Blocks), 4)
	last := letter.Blocks[len(letter.Blocks)-2:]
	assert.Equal(t, []LetterBlock{
		{Heading: "Rechtsbehelfsbelehrung", Text: "Gegen diesen Bescheid kann innerhalb eines Monats nach Bekanntgabe Widerspruch erhoben werden."},
		{Text: "Der Widerspruch ist bei Gemeinde Hardt einzulegen. Die Frist ist auch gewahrt, wenn der Widerspruch beim Landratsamt Rottweil eingelegt wird."},
	}, last)
	text := blockText(letter)
	for _, want := range []string{
		"für die Betreuung von Lina und Ben setzen wir für Winterferien 2027 folgende Gebühren fest:",
		"Gesamtbetrag: 92,50 €",
		"Bitte geben Sie bei allen Zahlungen das Buchungszeichen 5.0311.000123.4 an.",
		"Geben Sie als Verwendungszweck Ihr Buchungszeichen 5.0311.000123.4 an.",
	} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "Bestätigung", "a fee notice is no confirmation letter")
	assert.Empty(t, missingLetterValuesOf(templateKindOf(parentbilling.DocumentKindFeeNotice), letterValues(noticeContent(t))))
}

// TestFeeNoticeWithTheStandardTemplateStillPrintsTheLegalRemedy: the
// standard text of the fee notice has no legal remedy, the Träger's one is
// printed with it all the same.
func TestFeeNoticeWithTheStandardTemplateStillPrintsTheLegalRemedy(t *testing.T) {
	t.Parallel()
	content := noticeContent(t)
	content.letterhead.Template = parentbilling.LetterTemplate{LegalRemedy: "# Rechtsbehelf\nWiderspruch binnen eines Monats."}
	letter := confirmationLetter(content)
	assert.Equal(t, LetterBlock{Heading: "Rechtsbehelf", Text: "Widerspruch binnen eines Monats."}, letter.Blocks[len(letter.Blocks)-1])
	assert.Equal(t, "Bescheid über den Elternbeitrag GB-2026-0007", letter.Subject)
}

// TestInvoiceKindWritesAnInvoiceWithoutConfirmation: the holiday care can
// issue a plain invoice (#4688) with the template of the invoice.
func TestInvoiceKindWritesAnInvoiceWithoutConfirmation(t *testing.T) {
	t.Parallel()
	content := goldenLetterContent(t)
	content.document.DocumentKind = parentbilling.DocumentKindInvoice
	letter := confirmationLetter(content)
	assert.Equal(t, "Rechnung FE-2026-0007", letter.Title)
	assert.Equal(t, "Rechnung FE-2026-0007: Winterferien 2027", letter.Subject)
	assert.Equal(t, LetterFact{Label: "Rechnungsnummer", Value: "FE-2026-0007"}, letter.Facts[0])
	text := blockText(letter)
	assert.Contains(t, text, "für Lina und Ben berechnen wir Ihnen:")
	assert.NotContains(t, text, "Rechtsbehelfsbelehrung")
	assert.Contains(t, text, "Notfallnummern", "the invoice keeps the emergency numbers entered for its letter")
	assert.Equal(t, "rechnung-", letterFilePrefix(parentbilling.DocumentKindInvoice))
}

// TestConfirmationKindStaysTheLetterOfBefore: a document issued before
// #4688 has no kind and writes the confirmation letter.
func TestConfirmationKindStaysTheLetterOfBefore(t *testing.T) {
	t.Parallel()
	before := confirmationLetter(goldenLetterContent(t))
	content := goldenLetterContent(t)
	content.document.DocumentKind = parentbilling.DocumentKindConfirmation
	assert.Equal(t, before, confirmationLetter(content))
	assert.Equal(t, "Bestätigung und Rechnung FE-2026-0007", before.Title)
}

// TestStornoOfAFeeNoticeIsAnAufhebungsbescheid: a fee notice is lifted by a
// notice, not cancelled like an invoice (#4688, ADR 0119 addendum).
func TestStornoOfAFeeNoticeIsAnAufhebungsbescheid(t *testing.T) {
	t.Parallel()
	content := correctionContent(t, parentbilling.DocumentCancellation)
	content.invoice.DocumentKind = parentbilling.DocumentKindFeeNotice
	content.document.DocumentKind = parentbilling.DocumentKindFeeNotice
	content.letterhead.Template = hardtNoticeTemplate()
	letter := cancellationLetter(content)
	assert.Equal(t, "Aufhebungsbescheid FE-2026-0012 zum Bescheid FE-2026-0007", letter.Title)
	assert.Equal(t, []LetterFact{{Label: "Aufhebungsbescheid-Nummer", Value: "FE-2026-0012"}, {Label: "Zum Bescheid", Value: "FE-2026-0007"}},
		letter.Facts[:2])
	text := blockText(letter)
	assert.Contains(t, text, "wir heben den Gebührenbescheid FE-2026-0007 vom 01.12.2026 für „Winterferien 2027“ auf. Grund: Anschrift falsch, neue Rechnung folgt.")
	assert.Contains(t, text, "Aufgehoben gesamt: -102,50 €")
	assert.Contains(t, text, "# Rechtsbehelfsbelehrung")
	assert.NotContains(t, text, "Storno")

	content.letterhead.Template.LegalRemedy = ""
	assert.NotContains(t, blockText(cancellationLetter(content)), "Rechtsbehelfsbelehrung",
		"a notice that only lowers the fee prints no remedy it does not have")
}

// TestCreditOnAFeeNoticeIsAnAenderungsbescheid lowers the fee by a notice.
func TestCreditOnAFeeNoticeIsAnAenderungsbescheid(t *testing.T) {
	t.Parallel()
	content := correctionContent(t, parentbilling.DocumentCredit)
	content.document.DocumentKind = parentbilling.DocumentKindFeeNotice
	content.letterhead.Template = hardtNoticeTemplate()
	text := blockText(creditLetter(content))
	assert.Contains(t, text, "Änderungsbescheid FE-2026-0012 zum Bescheid FE-2026-0007")
	assert.Contains(t, text, "setzen den Beitrag um 20,00 € niedriger fest. Grund: Ausflug fiel aus.")
	assert.Contains(t, text, "Nach dem Bescheid FE-2026-0007 müssen Sie jetzt noch 72,50 € bezahlen.")
	assert.Contains(t, text, "# Rechtsbehelfsbelehrung")
}

// TestCareFeeNoticeFillsTheTemplateAndItsLegalRemedy: the monthly care can
// issue a fee notice in the Träger's template (#4688).
func TestCareFeeNoticeFillsTheTemplateAndItsLegalRemedy(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.invoice.DocumentKind = parentbilling.DocumentKindFeeNotice
	content.letterhead.Template = hardtNoticeTemplate()
	letter := careNoticeLetter(content)
	assert.Equal(t, "Gebührenbescheid MB-2027-0001", letter.Title)
	assert.Equal(t, LetterFact{Label: "Bescheidnummer", Value: "MB-2027-0001"}, letter.Facts[0])
	text := blockText(letter)
	for _, want := range []string{
		"Gebührenbescheid MB-2027-0001 über die Betreuungsgebühren",
		"für die Betreuung von Mia Beispiel setzen wir für Januar 2027 folgende Gebühren fest:",
		"Betreuung bis 14:00 Uhr (Mo): 32,00 €",
		"Gesamtbetrag: 54,01 €",
		"# Rechtsbehelfsbelehrung",
		"Der Widerspruch ist bei SJR Pforzheim einzulegen.",
		"Verwendungszweck die Bescheidnummer MB-2027-0001",
	} {
		assert.Contains(t, text, want)
	}
	assert.Empty(t, missingLetterValuesOf(templateKindOf(parentbilling.DocumentKindFeeNotice), careNoticeValues(content)))
	assert.NotContains(t, text, "Rechnungsnummer", "a notice names no invoice number")

	cancellation := parentbilling.CareCancellation{Number: "MB-2027-0009", Reason: "Vertrag endete", TotalCents: -5401,
		IssuedOn: *content.invoice.IssuedOn}
	content.cancellation = &cancellation
	text = blockText(careNoticeCancellationLetter(content))
	assert.Contains(t, text, "Aufhebungsbescheid MB-2027-0009 zum Bescheid MB-2027-0001")
	assert.Contains(t, text, "wir heben den Gebührenbescheid MB-2027-0001 vom 04.01.2027")
	assert.Contains(t, text, "# Rechtsbehelfsbelehrung")
}

// TestMailOfAFeeNoticeSaysWhatItIs names the notice in subject and mail.
func TestMailOfAFeeNoticeSaysWhatItIs(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mail              parentbilling.BillingLetterMail
		subject, template string
	}{
		{parentbilling.BillingLetterMail{LetterKind: parentbilling.DocumentKindFeeNotice}, "Ihr Gebührenbescheid", noticeLetterMailTemplate},
		{parentbilling.BillingLetterMail{LetterKind: parentbilling.DocumentKindFeeNotice, DocumentKind: parentbilling.DocumentCancellation},
			"Aufhebung Ihres Gebührenbescheids", noticeCorrectionMailTemplate},
		{parentbilling.BillingLetterMail{LetterKind: parentbilling.DocumentKindFeeNotice, DocumentKind: parentbilling.MailDocumentCareCancellation},
			"Aufhebung Ihres Gebührenbescheids", noticeCorrectionMailTemplate},
		{parentbilling.BillingLetterMail{LetterKind: parentbilling.DocumentKindFeeNotice, DocumentKind: parentbilling.DocumentCredit},
			"Änderung Ihres Gebührenbescheids", noticeCorrectionMailTemplate},
		{parentbilling.BillingLetterMail{LetterKind: parentbilling.DocumentKindFeeNotice, DocumentKind: parentbilling.MailDocumentCareInvoice},
			"Ihr Gebührenbescheid", noticeLetterMailTemplate},
		{parentbilling.BillingLetterMail{LetterKind: parentbilling.DocumentKindInvoice}, "Ihre Rechnung", invoiceLetterMailTemplate},
		{parentbilling.BillingLetterMail{}, "Bestätigung und Rechnung", letterMailTemplate},
	} {
		subject, template := letterMailOf(tc.mail)
		assert.Equal(t, tc.subject, subject, "%+v", tc.mail)
		assert.Equal(t, tc.template, template, "%+v", tc.mail)
	}
	assert.Equal(t, "", letterKindOf(parentbilling.DocumentKindConfirmation), "the payload of before stays as it was")
}

// documentKindDouble is a store and a template source for the gate.
type documentKindDouble struct {
	kinds  map[string]parentbilling.ServiceDocumentKind
	remedy string
	reads  int
}

func (d *documentKindDouble) DocumentKinds(context.Context, int64) (map[string]parentbilling.ServiceDocumentKind, error) {
	return d.kinds, nil
}

func (d *documentKindDouble) SetDocumentKind(_ context.Context, _ int64, service, kind string) error {
	if d.kinds == nil {
		d.kinds = map[string]parentbilling.ServiceDocumentKind{}
	}
	d.kinds[service] = parentbilling.ServiceDocumentKind{Service: service, DocumentKind: kind}
	return nil
}

func (d *documentKindDouble) DocumentTemplate(_ context.Context, _ int64, kind string) (parentbilling.LetterTemplate, error) {
	d.reads++
	if kind != parentbilling.DocumentKindFeeNotice {
		return parentbilling.LetterTemplate{}, nil
	}
	return parentbilling.LetterTemplate{Version: 3, LegalRemedy: d.remedy}, nil
}

// TestDocumentKindsOfferPerServiceAndRefuseOthers pins the choices: the
// holiday care issues the confirmation letter until the Träger chooses,
// the monthly care an invoice; the monthly care has no confirmation letter.
func TestDocumentKindsOfferPerServiceAndRefuseOthers(t *testing.T) {
	t.Parallel()
	double := &documentKindDouble{}
	kinds := NewDocumentKinds(double, double)
	overview, err := kinds.ListDocumentKinds(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, overview.Services, 2)
	assert.Equal(t, parentbilling.DocumentKindConfirmation, overview.Services[0].DocumentKind)
	assert.Equal(t, parentbilling.DocumentKindInvoice, overview.Services[1].DocumentKind)
	assert.Equal(t, []string{"invoice", "fee_notice"}, overview.Services[1].Choices)
	assert.False(t, overview.LegalRemedySet)

	_, err = kinds.SetDocumentKind(context.Background(), 1, parentbilling.ServiceCare, parentbilling.DocumentKindConfirmation)
	assert.Error(t, err, "the monthly care has no confirmation letter")
	_, err = kinds.SetDocumentKind(context.Background(), 1, "mensa", parentbilling.DocumentKindInvoice)
	assert.Error(t, err)
	_, err = kinds.SetDocumentKind(context.Background(), 0, parentbilling.ServiceCare, parentbilling.DocumentKindInvoice)
	assert.Error(t, err)

	double.remedy = "Widerspruch"
	overview, err = kinds.SetDocumentKind(context.Background(), 1, parentbilling.ServiceCare, parentbilling.DocumentKindFeeNotice)
	require.NoError(t, err)
	assert.Equal(t, parentbilling.DocumentKindFeeNotice, overview.Services[1].DocumentKind)
	assert.True(t, overview.LegalRemedySet)
	assert.Equal(t, 3, overview.FeeNoticeTemplateVersion)
}

// TestGateRefusesAFeeNoticeWithoutLegalRemedy: no fee notice is issued
// while the template has no legal remedy; any other kind reads no template.
func TestGateRefusesAFeeNoticeWithoutLegalRemedy(t *testing.T) {
	t.Parallel()
	double := &documentKindDouble{}
	gate := newDocumentKindGate(double, double)
	kind, err := gate.issuable(context.Background(), 1, parentbilling.ServiceHoliday)
	require.NoError(t, err)
	assert.Equal(t, parentbilling.DocumentKindConfirmation, kind)
	assert.Zero(t, double.reads, "an invoice needs no template read for the release")

	require.NoError(t, double.SetDocumentKind(context.Background(), 1, parentbilling.ServiceHoliday, parentbilling.DocumentKindFeeNotice))
	_, err = gate.issuable(context.Background(), 1, parentbilling.ServiceHoliday)
	assert.True(t, errors.Is(err, parentbilling.ErrLegalRemedyMissing))
	state, err := gate.current(context.Background(), 1, parentbilling.ServiceHoliday)
	require.NoError(t, err)
	assert.Equal(t, parentbilling.DocumentKindState{DocumentKind: parentbilling.DocumentKindFeeNotice, LegalRemedyMissing: true}, state)

	double.remedy = "  "
	_, err = gate.issuable(context.Background(), 1, parentbilling.ServiceHoliday)
	assert.ErrorIs(t, err, parentbilling.ErrLegalRemedyMissing, "blank is no legal remedy")
	double.remedy = "Widerspruch binnen eines Monats."
	kind, err = gate.issuable(context.Background(), 1, parentbilling.ServiceHoliday)
	require.NoError(t, err)
	assert.Equal(t, parentbilling.DocumentKindFeeNotice, kind)
}

// TestLettersOfAFeeNoticeNeedTheLegalRemedyItsStornoDoesNot: the letters of
// fee notices refuse without legal remedy, the letter of its Storno does not.
func TestLettersOfAFeeNoticeNeedTheLegalRemedyItsStornoDoesNot(t *testing.T) {
	t.Parallel()
	double := &documentKindDouble{}
	head := parentbilling.Letterhead{Template: parentbilling.LetterTemplate{Body: "Bestätigung"}}
	_, err := withDocumentTemplate(context.Background(), double, 1, head, parentbilling.DocumentKindFeeNotice, true)
	assert.ErrorIs(t, err, parentbilling.ErrLegalRemedyMissing)
	got, err := withDocumentTemplate(context.Background(), double, 1, head, parentbilling.DocumentKindFeeNotice, false)
	require.NoError(t, err)
	assert.Equal(t, 3, got.Template.Version)
	got, err = withDocumentTemplate(context.Background(), double, 1, head, parentbilling.DocumentKindConfirmation, true)
	require.NoError(t, err)
	assert.Equal(t, head, got, "the confirmation letter keeps the template of the letterhead")
	got, err = withNoticeTemplate(context.Background(), double, 1, head, parentbilling.DocumentKindInvoice, true)
	require.NoError(t, err)
	assert.Equal(t, head, got, "a monthly invoice keeps its built-in text")
}

func TestCareFeeNoticeUsesFrozenPaymentDeadline(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		payment *parentbilling.InvoicePayment
		due     string
	}{
		{name: "legacy deadline", due: "15.01.2027"},
		{name: "frozen deadline", payment: &parentbilling.InvoicePayment{
			Method: parentbilling.PaymentTransfer, DueOn: calendar.NewDate(2027, 1, 25),
		}, due: "25.01.2027"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := careLetterSJR()
			content.invoice.Payment = tc.payment
			content.letterhead.Template = hardtNoticeTemplate()
			content.letterhead.Template.Body += "\n\nZahlen bis {{zahlen_bis}}."
			text := blockText(careNoticeLetter(content))
			assert.Contains(t, text, "Zahlen bis: "+tc.due)
			assert.Contains(t, text, "Zahlen bis "+tc.due+".")
			assert.Contains(t, text, "bis zum "+tc.due)
		})
	}
}

func TestCareFeeNoticeKeepsNotesBeforeLegalRemedy(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.letterhead.Template = hardtNoticeTemplate()
	letter := careNoticeLetter(content)
	require.GreaterOrEqual(t, len(letter.Blocks), 3)
	assert.Equal(t, LetterBlock{Heading: "Hinweise", Text: content.input.Notes}, letter.Blocks[len(letter.Blocks)-3])
	assert.Equal(t, "Rechtsbehelfsbelehrung", letter.Blocks[len(letter.Blocks)-2].Heading)

	content.input.Notes = "  \n "
	withoutNotes := careNoticeLetter(content)
	assert.Len(t, withoutNotes.Blocks, len(letter.Blocks)-1)
}

type correctionNoticeStore struct {
	LetterStore
	file *parentbilling.BillingLetterFile
}

func (s *correctionNoticeStore) RunLetters(context.Context, int64, int64) (*parentbilling.BillingLetters, error) {
	return &parentbilling.BillingLetters{}, nil
}

func (s *correctionNoticeStore) InsertDocumentLetter(_ context.Context, _, _, _ int64, file parentbilling.BillingLetterFile) error {
	s.file = &file
	return nil
}

type correctionNoticeTemplates struct {
	documentKindDouble
}

func (s *correctionNoticeTemplates) Letterhead(context.Context, parentbilling.HolidayReviewer) (parentbilling.Letterhead, error) {
	return parentbilling.Letterhead{Template: parentbilling.LetterTemplate{Body: "Bestätigung"}}, nil
}

type correctionNoticeRenderer struct {
	letter Letter
}

func (r *correctionNoticeRenderer) RenderLetter(letter Letter, _ string) (parentbilling.BillingLetterFile, error) {
	r.letter = letter
	return parentbilling.BillingLetterFile{Data: []byte(blockText(letter))}, nil
}

func TestLaterHolidayNoticeCorrectionsLoadTheirLegalRemedy(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{parentbilling.DocumentCancellation, parentbilling.DocumentCredit} {
		for _, remedy := range []string{"Widerspruch binnen eines Monats.", ""} {
			t.Run(kind+"/"+remedy, func(t *testing.T) {
				content := correctionContent(t, kind)
				content.document.DocumentKind = parentbilling.DocumentKindFeeNotice
				content.invoice.DocumentKind = parentbilling.DocumentKindFeeNotice
				store := &correctionNoticeStore{}
				templates := &correctionNoticeTemplates{documentKindDouble{remedy: remedy}}
				renderer := &correctionNoticeRenderer{}
				letters := &BillingLetters{store: store, letterheads: templates, renderer: renderer}
				err := letters.correctionLetter(context.Background(), parentbilling.HolidayReviewer{}, correctionTarget{
					run: content.run, invoice: *content.invoice,
				}, content.document)
				require.NoError(t, err)
				require.NotNil(t, store.file)
				assert.Equal(t, 1, templates.reads)
				if remedy != "" {
					assert.Contains(t, string(store.file.Data), remedy)
					assert.Contains(t, blockText(renderer.letter), "Rechtsbehelfsbelehrung")
				} else {
					assert.NotContains(t, blockText(renderer.letter), "Rechtsbehelfsbelehrung")
				}
			})
		}
	}
}

func TestCareCancellationRemedyNamesTheCancellation(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.letterhead.Template.LegalRemedy = "Gegen Bescheid {{bescheidnummer}} vom {{datum}} ist Widerspruch möglich."
	content.cancellation = &parentbilling.CareCancellation{Number: "MB-2027-0009", IssuedOn: calendar.NewDate(2027, 1, 20)}
	text := blockText(careNoticeCancellationLetter(content))
	assert.Contains(t, text, "Gegen Bescheid MB-2027-0009 vom 20.01.2027 ist Widerspruch möglich.")
}

func TestCareFeeNoticeKeepsIntentionallyEmptyTemplateSections(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.letterhead.Template = hardtNoticeTemplate()
	content.letterhead.Template.Salutation, content.letterhead.Template.Closing = "", ""
	letter := careNoticeLetter(content)
	assert.Equal(t, "Gebührenbescheid MB-2027-0001", letter.Title)
	assert.Empty(t, letter.Salutation)
	assert.Empty(t, letter.Closing)
}

func TestHolidayInvoiceAndNoticeKeepLetterInputsOutsideTheirTemplates(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{parentbilling.DocumentKindInvoice, parentbilling.DocumentKindFeeNotice} {
		t.Run(kind, func(t *testing.T) {
			content := noticeContent(t)
			content.document.DocumentKind = kind
			if kind == parentbilling.DocumentKindInvoice {
				content.letterhead.Template = parentbilling.LetterTemplate{}
			}
			letter := confirmationLetter(content)
			text := blockText(letter)
			assert.Contains(t, text, content.input.Notes)
			for _, contact := range content.input.EmergencyContacts {
				assert.Contains(t, text, contact.Name+": "+contact.Phone)
			}
			if kind == parentbilling.DocumentKindFeeNotice {
				notes, remedy := strings.Index(text, content.input.Notes), strings.Index(text, "# Rechtsbehelfsbelehrung")
				require.NotEqual(t, -1, notes)
				require.NotEqual(t, -1, remedy)
				assert.Less(t, notes, remedy)
			}

			content.input.Notes, content.input.EmergencyContacts = "  ", nil
			empty := blockText(confirmationLetter(content))
			assert.NotContains(t, empty, "# Hinweise")
			assert.NotContains(t, empty, "# Notfallnummern")
		})
	}
}

func TestFeeNoticeGuardsRejectRemediesWithoutRenderedText(t *testing.T) {
	t.Parallel()
	double := &documentKindDouble{remedy: "{{ansprechperson}}"}
	require.NoError(t, double.SetDocumentKind(context.Background(), 1, parentbilling.ServiceHoliday, parentbilling.DocumentKindFeeNotice))
	gate := newDocumentKindGate(double, double)
	_, err := gate.issuable(context.Background(), 1, parentbilling.ServiceHoliday)
	assert.ErrorIs(t, err, parentbilling.ErrLegalRemedyMissing)
	_, err = withDocumentTemplate(context.Background(), double, 1, parentbilling.Letterhead{}, parentbilling.DocumentKindFeeNotice, true)
	assert.ErrorIs(t, err, parentbilling.ErrLegalRemedyMissing)
	overview, err := NewDocumentKinds(double, double).ListDocumentKinds(context.Background(), 1)
	require.NoError(t, err)
	assert.False(t, overview.LegalRemedySet)
}

// TestCareFeeNoticePrintsTheSharesOfCostBearers: a monthly fee notice names
// what a cost bearer pays and sets the rest as the family's part (#4687,
// #4688), like the monthly invoice.
func TestCareFeeNoticePrintsTheSharesOfCostBearers(t *testing.T) {
	t.Parallel()
	content := careLetterSJR()
	content.invoice.DocumentKind = parentbilling.DocumentKindFeeNotice
	content.letterhead.Template = hardtNoticeTemplate()
	basisPoints := 5000
	content.invoice.CostShares = []parentbilling.CareInvoiceCostShare{{
		Kind: parentbilling.CostShareYouthOffice, CostBearerName: "Jugendamt Enzkreis", ShareBasisPoints: &basisPoints, AmountCents: 2700,
	}}
	content.invoice.TotalCents = 2701
	text := blockText(careNoticeLetter(content))
	assert.Contains(t, text, "Monatsbeitrag: 54,01 €")
	assert.Contains(t, text, "Übernimmt Jugendamt Enzkreis (Jugendamt, 50 %): -27,00 €")
	assert.Contains(t, text, "Ihr Anteil: 27,01 €")
}

// TestMensaInvoiceOfItsOwnStaysAnInvoice: the kind of document belongs to
// the care contribution; a Mensa invoice of its own is never a fee notice
// (#4686, #4688).
func TestMensaInvoiceOfItsOwnStaysAnInvoice(t *testing.T) {
	t.Parallel()
	invoices := []parentbilling.CareInvoice{{Service: parentbilling.ServiceCare}, {Service: parentbilling.ServiceMeal}}
	excluded := []parentbilling.CareInvoice{{Service: parentbilling.ServiceCare}}
	markDocumentKind(parentbilling.DocumentKindFeeNotice, invoices, excluded)
	assert.Equal(t, parentbilling.DocumentKindFeeNotice, invoices[0].DocumentKind)
	assert.Equal(t, parentbilling.DocumentKindInvoice, invoices[1].DocumentKind)
	assert.Equal(t, parentbilling.DocumentKindFeeNotice, excluded[0].DocumentKind)
}
