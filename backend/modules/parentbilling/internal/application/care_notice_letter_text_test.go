package application

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The original of a change notice is a document of its own that names the
// notice it amends, shows bisher, neu and Zu-/Abgang, and prints nothing of
// the missing page 2 of MK-007 (#6347).
func TestFeeNoticeLetterOfAChangeNamesItsPredecessor(t *testing.T) {
	t.Parallel()
	issued, earlier := calendar.NewDate(2030, 10, 20), calendar.NewDate(2030, 9, 24)
	notice := parentbilling.CareNotice{
		Number: "MB-2030-0002", IssuedOn: &issued, Kind: parentbilling.NoticeKindChange, Year: 2030,
		ChildFirstName: "Mia", ChildLastName: "Beispiel", SchoolName: "Grundschule Ispringen",
		Recipient:     parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel", Email: "eva@example.org"},
		PaymentMethod: "transfer", PeriodFrom: calendar.NewDate(2030, 11, 1), PeriodUntil: calendar.NewDate(2030, 12, 31),
		Lines: []parentbilling.CareNoticeLine{{Kind: parentbilling.NoticeLineModule, Label: "Betreuung bis 16:30 Uhr", Detail: "Di",
			PeriodFrom: calendar.NewDate(2030, 11, 1), PeriodUntil: calendar.NewDate(2030, 12, 31), Months: 2, Quantity: 1,
			MonthlyPreviousCents: 4550, AnnualPreviousCents: 9100, DeltaCents: -9100}},
		Installments: []parentbilling.CareNoticeInstallment{{Month: calendar.NewDate(2030, 11, 1), DueOn: calendar.NewDate(2030, 11, 1),
			PreviousCents: 7750, AmountCents: 3200, DeltaCents: -4550}},
		PreviousCents: 15500, TotalCents: 6400, DeltaCents: -9100,
		Amends:      &parentbilling.CareNoticeReference{ID: 1, Number: "MB-2030-0001", IssuedOn: &earlier},
		LegalRemedy: "Gegen diesen Bescheid kann Widerspruch erhoben werden.",
	}
	letter := feeNoticeLetter(notice, parentbilling.Letterhead{SenderText: "SJR Pforzheim\nAm Markt 1"})
	assert.Equal(t, "Änderungsbescheid MB-2030-0002", letter.Title)
	assert.Equal(t, "Änderungsbescheid für die Betreuung 2030", letter.Subject)
	assert.Contains(t, letter.Facts, LetterFact{Label: "Ändert Bescheid", Value: "MB-2030-0001 vom 24.09.2030"})
	text := noticeLetterText(letter)
	assert.Contains(t, text, "ändern wir den Bescheid MB-2030-0001 vom 24.09.2030")
	assert.Contains(t, text, "Bisher festgesetzt")
	assert.Contains(t, text, "bisher 77,50 €, neu 32,00 €")
	assert.Contains(t, text, "Widerspruch")
	assert.NotContains(t, text, "Seite 2")
	assert.Equal(t, "aenderungsbescheid-MB-2030-0002", careNoticeFilename(notice))

	notice.Kind, notice.Amends = parentbilling.NoticeKindInitial, nil
	first := feeNoticeLetter(notice, parentbilling.Letterhead{})
	assert.Equal(t, "Gebührenbescheid MB-2030-0002", first.Title)
	require.NotContains(t, noticeLetterText(first), "Ändert")
}

func noticeLetterText(letter Letter) string {
	var parts []string
	for _, fact := range letter.Facts {
		parts = append(parts, fact.Label, fact.Value)
	}
	for _, block := range letter.Blocks {
		parts = append(parts, block.Heading, block.Text)
		for _, fact := range block.Facts {
			parts = append(parts, fact.Label, fact.Value)
		}
	}
	return strings.Join(parts, "\n")
}
