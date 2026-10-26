package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The number patterns and the preview are pure; the run as a whole, its
// numbers under parallel releases and its invoices are tested through the
// routes in api/parent_billing_runs_api_test.go.

func TestNumberPatternNamesOneRunningNumberAndAtMostOneYear(t *testing.T) {
	t.Parallel()
	for pattern, valid := range map[string]bool{
		"FE-{JJJJ}-{NNNN}":  true,
		" R/{JJ}/{N} ":      true,
		"{NNNNNNNN}":        true,
		"FE-{JJJJ}":         false,
		"{N}-{NN}":          false,
		"{JJJJ}-{JJ}-{NNN}": false,
		"FE {NNNN}":         false,
		"FE-{NNNNNNNNN}":    false,
		"FE-{X}-{NNNN}":     false,
		"":                  false,
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ-0123456789-{NNNN}": false,
	} {
		_, err := normalizeNumberPattern(pattern)
		assert.Equal(t, valid, err == nil, "pattern %q: %v", pattern, err)
	}
	normalized, err := normalizeNumberPattern(" R/{JJ}/{N} ")
	require.NoError(t, err)
	assert.Equal(t, "R/{JJ}/{N}", normalized)
}

func TestInvoiceNumbersCountPerYearOnlyWhenThePatternNamesIt(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "FE-2027-0007", formatInvoiceNumber("FE-{JJJJ}-{NNNN}", 2027, 7))
	assert.Equal(t, "R/27/12345", formatInvoiceNumber("R/{JJ}/{NNN}", 2027, 12345), "a number longer than its padding keeps every digit")
	assert.Equal(t, "FE-3", formatInvoiceNumber("FE-{N}", 2027, 3))
	assert.Equal(t, 2027, numberYear("FE-{JJJJ}-{NNNN}", 2027))
	assert.Equal(t, 2027, numberYear("{JJ}{NNNN}", 2027))
	assert.Equal(t, 0, numberYear("FE-{NNNN}", 2027), "without a year the numbers count on across the years")
}

func TestPreviewBillsEveryConfirmedFamilyOnceWithItsCorrections(t *testing.T) {
	t.Parallel()
	week := calendar.NewDate(2027, 1, 4)
	weekEnd := week.AddDays(4)
	second, secondEnd := week.AddDays(7), week.AddDays(11)
	line := func(starts, ends calendar.Date, cents int64) parentbilling.HolidayContributionLine {
		return parentbilling.HolidayContributionLine{Kind: parentbilling.LineBase, AmountCents: cents, WeekStartsOn: &starts, WeekEndsOn: &ends, TimeModelName: "bis 13 Uhr"}
	}
	run := parentbilling.BillingRun{ID: 30, ServiceFrom: week, ServiceUntil: secondEnd}
	registrations := []parentbilling.HolidayBillingRegistration{
		{ID: 12, GuardianFirstName: "Jan", Children: []parentbilling.HolidayBillingChild{{ID: 121, FirstName: "Mia"}}},
		{ID: 11, GuardianFirstName: "Eva", Children: []parentbilling.HolidayBillingChild{
			{ID: 111, FirstName: "Lia"}, {ID: 112, FirstName: "Ben"}, {ID: 113, FirstName: "Tom"},
		}},
		{ID: 13, GuardianFirstName: "Ole", Children: []parentbilling.HolidayBillingChild{{ID: 131, FirstName: "Ida"}}},
		{ID: 14, GuardianFirstName: "Ute", Children: []parentbilling.HolidayBillingChild{{ID: 141, FirstName: "Kai"}}},
	}
	contributions := map[int64]parentbilling.HolidayContribution{
		11: {Children: []parentbilling.HolidayChildContribution{
			{ChildID: 111, Lines: []parentbilling.HolidayContributionLine{line(second, secondEnd, 6750)}, TotalCents: 6750},
			{ChildID: 112, Lines: []parentbilling.HolidayContributionLine{line(week, weekEnd, 4500)}, TotalCents: 4500},
			{ChildID: 113, Lines: []parentbilling.HolidayContributionLine{}, TotalCents: 0},
		}},
		12: {Children: []parentbilling.HolidayChildContribution{
			{ChildID: 121, Lines: []parentbilling.HolidayContributionLine{line(week, weekEnd, 4500)}, TotalCents: 4500},
		}},
		14: {Children: []parentbilling.HolidayChildContribution{
			{ChildID: 141, Lines: []parentbilling.HolidayContributionLine{line(week, weekEnd, 4500)}, TotalCents: 4500},
		}},
	}
	corrections := []parentbilling.BillingCorrection{
		{ChildID: 112, AmountCents: -500, TotalBeforeCents: 4500, TotalAfterCents: 4000, Reason: "Woche krank"},
		{ChildID: 112, AmountCents: -4000, TotalBeforeCents: 4000, TotalAfterCents: 0, Reason: "Härtefall"},
	}

	result := buildPreview(previewInput{
		run: run, registrations: registrations, contributions: contributions,
		issued: map[int64]bool{14: true}, corrections: corrections,
	})

	require.Len(t, result.documents, 2, "a billed family and a family without a frozen contribution get no draft")
	assert.Equal(t, 1, result.unbilled, "the family without a frozen contribution is counted")
	eva, jan := result.documents[0], result.documents[1]
	assert.Equal(t, int64(11), eva.RegistrationID, "families in the order of their registration")
	require.Len(t, eva.Children, 2, "a child without a booking is not billed")
	ben := eva.Children[1]
	assert.Equal(t, "Ben", ben.FirstName)
	require.Len(t, ben.Positions, 3, "the frozen line, then every correction as a position")
	assert.Equal(t, parentbilling.LineCorrection, ben.Positions[2].Kind)
	assert.Equal(t, "Härtefall", ben.Positions[2].Note)
	assert.Zero(t, ben.TotalCents)
	assert.Len(t, ben.Corrections, 2, "the history keeps every correction")
	assert.Equal(t, int64(6750), eva.TotalCents)
	assert.Equal(t, week, eva.ServiceFrom)
	assert.Equal(t, secondEnd, eva.ServiceUntil)
	assert.Equal(t, weekEnd, jan.ServiceUntil, "the service days are the booked weeks")
	assert.Equal(t, int64(6750+4500), result.total())

	child, ok := result.child(11, 112)
	require.True(t, ok)
	assert.Equal(t, "Ben", child.FirstName)
	_, ok = result.child(12, 112)
	assert.False(t, ok, "a child is found only in its own family")
}

func TestCorrectionNeedsAChildAnAmountAndAReason(t *testing.T) {
	t.Parallel()
	_, err := normalizeCorrection(parentbilling.BillingCorrectionInput{RegistrationID: 3, ChildID: 4, TotalCents: -1, Reason: "  "})
	require.Error(t, err)
	assert.ErrorContains(t, err, "reason")
	assert.ErrorContains(t, err, "total_cents")
	input, err := normalizeCorrection(parentbilling.BillingCorrectionInput{RegistrationID: 3, ChildID: 4, TotalCents: 0, Reason: " Härtefall "})
	require.NoError(t, err)
	assert.Equal(t, "Härtefall", input.Reason)
}
