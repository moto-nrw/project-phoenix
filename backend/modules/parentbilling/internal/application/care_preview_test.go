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

func sjrVersion(t *testing.T, contractID, schoolID int64, number int, from, until string, services ...parentbilling.CareContractService) parentbilling.CareContractVersion {
	t.Helper()
	return parentbilling.CareContractVersion{
		ContractID: contractID, SchoolID: schoolID, SchoolName: "Grundschule Ispringen", Number: number,
		ChildFirstName: "Mia", ChildLastName: "Beispiel", From: day(t, from), Until: day(t, until), Services: services,
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

// TestCareMonthEntryWithinMonth bills a contract that starts on the 16th: a
// full month by its first version by default, its days pro rata.
func TestCareMonthEntryWithinMonth(t *testing.T) {
	t.Parallel()
	version := sjrVersion(t, contractMia, schoolIspringen, 1, "2027-01-16", januaryLastDay2027,
		booked("mon", moduleUntil1400, "Betreuung bis 14:00 Uhr", "14:00"))
	full, err := buildCareMonth(careMonthInput{month: day(t, january2027), rule: parentbilling.MonthRuleVersionOnFirst,
		versions: []parentbilling.CareContractVersion{version}, tariffs: sjrTariffs(t)})
	require.NoError(t, err)
	assert.Equal(t, int64(3200), full.invoices[0].TotalCents)
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

func TestShareOfMonthRoundsHalfUp(t *testing.T) {
	t.Parallel()
	assert.Equal(t, int64(50), shareOfMonth(100, 15, 30))
	assert.EqualValues(t, 2, shareOfMonth(3, 15, 30), "1.5 rounds up")
	assert.Equal(t, int64(100), shareOfMonth(100, 31, 31))
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
