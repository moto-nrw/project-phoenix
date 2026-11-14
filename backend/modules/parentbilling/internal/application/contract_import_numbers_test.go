package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

func importedStatuses(rows []importedNumber) []string {
	statuses := make([]string, len(rows))
	for i, row := range rows {
		statuses[i] = row.status
	}
	return statuses
}

// Per payer, siblings share a number: equal numbers are taken over once,
// different ones not at all; a stored number is never overwritten, and a
// row without an address names no family (#4723, #4693).
func TestContractImportNumbersPerPayer(t *testing.T) {
	t.Parallel()
	birth := calendar.NewDate(2019, 5, 3)
	rows := readImportedNumbers([]parentbilling.ContractImportNumber{
		{Line: 2, FamilyEmail: " Eva@Example.org", ChildFirstName: "Mia", ChildLastName: "Beispiel", ChildBirthDate: &birth, DebtorNumber: " 70001 "},
		{Line: 3, FamilyEmail: "eva@example.org", ChildFirstName: "Ben", ChildLastName: "Beispiel", DebtorNumber: "70001"},
		{Line: 4, FamilyEmail: "jan@example.org", ChildFirstName: "Ole", ChildLastName: "Anders", DebtorNumber: "70002"},
		{Line: 5, FamilyEmail: "jan@example.org", ChildFirstName: "Pia", ChildLastName: "Anders", DebtorNumber: "70003"},
		{Line: 6, FamilyEmail: "", ChildFirstName: "Tom", ChildLastName: "Ohne", DebtorNumber: "70004"},
		{Line: 7, FamilyEmail: "kim@example.org", ChildFirstName: "Kim", ChildLastName: "Alt", DebtorNumber: "70005"},
		{Line: 8, FamilyEmail: "lea@example.org", ChildFirstName: "Lea", ChildLastName: "Gleich", DebtorNumber: "70006"},
		{Line: 9, FamilyEmail: "max@example.org", ChildFirstName: "Max", ChildLastName: "Konto", DebtorNumber: "DE89 3704 0044 0532 0130 00"},
	}, parentbilling.DebtorNumberFormat{}, false)
	decideImportedNumbers(rows, map[string]string{"kim@example.org": "60000", "lea@example.org": "70006"}, false)
	assert.Equal(t, []string{parentbilling.DebtorNumberNew, parentbilling.DebtorNumberNew, parentbilling.DebtorNumberSiblingsDiffer,
		parentbilling.DebtorNumberSiblingsDiffer, parentbilling.DebtorNumberNoFamily, parentbilling.DebtorNumberDiffers,
		parentbilling.DebtorNumberSame, parentbilling.DebtorNumberInvalid}, importedStatuses(rows))
	assert.Equal(t, "70001", rows[0].number.DebtorNumber, "the number is stored trimmed")
	assert.Equal(t, "eva@example.org", rows[0].number.FamilyEmail)
}

// Per child, siblings keep their own numbers; rows of one child that
// disagree take over none (#7007).
func TestContractImportNumbersPerChild(t *testing.T) {
	t.Parallel()
	rows := readImportedNumbers([]parentbilling.ContractImportNumber{
		{Line: 2, FamilyEmail: "eva@example.org", ChildFirstName: "Mia", ChildLastName: "Beispiel", DebtorNumber: "70001"},
		{Line: 3, FamilyEmail: "eva@example.org", ChildFirstName: "Ben", ChildLastName: "Beispiel", DebtorNumber: "70002"},
		{Line: 4, FamilyEmail: "jan@example.org", ChildFirstName: "Ole", ChildLastName: "Anders", DebtorNumber: "70003"},
		{Line: 5, FamilyEmail: "jan@example.org", ChildFirstName: "ole", ChildLastName: "ANDERS", DebtorNumber: "70004"},
	}, parentbilling.DebtorNumberFormat{}, true)
	known := map[string]string{parentbilling.ChildDebtorNumberKey("eva@example.org", parentbilling.ChildKey("Ben", "Beispiel", nil)): "70002"}
	decideImportedNumbers(rows, known, true)
	assert.Equal(t, []string{parentbilling.DebtorNumberNew, parentbilling.DebtorNumberSame, parentbilling.DebtorNumberRowsDiffer,
		parentbilling.DebtorNumberRowsDiffer}, importedStatuses(rows))
}
