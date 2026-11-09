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

// assignDebtorNumbers reads the number of each row and decides per family.
func assignDebtorNumbers(candidates []importCandidate, table importTable, columns importColumnIndex, known map[string]string) {
	check := importCheck{columns: columns}
	numbers := map[string][]string{}
	for i := range candidates {
		row := &candidates[i].row
		row.DebtorNumber = strings.TrimSpace(check.cell(table.rows[i], parentbilling.ImportFieldDebtorNumber))
		switch {
		case row.DebtorNumber == "":
		case checkDebtorNumber(row.DebtorNumber) != nil || containsIBAN(row.DebtorNumber):
			row.DebtorNumber, row.DebtorNumberStatus = "", parentbilling.DebtorNumberInvalid
		case candidates[i].family == "":
			row.DebtorNumberStatus = parentbilling.DebtorNumberNoFamily
		default:
			family := candidates[i].family
			if !slices.Contains(numbers[family], row.DebtorNumber) {
				numbers[family] = append(numbers[family], row.DebtorNumber)
			}
		}
	}
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
