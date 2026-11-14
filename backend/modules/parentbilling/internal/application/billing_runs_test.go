package application

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
		issued: map[parentbilling.HolidayInvoiceKey]bool{{RegistrationID: 14}: true}, corrections: corrections,
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

// With a number per child each child gets an invoice of its own (#7007).
// A registration keeps the shape of its first invoice: billed as a whole
// before the switch, it stays one invoice; billed per child, a cancelled
// child's invoice is issued again for that child alone.
func TestPreviewBillsEachChildOnAnInvoiceOfItsOwn(t *testing.T) {
	t.Parallel()
	week := calendar.NewDate(2027, 1, 4)
	weekEnd := week.AddDays(4)
	booked := func(ids ...int64) parentbilling.HolidayContribution {
		contribution := parentbilling.HolidayContribution{}
		for _, id := range ids {
			contribution.Children = append(contribution.Children, parentbilling.HolidayChildContribution{ChildID: id, TotalCents: 4500,
				Lines: []parentbilling.HolidayContributionLine{{Kind: parentbilling.LineBase, AmountCents: 4500, WeekStartsOn: &week, WeekEndsOn: &weekEnd}}})
		}
		return contribution
	}
	children := func(ids ...int64) []parentbilling.HolidayBillingChild {
		list := make([]parentbilling.HolidayBillingChild, len(ids))
		for i, id := range ids {
			list[i] = parentbilling.HolidayBillingChild{ID: id, FirstName: fmt.Sprintf("Kind %d", id)}
		}
		return list
	}
	registrations := []parentbilling.HolidayBillingRegistration{
		{ID: 11, DebtorNumber: "10 0001", Children: children(111, 112)},
		{ID: 12, DebtorNumber: "20 0001", Children: children(121)},
		{ID: 13, Children: children(131, 132)},
		{ID: 14, Children: children(141, 142)},
		{ID: 15, Children: children(151, 152)},
	}
	contributions := map[int64]parentbilling.HolidayContribution{
		11: booked(111, 112), 12: booked(121), 13: booked(131, 132), 14: booked(141, 142), 15: booked(151, 152),
	}

	result := buildPreview(previewInput{
		run: parentbilling.BillingRun{ID: 30}, registrations: registrations, contributions: contributions, perChild: true,
		issued: map[parentbilling.HolidayInvoiceKey]bool{{RegistrationID: 13}: true, {RegistrationID: 14, ChildID: 141}: true},
		replaces: map[parentbilling.HolidayInvoiceKey]parentbilling.BillingDocumentLink{
			{RegistrationID: 14, ChildID: 142}: {ID: 900, Number: "FE-9"}, {RegistrationID: 15}: {ID: 901, Number: "FE-10"},
		},
	})

	type drafted struct {
		registration, child int64
		children            int
		debtor, replaces    string
	}
	var got []drafted
	for _, document := range result.documents {
		got = append(got, drafted{document.RegistrationID, document.RegistrationChildID, len(document.Children), document.DebtorNumber,
			document.ReferenceNumber})
	}
	assert.Equal(t, []drafted{
		{11, 111, 1, "", ""}, {11, 112, 1, "", ""},
		{12, 121, 1, "20 0001", ""},
		{14, 142, 1, "", "FE-9"},
		{15, 0, 2, "", "FE-10"},
	}, got, "siblings split, one child keeps the registration's number, an invoice of the whole stays whole")
	assert.Zero(t, result.unbilled)

	whole := buildPreview(previewInput{run: parentbilling.BillingRun{ID: 30}, registrations: registrations[:1], contributions: contributions})
	require.Len(t, whole.documents, 1, "with a number per payer the registration has one invoice")
	assert.Zero(t, whole.documents[0].RegistrationChildID)
	assert.Equal(t, "10 0001", whole.documents[0].DebtorNumber)
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
