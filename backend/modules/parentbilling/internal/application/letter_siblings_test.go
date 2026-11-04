package application

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// TestSiblingLetterNamesOnlyItsOwnRegistration pins #5628: two
// registrations of a sibling group, possibly from parents who live apart,
// get one invoice and one letter each. The discount stands on the letter of
// the child that gets it; no letter names a child or a parent of the other
// registration.
func TestSiblingLetterNamesOnlyItsOwnRegistration(t *testing.T) {
	t.Parallel()
	half := &parentbilling.ContributionRule{Kind: parentbilling.RuleSibling, SiblingFromChild: new(2), DiscountBasisPoints: new(5000)}
	first := parentbilling.HolidayBillingRegistration{
		ID: 1, GuardianFirstName: "Erika", GuardianLastName: "Kowalski", GuardianEmail: "erika@example.org",
		Children: []parentbilling.HolidayBillingChild{{ID: 11, FirstName: "Lina", LastName: "Kowalski"}},
	}
	second := parentbilling.HolidayBillingRegistration{
		ID: 2, GuardianFirstName: "Jonas", GuardianLastName: "Anders", GuardianEmail: "jonas@example.org",
		Children: []parentbilling.HolidayBillingChild{{ID: 21, FirstName: "Ben", LastName: "Anders"}},
	}
	contributions := computeHolidaySiblings([]holidayMember{
		{children: []parentbilling.HolidayChild{chainChild(11, 1, 10000)}},
		{children: []parentbilling.HolidayChild{chainChild(21, 1, 10000)}},
	}, holidayRules{sibling: half})
	run := goldenLetterContent(t).run
	previewed := buildPreview(previewInput{
		run:           run,
		registrations: []parentbilling.HolidayBillingRegistration{first, second},
		contributions: map[int64]parentbilling.HolidayContribution{1: contributions[0], 2: contributions[1]},
	})
	require.Len(t, previewed.documents, 2, "one invoice per registration")

	letters := map[int64]string{}
	for _, document := range previewed.documents {
		content := goldenLetterContent(t)
		content.document = document
		letters[document.RegistrationID] = fmt.Sprintf("%+v", confirmationLetter(content))
	}
	assert.Contains(t, letters[1], "Lina")
	assert.NotContains(t, letters[1], "Ermäßigung für Geschwister", "the first child pays in full")
	assert.Contains(t, letters[2], "Ben: Ermäßigung für Geschwister")
	assert.Contains(t, letters[2], "50,00 €")
	for _, name := range []string{"Lina", "Erika", "Kowalski", "erika@example.org"} {
		assert.NotContains(t, letters[2], name, "the letter of the second registration never names the first one")
	}
	for _, name := range []string{"Ben", "Jonas", "Anders", "jonas@example.org"} {
		assert.NotContains(t, letters[1], name, "the letter of the first registration never names the second one")
	}
}
