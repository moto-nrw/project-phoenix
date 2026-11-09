package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The notice dates follow the terms frozen in the version (#4682): the
// first allowed end that leaves the notice period, never earlier, and a
// contract that ends earlier on its own ends then.
func TestNoticeDatesFollowTheFrozenTerms(t *testing.T) {
	t.Parallel()
	ownEnd := calendar.NewDate(2027, 1, 15)
	cases := []struct {
		name     string
		terms    carecontract.TemplateInput
		received calendar.Date
		end      *calendar.Date
		ordinary calendar.Date
		special  *calendar.Date
	}{
		{"one month to the month end", carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2026, 12, 31), nil},
		{"received on the last day of a month", carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd},
			calendar.NewDate(2026, 11, 30), nil, calendar.NewDate(2026, 12, 31), nil},
		{"31 January plus one month stays in February", carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd},
			calendar.NewDate(2027, 1, 31), nil, calendar.NewDate(2027, 2, 28), nil},
		{"without notice to this month's end", carecontract.TemplateInput{NoticeMonths: 0, NoticeTo: carecontract.NoticeMonthEnd},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2026, 11, 30), nil},
		{"three months to the half year", carecontract.TemplateInput{NoticeMonths: 3, NoticeTo: carecontract.NoticeHalfYear},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2027, 7, 31), nil},
		{"to the end of the school year", carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeSchoolYearEnd},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2027, 7, 31), nil},
		{"the special termination ends earlier at the half year",
			carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeSchoolYearEnd, SpecialTermination: true},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2027, 7, 31), new(calendar.NewDate(2027, 1, 31))},
		{"a special termination no earlier is not offered",
			carecontract.TemplateInput{NoticeMonths: 1, NoticeTo: carecontract.NoticeMonthEnd, SpecialTermination: true},
			calendar.NewDate(2026, 11, 7), nil, calendar.NewDate(2026, 12, 31), nil},
		{"a contract that ends earlier ends then", carecontract.TemplateInput{NoticeMonths: 3, NoticeTo: carecontract.NoticeHalfYear},
			calendar.NewDate(2026, 11, 7), &ownEnd, ownEnd, nil},
	}
	for _, tc := range cases {
		ordinary, special := noticeDates(tc.terms, tc.received, tc.end)
		assert.Equal(t, tc.ordinary, ordinary, tc.name)
		assert.Equal(t, tc.special, special, tc.name)
	}
}

// The withdrawal period runs 14 days from the conclusion when the PDF
// carried the notice, else 12 months and 14 days; a paper contract from
// before has none (§ 355, § 356 Abs. 3 BGB).
func TestWithdrawalPeriodFollowsTheNotice(t *testing.T) {
	t.Parallel()
	concludedAt := time.Date(2026, 11, 6, 9, 0, 0, 0, time.UTC)
	informed := concludedAt
	electronic := &carecontract.Concluded{Method: carecontract.MethodWithApplication, At: concludedAt}
	paper := &carecontract.Concluded{Method: carecontract.MethodPaper, At: concludedAt}

	assert.Equal(t, new(calendar.NewDate(2026, 11, 20)), withdrawalUntil(electronic, &informed))
	assert.Equal(t, new(calendar.NewDate(2026, 11, 20)), withdrawalUntil(paper, &informed), "the PDF granted it")
	assert.Equal(t, new(calendar.NewDate(2027, 11, 22)), withdrawalUntil(electronic, nil),
		"without the notice: 20 November 2027 is a Saturday, the period ends on Monday (§ 193 BGB)")
	assert.Nil(t, withdrawalUntil(paper, nil), "signed at the office before the notice")
	assert.Nil(t, withdrawalUntil(nil, nil), "no conclusion, no period")

	saturday := time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, new(calendar.NewDate(2026, 11, 23)), withdrawalUntil(
		&carecontract.Concluded{Method: carecontract.MethodWithApplication, At: saturday}, &saturday),
		"21 November 2026 is a Saturday: the period ends on Monday 23 November (§ 193 BGB)")
	christmas := time.Date(2026, 12, 11, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, new(calendar.NewDate(2026, 12, 28)), withdrawalUntil(
		&carecontract.Concluded{Method: carecontract.MethodWithApplication, At: christmas}, &christmas),
		"25 December (holiday), 26 December (holiday) and Sunday 27 December move it to Monday 28 December")
	late := time.Date(2027, 11, 20, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, new(calendar.NewDate(2027, 11, 22)), withdrawalUntil(electronic, &late),
		"a notice provided late never reaches beyond 12 months and 14 days after the conclusion")
}

// Easter and the nationwide holidays that depend on it.
func TestNationwideHolidays(t *testing.T) {
	t.Parallel()
	assert.Equal(t, calendar.NewDate(2026, 4, 5), easterSunday(2026))
	assert.Equal(t, calendar.NewDate(2027, 3, 28), easterSunday(2027))
	for _, day := range []calendar.Date{calendar.NewDate(2026, 4, 3), calendar.NewDate(2026, 4, 6), calendar.NewDate(2026, 5, 14),
		calendar.NewDate(2026, 5, 25), calendar.NewDate(2026, 10, 3), calendar.NewDate(2026, 12, 26)} {
		assert.True(t, nationwideHoliday(day), day.String())
	}
	assert.False(t, nationwideHoliday(calendar.NewDate(2026, 6, 4)), "Fronleichnam is regional")
}

// The confirmation fills only the Träger's placeholders and refuses an
// empty template instead of falling back to a built-in text.
func TestConfirmationTextFillsTheTemplate(t *testing.T) {
	t.Parallel()
	endsOn := calendar.NewDate(2026, 12, 31)
	text, err := confirmationText("Kündigung für {Kind} ({Module}, {Schule}) am {Eingang}, Ende {Vertragsende}. {Träger}",
		confirmationValues{Child: "Lina Muster", Modules: "Ganztag", School: "OGS Am Lindenhof", EndsOn: &endsOn,
			Received: "07.11.2026", Traeger: "Ganztag im Quartier e.V."})
	assert.NoError(t, err)
	assert.Equal(t, "Kündigung für Lina Muster (Ganztag, OGS Am Lindenhof) am 07.11.2026, Ende 31.12.2026. Ganztag im Quartier e.V.", text)

	_, err = confirmationText("  ", confirmationValues{})
	assert.ErrorIs(t, err, carecontract.ErrNoticeTemplateMissing)
}

// § 312k Abs. 4 BGB: the confirmation of a declaration with the button names
// day and time of its receipt; a letter the office records has its day.
// A template that leaves out {Eingang}, or {Grund} for an extraordinary
// termination, still names them (#4682).
func TestConfirmationNamesTheReceiptWithItsTime(t *testing.T) {
	t.Parallel()
	declared := time.Date(2026, 11, 7, 8, 30, 0, 0, time.UTC) // 09:30 in Berlin
	parents := ports.NewNotice{Channel: carecontract.NoticeChannelParents, DeclaredAt: declared, ReceivedOn: calendar.NewDate(2026, 11, 7)}
	assert.Equal(t, "07.11.2026 um 09:30 Uhr", receipt(parents))
	office := ports.NewNotice{Channel: carecontract.NoticeChannelOffice, DeclaredAt: declared, ReceivedOn: calendar.NewDate(2026, 11, 5)}
	assert.Equal(t, "05.11.2026", receipt(office), "a letter has no time of receipt")

	text, err := confirmationText("Ihre Kündigung für {Kind}.", confirmationValues{Child: "Lina", Received: receipt(parents),
		Reason: "Wir ziehen um.", NeedsReason: true})
	assert.NoError(t, err)
	assert.Equal(t, "Ihre Kündigung für Lina.\n\nEingang Ihrer Erklärung: 07.11.2026 um 09:30 Uhr\n\nIhr Grund: Wir ziehen um.", text)
	text, err = confirmationText("Eingang {Eingang}. Grund: {Grund}", confirmationValues{Received: receipt(parents),
		Reason: "Umzug", NeedsReason: true})
	assert.NoError(t, err)
	assert.Equal(t, "Eingang 07.11.2026 um 09:30 Uhr. Grund: Umzug", text, "placed placeholders are not repeated")
	assert.Equal(t, carecontract.TerminationExtraordinary,
		templateKind(ports.NewNotice{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationExtraordinary}))
	assert.Equal(t, carecontract.NoticeTermination,
		templateKind(ports.NewNotice{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationOrdinary}))
}

// Fronleichnam is Easter Sunday + 60 days (PTB, „Wann ist Ostern?“): Easter
// fell on 5 April 2026 and falls on 28 March 2027, so Fronleichnam is
// 4 June 2026 and 27 May 2027. It is a holiday only in some federal states;
// § 193 BGB moves a deadline here only for nationwide holidays (#4682), so it
// moves nothing, while Christi Himmelfahrt (+39) and Pfingstmontag (+50) do.
func TestHolidaysFollowEasterIn2026And2027(t *testing.T) {
	t.Parallel()
	for _, year := range []struct {
		easter, ascension, whitMonday, corpusChristi calendar.Date
	}{
		{calendar.NewDate(2026, 4, 5), calendar.NewDate(2026, 5, 14), calendar.NewDate(2026, 5, 25), calendar.NewDate(2026, 6, 4)},
		{calendar.NewDate(2027, 3, 28), calendar.NewDate(2027, 5, 6), calendar.NewDate(2027, 5, 17), calendar.NewDate(2027, 5, 27)},
	} {
		easter := easterSunday(year.easter.Year())
		assert.Equal(t, year.easter, easter)
		assert.True(t, nationwideHoliday(year.ascension), year.ascension.String())
		assert.True(t, nationwideHoliday(year.whitMonday), year.whitMonday.String())
		assert.False(t, nationwideHoliday(year.corpusChristi), "Fronleichnam is regional: "+year.corpusChristi.String())
		assert.Equal(t, year.corpusChristi, nextWorkingDay(year.corpusChristi), "a deadline on Fronleichnam stays")
		assert.Equal(t, year.ascension.AddDays(1), nextWorkingDay(year.ascension), "a deadline on Christi Himmelfahrt moves to Friday")
	}
}

// The worker's mail carries the confirmation as stored, paragraph by
// paragraph, and a subject without the child's name.
func TestRenderNoticeMailKeepsTheConfirmation(t *testing.T) {
	t.Parallel()
	payload := []byte(`{"kind":"widerruf","traeger_name":"Ganztag e.V.","guardian_name":"Erika Muster",` +
		`"confirmation_text":"wir bestätigen Ihren Widerruf.\n\nMit freundlichen Grüßen\nGanztag e.V."}`)
	mail, err := RenderNoticeMail(t.Context(), payload)
	assert.NoError(t, err)
	assert.Equal(t, "Bestätigung Ihres Widerrufs", mail.Subject)
	assert.True(t, mail.Withdrawal)
	assert.Equal(t, []string{"wir bestätigen Ihren Widerruf.", "Mit freundlichen Grüßen\nGanztag e.V."}, mail.Paragraphs)

	_, err = RenderNoticeMail(t.Context(), []byte(`{"kind":"kuendigung","confirmation_text":" "}`))
	assert.Error(t, err, "a confirmation without text is never sent")
}
