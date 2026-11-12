package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

func specialTerms() carecontract.TemplateInput {
	return carecontract.TemplateInput{TermKind: carecontract.TermSchoolYear, NoticeMonths: 2,
		NoticeTo: carecontract.NoticeMonthEnd, ContractPartner: "Träger",
		SpecialTermination: true, SpecialTerminationNote: "Bei Umzug"}
}

// The parents read the date of a special termination (#6349): moto offers
// it only to the half year with the notice months of the terms, so the PDF
// and the consent text name that rule before the cases of the terms.
func TestSpecialTerminationNamesTheHalfYearInLetterAndConsent(t *testing.T) {
	t.Parallel()
	terms := specialTerms()
	letter := contractLetter(ContractRecord{Version: carecontract.Version{Terms: terms}}, carecontract.Letterhead{})
	var special *LetterBlock
	for i := range letter.Blocks {
		if letter.Blocks[i].Heading == "Sonderkündigung" {
			special = &letter.Blocks[i]
		}
	}
	require.NotNil(t, special, "the PDF prints the special termination")
	assert.Equal(t, "Möglich mit 2 Monaten Frist zum Halbjahr (31. Januar oder 31. Juli).\n\nBei Umzug", special.Text)
	assert.Contains(t, acceptanceText(terms),
		"Sonderkündigung:\n  Ja, mit 2 Monaten Frist zum Halbjahr (31. Januar oder 31. Juli)\n  Bei Umzug")

	terms.SpecialTermination, terms.SpecialTerminationNote = false, ""
	letter = contractLetter(ContractRecord{Version: carecontract.Version{Terms: terms}}, carecontract.Letterhead{})
	for _, block := range letter.Blocks {
		assert.NotEqual(t, "Sonderkündigung", block.Heading, "no rule without a special termination")
	}
	assert.Equal(t, []string{acceptanceText(terms)}, acceptanceTexts(terms), "nothing changed, no second wording")
}

// A consent ticked before #6349 named the special termination without its
// date. It covers the same conditions, so it still concludes the contract;
// it binds nothing else: changed conditions are no proof.
func TestConsentToTheEarlierSpecialTerminationWordingStaysValid(t *testing.T) {
	t.Parallel()
	submitted := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	accepted := submitted.Add(24 * time.Hour)
	// The text schools copied into their forms before #6349, verbatim.
	earlier := "Vertragsbedingungen für die Betreuung\n\nLaufzeit:\n  bis zum Ende des Schuljahres\n\nKündigung:\n  mit 2 Monaten Frist zum Monatsende\n\nVertragspartner:\n  Träger\n\nSonderkündigung:\n  Ja\n  Bei Umzug\n\nSatzung:\n  Nein"
	proof := carecontract.AcceptedText{Key: "terms", Text: earlier, SHA256: "abc", SubmittedAt: submitted, Required: true}
	draft := func(terms carecontract.TemplateInput) NewContract {
		return NewContract{AcceptedAt: &accepted, Acceptances: []carecontract.AcceptedText{proof},
			Version: carecontract.Version{Terms: terms}}
	}

	terms := specialTerms()
	require.NotEqual(t, earlier, acceptanceText(terms), "the current wording names the date")
	matched, ok := acceptanceProof(draft(terms))
	require.True(t, ok, "the earlier wording still proves consent")
	assert.Equal(t, proof, matched)
	assert.Empty(t, acceptanceIssue(draft(terms), submitted.Add(-time.Hour)))

	changed := terms
	changed.SpecialTerminationNote = "Bei Umzug oder Schulwechsel"
	assert.Equal(t, carecontract.SkipAcceptanceMissing, acceptanceIssue(draft(changed), submitted.Add(-time.Hour)),
		"the earlier wording binds only its own conditions")
	changed = terms
	changed.NoticeMonths = 1
	assert.Equal(t, carecontract.SkipAcceptanceChanged, acceptanceIssue(draft(changed), submitted.Add(time.Minute)),
		"conditions changed after the submission")
}
