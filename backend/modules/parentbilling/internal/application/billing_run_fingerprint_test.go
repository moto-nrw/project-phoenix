package application

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The fingerprint of a holiday preview (#6506) is pure; the release that
// compares it runs through the routes in
// api/parent_billing_run_fingerprint_api_test.go.

func fingerprintPreviewInput() previewInput {
	starts, ends := calendar.NewDate(2027, 1, 4), calendar.NewDate(2027, 1, 8)
	line := func(cents int64) parentbilling.HolidayContributionLine {
		return parentbilling.HolidayContributionLine{Kind: parentbilling.LineBase, AmountCents: cents, WeekStartsOn: &starts, WeekEndsOn: &ends}
	}
	return previewInput{
		run: parentbilling.BillingRun{ID: 40, ServiceFrom: starts, ServiceUntil: ends},
		registrations: []parentbilling.HolidayBillingRegistration{
			{ID: 21, GuardianFirstName: "Eva", GuardianEmail: "eva@example.org", Children: []parentbilling.HolidayBillingChild{{ID: 211, FirstName: "Mia"}}},
			{ID: 22, GuardianFirstName: "Jan", GuardianEmail: "jan@example.org", Children: []parentbilling.HolidayBillingChild{{ID: 221, FirstName: "Ole"}}},
		},
		contributions: map[int64]parentbilling.HolidayContribution{
			21: {Children: []parentbilling.HolidayChildContribution{{ChildID: 211, TotalCents: 1000, Lines: []parentbilling.HolidayContributionLine{line(1000)}}}},
			22: {Children: []parentbilling.HolidayChildContribution{{ChildID: 221, TotalCents: 2000, Lines: []parentbilling.HolidayContributionLine{line(2000)}}}},
		},
	}
}

func fingerprintOf(in previewInput) string {
	return holidayPreviewFingerprint(in.run, buildPreview(in), parentbilling.DocumentKindInvoice, "")
}

func TestHolidayFingerprintIgnoresTheOrderTheRegistrationsAreRead(t *testing.T) {
	t.Parallel()
	in := fingerprintPreviewInput()
	reversed := in
	reversed.registrations = slices.Clone(in.registrations)
	slices.Reverse(reversed.registrations)

	assert.NotEmpty(t, fingerprintOf(in))
	assert.Equal(t, fingerprintOf(in), fingerprintOf(reversed))
}

func TestHolidayFingerprintNamesEveryCheckedPartOfTheContent(t *testing.T) {
	t.Parallel()
	in := fingerprintPreviewInput()
	checked := fingerprintOf(in)
	changed := map[string]func(*previewInput){
		"amounts moved between families": func(p *previewInput) {
			p.contributions = map[int64]parentbilling.HolidayContribution{21: p.contributions[22], 22: p.contributions[21]}
			p.contributions[21].Children[0].ChildID, p.contributions[22].Children[0].ChildID = 211, 221
		},
		"recipient": func(p *previewInput) { p.registrations[0].GuardianEmail = "oma@example.org" },
		"debtor number": func(p *previewInput) {
			p.registrations[1].DebtorNumber = "10 4711"
		},
		"run": func(p *previewInput) { p.run.ID = 41 },
	}
	for name, change := range changed {
		next := fingerprintPreviewInput()
		change(&next)
		assert.NotEqual(t, checked, fingerprintOf(next), name)
	}
	current := buildPreview(in)
	assert.NotEqual(t, checked, holidayPreviewFingerprint(in.run, current, parentbilling.DocumentKindFeeNotice, ""), "kind of document")
	assert.NotEqual(t, checked, holidayPreviewFingerprint(in.run, current, parentbilling.DocumentKindInvoice, "1:Widerspruch"), "legal remedy")
	current.dueDates = parentbilling.RunDueDates{parentbilling.PaymentMethods[0]: calendar.NewDate(2027, 2, 1)}
	assert.NotEqual(t, checked, holidayPreviewFingerprint(in.run, current, parentbilling.DocumentKindInvoice, ""), "due dates")
	marked := buildPreview(in)
	marked.documents[0].DocumentKind = parentbilling.DocumentKindFeeNotice
	assert.Equal(t, checked, holidayPreviewFingerprint(in.run, marked, parentbilling.DocumentKindInvoice, ""),
		"the kind shown on the drafts counts once, beside them")
}
