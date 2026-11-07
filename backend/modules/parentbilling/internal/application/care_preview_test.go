package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Modules of the SJR example (#4684, SJR alignment line 1): Monday until
// 14:00, Tuesday until 16:30, Wednesday without care.
const (
	schoolIspringen    int64 = 11
	schoolBirkenfeld   int64 = 12
	moduleUntil1400    int64 = 101
	moduleUntil1630    int64 = 102
	moduleEarly        int64 = 103
	contractMia        int64 = 501
	contractNoah       int64 = 502
	january2027              = "2027-01-01"
	januaryLastDay2027       = "2027-01-31"
)

func day(t *testing.T, value string) calendar.Date {
	t.Helper()
	parsed, err := calendar.ParseDate(value)
	require.NoError(t, err)
	return parsed
}

// sjrVersion is a version of a contract of its own child: every contract
// names another child record unless a test gives two contracts one child.
func sjrVersion(t *testing.T, contractID, schoolID int64, number int, from, until string, services ...parentbilling.CareContractService) parentbilling.CareContractVersion {
	t.Helper()
	student := contractID + 9000
	return parentbilling.CareContractVersion{
		ContractID: contractID, SchoolID: schoolID, SchoolName: "Grundschule Ispringen", Number: number,
		ChildFirstName: "Mia", ChildLastName: "Beispiel", StudentID: &student,
		From: day(t, from), Until: day(t, until), Services: services,
	}
}

func booked(weekday string, offeringID int64, name, endsAt string) parentbilling.CareContractService {
	return parentbilling.CareContractService{Weekday: weekday, CareOfferingID: offeringID, ModuleName: name, EndsAt: endsAt}
}

func price(t *testing.T, schoolID, offeringID int64, name, weekday, from string, cents int64) parentbilling.CareTariff {
	t.Helper()
	return parentbilling.CareTariff{SchoolID: schoolID, CareOfferingID: offeringID, ModuleName: name, Weekday: weekday,
		ValidFrom: day(t, from), MonthlyCents: cents}
}

// sjrTariffs are the prices of Ispringen: per booked weekday and month,
// 32 € until 14:00 and 45,50 € until 16:30 (placeholders until Ute's list).
func sjrTariffs(t *testing.T) []parentbilling.CareTariff {
	t.Helper()
	var tariffs []parentbilling.CareTariff
	for _, weekday := range []string{"mon", "tue", "wed", "thu", "fri"} {
		tariffs = append(tariffs,
			price(t, schoolIspringen, moduleUntil1400, "Betreuung bis 14:00 Uhr", weekday, "2026-09-01", 3200),
			price(t, schoolIspringen, moduleUntil1630, "Betreuung bis 16:30 Uhr", weekday, "2026-09-01", 4550))
	}
	return tariffs
}

// TestCareMonthGoldenSJR is the golden example of the SJR pattern: Monday
// until 14:00, Tuesday until 16:30, Wednesday free. The invoice has one
// position per module with its weekdays, never one line per day, and the
// contribution is the sum of the booked weekdays.
func TestCareMonthGoldenSJR(t *testing.T) {
	t.Parallel()
	version := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"),
		booked("tue", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30"))
	month, err := buildCareMonth(careMonthInput{
		month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{version}, tariffs: sjrTariffs(t),
		recipients: map[int64]parentbilling.CareRecipient{contractMia: {FirstName: "Eva", LastName: "Beispiel", Email: "eva@example.org"}},
	})
	require.NoError(t, err)

	got, err := json.MarshalIndent(month.invoices, "", "  ")
	require.NoError(t, err)
	assert.JSONEq(t, `[{
		"id": 0, "number": "", "issued_on": null, "contract_id": 501, "school_id": 11,
		"school_name": "Grundschule Ispringen", "child_first_name": "Mia", "child_last_name": "Beispiel",
		"recipient": {"first_name": "Eva", "last_name": "Beispiel", "email": "eva@example.org"},
		"positions": [
			{"care_offering_id": 101, "module_name": "Betreuung bis 14:00 Uhr", "version_number": 1,
			 "service_from": "2027-01-01", "service_until": "2027-01-31",
			 "weekdays": [{"weekday": "mon", "monthly_cents": 3200, "amount_cents": 3200}], "amount_cents": 3200},
			{"care_offering_id": 102, "module_name": "Betreuung bis 16:30 Uhr", "version_number": 1,
			 "service_from": "2027-01-01", "service_until": "2027-01-31",
			 "weekdays": [{"weekday": "tue", "monthly_cents": 4550, "amount_cents": 4550}], "amount_cents": 4550}
		],
		"total_cents": 7750
	}]`, string(got))
	assert.Empty(t, month.unpriced)
	assert.Equal(t, int64(7750), month.total())
}

// TestCareMonthTwoModulesSameDay bills the early and the afternoon care of
// one child as two positions on one invoice (Ispringen, SJR line 4).
func TestCareMonthTwoModulesSameDay(t *testing.T) {
	t.Parallel()
	version := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleEarly, "Frühbetreuung", "08:00"), booked("thu", moduleEarly, "Frühbetreuung", "08:00"),
		booked("mon", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30"))
	tariffs := append(sjrTariffs(t),
		price(t, schoolIspringen, moduleEarly, "Frühbetreuung", "mon", "2026-09-01", 1500),
		price(t, schoolIspringen, moduleEarly, "Frühbetreuung", "thu", "2026-09-01", 1500))
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{version}, tariffs: tariffs})
	require.NoError(t, err)
	require.Len(t, month.invoices, 1)
	positions := month.invoices[0].Positions
	require.Len(t, positions, 2)
	assert.Equal(t, "Frühbetreuung", positions[0].ModuleName)
	assert.Len(t, positions[0].Weekdays, 2, "two booked weekdays, one position")
	assert.Equal(t, int64(3000), positions[0].AmountCents)
	assert.Equal(t, int64(4550), positions[1].AmountCents)
	assert.Equal(t, int64(7550), month.invoices[0].TotalCents)
}

// TestCareMonthSchoolsHaveTheirOwnPrices bills the same module name at two
// schools of one Träger with each school's price in the same run (SJR
// line 2).
func TestCareMonthSchoolsHaveTheirOwnPrices(t *testing.T) {
	t.Parallel()
	ispringen := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	birkenfeld := sjrVersion(t, contractNoah, schoolBirkenfeld, 1, january2027, januaryLastDay2027,
		booked("mon", 201, "Betreuung bis 14:00 Uhr", "14:00"))
	birkenfeld.SchoolName = "Grundschule Birkenfeld"
	tariffs := append(sjrTariffs(t), price(t, schoolBirkenfeld, 201, "Betreuung bis 14:00 Uhr", "mon", "2026-09-01", 2800))
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{ispringen, birkenfeld}, tariffs: tariffs})
	require.NoError(t, err)
	require.Len(t, month.invoices, 2)
	assert.Equal(t, "Grundschule Birkenfeld", month.invoices[0].SchoolName)
	assert.Equal(t, int64(2800), month.invoices[0].TotalCents)
	assert.Equal(t, int64(3200), month.invoices[1].TotalCents)
}

// TestCareMonthOneVersionPerDay refuses a month for which two versions of
// one contract hold on the same day: without the check the run would bill
// the 15th twice.
func TestCareMonthOneVersionPerDay(t *testing.T) {
	t.Parallel()
	first := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, "2027-01-15",
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	second := sjrVersion(t, contractMia, schoolIspringen, 2, "2027-01-15", januaryLastDay2027,
		booked("tue", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30"))
	for _, rule := range []string{parentbilling.MonthRuleVersionOnFirst, parentbilling.MonthRuleProRata} {
		_, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: rule,
			versions: []parentbilling.CareContractVersion{second, first}, tariffs: sjrTariffs(t)})
		assert.True(t, errors.Is(err, parentbilling.ErrCareVersionsOverlap), "%s: %v", rule, err)
	}
	// Adjacent versions are one version per day.
	second.From = day(t, "2027-01-16")
	_, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleProRata,
		versions: []parentbilling.CareContractVersion{first, second}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
}

// TestCareMonthChangeWithinMonth bills a change on the 16th: the full month
// by the version on the 1st by default, every version for its days pro
// rata.
func TestCareMonthChangeWithinMonth(t *testing.T) {
	t.Parallel()
	first := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, "2027-01-15",
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	second := sjrVersion(t, contractMia, schoolIspringen, 2, "2027-01-16", januaryLastDay2027,
		booked("mon", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30"))
	versions := []parentbilling.CareContractVersion{first, second}

	full, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: versions, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	require.Len(t, full.invoices, 1)
	require.Len(t, full.invoices[0].Positions, 1)
	assert.Equal(t, 1, full.invoices[0].Positions[0].VersionNumber)
	assert.Equal(t, int64(3200), full.invoices[0].TotalCents, "the version on the 1st bills the full month")

	share, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleProRata,
		versions: versions, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	require.Len(t, share.invoices, 1)
	positions := share.invoices[0].Positions
	require.Len(t, positions, 2)
	// 3200 * 15/31 = 1548.39 → 1548; 4550 * 16/31 = 2348.39 → 2348.
	assert.Equal(t, int64(1548), positions[0].AmountCents)
	assert.Equal(t, "2027-01-15", positions[0].ServiceUntil.String())
	assert.Equal(t, int64(2348), positions[1].AmountCents)
	assert.Equal(t, "2027-01-16", positions[1].ServiceFrom.String())
	assert.Equal(t, int64(3896), share.invoices[0].TotalCents)
}

// TestCareMonthEntryWithinMonth bills a contract that starts on the 16th
// by E2 (ADR 0158, Nachtrag Kind-Monat): under the full month only the
// contract in force on the 1st bills, so the entry month bills nothing;
// pro rata it bills its own days.
func TestCareMonthEntryWithinMonth(t *testing.T) {
	t.Parallel()
	version := sjrVersion(t, contractMia, schoolIspringen, 1, "2027-01-16", januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	full, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{version}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Empty(t, full.invoices, "not in force on the 1st: nothing this month")
	share, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleProRata,
		versions: []parentbilling.CareContractVersion{version}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Equal(t, int64(1652), share.invoices[0].TotalCents, "3200 * 16/31 = 1651.61")
}

// TestCareMonthPriceOfTheMonth takes the price that starts on or before
// the month; a later price changes nothing back.
func TestCareMonthPriceOfTheMonth(t *testing.T) {
	t.Parallel()
	version := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	tariffs := append(sjrTariffs(t), price(t, schoolIspringen, moduleUntil1400, "Betreuung bis 14:00 Uhr", "mon", "2027-02-01", 3600))
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{version}, tariffs: tariffs})
	require.NoError(t, err)
	assert.Equal(t, int64(3200), month.invoices[0].TotalCents)
	february := version
	february.From, february.Until = day(t, "2027-02-01"), day(t, "2027-02-28")
	month, err = buildCareMonth(careMonthInput{month: day(t, "2027-02-01"), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{february}, tariffs: tariffs})
	require.NoError(t, err)
	assert.Equal(t, int64(3600), month.invoices[0].TotalCents)
}

// TestCareMonthLeavesOutUnpricedAndBilled names a booked weekday without a
// price and leaves its contract out; a contract an invoice of the month
// already bills is counted, not billed again.
func TestCareMonthLeavesOutUnpricedAndBilled(t *testing.T) {
	t.Parallel()
	unpriced := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"), booked("sat", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	billed := sjrVersion(t, contractNoah, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{unpriced, billed}, tariffs: sjrTariffs(t), invoiced: map[int64]bool{contractNoah: true}})
	require.NoError(t, err)
	assert.Empty(t, month.invoices)
	assert.Equal(t, 1, month.billed)
	require.Len(t, month.unpriced, 1)
	assert.Equal(t, "sat", month.unpriced[0].Weekday)
}

// TestCareMonthModuleEndsWithinMonth stops a module after its last booked
// day pro rata and keeps the full month by default.
func TestCareMonthModuleEndsWithinMonth(t *testing.T) {
	t.Parallel()
	lastDay := day(t, "2027-01-10")
	service := booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00")
	service.LastDay = &lastDay
	version := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027, service)
	share, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleProRata,
		versions: []parentbilling.CareContractVersion{version}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Equal(t, int64(1032), share.invoices[0].TotalCents, "3200 * 10/31 = 1032.26")
	full, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{version}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Equal(t, int64(3200), full.invoices[0].TotalCents)
}

func TestRoundHalfUp(t *testing.T) {
	t.Parallel()
	assert.EqualValues(t, 50, roundHalfUp(100*15, 30))
	assert.EqualValues(t, 2, roundHalfUp(3*15, 30), "1.5 rounds up")
	assert.EqualValues(t, 100, roundHalfUp(100*31, 31))
}

func TestWithValidityEndsBeforeNextPrice(t *testing.T) {
	t.Parallel()
	tariffs := withValidity([]parentbilling.CareTariff{
		price(t, schoolIspringen, moduleUntil1400, "M", "mon", "2027-02-01", 3600),
		price(t, schoolIspringen, moduleUntil1400, "M", "mon", "2026-09-01", 3200),
	})
	require.Len(t, tariffs, 2)
	require.NotNil(t, tariffs[0].ValidUntil)
	assert.Equal(t, "2027-01-31", tariffs[0].ValidUntil.String())
	assert.Nil(t, tariffs[1].ValidUntil)
}

func TestMonthEndCrossesTheYear(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "2026-12-31", monthEnd(day(t, "2026-12-01")).String())
	assert.Equal(t, "2028-02-29", monthEnd(day(t, "2028-02-01")).String())
	assert.Equal(t, time.December, firstOfMonth(day(t, "2026-12-24")).Month())
}

func TestSuggestedMonthSkipsReleasedMonths(t *testing.T) {
	t.Parallel()
	runs := []parentbilling.CareRun{
		{BillingMonth: day(t, "2026-11-01"), Status: parentbilling.RunReleased},
		{BillingMonth: day(t, "2026-12-01"), Status: parentbilling.RunCalculated},
	}
	assert.Equal(t, "2026-12-01", suggestedMonth(day(t, "2026-11-07"), runs).String())
	assert.Equal(t, "2026-11-01", suggestedMonth(day(t, "2026-11-07"), nil).String())
	runs[1].Status = parentbilling.RunReleased
	assert.Equal(t, "2027-01-01", suggestedMonth(day(t, "2026-11-30"), runs).String())
}

// spanContracts records the spans the price list asks the contracts for.
type spanContracts struct {
	spans [][2]string
}

func (s *spanContracts) VersionsInForce(_ context.Context, _ int64, from, to calendar.Date) ([]parentbilling.CareContractVersion, error) {
	s.spans = append(s.spans, [2]string{from.String(), to.String()})
	return nil, nil
}

func (s *spanContracts) Recipients(context.Context, int64, []int64) (map[int64]parentbilling.CareRecipient, error) {
	return nil, nil
}

// TestPriceListReadsTheBookedModulesOfTheYearBefore asks for the modules a
// year back as well, so a module whose contracts ended can still get the
// price of a month that no invoice bills yet.
func TestPriceListReadsTheBookedModulesOfTheYearBefore(t *testing.T) {
	t.Parallel()
	contracts := &spanContracts{}
	billing := &CareBilling{contracts: contracts, today: func() calendar.Date { return day(t, "2026-11-07") }}
	_, err := billing.bookedVersions(context.Background(), 7)
	require.NoError(t, err)
	assert.Equal(t, [][2]string{{"2025-11-01", "2026-10-31"}, {"2026-11-01", "2027-11-01"}}, contracts.spans)
}

// TestBilledPricesNeverChange refuses only a change of a price an invoice
// billed; a missing price may be added in a billed month.
func TestBilledPricesNeverChange(t *testing.T) {
	t.Parallel()
	billed := []parentbilling.CareBilledPrice{{CareOfferingID: moduleUntil1400, Weekday: "mon", Month: day(t, january2027)}}
	before := sjrTariffs(t)
	added := append(sjrTariffs(t), price(t, schoolIspringen, moduleUntil1400, "Betreuung bis 14:00 Uhr", "sat", "2026-09-01", 900))
	assert.False(t, changesBilledPrice(schoolIspringen, day(t, "2026-09-01"), billed, newPriceBook(before), newPriceBook(added)),
		"a Saturday price changes no billed price")
	changed := append(sjrTariffs(t), price(t, schoolIspringen, moduleUntil1400, "Betreuung bis 14:00 Uhr", "mon", "2027-01-01", 9900))
	assert.True(t, changesBilledPrice(schoolIspringen, day(t, january2027), billed, newPriceBook(before), newPriceBook(changed)),
		"a new Monday price from the billed month changes it")
	assert.False(t, changesBilledPrice(schoolIspringen, day(t, "2027-02-01"), billed, newPriceBook(before), newPriceBook(changed)),
		"a change after the billed month keeps it")
	removed := sjrTariffs(t)[1:]
	assert.True(t, changesBilledPrice(schoolIspringen, day(t, "2026-09-01"), billed, newPriceBook(before), newPriceBook(removed)),
		"removing the billed Monday price changes it")
}

// TestCareMonthDuplicateContractsAreHeldForReview bills a weekday of a
// module once per child and month at most (Betragsregel 1): two active
// contracts of one child that both book Monday until 14:00 bill nothing and
// show the child with both contracts. Without the rule the child paid
// 155,00 € instead of 77,50 €.
func TestCareMonthDuplicateContractsAreHeldForReview(t *testing.T) {
	t.Parallel()
	first := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"), booked("tue", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30"))
	second := sjrVersion(t, contractNoah, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"), booked("tue", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30"))
	second.StudentID = first.StudentID
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{first, second}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Empty(t, month.invoices, "neither contract is billed")
	assert.Zero(t, month.total())
	// Every module and weekday both contracts book is named, once.
	require.Len(t, month.duplicates, 2)
	assert.Equal(t, [][2]string{{"Betreuung bis 14:00 Uhr", "mon"}, {"Betreuung bis 16:30 Uhr", "tue"}},
		[][2]string{{month.duplicates[0].ModuleName, month.duplicates[0].Weekday}, {month.duplicates[1].ModuleName, month.duplicates[1].Weekday}})
	for _, duplicate := range month.duplicates {
		assert.Equal(t, "Mia", duplicate.ChildFirstName)
		assert.Equal(t, []int64{contractMia, contractNoah},
			[]int64{duplicate.Contracts[0].ContractID, duplicate.Contracts[1].ContractID})
	}

	// Without a child record the name and the date of birth name the child.
	birth := day(t, "2019-03-04")
	first.StudentID, second.StudentID = nil, nil
	first.ChildBirthDate, second.ChildBirthDate = &birth, &birth
	month, err = buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleProRata,
		versions: []parentbilling.CareContractVersion{first, second}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Empty(t, month.invoices)
	require.Len(t, month.duplicates, 2)

	// Another child of the same name is no duplicate.
	other := day(t, "2020-05-06")
	second.ChildBirthDate = &other
	month, err = buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{first, second}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Len(t, month.invoices, 2)
	assert.Empty(t, month.duplicates)
}

// TestCareMonthChildWithAndWithoutRecordIsOneChild matches a contract
// linked to the child record with one that is not by name and date of
// birth (Betragsregel 1): before, the keys student:… and name:… never met
// and the child paid twice.
func TestCareMonthChildWithAndWithoutRecordIsOneChild(t *testing.T) {
	t.Parallel()
	birth := day(t, "2019-03-04")
	linked := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	unlinked := sjrVersion(t, contractNoah, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	linked.ChildBirthDate, unlinked.ChildBirthDate = &birth, &birth
	unlinked.StudentID = nil
	unlinked.ChildFirstName, unlinked.ChildLastName = " mia", "BEISPIEL "
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{linked, unlinked}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Empty(t, month.invoices)
	require.Len(t, month.duplicates, 1)

	// Two different records stay two children, even with the same name.
	other := *linked.StudentID + 1
	unlinked.StudentID = &other
	month, err = buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{linked, unlinked}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Empty(t, month.duplicates)
	assert.Len(t, month.invoices, 2)
}

// TestCareMonthLinkedContractsWithAnotherNameAreOneChild matches contracts
// by the child record even when a later contract spells the name
// differently (Betragsregel 1): before, the name kept them apart and the
// child paid twice.
func TestCareMonthLinkedContractsWithAnotherNameAreOneChild(t *testing.T) {
	t.Parallel()
	first := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	second := sjrVersion(t, contractNoah, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	second.StudentID = first.StudentID
	second.ChildFirstName, second.ChildLastName = "Maria", "Beispiel-Krause"
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{first, second}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Empty(t, month.invoices, "not 64,00 € for one child")
	require.Len(t, month.duplicates, 1)
}

// TestCareMonthContractsOfOneChildWithoutOverlapAreBilled bills one child's
// contracts that share no module and day: the early care of one contract
// and the afternoon of another, or the modules of two schools.
func TestCareMonthContractsOfOneChildWithoutOverlapAreBilled(t *testing.T) {
	t.Parallel()
	early := sjrVersion(t, contractMia, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleEarly, "Frühbetreuung", "08:00"))
	afternoon := sjrVersion(t, contractNoah, schoolIspringen, 1, january2027, januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	afternoon.StudentID = early.StudentID
	elsewhere := sjrVersion(t, contractNoah+1, schoolBirkenfeld, 1, january2027, januaryLastDay2027,
		booked("mon", 201, "Betreuung bis 14:00 Uhr", "14:00"))
	elsewhere.StudentID = early.StudentID
	tariffs := append(sjrTariffs(t),
		price(t, schoolIspringen, moduleEarly, "Frühbetreuung", "mon", "2026-09-01", 1500),
		price(t, schoolBirkenfeld, 201, "Betreuung bis 14:00 Uhr", "mon", "2026-09-01", 2800))
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{early, afternoon, elsewhere}, tariffs: tariffs})
	require.NoError(t, err)
	assert.Empty(t, month.duplicates)
	assert.Equal(t, int64(1500+3200+2800), month.total())
}

// TestCareMonthUnchangedModuleKeepsOnePosition keeps a module that three
// versions of January book alike as one position at the full monthly price
// (Betragsregel 2); before, it cost 31,99 € in three positions. The module
// that changes splits.
func TestCareMonthUnchangedModuleKeepsOnePosition(t *testing.T) {
	t.Parallel()
	monday := booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00")
	versions := []parentbilling.CareContractVersion{
		sjrVersion(t, contractMia, schoolIspringen, 1, january2027, "2027-01-10", monday),
		sjrVersion(t, contractMia, schoolIspringen, 2, "2027-01-11", "2027-01-21", monday,
			booked("tue", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30")),
		sjrVersion(t, contractMia, schoolIspringen, 3, "2027-01-22", januaryLastDay2027, monday),
	}
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleProRata,
		versions: versions, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	require.Len(t, month.invoices, 1)
	positions := month.invoices[0].Positions
	require.Len(t, positions, 2, "Monday is one position, Tuesday its own")
	assert.Equal(t, "Betreuung bis 14:00 Uhr", positions[0].ModuleName)
	assert.Equal(t, int64(3200), positions[0].AmountCents, "the full monthly price, not 31,99 €")
	assert.Equal(t, january2027, positions[0].ServiceFrom.String())
	assert.Equal(t, januaryLastDay2027, positions[0].ServiceUntil.String())
	assert.Equal(t, 1, positions[0].VersionNumber)
	// Tuesday for 11 of 31 days: 4550 * 11 / 31 = 1614.52.
	assert.Equal(t, int64(1615), positions[1].AmountCents)
}

// TestCareMonthSplitPositionsAddUpToTheCent splits a module whose weekdays
// change and shares its exact amount by largest remainder (Betragsregel 2):
// Monday 30,01 € all month, Tuesday 45,50 € on 11 days cost 4615.97 → 46,16 €;
// rounding each part on its own made 46,15 €.
func TestCareMonthSplitPositionsAddUpToTheCent(t *testing.T) {
	t.Parallel()
	tariffs := []parentbilling.CareTariff{
		price(t, schoolIspringen, moduleUntil1630, "Betreuung bis 16:30 Uhr", "mon", "2026-09-01", 3001),
		price(t, schoolIspringen, moduleUntil1630, "Betreuung bis 16:30 Uhr", "tue", "2026-09-01", 4550),
	}
	monday := booked("mon", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30")
	tuesday := booked("tue", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30")
	versions := []parentbilling.CareContractVersion{
		sjrVersion(t, contractMia, schoolIspringen, 1, january2027, "2027-01-10", monday),
		sjrVersion(t, contractMia, schoolIspringen, 2, "2027-01-11", "2027-01-21", monday, tuesday),
		sjrVersion(t, contractMia, schoolIspringen, 3, "2027-01-22", januaryLastDay2027, monday),
	}
	month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleProRata,
		versions: versions, tariffs: tariffs})
	require.NoError(t, err)
	require.Len(t, month.invoices, 1)
	positions := month.invoices[0].Positions
	require.Len(t, positions, 3)
	var sum int64
	for _, position := range positions {
		var weekdays int64
		for _, weekday := range position.Weekdays {
			weekdays += weekday.AmountCents
		}
		assert.Equal(t, position.AmountCents, weekdays, "the weekdays add up to their position")
		sum += position.AmountCents
	}
	assert.Equal(t, int64(4616), sum, "(3001*10 + 7551*11 + 3001*10) / 31 = 4615.97")
	assert.Equal(t, int64(4616), month.invoices[0].TotalCents)
}

func TestLargestRemainderKeepsTheTotal(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []int64{968, 2680, 968}, largestRemainder([]int64{30010, 83061, 30010}, 31, 4616))
	assert.Equal(t, []int64{3200}, largestRemainder([]int64{3200 * 31}, 31, 3200))
}

// TestDuplicateChildrenCountsChildrenNotEntries counts a child once however
// many entries and contracts its duplicate has: three contracts of Mia that
// meet on Monday and Tuesday are one child, Noah's pair is another.
func TestDuplicateChildrenCountsChildrenNotEntries(t *testing.T) {
	t.Parallel()
	pair := func(a, b int64) parentbilling.CareDuplicate {
		return parentbilling.CareDuplicate{Contracts: []parentbilling.CareDuplicateContract{{ContractID: a}, {ContractID: b}}}
	}
	assert.Zero(t, duplicateChildren(nil))
	assert.Equal(t, 2, duplicateChildren([]parentbilling.CareDuplicate{pair(1, 2), pair(1, 2), pair(2, 3), pair(1, 3), pair(7, 8)}))
}

// The contracts of the child-month table: Mia's contracts A to D share one
// child record, E is another child. January 2027 starts on a Friday; its
// Mondays are the 4th, 11th, 18th and 25th, its Tuesdays the 5th, 12th,
// 19th and 26th. Prices per weekday and month: until 14:00 32,00 €, until
// 16:30 45,50 €, early care 15,00 € (Ispringen), until 14:00 in Birkenfeld
// 28,00 €.
const (
	tableA int64 = 601
	tableB int64 = 602
	tableC int64 = 603
	tableD int64 = 604
	tableE int64 = 605
)

func tableContract(t *testing.T, contractID, schoolID int64, from, until string, services ...parentbilling.CareContractService) parentbilling.CareContractVersion {
	t.Helper()
	version := sjrVersion(t, contractID, schoolID, 1, from, until, services...)
	if contractID != tableE {
		record := tableA + 9000
		version.StudentID = &record
	}
	return version
}

// TestCareMonthChildMonthTable is E6 of ADR 0158, Nachtrag Kind-Monat: every
// sequence of contracts under both rules, against an amount or a hold worked
// out by hand. Rows 15./16. full month, three contracts and the release
// snapshot are the cases of the final review.
func TestCareMonthChildMonthTable(t *testing.T) {
	t.Parallel()
	until1400 := booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00")
	until1630 := booked("tue", moduleUntil1630, "Betreuung bis 16:30 Uhr", "16:30")
	early := booked("mon", moduleEarly, "Frühbetreuung", "08:00")
	birkenfeld := booked("mon", 201, "Betreuung bis 14:00 Uhr", "14:00")
	tariffs := append(sjrTariffs(t),
		price(t, schoolIspringen, moduleEarly, "Frühbetreuung", "mon", "2026-09-01", 1500),
		price(t, schoolBirkenfeld, 201, "Betreuung bis 14:00 Uhr", "mon", "2026-09-01", 2800))
	full, share := parentbilling.MonthRuleVersionOnFirst, parentbilling.MonthRuleProRata
	followEachOther := func() []parentbilling.CareContractVersion {
		return []parentbilling.CareContractVersion{
			tableContract(t, tableA, schoolIspringen, january2027, "2027-01-15", until1400),
			tableContract(t, tableB, schoolIspringen, "2027-01-16", januaryLastDay2027, until1400),
		}
	}
	sameMonth := func() []parentbilling.CareContractVersion {
		return []parentbilling.CareContractVersion{
			tableContract(t, tableA, schoolIspringen, january2027, januaryLastDay2027, until1400),
			tableContract(t, tableB, schoolIspringen, january2027, januaryLastDay2027, until1400),
		}
	}
	meetOnAMonday := func() []parentbilling.CareContractVersion {
		return []parentbilling.CareContractVersion{
			tableContract(t, tableA, schoolIspringen, january2027, "2027-01-18", until1400),
			tableContract(t, tableB, schoolIspringen, "2027-01-18", januaryLastDay2027, until1400),
		}
	}
	otherModule := func() []parentbilling.CareContractVersion {
		return []parentbilling.CareContractVersion{
			tableContract(t, tableA, schoolIspringen, january2027, januaryLastDay2027, until1400),
			tableContract(t, tableB, schoolIspringen, january2027, januaryLastDay2027, early),
		}
	}
	threeContracts := func() []parentbilling.CareContractVersion {
		return []parentbilling.CareContractVersion{
			tableContract(t, tableA, schoolIspringen, january2027, januaryLastDay2027, until1400),
			tableContract(t, tableB, schoolIspringen, january2027, januaryLastDay2027, until1400),
			tableContract(t, tableC, schoolIspringen, january2027, januaryLastDay2027, until1630),
		}
	}
	twoSchools := func() []parentbilling.CareContractVersion {
		return []parentbilling.CareContractVersion{
			tableContract(t, tableA, schoolIspringen, january2027, januaryLastDay2027, until1400),
			tableContract(t, tableB, schoolBirkenfeld, january2027, januaryLastDay2027, birkenfeld),
		}
	}
	rows := []struct {
		name     string
		rule     string
		versions []parentbilling.CareContractVersion
		billed   map[int64]int64 // contract → cents
		held     []int64
	}{
		// A holds on the 1st and bills the whole month: 4 Mondays, 32,00 €.
		// B starts on the 16th and bills nothing (E2); no day is billed twice.
		{"15./16. full month", full, followEachOther(), map[int64]int64{tableA: 3200}, []int64{}},
		// A bills the 1st to 15th: 3200 * 15/31 = 1548.39 → 15,48 €; B the
		// 16th to 31st: 3200 * 16/31 = 1651.61 → 16,52 €. A's Mondays are the
		// 4th and 11th, B's the 18th and 25th: no common day (E3).
		{"15./16. pro rata", share, followEachOther(), map[int64]int64{tableA: 1548, tableB: 1652}, []int64{}},
		// Both hold on the 1st and bill every Monday until 14:00: the 4th is
		// billed twice, so Mia is held with both contracts (E3, E4).
		{"1./1. full month", full, sameMonth(), map[int64]int64{}, []int64{tableA, tableB}},
		// Pro rata both bill the 1st to 31st: the same Mondays twice.
		{"1./1. pro rata", share, sameMonth(), map[int64]int64{}, []int64{tableA, tableB}},
		// A holds on the 1st and bills the whole month, 32,00 €; B starts on
		// the 18th and bills nothing. The common 18th is no billed day of B.
		{"meet on a Monday full month", full, meetOnAMonday(), map[int64]int64{tableA: 3200}, []int64{}},
		// Pro rata A bills the 1st to 18th and B the 18th to 31st: Monday the
		// 18th until 14:00 is billed by both, so Mia is held.
		{"meet on a Monday pro rata", share, meetOnAMonday(), map[int64]int64{}, []int64{tableA, tableB}},
		// Monday until 14:00 and Monday early care are two modules: 32,00 €
		// and 15,00 €, no module twice on a day.
		{"other module full month", full, otherModule(), map[int64]int64{tableA: 3200, tableB: 1500}, []int64{}},
		// Pro rata both bill their whole month: 31/31 of each price.
		{"other module pro rata", share, otherModule(), map[int64]int64{tableA: 3200, tableB: 1500}, []int64{}},
		// A and B bill Monday until 14:00 twice; C (Tuesday until 16:30,
		// 45,50 €) doubles nothing but is Mia's too: all three are held (E4).
		{"three contracts full month", full, threeContracts(), map[int64]int64{}, []int64{tableA, tableB, tableC}},
		{"three contracts pro rata", share, threeContracts(), map[int64]int64{}, []int64{tableA, tableB, tableC}},
		// Two schools have their own modules (101 and 201): 32,00 € in
		// Ispringen and 28,00 € in Birkenfeld, nothing twice.
		{"two schools full month", full, twoSchools(), map[int64]int64{tableA: 3200, tableB: 2800}, []int64{}},
		{"two schools pro rata", share, twoSchools(), map[int64]int64{tableA: 3200, tableB: 2800}, []int64{}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			month, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: row.rule, versions: row.versions, tariffs: tariffs})
			require.NoError(t, err)
			billed := map[int64]int64{}
			for _, invoice := range month.invoices {
				billed[invoice.ContractID] = invoice.TotalCents
			}
			assert.Equal(t, row.billed, billed)
			assert.Equal(t, row.held, heldContracts(month.duplicates), "held contracts")
		})
	}

	// E5: the release of "three contracts" stored A, B and C. After it, Mia
	// gets a fourth contract D and another child E one of its own; the
	// released run still shows Mia with A, B and C only, and not E.
	t.Run("release snapshot", func(t *testing.T) {
		later := append(threeContracts(),
			tableContract(t, tableD, schoolIspringen, january2027, januaryLastDay2027, early),
			tableContract(t, tableE, schoolIspringen, january2027, januaryLastDay2027, until1400))
		shown := heldView([]int64{tableA, tableB, tableC}, later)
		require.Len(t, shown, 1, "one child, Mia")
		var contracts []int64
		for _, contract := range shown[0].Contracts {
			contracts = append(contracts, contract.ContractID)
		}
		assert.Equal(t, []int64{tableA, tableB, tableC}, contracts)
	})
}
