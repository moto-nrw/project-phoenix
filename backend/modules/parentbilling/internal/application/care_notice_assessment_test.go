package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// mk007Months are the months of the Hardt notice MK-007: 01.09. to
// 31.12.2025.
var mk007Months = []calendar.Date{
	calendar.NewDate(2025, 9, 1), calendar.NewDate(2025, 10, 1), calendar.NewDate(2025, 11, 1), calendar.NewDate(2025, 12, 1),
}

// mk007Position is one module of a month as the monthly computation bills it.
func mk007Position(offering int64, name string, cents int64, month calendar.Date, weekdays ...string) parentbilling.CarePosition {
	prices := make([]parentbilling.CareWeekdayPrice, len(weekdays))
	for i, weekday := range weekdays {
		prices[i] = parentbilling.CareWeekdayPrice{Weekday: weekday, MonthlyCents: cents / int64(len(weekdays)), AmountCents: cents / int64(len(weekdays))}
	}
	return parentbilling.CarePosition{CareOfferingID: offering, ModuleName: name, VersionNumber: 1, ServiceFrom: month,
		ServiceUntil: monthEnd(month), Weekdays: prices, AmountCents: cents, PayableCents: cents}
}

// mk007Month is the month of the child of MK-007: Verlässliche Grundschule
// at 12,00 € and the afternoon 13 - 16 Uhr at 20,00 €, one day a week each.
func mk007Month(month calendar.Date, positions ...parentbilling.CarePosition) parentbilling.CareInvoice {
	if positions == nil {
		positions = []parentbilling.CarePosition{
			mk007Position(1, "Verlässliche Grundschule", 1200, month, "wed"),
			mk007Position(2, "Nachmittagsbetreuung 13 - 16 Uhr", 2000, month, "wed"),
		}
	}
	var total int64
	for _, position := range positions {
		total += position.AmountCents
	}
	return parentbilling.CareInvoice{ContractID: 7, SchoolID: 3, SchoolName: "Grundschule Hardt", ChildFirstName: "Lena",
		ChildLastName: "Muster", Service: parentbilling.ServiceCare,
		Recipient: parentbilling.CareRecipient{FirstName: "Eva", LastName: "Muster", Email: "eva@example.org"},
		Positions: positions, TotalCents: total}
}

func mk007Drafts(build func(calendar.Date) []parentbilling.CareInvoice) map[calendar.Date][]parentbilling.CareInvoice {
	drafts := map[calendar.Date][]parentbilling.CareInvoice{}
	for _, month := range mk007Months {
		drafts[month] = build(month)
	}
	return drafts
}

func mk007Notice(t *testing.T) parentbilling.CareNotice {
	t.Helper()
	notices, unchanged := assessNotices(noticeAssessment{from: mk007Months[0], months: mk007Months,
		drafts: mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
			return []parentbilling.CareInvoice{mk007Month(month)}
		}),
		previous: map[int64]parentbilling.CareNotice{}, issuedOn: calendar.NewDate(2025, 9, 24)})
	require.Len(t, notices, 1)
	assert.Zero(t, unchanged)
	return notices[0]
}

// TestCareNoticeMatchesPageOneOfMK007 is the golden test of the Jahresbescheid
// against the facts of page 1 of the Hardt notice MK-007 (the original PDF is
// missing; swap the expectation for it once it arrives): Zeitraum 01.09. to
// 31.12.25, one line per module with 12,00 € and 20,00 € a month, "Anzahl" 1,
// proportional annual amounts 48,00 € and 80,00 € with bisher 0,00 and
// Zu-/Abgang equal to neu (a first assessment), total 128,00 €, four
// installments of 32,00 € on the first of the month, September due at once.
func TestCareNoticeMatchesPageOneOfMK007(t *testing.T) {
	t.Parallel()
	notice := mk007Notice(t)

	assert.Equal(t, parentbilling.NoticeKindInitial, notice.Kind)
	assert.Nil(t, notice.Amends)
	assert.Equal(t, 2025, notice.Year)
	assert.Equal(t, calendar.NewDate(2025, 9, 1), notice.PeriodFrom)
	assert.Equal(t, calendar.NewDate(2025, 12, 31), notice.PeriodUntil)
	assert.Equal(t, "Lena", notice.ChildFirstName)

	require.Len(t, notice.Lines, 2)
	for i, want := range []struct {
		label          string
		monthly, total int64
	}{{"Verlässliche Grundschule", 1200, 4800}, {"Nachmittagsbetreuung 13 - 16 Uhr", 2000, 8000}} {
		line := notice.Lines[i]
		assert.Equal(t, parentbilling.NoticeLineModule, line.Kind)
		assert.Equal(t, want.label, line.Label)
		assert.Equal(t, "an 1 Tag/Woche (Mi)", line.Detail)
		assert.Equal(t, calendar.NewDate(2025, 9, 1), line.PeriodFrom)
		assert.Equal(t, calendar.NewDate(2025, 12, 31), line.PeriodUntil)
		assert.Equal(t, 4, line.Months)
		assert.Equal(t, 1, line.Quantity, "Anzahl")
		assert.Equal(t, int64(0), line.MonthlyPreviousCents, "Betrag bisher")
		assert.Equal(t, want.monthly, line.MonthlyCents, "Betrag neu")
		assert.Equal(t, int64(0), line.AnnualPreviousCents, "anteiliger Jahresbetrag bisher")
		assert.Equal(t, want.total, line.AnnualCents, "anteiliger Jahresbetrag neu")
		assert.Equal(t, want.total, line.DeltaCents, "Zu-/Abgang")
	}

	assert.Equal(t, int64(0), notice.PreviousCents)
	assert.Equal(t, int64(12800), notice.TotalCents, "Gesamtbetrag dieser Rechnung")
	assert.Equal(t, int64(12800), notice.DeltaCents)

	require.Len(t, notice.Installments, 4)
	due := []calendar.Date{calendar.NewDate(2025, 9, 24), calendar.NewDate(2025, 10, 1), calendar.NewDate(2025, 11, 1), calendar.NewDate(2025, 12, 1)}
	for i, installment := range notice.Installments {
		assert.Equal(t, mk007Months[i], installment.Month)
		assert.Equal(t, int64(3200), installment.AmountCents, "Rate %d", i+1)
		assert.Equal(t, due[i], installment.DueOn, "Rate %d", i+1)
		assert.Equal(t, i == 0, installment.ImmediatelyDue, "only the past September is due at once")
	}
	require.Len(t, notice.Months, 4, "each month keeps what it bills for the monthly run")
}

// mk007Previous is the issued MK-007 notice as the store reads it back.
func mk007Previous(t *testing.T) parentbilling.CareNotice {
	t.Helper()
	notice := mk007Notice(t)
	issuedOn := calendar.NewDate(2025, 9, 24)
	notice.ID, notice.Number, notice.IssuedOn = 41, "1.2025.000041.1", &issuedOn
	return notice
}

// TestCareNoticeChangeShowsBeforeNowAndTheDifference: from November the child
// comes two days a week to the Verlässliche Grundschule (20,00 €). The change
// notice names the notice it amends and shows per basis bisher, neu and
// Zu-/Abgang; the rates of November and December rise by 8,00 €, the months
// before keep theirs.
func TestCareNoticeChangeShowsBeforeNowAndTheDifference(t *testing.T) {
	t.Parallel()
	previous := mk007Previous(t)
	drafts := mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
		if month.Before(calendar.NewDate(2025, 11, 1)) {
			return []parentbilling.CareInvoice{mk007Month(month)}
		}
		return []parentbilling.CareInvoice{mk007Month(month,
			mk007Position(1, "Verlässliche Grundschule", 2000, month, "mon", "wed"),
			mk007Position(2, "Nachmittagsbetreuung 13 - 16 Uhr", 2000, month, "wed"))}
	})
	notices, unchanged := assessNotices(noticeAssessment{from: calendar.NewDate(2025, 11, 1), months: mk007Months, drafts: drafts,
		previous: map[int64]parentbilling.CareNotice{7: previous}, issuedOn: calendar.NewDate(2025, 10, 20)})
	require.Len(t, notices, 1)
	assert.Zero(t, unchanged)
	notice := notices[0]
	assert.Equal(t, parentbilling.NoticeKindChange, notice.Kind)
	require.NotNil(t, notice.Amends)
	assert.Equal(t, previous.Number, notice.Amends.Number, "the change names the notice it amends")
	assert.Equal(t, calendar.NewDate(2025, 9, 1), notice.PeriodFrom, "the change covers every month of the amended notice")

	type row struct {
		detail                     string
		from                       calendar.Date
		months                     int
		monthlyBefore, monthlyNow  int64
		annualBefore, annualNow, d int64
	}
	got := make([]row, len(notice.Lines))
	for i, line := range notice.Lines {
		got[i] = row{line.Detail, line.PeriodFrom, line.Months, line.MonthlyPreviousCents, line.MonthlyCents,
			line.AnnualPreviousCents, line.AnnualCents, line.DeltaCents}
	}
	assert.Equal(t, []row{
		{"an 1 Tag/Woche (Mi)", calendar.NewDate(2025, 9, 1), 2, 1200, 1200, 2400, 2400, 0},
		{"an 1 Tag/Woche (Mi)", calendar.NewDate(2025, 11, 1), 2, 1200, 0, 2400, 0, -2400},
		{"an 2 Tagen/Woche (Mo, Mi)", calendar.NewDate(2025, 11, 1), 2, 0, 2000, 0, 4000, 4000},
		{"an 1 Tag/Woche (Mi)", calendar.NewDate(2025, 9, 1), 4, 2000, 2000, 8000, 8000, 0},
	}, got)

	assert.Equal(t, int64(12800), notice.PreviousCents)
	assert.Equal(t, int64(14400), notice.TotalCents)
	assert.Equal(t, int64(1600), notice.DeltaCents)
	require.Len(t, notice.Installments, 4)
	for i, want := range []struct {
		before, now int64
		due         calendar.Date
		atOnce      bool
	}{
		// September of the amended notice was due at once on 24.09.; the
		// change keeps that date.
		{3200, 3200, calendar.NewDate(2025, 9, 24), true},
		{3200, 3200, calendar.NewDate(2025, 10, 1), false},
		{3200, 4000, calendar.NewDate(2025, 11, 1), false},
		{3200, 4000, calendar.NewDate(2025, 12, 1), false},
	} {
		installment := notice.Installments[i]
		assert.Equal(t, want.before, installment.PreviousCents)
		assert.Equal(t, want.now, installment.AmountCents)
		assert.Equal(t, want.now-want.before, installment.DeltaCents)
		assert.Equal(t, want.due, installment.DueOn)
		assert.Equal(t, want.atOnce, installment.ImmediatelyDue)
	}
}

// TestCareNoticeChangeOfAPastMonthIsDueAtOnce: a raise of a month already
// past on the notice date is due at once; the original rate is not.
func TestCareNoticeChangeOfAPastMonthIsDueAtOnce(t *testing.T) {
	t.Parallel()
	previous := mk007Previous(t)
	drafts := mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
		return []parentbilling.CareInvoice{mk007Month(month,
			mk007Position(1, "Verlässliche Grundschule", 1500, month, "wed"),
			mk007Position(2, "Nachmittagsbetreuung 13 - 16 Uhr", 2000, month, "wed"))}
	})
	notices, _ := assessNotices(noticeAssessment{from: calendar.NewDate(2025, 11, 1), months: mk007Months, drafts: drafts,
		previous: map[int64]parentbilling.CareNotice{7: previous}, issuedOn: calendar.NewDate(2025, 11, 10)})
	require.Len(t, notices, 1)
	installments := notices[0].Installments
	require.Len(t, installments, 4)
	for i := range 3 {
		assert.True(t, installments[i].ImmediatelyDue, "the raise of %s is due at once", installments[i].Month)
		assert.Equal(t, calendar.NewDate(2025, 11, 10), installments[i].DueOn)
	}
	assert.False(t, installments[3].ImmediatelyDue)
}

// TestCareNoticeThatStillHoldsIsNotIssuedAgain: a contract whose months bill
// what its effective notice assessed gets no new notice.
func TestCareNoticeThatStillHoldsIsNotIssuedAgain(t *testing.T) {
	t.Parallel()
	previous := mk007Previous(t)
	notices, unchanged := assessNotices(noticeAssessment{from: calendar.NewDate(2025, 10, 1), months: mk007Months,
		drafts: mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
			return []parentbilling.CareInvoice{mk007Month(month)}
		}),
		previous: map[int64]parentbilling.CareNotice{7: previous}, issuedOn: calendar.NewDate(2025, 10, 5)})
	assert.Empty(t, notices)
	assert.Equal(t, 1, unchanged)
}

// TestCareNoticeOfAnEndedContractTakesTheMonthsBack: the contract ends with
// October. The change notice lowers November and December to 0,00 € (an
// Abgang) and keeps them assessed, so no monthly run bills them.
func TestCareNoticeOfAnEndedContractTakesTheMonthsBack(t *testing.T) {
	t.Parallel()
	previous := mk007Previous(t)
	drafts := mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
		if month.Before(calendar.NewDate(2025, 11, 1)) {
			return []parentbilling.CareInvoice{mk007Month(month)}
		}
		return nil
	})
	notices, _ := assessNotices(noticeAssessment{from: calendar.NewDate(2025, 11, 1), months: mk007Months, drafts: drafts,
		previous: map[int64]parentbilling.CareNotice{7: previous}, issuedOn: calendar.NewDate(2025, 10, 28)})
	require.Len(t, notices, 1)
	notice := notices[0]
	assert.Equal(t, int64(6400), notice.TotalCents)
	assert.Equal(t, int64(-6400), notice.DeltaCents)
	assert.Equal(t, "Lena", notice.ChildFirstName, "the head comes from the months that still bill")
	require.Len(t, notice.Installments, 4)
	assert.Equal(t, int64(0), notice.Installments[2].AmountCents)
	assert.Equal(t, int64(-3200), notice.Installments[3].DeltaCents)
	require.Len(t, notice.Months, 4, "the months taken back stay assessed")
	assert.Empty(t, notice.Months[3].Positions)
}

// TestCareNoticeLeavesMonthsBeforeTheRunToTheMonthlyRun: a first notice
// starts with the run; an earlier month with a draft stays out.
func TestCareNoticeLeavesMonthsBeforeTheRunToTheMonthlyRun(t *testing.T) {
	t.Parallel()
	notices, _ := assessNotices(noticeAssessment{from: calendar.NewDate(2025, 10, 1), months: mk007Months,
		drafts: mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
			return []parentbilling.CareInvoice{mk007Month(month)}
		}),
		previous: map[int64]parentbilling.CareNotice{}, issuedOn: calendar.NewDate(2025, 9, 24)})
	require.Len(t, notices, 1)
	assert.Equal(t, calendar.NewDate(2025, 10, 1), notices[0].PeriodFrom)
	assert.Len(t, notices[0].Installments, 3)
	assert.Equal(t, int64(9600), notices[0].TotalCents)
}

// TestCareNoticeLinesCarryReductionsAndShares: a reduction and a cost
// bearer share are lines of their own and lower the total.
func TestCareNoticeLinesCarryReductionsAndShares(t *testing.T) {
	t.Parallel()
	drafts := mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
		invoice := mk007Month(month)
		invoice.Reductions = []parentbilling.CareReduction{{Kind: "sibling", RuleID: 5, Name: "Geschwister", AmountCents: 600}}
		invoice.CostShares = []parentbilling.CareInvoiceCostShare{{CostBearerID: 9, CostBearerName: "Jugendamt", AmountCents: 1000}}
		invoice.TotalCents = 3200 - 600 - 1000
		return []parentbilling.CareInvoice{invoice}
	})
	notices, _ := assessNotices(noticeAssessment{from: mk007Months[0], months: mk007Months, drafts: drafts,
		previous: map[int64]parentbilling.CareNotice{}, issuedOn: calendar.NewDate(2025, 9, 1)})
	require.Len(t, notices, 1)
	lines := notices[0].Lines
	require.Len(t, lines, 4)
	assert.Equal(t, parentbilling.NoticeLineReduction, lines[2].Kind)
	assert.Equal(t, int64(-2400), lines[2].AnnualCents)
	assert.Equal(t, parentbilling.NoticeLineCostShare, lines[3].Kind)
	assert.Equal(t, int64(-4000), lines[3].AnnualCents)
	var sum int64
	for _, line := range lines {
		sum += line.AnnualCents
	}
	assert.Equal(t, notices[0].TotalCents, sum, "the lines add up to the total")
	assert.False(t, notices[0].Installments[0].ImmediatelyDue, "a notice dated on the first is due that day")
}

// TestCareNoticeLetterFillsThePlaceholdersOfTheLegalRemedy: the legal remedy
// the Träger writes may name the Träger and the notice; page 1 prints them
// filled, never as "{{traeger}}".
func TestCareNoticeLetterFillsThePlaceholdersOfTheLegalRemedy(t *testing.T) {
	t.Parallel()
	notice := mk007Previous(t)
	notice.LegalRemedy = "Widerspruch bei {{traeger}} gegen den Bescheid {{bescheidnummer}} vom {{datum}}."
	filled := filledLegalRemedy(notice, parentbilling.Letterhead{SenderText: "Gemeinde Hardt\nMariazeller Str. 1"})
	assert.Equal(t, "Widerspruch bei Gemeinde Hardt gegen den Bescheid 1.2025.000041.1 vom 24.09.2025.", filled)
	assert.Empty(t, filledLegalRemedy(parentbilling.CareNotice{}, parentbilling.Letterhead{}))
}

func TestCareNoticeChangeKeepsPreviouslyAssessedZeroMonths(t *testing.T) {
	t.Parallel()
	previous := mk007Previous(t)
	previous.Installments[2].AmountCents = 0
	previous.Months[2] = noticeMonth(previous, nil, mk007Months[2])
	drafts := mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
		if month == mk007Months[2] {
			return nil
		}
		invoice := mk007Month(month)
		if month == mk007Months[3] {
			invoice = mk007Month(month, mk007Position(1, "Verlässliche Grundschule", 1300, month, "wed"),
				mk007Position(2, "Nachmittagsbetreuung 13 - 16 Uhr", 2000, month, "wed"))
		}
		return []parentbilling.CareInvoice{invoice}
	})
	notices, _ := assessNotices(noticeAssessment{from: mk007Months[3], months: mk007Months, drafts: drafts,
		previous: map[int64]parentbilling.CareNotice{7: previous}, issuedOn: mk007Months[3]})
	require.Len(t, notices, 1)
	require.Len(t, notices[0].Installments, 4, "the zero month stays assessed after another month changes")
	assert.Equal(t, mk007Months[2], notices[0].Installments[2].Month)
	assert.Zero(t, notices[0].Installments[2].AmountCents)
}

// TestCareNoticeMonthCountsAKeptAssessmentBeforeTheRuleChain: a noticed
// contract the month cannot price keeps its assessment, and the month is
// computed again with it as issued, so siblings and caps of the family see
// it like any issued month; the first computation still names the booking
// without a price.
func TestCareNoticeMonthCountsAKeptAssessmentBeforeTheRuleChain(t *testing.T) {
	t.Parallel()
	unpriced := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"), booked("sat", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	priced := sjrVersion(t, contractNoah, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	assessed := parentbilling.CareInvoice{ContractID: contractMia, SchoolID: schoolIspringen, TotalCents: 3200, Number: "JB-1",
		DocumentKind: parentbilling.DocumentKindFeeNotice}
	in := careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{unpriced, priced}, tariffs: sjrTariffs(t)}
	review, final, err := computeNoticeMonth(in, map[int64]parentbilling.CareInvoice{contractMia: assessed},
		map[int64]bool{contractMia: true}, []int64{schoolIspringen})
	require.NoError(t, err)
	require.Len(t, review.unpriced, 1, "the review names the booking without a price")
	assert.Zero(t, review.billed, "the first computation computes the noticed contract again")
	assert.Equal(t, 1, final.billed, "the kept assessment is issued for the rule chain")
	require.Len(t, final.invoices, 1)
	assert.Equal(t, int64(contractNoah), final.invoices[0].ContractID)
}

// TestCareNoticeKeepsTheFamilyOfTheContract: the family the sibling rules
// count by is the contract party's, not the payer's a deviation names.
func TestCareNoticeKeepsTheFamilyOfTheContract(t *testing.T) {
	t.Parallel()
	notices, _ := assessNotices(noticeAssessment{from: mk007Months[0], months: mk007Months,
		drafts: mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
			invoice := mk007Month(month)
			invoice.FamilyEmail = "eva@example.org"
			return []parentbilling.CareInvoice{invoice}
		}),
		previous: map[int64]parentbilling.CareNotice{}, issuedOn: calendar.NewDate(2025, 9, 24)})
	require.Len(t, notices, 1)
	notices[0].Recipient.Email = "jugendamt@example.org"
	assert.Equal(t, "eva@example.org", notices[0].FamilyEmail)
}

// TestCareNoticeChangeKeepsTheDueDateOfAnUnraisedPastRate: September of
// MK-007 was due at once on 24.09.; a change of November keeps that date.
func TestCareNoticeChangeKeepsTheDueDateOfAnUnraisedPastRate(t *testing.T) {
	t.Parallel()
	previous := mk007Previous(t)
	drafts := mk007Drafts(func(month calendar.Date) []parentbilling.CareInvoice {
		if month.Before(calendar.NewDate(2025, 11, 1)) {
			return []parentbilling.CareInvoice{mk007Month(month)}
		}
		return []parentbilling.CareInvoice{mk007Month(month,
			mk007Position(1, "Verlässliche Grundschule", 2000, month, "mon", "wed"),
			mk007Position(2, "Nachmittagsbetreuung 13 - 16 Uhr", 2000, month, "wed"))}
	})
	notices, _ := assessNotices(noticeAssessment{from: calendar.NewDate(2025, 11, 1), months: mk007Months, drafts: drafts,
		previous: map[int64]parentbilling.CareNotice{7: previous}, issuedOn: calendar.NewDate(2025, 10, 20)})
	require.Len(t, notices, 1)
	september := notices[0].Installments[0]
	assert.Equal(t, calendar.NewDate(2025, 9, 24), september.DueOn)
	assert.True(t, september.ImmediatelyDue)
}

// TestCareFixedShareTakenByANoticeLeavesTheMensaToTheFamily, by hand: the
// Gemeinde pays a fixed 20,00 € a month for care and Mensa. The Jahresbescheid
// of the month took all 20,00 € of the care; the monthly run bills 3 × 4,80 €
// = 14,40 € of Mensa. The share is spent, so the family pays 14,40 €, not 0.
func TestCareFixedShareTakenByANoticeLeavesTheMensaToTheFamily(t *testing.T) {
	t.Parallel()
	month := day(t, mealRunMonth)
	twenty := int64(2000)
	share := parentbilling.CareCostShare{ContractID: contractMia, Kind: parentbilling.CostShareOtherSubsidy, CostBearerID: 77,
		CostBearerName: "Gemeinde", ShareCents: &twenty, Scope: parentbilling.CostShareScopeBoth, ValidFrom: month,
		ValidUntil: monthEnd(month)}
	mensa := careMonth{invoices: []parentbilling.CareInvoice{{ContractID: contractMia, Service: parentbilling.ServiceCare,
		MealPositions: []parentbilling.CareMealPosition{{AmountCents: 1440}}, TotalCents: 1440}}}
	notice := parentbilling.CareInvoice{ContractID: contractMia, Number: "JB-1", DocumentKind: parentbilling.DocumentKindFeeNotice,
		CostShares: []parentbilling.CareInvoiceCostShare{{Kind: parentbilling.CostShareOtherSubsidy, CostBearerID: 77, AmountCents: 2000}}}
	applyMealCostShares(&mensa, []parentbilling.CareCostShare{share}, month, map[int64]parentbilling.CareInvoice{contractMia: notice})
	assert.Equal(t, int64(1440), mensa.invoices[0].TotalCents, "the family pays the Mensa")
	assert.Empty(t, mensa.invoices[0].CostShares, "the spent share takes nothing more")
}

// TestCareMonthNamesTheContractsANoticeTookBack: a month an amended notice
// set to nothing stays assessed (ADR 0182); the monthly run names the child
// so the office issues a new notice. A notice month that bills something
// is no hold, nor is a contract without a version in the month.
func TestCareMonthNamesTheContractsANoticeTookBack(t *testing.T) {
	t.Parallel()
	versions := map[int64][]parentbilling.CareContractVersion{
		contractMia:  {{ContractID: contractMia, SchoolID: schoolIspringen, ChildFirstName: "Mia", ChildLastName: "Beispiel"}},
		contractNoah: {{ContractID: contractNoah, SchoolID: schoolIspringen, ChildFirstName: "Noah", ChildLastName: "Beispiel"}},
	}
	invoiced := map[int64]parentbilling.CareInvoice{
		contractMia: {ContractID: contractMia, Number: "JB-2", DocumentKind: parentbilling.DocumentKindFeeNotice},
		contractNoah: {ContractID: contractNoah, Number: "JB-3", DocumentKind: parentbilling.DocumentKindFeeNotice,
			Positions: []parentbilling.CarePosition{{CareOfferingID: 1, AmountCents: 3200}}},
		99: {ContractID: 99, Number: "JB-4", DocumentKind: parentbilling.DocumentKindFeeNotice},
	}
	holds := feeNoticeHolds(invoiced, versions)
	require.Len(t, holds, 1)
	assert.Equal(t, parentbilling.CareFeeNoticeHold{ContractID: contractMia, SchoolID: schoolIspringen, ChildFirstName: "Mia",
		ChildLastName: "Beispiel", NoticeNumber: "JB-2"}, holds[0])
}
