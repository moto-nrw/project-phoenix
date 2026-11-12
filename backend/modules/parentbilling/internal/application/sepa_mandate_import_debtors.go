package application

import (
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The debtor numbers of a mandate import (#4693, decision E-c of 09.11.):
// OGS Connect keeps "kinder-Debitorennummer" per child, moto keeps one per
// family. The import takes a number over only when every row of the family
// carries the same one and the family has none yet; siblings with
// different numbers, or a family with another number in moto, show as a
// conflict and nothing is overwritten. The number does not change what the
// import does with the mandate of the row.

// invalidDebtorNumber stands for an unreadable number of a family's row.
const invalidDebtorNumber = "\x00invalid"

// assignDebtorNumbers reads the number of each row and decides per family.
// A number outside the Träger's format (#6675) is unreadable like a number
// with a line break: the family gets none from the file.
func assignDebtorNumbers(candidates []importCandidate, table importTable, columns importColumnIndex, known map[string]string,
	format parentbilling.DebtorNumberFormat,
) {
	numbers := readDebtorNumbers(candidates, table, columns, format)
	for i := range candidates {
		row := &candidates[i].row
		if row.DebtorNumber == "" || row.DebtorNumberStatus != "" {
			continue
		}
		family := candidates[i].family
		stored, has := known[family]
		switch {
		case len(numbers[family]) > 1:
			row.DebtorNumberStatus = parentbilling.DebtorNumberSiblingsDiffer
		case has && stored == row.DebtorNumber:
			row.DebtorNumberStatus = parentbilling.DebtorNumberSame
		case has:
			row.DebtorNumberStatus = parentbilling.DebtorNumberDiffers
		default:
			row.DebtorNumberStatus = parentbilling.DebtorNumberNew
		}
	}
}

// readDebtorNumbers reads the number of each row and answers the distinct
// numbers of each family. An unreadable number still disagrees with its
// siblings: the family gets no number the file does not agree on.
func readDebtorNumbers(candidates []importCandidate, table importTable, columns importColumnIndex,
	format parentbilling.DebtorNumberFormat,
) map[string][]string {
	check := importCheck{columns: columns}
	numbers := map[string][]string{}
	note := func(family, number string) {
		if !slices.Contains(numbers[family], number) {
			numbers[family] = append(numbers[family], number)
		}
	}
	for i := range candidates {
		row, family := &candidates[i].row, candidates[i].family
		row.DebtorNumber = strings.TrimSpace(check.cell(table.rows[i], parentbilling.ImportFieldDebtorNumber))
		switch {
		case row.DebtorNumber == "":
		case !validImportedDebtorNumber(row.DebtorNumber, format):
			if family != "" {
				note(family, invalidDebtorNumber)
			}
			row.DebtorNumber, row.DebtorNumberStatus = "", parentbilling.DebtorNumberInvalid
		case family == "":
			row.DebtorNumberStatus = parentbilling.DebtorNumberNoFamily
		default:
			note(family, row.DebtorNumber)
		}
	}
	return numbers
}

// validImportedDebtorNumber checks a number of the file through the one rule
// of a debtor number; an IBAN is never one.
func validImportedDebtorNumber(number string, format parentbilling.DebtorNumberFormat) bool {
	_, err := parentbilling.NormalizeDebtorNumber(number, format)
	return err == nil && !containsIBAN(number)
}

// debtorNumbersToSet are the families whose number the import takes over,
// once each, in line order.
func debtorNumbersToSet(candidates []importCandidate) []parentbilling.FamilyDebtorNumber {
	var set []parentbilling.FamilyDebtorNumber
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if candidate.row.DebtorNumberStatus != parentbilling.DebtorNumberNew || seen[candidate.family] {
			continue
		}
		seen[candidate.family] = true
		set = append(set, parentbilling.FamilyDebtorNumber{FamilyEmail: candidate.family, ContractID: candidate.row.ContractID,
			DebtorNumber: candidate.row.DebtorNumber})
	}
	return set
}

func countDebtorNumbers(counts *parentbilling.SEPAMandateImportCounts, candidates []importCandidate) {
	counts.DebtorNumbers = len(debtorNumbersToSet(candidates))
	for _, candidate := range candidates {
		switch candidate.row.DebtorNumberStatus {
		case parentbilling.DebtorNumberSiblingsDiffer, parentbilling.DebtorNumberDiffers:
			counts.DebtorConflicts++
		}
	}
}
