package application

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The two SJR price lists 2026/27 (#6196), fully configured: Ispringen
// prices every booked weekday (Regelschule modules 1 to 3, Ganztagsschule
// modules 1, 4 and 5) with a fixed price for every further child;
// Birkenfeld prices modules 1 to 4 by the number of days per week and takes
// 25 % off from the second child on. Siblings count per school. The
// expected amounts are the hand calculations of the issue.

const (
	ispRS1  int64 = 601
	ispRS2  int64 = 602
	ispRS3  int64 = 603
	ispGTS1 int64 = 604
	ispGTS4 int64 = 605
	ispGTS5 int64 = 606
	birk1   int64 = 701
	birk2   int64 = 702
	birk3   int64 = 703
	birk4   int64 = 704

	sjrListStart = "2027-01-01"
)

var weekdaysMonFri = []string{"mon", "tue", "wed", "thu", "fri"}

// weekdayPrices prices weekdays of an Ispringen module: first child and
// every further child, the same on each day.
func weekdayPrices(t *testing.T, offering int64, name string, weekdays []string, first, further int64) []parentbilling.CareTariff {
	t.Helper()
	tariffs := make([]parentbilling.CareTariff, 0, len(weekdays))
	for _, weekday := range weekdays {
		tariffs = append(tariffs, parentbilling.CareTariff{SchoolID: schoolIspringen, CareOfferingID: offering, ModuleName: name,
			Weekday: weekday, ValidFrom: day(t, sjrListStart), MonthlyCents: first, SiblingCents: ptr(further)})
	}
	return tariffs
}

// scalePrices prices a Birkenfeld module by days per week, steps 1 to 5;
// further, when given, are the sibling steps.
func scalePrices(t *testing.T, offering int64, name string, meal bool, steps, further []int64) []parentbilling.CareTariff {
	t.Helper()
	tariffs := make([]parentbilling.CareTariff, 0, len(steps))
	for i, cents := range steps {
		tariff := parentbilling.CareTariff{SchoolID: schoolBirkenfeld, CareOfferingID: offering, ModuleName: name,
			DaysPerWeek: ptr(i + 1), ValidFrom: day(t, sjrListStart), MonthlyCents: cents, MealExtra: meal}
		if further != nil {
			tariff.SiblingCents = ptr(further[i])
		}
		tariffs = append(tariffs, tariff)
	}
	return tariffs
}

var (
	birkModule1 = []int64{500, 900, 1400, 1800, 2200}
	birkModule2 = []int64{1400, 2700, 4000, 5400, 6700}
	birkModule4 = []int64{2000, 4000, 6000, 8000, 10000}
	// The Birkenfeld list shows the sibling steps too: 75 % of each.
	birkModule1Further = []int64{375, 675, 1050, 1350, 1650}
	birkModule2Further = []int64{1050, 2025, 3000, 4050, 5025}
	birkModule4Further = []int64{1500, 3000, 4500, 6000, 7500}
)

// sjrPriceLists are both price lists; withBirkSiblingSteps also enters
// Birkenfeld's sibling steps, which a price-list rule then reads instead of
// the 25 % share.
func sjrPriceLists(t *testing.T, withBirkSiblingSteps bool) []parentbilling.CareTariff {
	t.Helper()
	var tariffs []parentbilling.CareTariff
	tariffs = append(tariffs, weekdayPrices(t, ispRS1, "Regelschule Modul 1", weekdaysMonFri, 650, 500)...)
	tariffs = append(tariffs, weekdayPrices(t, ispRS2, "Regelschule Modul 2", []string{"mon", "tue", "wed", "thu"}, 1600, 1250)...)
	tariffs = append(tariffs, weekdayPrices(t, ispRS2, "Regelschule Modul 2", []string{"fri"}, 1400, 1050)...)
	tariffs = append(tariffs, weekdayPrices(t, ispRS3, "Regelschule Modul 3", weekdaysMonFri, 650, 500)...)
	tariffs = append(tariffs, weekdayPrices(t, ispGTS1, "Ganztagsschule Modul 1", weekdaysMonFri, 650, 500)...)
	tariffs = append(tariffs, weekdayPrices(t, ispGTS4, "Ganztagsschule Modul 4", []string{"tue", "wed", "thu"}, 450, 350)...)
	tariffs = append(tariffs, weekdayPrices(t, ispGTS5, "Ganztagsschule Modul 5", []string{"mon"}, 1600, 1250)...)
	tariffs = append(tariffs, weekdayPrices(t, ispGTS5, "Ganztagsschule Modul 5", []string{"fri"}, 1400, 1050)...)
	var further1, further2, further4 []int64
	if withBirkSiblingSteps {
		further1, further2, further4 = birkModule1Further, birkModule2Further, birkModule4Further
	}
	tariffs = append(tariffs, scalePrices(t, birk1, "Modul 1 ohne Mittagessen", false, birkModule1, further1)...)
	tariffs = append(tariffs, scalePrices(t, birk2, "Modul 2 ohne Mittagessen", false, birkModule2, further2)...)
	// Modul 3 costs what Modul 2 costs; the meal comes on top (Mensa).
	tariffs = append(tariffs, scalePrices(t, birk3, "Modul 3 mit Mittagessen", true, birkModule2, further2)...)
	tariffs = append(tariffs, scalePrices(t, birk4, "Modul 4 mit Mittagessen", true, birkModule4, further4)...)
	return tariffs
}

// sjrLists are the heads: Ispringen bills nothing in August.
func sjrLists(t *testing.T) []parentbilling.CarePriceListHead {
	t.Helper()
	return []parentbilling.CarePriceListHead{
		{SchoolID: schoolIspringen, ValidFrom: day(t, sjrListStart), FreeMonths: []int{8}},
		{SchoolID: schoolBirkenfeld, ValidFrom: day(t, sjrListStart), FreeMonths: []int{}},
	}
}

// sjrRules: Ispringen charges each further child the sibling price of the
// price list, Birkenfeld takes 25 % off from the second child; each rule
// counts only the children of its own school.
func sjrRules(t *testing.T, birkByPriceList bool) []parentbilling.ContributionRule {
	t.Helper()
	isp := careRule(t, 1, parentbilling.RuleSibling, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
		rule.Name, rule.SiblingFromChild, rule.TariffSiblingPrice = "Geschwisterpreis Ispringen", ptr(2), true
	})
	birk := careRule(t, 2, parentbilling.RuleSibling, []int64{schoolBirkenfeld}, func(rule *parentbilling.ContributionRule) {
		rule.Name, rule.SiblingFromChild = "Geschwister Birkenfeld", ptr(2)
		if birkByPriceList {
			rule.TariffSiblingPrice = true
		} else {
			rule.DiscountBasisPoints = ptr(2500)
		}
	})
	return []parentbilling.ContributionRule{isp, birk}
}

// sjrChild is one contract of a child: its school, family, date of birth
// and booked modules; from is the start of the contract.
type sjrChild struct {
	contract int64
	name     string
	school   int64
	family   string
	born     string
	from     string
	services []parentbilling.CareContractService
}

func bookOn(offering int64, name string, weekdays ...string) []parentbilling.CareContractService {
	services := make([]parentbilling.CareContractService, len(weekdays))
	for i, weekday := range weekdays {
		services[i] = booked(weekday, offering, name, fmt.Sprintf("%02d:00", 8+offering%10))
	}
	return services
}

func join(groups ...[]parentbilling.CareContractService) []parentbilling.CareContractService {
	var all []parentbilling.CareContractService
	for _, group := range groups {
		all = append(all, group...)
	}
	return all
}

// sjrFamilies are the example families of the golden test.
func sjrFamilies() []sjrChild {
	rs2 := func(weekdays ...string) []parentbilling.CareContractService {
		return bookOn(ispRS2, "Regelschule Modul 2", weekdays...)
	}
	return []sjrChild{
		// Ispringen, Modul 2 Mo–Fr, two children: 78,00 € + 60,50 €.
		{801, "Anna", schoolIspringen, "a", "2017-03-01", sjrListStart, rs2(weekdaysMonFri...)},
		{802, "Ben", schoolIspringen, "a", "2019-05-01", sjrListStart, rs2(weekdaysMonFri...)},
		// Modul 2 Mo + Fr as second child: 23,00 €; the first child books
		// Modul 1 Mo–Fr: 32,50 €.
		{803, "Clara", schoolIspringen, "b", "2017-01-01", sjrListStart, bookOn(ispRS1, "Regelschule Modul 1", weekdaysMonFri...)},
		{804, "Dora", schoolIspringen, "b", "2019-01-01", sjrListStart, rs2("mon", "fri")},
		// Modul 2 Mo + Fr alone: 30,00 €.
		{805, "Emil", schoolIspringen, "c", "2018-01-01", sjrListStart, rs2("mon", "fri")},
		// Ganztagsschule: Modul 5 Mo + Fr first (30,00 €), Modul 4 Di–Do
		// second: 3 × 3,50 € = 10,50 €.
		{806, "Finn", schoolIspringen, "d", "2017-01-01", sjrListStart, bookOn(ispGTS5, "Ganztagsschule Modul 5", "mon", "fri")},
		{807, "Greta", schoolIspringen, "d", "2019-01-01", sjrListStart, bookOn(ispGTS4, "Ganztagsschule Modul 4", "tue", "wed", "thu")},
		// Several modules: Modul 1 + 2 Mo–Fr first (32,50 + 78,00 =
		// 110,50 €); Modul 1 + 3 Mo–Fr second: 25,00 + 25,00 = 50,00 €.
		{808, "Hanna", schoolIspringen, "e", "2016-01-01", sjrListStart,
			join(bookOn(ispRS1, "Regelschule Modul 1", weekdaysMonFri...), rs2(weekdaysMonFri...))},
		{809, "Ida", schoolIspringen, "e", "2018-01-01", sjrListStart,
			join(bookOn(ispRS1, "Regelschule Modul 1", weekdaysMonFri...), bookOn(ispRS3, "Regelschule Modul 3", weekdaysMonFri...))},
		// Birkenfeld, Modul 4 with 3 days, two children: 60,00 + 45,00 €.
		{811, "Jonas", schoolBirkenfeld, "f", "2017-01-01", sjrListStart, bookOn(birk4, "Modul 4 mit Mittagessen", "mon", "tue", "wed")},
		{812, "Klara", schoolBirkenfeld, "f", "2019-01-01", sjrListStart, bookOn(birk4, "Modul 4 mit Mittagessen", "mon", "wed", "fri")},
		// Modul 1 with 3 days alone: the step, not 3 × 5,00 €: 14,00 €.
		{813, "Lena", schoolBirkenfeld, "g", "2018-01-01", sjrListStart, bookOn(birk1, "Modul 1 ohne Mittagessen", "mon", "wed", "fri")},
		// Modul 1 with 3 days second (10,50 €) after Modul 2 with 2 days.
		{814, "Max", schoolBirkenfeld, "h", "2017-01-01", sjrListStart, bookOn(birk2, "Modul 2 ohne Mittagessen", "mon", "tue")},
		{815, "Nora", schoolBirkenfeld, "h", "2019-01-01", sjrListStart, bookOn(birk1, "Modul 1 ohne Mittagessen", "tue", "wed", "thu")},
		// Modul 2 with 2 days second (20,25 €) after Modul 4 with 2 days.
		{816, "Ole", schoolBirkenfeld, "i", "2017-01-01", sjrListStart, bookOn(birk4, "Modul 4 mit Mittagessen", "mon", "tue")},
		{817, "Paula", schoolBirkenfeld, "i", "2019-01-01", sjrListStart, bookOn(birk2, "Modul 2 ohne Mittagessen", "wed", "thu")},
		// One family at both schools: no child is the second of the other
		// school's child.
		{818, "Quentin", schoolIspringen, "j", "2019-01-01", sjrListStart, rs2(weekdaysMonFri...)},
		{819, "Rosa", schoolBirkenfeld, "j", "2017-01-01", sjrListStart, bookOn(birk4, "Modul 4 mit Mittagessen", "mon", "tue", "wed")},
		// Entry in the middle of the month bills the full month: Modul 3
		// with 3 days costs Modul 2's 40,00 €, the meal on top.
		{820, "Sven", schoolBirkenfeld, "k", "2018-01-01", "2027-01-15", bookOn(birk3, "Modul 3 mit Mittagessen", "mon", "tue", "thu")},
	}
}

// sjrInput is the month of the example families under both price lists.
func sjrInput(t *testing.T, month, rule string, children []sjrChild, birkByPriceList bool) careMonthInput {
	t.Helper()
	versions := make([]parentbilling.CareContractVersion, 0, len(children))
	recipients := map[int64]parentbilling.CareRecipient{}
	for _, child := range children {
		schoolName := "Grundschule Ispringen"
		if child.school == schoolBirkenfeld {
			schoolName = "Grundschule Birkenfeld"
		}
		student := child.contract + 9000
		born := day(t, child.born)
		versions = append(versions, parentbilling.CareContractVersion{
			ContractID: child.contract, SchoolID: child.school, SchoolName: schoolName, Number: 1,
			ChildFirstName: child.name, ChildLastName: "Beispiel", StudentID: &student, ChildBirthDate: &born,
			From: day(t, child.from), Until: monthEnd(day(t, month)), Services: child.services,
		})
		recipients[child.contract] = parentbilling.CareRecipient{FirstName: "Eva", LastName: "Beispiel",
			Email: "familie-" + child.family + "@example.org"}
	}
	return careMonthInput{month: day(t, month), rule: rule, versions: versions, tariffs: sjrPriceLists(t, birkByPriceList),
		recipients: recipients, rules: sjrRules(t, birkByPriceList), lists: sjrLists(t)}
}

func totalsByContract(month careMonth) map[int64]int64 {
	totals := map[int64]int64{}
	for _, invoice := range month.invoices {
		totals[invoice.ContractID] = invoice.TotalCents
	}
	return totals
}

// sjrExpected are the hand calculations of #6196 per contract.
var sjrExpected = map[int64]int64{
	801: 7800, 802: 6050, // Ispringen Modul 2 Mo–Fr: 1. and 2. child, family 138,50 €
	803: 3250, 804: 2300, // Modul 1 Mo–Fr first; Modul 2 Mo + Fr second
	805: 3000,            // Modul 2 Mo + Fr alone
	806: 3000, 807: 1050, // Ganztagsschule Modul 4 Di–Do second
	808: 11050, 809: 5000, // several modules
	811: 6000, 812: 4500, // Birkenfeld Modul 4, 3 days, family 105,00 €
	813: 1400,            // Modul 1, 3 days alone
	814: 2700, 815: 1050, // Modul 1, 3 days second
	816: 4000, 817: 2025, // Modul 2, 2 days second
	818: 7800, 819: 6000, // two schools count apart
	820: 4000, // entry on the 15th, full month
}

func TestSJRPriceListsGolden(t *testing.T) {
	t.Parallel()
	for _, birkByPriceList := range []bool{false, true} {
		t.Run(fmt.Sprintf("birkenfeld sibling steps from the price list %v", birkByPriceList), func(t *testing.T) {
			t.Parallel()
			month, err := buildCareMonth(sjrInput(t, "2027-01-01", parentbilling.MonthRuleVersionOnFirst, sjrFamilies(), birkByPriceList))
			require.NoError(t, err)
			assert.Empty(t, month.unpriced)
			assert.Equal(t, sjrExpected, totalsByContract(month))
		})
	}
}

func TestSJRPriceListsGoldenPositions(t *testing.T) {
	t.Parallel()
	month, err := buildCareMonth(sjrInput(t, "2027-01-01", parentbilling.MonthRuleVersionOnFirst, sjrFamilies(), false))
	require.NoError(t, err)
	invoices := map[int64]parentbilling.CareInvoice{}
	for _, invoice := range month.invoices {
		invoices[invoice.ContractID] = invoice
	}

	// Ispringen: one position per module with its weekday prices; the
	// second child's reduction lowers it to the sibling prices.
	ben := invoices[802]
	require.Len(t, ben.Positions, 1)
	assert.Nil(t, ben.Positions[0].DaysPerWeek, "Ispringen prices per weekday")
	assert.Equal(t, int64(7800), ben.Positions[0].AmountCents)
	assert.Equal(t, int64(6050), ben.Positions[0].PayableCents)
	weekdayCents := map[string]int64{}
	for _, weekday := range ben.Positions[0].Weekdays {
		weekdayCents[weekday.Weekday] = weekday.MonthlyCents
	}
	assert.Equal(t, map[string]int64{"mon": 1600, "tue": 1600, "wed": 1600, "thu": 1600, "fri": 1400}, weekdayCents)
	require.Len(t, ben.Reductions, 1)
	assert.True(t, ben.Reductions[0].TariffSiblingPrice)
	assert.Equal(t, int64(-1750), ben.Reductions[0].AmountCents)
	assert.Equal(t, 2, *ben.Reductions[0].ChildRank)
	assert.Empty(t, invoices[801].Reductions, "the first child pays the full price")

	// Several modules: each position lands on its own sibling price.
	ida := invoices[809]
	require.Len(t, ida.Positions, 2)
	for _, position := range ida.Positions {
		assert.Equal(t, int64(3250), position.AmountCents)
		assert.Equal(t, int64(2500), position.PayableCents, position.ModuleName)
	}

	// Birkenfeld: the scale step of the booked days, shared over them.
	klara := invoices[812]
	require.Len(t, klara.Positions, 1)
	position := klara.Positions[0]
	require.NotNil(t, position.DaysPerWeek)
	assert.Equal(t, 3, *position.DaysPerWeek)
	assert.Equal(t, int64(6000), *position.TierMonthlyCents)
	assert.Equal(t, int64(6000), position.AmountCents)
	assert.True(t, position.MealExtra, "Modul 4 bills the meal apart")
	var shared int64
	for _, weekday := range position.Weekdays {
		shared += weekday.AmountCents
	}
	assert.Equal(t, int64(6000), shared, "the weekdays add up to the step")
	require.Len(t, klara.Reductions, 1)
	assert.Equal(t, int64(-1500), klara.Reductions[0].AmountCents, "25 % from the second child")

	// Modul 3: Modul 2's price, the meal never as a silent zero.
	sven := invoices[820]
	require.Len(t, sven.Positions, 1)
	assert.True(t, sven.Positions[0].MealExtra)
	assert.Equal(t, int64(4000), sven.Positions[0].AmountCents)
}

func TestSJRFamilyTotals(t *testing.T) {
	t.Parallel()
	month, err := buildCareMonth(sjrInput(t, "2027-01-01", parentbilling.MonthRuleVersionOnFirst, sjrFamilies(), false))
	require.NoError(t, err)
	families := map[string]int64{}
	for _, invoice := range month.invoices {
		families[invoice.FamilyEmail] += invoice.TotalCents
	}
	assert.Equal(t, int64(13850), families["familie-a@example.org"], "Ispringen Modul 2 Mo–Fr, two children")
	assert.Equal(t, int64(10500), families["familie-f@example.org"], "Birkenfeld Modul 4 with 3 days, two children")
	assert.Equal(t, int64(13800), families["familie-j@example.org"], "a child per school gets no sibling price")
}

// August is free at Ispringen only; the run bills Birkenfeld and counts the
// Ispringen contracts it leaves out.
func TestSJRAugustIsFreeAtIspringen(t *testing.T) {
	t.Parallel()
	month, err := buildCareMonth(sjrInput(t, "2027-08-01", parentbilling.MonthRuleVersionOnFirst, sjrFamilies()[:12], false))
	require.NoError(t, err)
	for _, invoice := range month.invoices {
		assert.Equal(t, schoolBirkenfeld, invoice.SchoolID, "no Ispringen invoice in August")
	}
	assert.Equal(t, map[int64]int{schoolIspringen: 9}, month.freeBySchool)
	assert.Equal(t, int64(6000), totalsByContract(month)[811])

	september, err := buildCareMonth(sjrInput(t, "2027-09-01", parentbilling.MonthRuleVersionOnFirst, sjrFamilies()[:2], false))
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{801: 7800, 802: 6050}, totalsByContract(september))
}

// Pro rata a scale step costs its share of the month, rounded half up to
// the cent: 40,00 € × 17/31 = 21,935… → 21,94 €.
func TestSJRScaleProRataRoundsToTheCent(t *testing.T) {
	t.Parallel()
	children := []sjrChild{sjrFamilies()[18]}
	month, err := buildCareMonth(sjrInput(t, "2027-01-01", parentbilling.MonthRuleProRata, children, false))
	require.NoError(t, err)
	require.Len(t, month.invoices, 1)
	assert.Equal(t, int64(2194), month.invoices[0].TotalCents)
}

// A booked number of days without a step is unpriced, never billed as
// zero; it names the missing step.
func TestSJRMissingScaleStepIsUnpriced(t *testing.T) {
	t.Parallel()
	input := sjrInput(t, "2027-01-01", parentbilling.MonthRuleVersionOnFirst, []sjrChild{
		{830, "Tom", schoolBirkenfeld, "z", "2018-01-01", sjrListStart, bookOn(birk1, "Modul 1 ohne Mittagessen", "mon", "tue", "wed", "thu", "fri", "sat")},
	}, false)
	month, err := buildCareMonth(input)
	require.NoError(t, err)
	assert.Empty(t, month.invoices)
	require.Len(t, month.unpriced, 1)
	require.NotNil(t, month.unpriced[0].DaysPerWeek)
	assert.Equal(t, 6, *month.unpriced[0].DaysPerWeek)
	assert.Empty(t, month.unpriced[0].Weekday)
}

// A module that changes its form bills only by the new one: weekday
// prices before the change do not count once a scale starts.
func TestPriceFormChangeTakesEffectFromItsMonth(t *testing.T) {
	t.Parallel()
	tariffs := []parentbilling.CareTariff{
		price(t, schoolBirkenfeld, birk1, "Modul 1", "mon", "2026-09-01", 700),
		price(t, schoolBirkenfeld, birk1, "Modul 1", "tue", "2026-09-01", 700),
	}
	tariffs = append(tariffs, scalePrices(t, birk1, "Modul 1", false, birkModule1, nil)...)
	book := newPriceBook(tariffs)
	december := day(t, "2026-12-01")
	january := day(t, sjrListStart)
	form, _, ok := book.module(schoolBirkenfeld, birk1, december)
	require.True(t, ok)
	assert.Equal(t, parentbilling.PriceFormWeekday, form)
	cents, ok := book.on(schoolBirkenfeld, birk1, "mon", december)
	assert.True(t, ok)
	assert.Equal(t, int64(700), cents)
	_, ok = book.on(schoolBirkenfeld, birk1, "mon", january)
	assert.False(t, ok, "a weekday price does not hold for a module priced by a scale")
	step, ok := book.scale(schoolBirkenfeld, birk1, 2, january)
	assert.True(t, ok)
	assert.Equal(t, int64(900), step.monthly)
	assert.Equal(t, int64(900), step.sibling, "without a sibling step the first child's price holds")
}

func TestBilledScaleStepsNeverChange(t *testing.T) {
	t.Parallel()
	before := sjrPriceLists(t, false)
	billed := []parentbilling.CareBilledPrice{{CareOfferingID: birk4, DaysPerWeek: ptr(3), Month: day(t, sjrListStart)}}
	changed := make([]parentbilling.CareTariff, 0, len(before))
	added := make([]parentbilling.CareTariff, 0, len(before))
	for _, tariff := range before {
		next := tariff
		if tariff.CareOfferingID == birk4 && *tariff.DaysPerWeek == 3 {
			next.MonthlyCents = 6500
		}
		changed = append(changed, next)
		if tariff.CareOfferingID == birk4 && *tariff.DaysPerWeek == 5 {
			continue
		}
		added = append(added, tariff)
	}
	start := day(t, sjrListStart)
	assert.True(t, changesBilledPrice(schoolBirkenfeld, start, billed, newPriceBook(before), newPriceBook(changed)),
		"the billed step keeps its price")
	assert.False(t, changesBilledPrice(schoolBirkenfeld, start, billed, newPriceBook(added), newPriceBook(before)),
		"a step no invoice billed may get a price")
	siblings := sjrPriceLists(t, true)
	assert.True(t, changesBilledPrice(schoolBirkenfeld, start, billed, newPriceBook(before), newPriceBook(siblings)),
		"the billed step keeps its sibling price")
	weekly := []parentbilling.CareTariff{price(t, schoolBirkenfeld, birk4, "Modul 4", "mon", sjrListStart, 2000)}
	assert.True(t, changesBilledPrice(schoolBirkenfeld, start, billed, newPriceBook(before), newPriceBook(weekly)),
		"a billed month keeps its price form")

	free := newFreeMonths([]parentbilling.CarePriceListHead{{SchoolID: schoolBirkenfeld, ValidFrom: start, FreeMonths: []int{1}}})
	assert.True(t, freesBilledMonth(schoolBirkenfeld, start, billed, free), "a billed month never turns free")
	assert.False(t, freesBilledMonth(schoolBirkenfeld, start, billed, newFreeMonths(nil)))
}

func TestPriceListValidationKeepsOneFormPerModule(t *testing.T) {
	t.Parallel()
	start := day(t, sjrListStart)
	valid := parentbilling.CarePriceList{SchoolID: schoolBirkenfeld, ValidFrom: start, FreeMonths: []int{8, 8, 7}, Prices: []parentbilling.CarePrice{
		{CareOfferingID: birk1, ModuleName: " Modul 1 ", DaysPerWeek: ptr(1), MonthlyCents: 500, SiblingCents: ptr(int64(375))},
		{CareOfferingID: birk1, ModuleName: "Modul 1", DaysPerWeek: ptr(2), MonthlyCents: 900},
	}}
	list, err := normalizePriceList(valid)
	require.NoError(t, err)
	assert.Equal(t, []int{7, 8}, list.FreeMonths)
	assert.Equal(t, "Modul 1", list.Prices[0].ModuleName)

	for name, broken := range map[string]parentbilling.CarePrice{
		"weekday and step":       {CareOfferingID: birk2, ModuleName: "M", Weekday: "mon", DaysPerWeek: ptr(1), MonthlyCents: 1},
		"neither":                {CareOfferingID: birk2, ModuleName: "M", MonthlyCents: 1},
		"step above seven":       {CareOfferingID: birk2, ModuleName: "M", DaysPerWeek: ptr(8), MonthlyCents: 1},
		"sibling above first":    {CareOfferingID: birk2, ModuleName: "M", DaysPerWeek: ptr(1), MonthlyCents: 100, SiblingCents: ptr(int64(101))},
		"negative sibling price": {CareOfferingID: birk2, ModuleName: "M", DaysPerWeek: ptr(1), MonthlyCents: 100, SiblingCents: ptr(int64(-1))},
		"other form same module": {CareOfferingID: birk1, ModuleName: "Modul 1", Weekday: "mon", MonthlyCents: 100},
		"other meal same module": {CareOfferingID: birk1, ModuleName: "Modul 1", DaysPerWeek: ptr(3), MonthlyCents: 1400, MealExtra: true},
		"step twice":             {CareOfferingID: birk1, ModuleName: "Modul 1", DaysPerWeek: ptr(2), MonthlyCents: 900},
	} {
		list := valid
		list.FreeMonths = []int{8}
		list.Prices = append([]parentbilling.CarePrice{}, valid.Prices...)
		list.Prices = append(list.Prices, broken)
		_, err := normalizePriceList(list)
		assert.Error(t, err, name)
	}
	invalidMonth := valid
	invalidMonth.FreeMonths = []int{13}
	_, err = normalizePriceList(invalidMonth)
	assert.Error(t, err)
}

// The price form is part of what a release checks: the same amount priced
// another way, or a free month, changes the fingerprint (#6177).
func TestFingerprintCoversThePriceForm(t *testing.T) {
	t.Parallel()
	run := parentbilling.CareRun{ID: 1, CareRunScope: parentbilling.CareRunScope{SchoolIDs: []int64{schoolBirkenfeld}}}
	invoice := parentbilling.CareInvoice{ContractID: 1, SchoolID: schoolBirkenfeld, TotalCents: 6000, Positions: []parentbilling.CarePosition{
		{CareOfferingID: birk4, AmountCents: 6000, Weekdays: []parentbilling.CareWeekdayPrice{{Weekday: "mon", MonthlyCents: 6000, AmountCents: 6000}}},
	}}
	weekly := reviewedMonth{careMonth: careMonth{invoices: []parentbilling.CareInvoice{invoice}}}
	scaled := invoice
	scaled.Positions = []parentbilling.CarePosition{invoice.Positions[0]}
	scaled.Positions[0].DaysPerWeek, scaled.Positions[0].TierMonthlyCents = ptr(1), ptr(int64(6000))
	byScale := reviewedMonth{careMonth: careMonth{invoices: []parentbilling.CareInvoice{scaled}}}
	freeMonth := weekly
	freeMonth.free = 1
	base := previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, weekly)
	require.NotEmpty(t, base)
	assert.NotEqual(t, base, previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, byScale))
	assert.NotEqual(t, base, previewFingerprint(run, parentbilling.MonthRuleVersionOnFirst, freeMonth))
}

// A sibling rule takes the sibling prices of the price list instead of a
// share, an amount or a price, never alongside one; no other kind keeps it.
func TestCareRuleTakesTheSiblingPricesOfThePriceList(t *testing.T) {
	t.Parallel()
	schools := []parentbilling.CareSchool{{ID: schoolIspringen}}
	input := parentbilling.CareRuleInput{Kind: parentbilling.RuleSibling, Name: "Geschwisterpreis", ValidFrom: day(t, sjrListStart),
		SchoolIDs: []int64{schoolIspringen}, SiblingFromChild: ptr(2), TariffSiblingPrice: true}
	_, err := normalizeCareRule(input, schools)
	require.NoError(t, err)

	both := input
	both.DiscountBasisPoints = ptr(2500)
	_, err = normalizeCareRule(both, schools)
	assert.Error(t, err, "the price-list mode excludes a share")

	income := input
	income.Kind = parentbilling.RuleIncomeLevel
	income.SiblingFromChild = nil
	_, err = normalizeCareRule(income, schools)
	assert.Error(t, err, "an income level has no sibling prices, so it lacks its share")
	income.DiscountBasisPoints = ptr(1000)
	normalized, err := normalizeCareRule(income, schools)
	require.NoError(t, err)
	assert.False(t, normalized.TariffSiblingPrice)
}

// An income reduction before the sibling stage counts towards the sibling
// price: the position never ends below it because of both.
func TestTariffSiblingPriceIsAnEndPrice(t *testing.T) {
	t.Parallel()
	input := sjrInput(t, "2027-01-01", parentbilling.MonthRuleVersionOnFirst, sjrFamilies()[:2], false)
	level := careRule(t, 3, parentbilling.RuleIncomeLevel, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
		rule.Name, rule.DiscountBasisPoints = "Stufe 1", ptr(1000)
	})
	input.rules = append(input.rules, level)
	input.evidences = []parentbilling.CareEvidence{evidence(t, 802, parentbilling.EvidenceIncomeLevel, ptr(int64(3)), sjrListStart, nil)}
	month, err := buildCareMonth(input)
	require.NoError(t, err)
	// Ben: 78,00 − 10 % = 70,20 €, then down to the sibling price 60,50 €.
	assert.Equal(t, int64(6050), totalsByContract(month)[802])
}

func TestTariffSiblingRoundingKeepsTheModuleTotal(t *testing.T) {
	t.Parallel()
	versions := []parentbilling.CareContractVersion{
		sjrVersion(t, contractMia, schoolIspringen, 1, january2027, january2027,
			booked("mon", moduleUntil1400, "Modul", "14:00")),
		sjrVersion(t, contractMia, schoolIspringen, 2, "2027-01-02", januaryLastDay2027,
			booked("tue", moduleUntil1400, "Modul", "14:00")),
	}
	first := price(t, schoolIspringen, moduleUntil1400, "Modul", "mon", january2027, 198)
	first.SiblingCents = ptr(int64(198))
	second := price(t, schoolIspringen, moduleUntil1400, "Modul", "tue", january2027, 250)
	second.SiblingCents = ptr(int64(50))
	invoice, missing := billContract(day(t, january2027), parentbilling.MonthRuleProRata, versions, true, newPriceBook([]parentbilling.CareTariff{first, second}))
	require.Empty(t, missing)
	require.Len(t, invoice.Positions, 2)
	assert.Equal(t, int64(248), invoice.TotalCents)
	entry := careChainEntry{invoice: &invoice, remaining: []int64{invoice.Positions[0].AmountCents, invoice.Positions[1].AmountCents}}
	rule := careRule(t, ruleSiblingIspringen, parentbilling.RuleSibling, []int64{schoolIspringen}, func(rule *parentbilling.ContributionRule) {
		rule.SiblingFromChild, rule.TariffSiblingPrice = ptr(2), true
	})
	entry.reduceToSiblingPrices(&rule, ptr(2), ptr(2))
	assert.Equal(t, int64(55), entry.remaining[0]+entry.remaining[1])
	for _, position := range invoice.Positions {
		assert.LessOrEqual(t, position.SiblingCents, position.AmountCents)
	}
}

func TestCareModulesKeepActualBookingSizes(t *testing.T) {
	t.Parallel()
	first := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Modul", "14:00"), booked("tue", moduleUntil1400, "Modul", "14:00"),
		booked("wed", moduleUntil1400, "Modul", "14:00"))
	second := sjrVersion(t, contractNoah, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("thu", moduleUntil1400, "Modul", "14:00"), booked("fri", moduleUntil1400, "Modul", "14:00"),
		booked("sat", moduleUntil1400, "Modul", "14:00"))
	modules := careModules([]parentbilling.CareContractVersion{first, second}, nil)
	require.Len(t, modules, 1)
	assert.Len(t, modules[0].BookedWeekdays, 6)
	assert.Equal(t, []int{3}, modules[0].BookedDaysPerWeek)
	first.Services[0].LastDay = ptr(day(t, "2027-01-15"))
	modules = careModules([]parentbilling.CareContractVersion{first, second}, nil)
	assert.Equal(t, []int{2, 3}, modules[0].BookedDaysPerWeek)
}
