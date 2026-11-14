package application

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

func TestPayerNumbersStayAmbiguousWithoutEligibleContracts(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"family", "child"} {
		for eligible := 0; eligible <= 2; eligible++ {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%d eligible/reverse %t", scope, eligible, reverse), func(t *testing.T) {
					payers := importPayers{byFamily: map[string]importPayer{}, byChild: map[string]importPayer{},
						byNumber: map[string]numberPayer{}, ambiguous: map[string]bool{}}
					families := []parentbilling.FamilyBilling{
						{FamilyEmail: "a@example.org", DebtorNumber: "10001"},
						{FamilyEmail: "b@example.org", DebtorNumber: "10001"},
					}
					children := []parentbilling.ChildDebtorNumber{
						{FamilyEmail: "a@example.org", ChildFirstName: "Mia", ChildLastName: "Beispiel", DebtorNumber: "10001"},
						{FamilyEmail: "a@example.org", ChildFirstName: "Ben", ChildLastName: "Beispiel", DebtorNumber: "10001"},
					}
					for i := 0; i < eligible; i++ {
						payers.byFamily[families[i].FamilyEmail] = importPayer{contractID: int64(10 + i), family: families[i].FamilyEmail}
						payers.byChild[children[i].Key()] = importPayer{contractID: int64(10 + i), family: children[i].FamilyEmail}
					}
					if reverse {
						families[0], families[1] = families[1], families[0]
						children[0], children[1] = children[1], children[0]
					}
					if scope == "family" {
						payers.indexFamilyNumbers(families)
					} else {
						payers.indexChildNumbers(children)
					}
					candidate := importCandidate{}
					payer, problem := payers.payerOfNumber(&candidate, "10001")
					assert.Equal(t, parentbilling.ImportProblemPayerAmbiguous, problem)
					assert.Zero(t, payer.contractID)
					assert.Empty(t, candidate.payerNumber)
				})
			}
		}
	}
}

func TestImportCheckDoesNotDeduplicateDifferentCollectionHistories(t *testing.T) {
	t.Parallel()
	for name, histories := range map[string][2][2]string{
		"first versus recurring without date": {{"FRST", ""}, {"RCUR", ""}},
		"first versus recurring":              {{"FRST", ""}, {"RCUR", "2026-10-01"}},
		"different collection dates":          {{"RCUR", "2026-09-01"}, {"RCUR", "2026-10-01"}},
		"known versus unknown collection":     {{"RCUR", ""}, {"RCUR", "2026-10-01"}},
		"unclear versus recurring":            {{"", ""}, {"RCUR", ""}},
	} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse %t", name, reverse), func(t *testing.T) {
				if reverse {
					histories[0], histories[1] = histories[1], histories[0]
				}
				check := newTestImportCheck(nil, nil)
				first := check.row(importRow("SJR-1", "2026-02-01", importIBAN, "a@example.org", importCreditor,
					histories[0][0], histories[0][1]), 2)
				require.Contains(t, []string{parentbilling.ImportRowReady, parentbilling.ImportRowClarify}, first.row.Status)
				second := check.row(importRow("SJR-1", "2026-02-01", importIBAN, "a@example.org", importCreditor,
					histories[1][0], histories[1][1]), 3)
				assert.Equal(t, parentbilling.ImportRowConflict, second.row.Status)
				assert.Contains(t, problemCodes(second.row), parentbilling.ImportFieldReference+":"+parentbilling.ImportProblemReferenceRepeated)
				assert.Zero(t, second.row.SameMandateLine)
			})
		}
	}
}

func TestImportCheckDeduplicatesMatchingCollectionDates(t *testing.T) {
	t.Parallel()
	check := newTestImportCheck(nil, nil)
	cells := importRow("SJR-1", "2026-02-01", importIBAN, "a@example.org", importCreditor, "RCUR", "2026-10-01")
	first := check.row(cells, 2)
	require.Equal(t, parentbilling.ImportRowReady, first.row.Status)
	second := check.row(cells, 3)
	assert.Equal(t, parentbilling.ImportRowSameMandate, second.row.Status)
	assert.Equal(t, 2, second.row.SameMandateLine)
}
