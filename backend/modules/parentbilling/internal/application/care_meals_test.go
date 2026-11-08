package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Mensa of the monthly run (#4686, ADR 0168), computed by hand. The
// run bills April 2026 on 2 April 2026; the school is Ispringen (11), the
// child Mia (student 9501, contract 501).

const (
	mealRunMonth = "2026-04-01"
	mealToday    = "2026-04-02"
	mealStudent  = contractMia + 9000
)

func mealTariff(t *testing.T, mode, from string, cents int64) parentbilling.CareMealTariff {
	t.Helper()
	return parentbilling.CareMealTariff{SchoolID: schoolIspringen, ValidFrom: day(t, from), Mode: mode, AmountCents: cents}
}

func mealRelease(t *testing.T, month string, children ...parentbilling.CareMealChild) parentbilling.CareMealRelease {
	t.Helper()
	return parentbilling.CareMealRelease{SchoolID: schoolIspringen, Month: day(t, month), Children: children}
}

func mia(meals int) parentbilling.CareMealChild {
	return parentbilling.CareMealChild{StudentID: mealStudent, FirstName: "Mia", LastName: "Beispiel", MealDays: meals}
}

func TestCareMealsMatchOnlyAnUnambiguousChild(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"exact link before name", "two children with the same name", "two unlinked contracts", "one unlinked contract"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			month, versions := mealMonth(t)
			versions[0].StudentID = nil
			releases := []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01", mia(2))}
			wantContract := int64(0)
			switch scenario {
			case "exact link before name":
				exact := versions[0]
				exact.ContractID++
				student := int64(mealStudent)
				exact.StudentID = &student
				versions = append(versions, exact)
				wantContract = exact.ContractID
			case "two children with the same name":
				other := mia(3)
				other.StudentID++
				releases = append(releases, mealRelease(t, "2026-02-01", other))
			case "two unlinked contracts":
				other := versions[0]
				other.ContractID++
				versions = append(versions, other)
			case "one unlinked contract":
				wantContract = versions[0].ContractID
			}
			billMeals(&month, careMealInput{month: day(t, mealRunMonth), today: day(t, mealToday),
				tariffs:  []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-01-01", 480)},
				releases: releases, versions: versions})
			if wantContract == 0 {
				assert.Empty(t, month.invoices, "ambiguous names must not charge any family")
				assert.Len(t, month.mealUnbilled, len(releases))
				return
			}
			require.Len(t, month.invoices, 1)
			assert.Equal(t, wantContract, month.invoices[0].ContractID)
		})
	}
}

func TestCareMealOnlyInvoiceKeepsTheContractsStudentLink(t *testing.T) {
	t.Parallel()
	month, versions := mealMonth(t)
	require.NotNil(t, versions[0].StudentID)
	billMeals(&month, careMealInput{month: day(t, mealRunMonth), today: day(t, mealToday),
		tariffs:  []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 480)},
		releases: []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01", mia(2))}, versions: versions})
	require.Len(t, month.invoices, 1)
	assert.Equal(t, versions[0].StudentID, month.invoices[0].StudentID)
}

// mealMonth is the preview of the run before the Mensa: the contract of Mia
// in April, booking no module (a Mensa-only contract) unless services name
// some.
func mealMonth(t *testing.T, services ...parentbilling.CareContractService) (careMonth, []parentbilling.CareContractVersion) {
	t.Helper()
	versions := []parentbilling.CareContractVersion{sjrVersion(t, contractMia, schoolIspringen, 1, mealRunMonth, "2026-04-30", services...)}
	month, err := buildCareMonth(careMonthInput{month: day(t, mealRunMonth), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: versions, tariffs: sjrTariffs(t), invoiced: map[int64]parentbilling.CareInvoice{},
		recipients: map[int64]parentbilling.CareRecipient{contractMia: {FirstName: "Eva", LastName: "Beispiel"}}})
	require.NoError(t, err)
	return month, versions
}

type mealLine struct {
	month    string
	quantity int
	unit     int64
	amount   int64
}

func TestCareMealTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		tariffs  func(t *testing.T) []parentbilling.CareMealTariff
		releases func(t *testing.T) []parentbilling.CareMealRelease
		billed   []string
		invoiced bool
		lines    []mealLine
		total    int64
		gaps     []string
		unbilled []string
	}{
		{
			// MK-001 (Hardt): 2 Mittagessen im Januar, Februar und März à
			// 4,80 € = 9,60 € je Monat, Gesamtbetrag 28,80 € auf einer
			// Rechnung vom 01.04.2026.
			name: "Hardt MK-001: several months on one invoice",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-01-01", 480)}
			},
			releases: func(t *testing.T) []parentbilling.CareMealRelease {
				return []parentbilling.CareMealRelease{mealRelease(t, "2026-01-01", mia(2)), mealRelease(t, "2026-02-01", mia(2)), mealRelease(t, "2026-03-01", mia(2))}
			},
			lines: []mealLine{{"2026-01-01", 2, 480, 960}, {"2026-02-01", 2, 480, 960}, {"2026-03-01", 2, 480, 960}},
			total: 2880,
		},
		{
			// Pauschale 55,00 € je Kind und Monat, whatever it ate: March
			// with no meal still bills 55,00 €.
			name: "flat amount per child and month",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModeFlat, "2026-03-01", 5500)}
			},
			releases: func(t *testing.T) []parentbilling.CareMealRelease {
				return []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01", mia(0))}
			},
			lines: []mealLine{{"2026-03-01", 1, 5500, 5500}},
			total: 5500,
		},
		{
			// Late release: February was billed in the March run; January
			// was released only now. The run bills January and March
			// (2 × 4,80 € each), never February again: 19,20 €.
			name: "late release bills the months not billed yet",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-01-01", 480)}
			},
			releases: func(t *testing.T) []parentbilling.CareMealRelease {
				return []parentbilling.CareMealRelease{mealRelease(t, "2026-01-01", mia(2)), mealRelease(t, "2026-02-01", mia(2)), mealRelease(t, "2026-03-01", mia(2))}
			},
			billed: []string{"2026-02-01"},
			lines:  []mealLine{{"2026-01-01", 2, 480, 960}, {"2026-03-01", 2, 480, 960}},
			total:  1920,
		},
		{
			// Cent exact without floating point: 7 × 4,85 € = 33,95 €.
			name: "cents stay exact",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 485)}
			},
			releases: func(t *testing.T) []parentbilling.CareMealRelease {
				return []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01", mia(7))}
			},
			lines: []mealLine{{"2026-03-01", 7, 485, 3395}},
			total: 3395,
		},
		{
			// The price of the meal month counts, not the run's: January
			// 2 × 4,80 € = 9,60 €, March after the new price 2 × 5,00 € =
			// 10,00 €; February is a gap.
			name: "each month at its own price",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-01-01", 480),
					mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 500)}
			},
			releases: func(t *testing.T) []parentbilling.CareMealRelease {
				return []parentbilling.CareMealRelease{mealRelease(t, "2026-01-01", mia(2)), mealRelease(t, "2026-03-01", mia(2))}
			},
			lines: []mealLine{{"2026-01-01", 2, 480, 960}, {"2026-03-01", 2, 500, 1000}},
			total: 1960,
			gaps:  []string{"2026-02-01"},
		},
		{
			// No release, nothing billed: January to March ended without
			// one and show as gaps; April has not ended and is no gap.
			name: "an ended month without release is a gap",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-01-01", 480)}
			},
			releases: func(*testing.T) []parentbilling.CareMealRelease { return nil },
			gaps:     []string{"2026-01-01", "2026-02-01", "2026-03-01"},
		},
		{
			// Per meal, a month without a meal bills nothing.
			name: "no meal, no position",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 480)}
			},
			releases: func(t *testing.T) []parentbilling.CareMealRelease {
				return []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01", mia(0))}
			},
		},
		{
			// A child of the release without a contract at the school.
			name: "a child without a contract stays out",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 480)}
			},
			releases: func(t *testing.T) []parentbilling.CareMealRelease {
				return []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01",
					parentbilling.CareMealChild{StudentID: 77, FirstName: "Ben", LastName: "Fremd", MealDays: 3})}
			},
			unbilled: []string{parentbilling.MealUnbilledNoContract},
		},
		{
			// An earlier run of April issued the contract's invoice: the
			// meals wait for the May run.
			name: "an issued contract keeps its meals for the next month",
			tariffs: func(t *testing.T) []parentbilling.CareMealTariff {
				return []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 480)}
			},
			releases: func(t *testing.T) []parentbilling.CareMealRelease {
				return []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01", mia(2))}
			},
			invoiced: true,
			unbilled: []string{parentbilling.MealUnbilledContractBilled},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			month, versions := mealMonth(t)
			in := careMealInput{month: day(t, mealRunMonth), today: day(t, mealToday), schools: map[int64]string{schoolIspringen: "Ispringen"},
				tariffs: tc.tariffs(t), releases: tc.releases(t), billed: map[CareMealKey]bool{}, versions: versions,
				invoiced: map[int64]parentbilling.CareInvoice{}, recipients: map[int64]parentbilling.CareRecipient{contractMia: {FirstName: "Eva", LastName: "Beispiel"}}}
			for _, billed := range tc.billed {
				in.billed[CareMealKey{SchoolID: schoolIspringen, StudentID: mealStudent, Month: day(t, billed)}] = true
			}
			if tc.invoiced {
				in.invoiced[contractMia] = parentbilling.CareInvoice{ContractID: contractMia}
			}
			billMeals(&month, in)

			gaps := []string{}
			for _, gap := range month.mealGaps {
				gaps = append(gaps, gap.Month.String())
			}
			assert.Equal(t, append([]string{}, tc.gaps...), gaps, "gaps")
			unbilled := []string{}
			for _, entry := range month.mealUnbilled {
				unbilled = append(unbilled, entry.Reason)
			}
			assert.Equal(t, append([]string{}, tc.unbilled...), unbilled, "unbilled")
			if len(tc.lines) == 0 {
				assert.Empty(t, month.invoices, "no invoice")
				return
			}
			require.Len(t, month.invoices, 1)
			invoice := month.invoices[0]
			lines := []mealLine{}
			for _, position := range invoice.MealPositions {
				lines = append(lines, mealLine{position.MealMonth.String(), position.Quantity, position.UnitCents, position.AmountCents})
			}
			assert.Equal(t, tc.lines, lines)
			assert.Equal(t, tc.total, invoice.TotalCents)
			assert.Equal(t, contractMia, invoice.ContractID)
			assert.Equal(t, "Eva", invoice.Recipient.FirstName, "a Mensa-only invoice goes to the contract's recipient")
		})
	}
}

// TestCareMealsJoinTheModuleInvoiceAndNoRuleReducesThem pins ADR 0168:
// the Mensa goes onto the invoice of the child's modules, and no rule of
// the chain (#4685) reaches it, because the rules act on modules. Mia books
// Monday until 14:00 (32,00 €) and Noah Monday until 16:30 (45,50 €), one
// family. Half from the second child: Noah is first (45,50 €), Mia second
// (32,00 − 16,00 = 16,00 €). Both ate 2 × 4,80 € = 9,60 € in March, in full:
// Mia 16,00 + 9,60 = 25,60 €, Noah 45,50 + 9,60 = 55,10 €.
func TestCareMealsJoinTheModuleInvoiceAndNoRuleReducesThem(t *testing.T) {
	t.Parallel()
	mon := booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00")
	monLate := booked("mon", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30")
	versions := []parentbilling.CareContractVersion{
		sjrVersion(t, contractMia, schoolIspringen, 1, mealRunMonth, "2026-04-30", mon),
		sjrVersion(t, contractNoah, schoolIspringen, 1, mealRunMonth, "2026-04-30", monLate),
	}
	family := parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel", Email: familyEva}
	recipients := map[int64]parentbilling.CareRecipient{contractMia: family, contractNoah: family}
	month, err := buildCareMonth(careMonthInput{month: day(t, mealRunMonth), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: versions, invoiced: map[int64]parentbilling.CareInvoice{}, recipients: recipients,
		tariffs: []parentbilling.CareTariff{price(t, schoolIspringen, moduleUntil1400, "Betreuung bis 14:00 Uhr", "mon", "2026-01-01", 3200),
			price(t, schoolIspringen, moduleUntil1630, "Betreuung bis 16:30 Uhr", "mon", "2026-01-01", 4550)},
		rules: []parentbilling.ContributionRule{careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen},
			func(rule *parentbilling.ContributionRule) {
				halfFromSecond(rule)
				rule.ValidFrom = day(t, "2026-01-01")
			})}})
	require.NoError(t, err)
	noah := parentbilling.CareMealChild{StudentID: contractNoah + 9000, FirstName: "Noah", LastName: "Beispiel", MealDays: 2}
	billMeals(&month, careMealInput{month: day(t, mealRunMonth), today: day(t, mealToday), schools: map[int64]string{schoolIspringen: "Ispringen"},
		tariffs:  []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 480)},
		releases: []parentbilling.CareMealRelease{mealRelease(t, "2026-03-01", mia(2), noah)}, billed: map[CareMealKey]bool{},
		versions: versions, invoiced: map[int64]parentbilling.CareInvoice{}, recipients: recipients})

	totals := map[int64]int64{}
	for _, invoice := range month.invoices {
		totals[invoice.ContractID] = invoice.TotalCents
		require.Len(t, invoice.MealPositions, 1)
		assert.Equal(t, int64(960), invoice.MealPositions[0].AmountCents, "the Mensa stays in full")
		require.Len(t, invoice.Positions, 1, "the Mensa joins the module invoice")
	}
	assert.Equal(t, map[int64]int64{contractMia: 1600 + 960, contractNoah: 4550 + 960}, totals)
	assert.Equal(t, int64(1600+960+4550+960), month.total(), "the release compares this sum")
}

// A month that starts before the attendance the school still keeps can no
// longer be released: the run names it as no longer billable instead of a
// release that never comes. The other gaps stay open.
func TestCareMealGapBeforeTheAttendanceWindowIsExpired(t *testing.T) {
	t.Parallel()
	month, versions := mealMonth(t)
	billMeals(&month, careMealInput{month: day(t, mealRunMonth), today: day(t, mealToday),
		schools:    map[int64]string{schoolIspringen: "Ispringen"},
		tariffs:    []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-01-01", 480)},
		attendance: map[int64]calendar.Date{schoolIspringen: day(t, "2026-02-10")},
		versions:   versions})
	assert.Equal(t, []parentbilling.CareMealGap{
		{SchoolID: schoolIspringen, SchoolName: "Ispringen", Month: day(t, "2026-01-01"), Expired: true},
		{SchoolID: schoolIspringen, SchoolName: "Ispringen", Month: day(t, "2026-02-01"), Expired: true},
		{SchoolID: schoolIspringen, SchoolName: "Ispringen", Month: day(t, "2026-03-01")},
	}, month.mealGaps)
}

// A school that left the Träger keeps the prices of the months it billed,
// but its months wait for no release: the run names no gap without a
// school name in every later month.
func TestCareMealGapsSkipASchoolOutsideTheTraeger(t *testing.T) {
	t.Parallel()
	month, versions := mealMonth(t)
	departed := parentbilling.CareMealTariff{SchoolID: schoolIspringen + 1, ValidFrom: day(t, "2026-01-01"),
		Mode: parentbilling.MealModePerMeal, AmountCents: 480}
	billMeals(&month, careMealInput{month: day(t, mealRunMonth), today: day(t, mealToday),
		schools:  map[int64]string{schoolIspringen: "Ispringen"},
		tariffs:  []parentbilling.CareMealTariff{mealTariff(t, parentbilling.MealModePerMeal, "2026-03-01", 480), departed},
		versions: versions})
	assert.Equal(t, []parentbilling.CareMealGap{
		{SchoolID: schoolIspringen, SchoolName: "Ispringen", Month: day(t, "2026-03-01")},
	}, month.mealGaps)
}

// The fingerprint of the review covers the Mensa months a run does not
// bill (#5916): a month without a release that becomes no longer billable,
// or a school that releases it, changes the reviewed state.
func TestCareReviewFingerprintCoversTheMensaGaps(t *testing.T) {
	t.Parallel()
	run := parentbilling.CareRun{ID: 7, SchoolIDs: []int64{schoolIspringen}}
	gap := parentbilling.CareMealGap{SchoolID: schoolIspringen, SchoolName: "Ispringen", Month: day(t, "2026-02-01")}
	review := func(gaps ...parentbilling.CareMealGap) reviewedMonth {
		return reviewedMonth{careMonth: careMonth{mealGaps: gaps, mealUnbilled: []parentbilling.CareMealUnbilled{}}}
	}
	open := previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, review(gap))
	expired := gap
	expired.Expired = true
	assert.NotEqual(t, open, previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, review(expired)), "no longer billable")
	assert.NotEqual(t, open, previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, review()), "released since the review")
	assert.Equal(t, open, previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, review(gap)), "the same state")
}

// Billed apart, the Mensa of a contract leaves the care's invoice with its
// amount; an invoice with nothing but Mensa becomes the Mensa invoice.
func TestSplitMealInvoicesPutsTheMensaOnAnInvoiceOfItsOwn(t *testing.T) {
	t.Parallel()
	meal := parentbilling.CareMealPosition{MealMonth: day(t, "2026-03-01"), Mode: parentbilling.MealModePerMeal, Quantity: 2, UnitCents: 480, AmountCents: 960}
	month := careMonth{invoices: []parentbilling.CareInvoice{
		{ContractID: 1, Service: parentbilling.ServiceCare, Positions: []parentbilling.CarePosition{{AmountCents: 7750}},
			MealPositions: []parentbilling.CareMealPosition{meal}, TotalCents: 7750 + 960},
		{ContractID: 2, Service: parentbilling.ServiceCare, Positions: []parentbilling.CarePosition{},
			MealPositions: []parentbilling.CareMealPosition{meal}, TotalCents: 960},
		{ContractID: 3, Service: parentbilling.ServiceCare, Positions: []parentbilling.CarePosition{{AmountCents: 3200}}, TotalCents: 3200},
	}}
	splitMealInvoices(&month)
	require.Len(t, month.invoices, 4)
	assert.Equal(t, []string{"care", "meal", "meal", "care"}, []string{month.invoices[0].Service, month.invoices[1].Service,
		month.invoices[2].Service, month.invoices[3].Service})
	assert.Equal(t, int64(7750), month.invoices[0].TotalCents)
	assert.Empty(t, month.invoices[0].MealPositions)
	assert.Equal(t, int64(960), month.invoices[1].TotalCents)
	assert.Empty(t, month.invoices[1].Positions)
	assert.Equal(t, int64(2), month.invoices[2].ContractID, "the Mensa-only invoice is the Mensa invoice")
	assert.Equal(t, int64(3200), month.invoices[3].TotalCents)
}
