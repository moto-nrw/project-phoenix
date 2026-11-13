package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
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
		if candidate.perChild || candidate.row.DebtorNumberStatus != parentbilling.DebtorNumberNew || seen[candidate.family] {
			continue
		}
		seen[candidate.family] = true
		set = append(set, parentbilling.FamilyDebtorNumber{FamilyEmail: candidate.family, ContractID: candidate.row.ContractID,
			DebtorNumber: candidate.row.DebtorNumber})
	}
	return set
}

func countDebtorNumbers(counts *parentbilling.SEPAMandateImportCounts, candidates []importCandidate) {
	counts.DebtorNumbers = len(debtorNumbersToSet(candidates)) + len(childNumbersToSet(candidates))
	for _, candidate := range candidates {
		switch candidate.row.DebtorNumberStatus {
		case parentbilling.DebtorNumberSiblingsDiffer, parentbilling.DebtorNumberDiffers, parentbilling.DebtorNumberRowsDiffer,
			parentbilling.DebtorNumberNoChild:
			counts.DebtorConflicts++
		}
	}
}

// The debtor numbers of a mandate import when the Träger keeps one per
// child (#7007): each row gives its number to the child it names, the child
// of the contract in the payer column, or the only child under contract of
// the family the address names. Siblings with different numbers are no
// conflict. A child moto already has another number for, or whose rows
// disagree, keeps what it has; nothing is overwritten.

// assignImportedNumbers decides the number of each row by the Träger's
// scope.
func (m *SEPAMandates) assignImportedNumbers(ctx context.Context, reviewer parentbilling.HolidayReviewer, candidates []importCandidate,
	table importTable, columns importColumnIndex,
) error {
	organizationID := reviewer.OrganizationID
	format, err := debtorNumberFormatOf(ctx, m.formats, organizationID)
	if err != nil {
		return err
	}
	scope, err := debtorNumberScopeOf(ctx, m.formats, organizationID)
	if err != nil {
		return err
	}
	if scope != parentbilling.DebtorNumberScopeChild {
		known, err := m.store.FamilyDebtorNumbers(ctx, organizationID)
		if err != nil {
			return err
		}
		assignDebtorNumbers(candidates, table, columns, known, format)
		return nil
	}
	stored, err := m.store.ChildDebtorNumbers(ctx, organizationID)
	if err != nil {
		return err
	}
	known := make(map[string]string, len(stored))
	byKey := make(map[string]parentbilling.ChildDebtorNumber, len(stored))
	for _, number := range stored {
		// A child whose number was removed has none and takes the file's.
		if number.DebtorNumber == "" {
			continue
		}
		known[number.Key()] = number.DebtorNumber
		byKey[number.Key()] = number
	}
	assignChildDebtorNumbers(candidates, table, columns, known, format)
	// The preview compares with a stored number and says so: the read is in
	// the number's trail, once per number.
	var read []parentbilling.ChildDebtorNumber
	seen := map[int64]bool{}
	for _, candidate := range candidates {
		if number, ok := byKey[importChildKey(candidate)]; ok && candidate.row.DebtorNumber != "" && !seen[number.ID] {
			seen[number.ID] = true
			read = append(read, number)
		}
	}
	if err := m.store.InsertChildDebtorNumberEvents(ctx, organizationID, reviewer.AccountID, familyBillingRead, read); err != nil {
		return fmt.Errorf("parent billing: record the read of the children's numbers: %w", err)
	}
	return nil
}

// importChildKey is the key of the child a row names, empty without one.
func importChildKey(candidate importCandidate) string {
	child := candidate.child
	key := parentbilling.ChildKey(child.FirstName, child.LastName, child.BirthDate)
	if key == "" || candidate.family == "" {
		return ""
	}
	return parentbilling.ChildDebtorNumberKey(candidate.family, key)
}

// readChildDebtorNumbers reads the number of each row and answers the
// distinct numbers of each child. An unreadable number still disagrees with
// the other rows of its child.
func readChildDebtorNumbers(candidates []importCandidate, table importTable, columns importColumnIndex,
	format parentbilling.DebtorNumberFormat,
) map[string][]string {
	check := importCheck{columns: columns}
	numbers := map[string][]string{}
	note := func(key, number string) {
		if key != "" && !slices.Contains(numbers[key], number) {
			numbers[key] = append(numbers[key], number)
		}
	}
	for i := range candidates {
		candidate := &candidates[i]
		candidate.perChild = true
		row := &candidate.row
		row.DebtorNumber = strings.TrimSpace(check.cell(table.rows[i], parentbilling.ImportFieldDebtorNumber))
		key := importChildKey(*candidate)
		switch {
		case row.DebtorNumber == "":
		case !validImportedDebtorNumber(row.DebtorNumber, format):
			note(key, invalidDebtorNumber)
			row.DebtorNumber, row.DebtorNumberStatus = "", parentbilling.DebtorNumberInvalid
		case candidate.family == "":
			row.DebtorNumberStatus = parentbilling.DebtorNumberNoFamily
		case key == "":
			row.DebtorNumberStatus = parentbilling.DebtorNumberNoChild
		default:
			note(key, row.DebtorNumber)
		}
	}
	return numbers
}

// assignChildDebtorNumbers reads the number of each row and decides per
// child.
func assignChildDebtorNumbers(candidates []importCandidate, table importTable, columns importColumnIndex, known map[string]string,
	format parentbilling.DebtorNumberFormat,
) {
	numbers := readChildDebtorNumbers(candidates, table, columns, format)
	for i := range candidates {
		row := &candidates[i].row
		if row.DebtorNumber == "" || row.DebtorNumberStatus != "" {
			continue
		}
		key := importChildKey(candidates[i])
		stored, has := known[key]
		switch {
		case len(numbers[key]) > 1:
			row.DebtorNumberStatus = parentbilling.DebtorNumberRowsDiffer
		case has && stored == row.DebtorNumber:
			row.DebtorNumberStatus = parentbilling.DebtorNumberSame
		case has:
			row.DebtorNumberStatus = parentbilling.DebtorNumberDiffers
		default:
			row.DebtorNumberStatus = parentbilling.DebtorNumberNew
		}
	}
}

// childNumbersToSet are the children whose number the import takes over,
// once each, in line order.
func childNumbersToSet(candidates []importCandidate) []parentbilling.ChildDebtorNumber {
	var set []parentbilling.ChildDebtorNumber
	at := map[string]int{}
	for _, candidate := range candidates {
		key := importChildKey(candidate)
		if !candidate.perChild || candidate.row.DebtorNumberStatus != parentbilling.DebtorNumberNew {
			continue
		}
		// Rows of one child through several contracts: the latest end counts.
		if i, seen := at[key]; seen {
			if candidate.until.After(set[i].LastUsedOn) {
				set[i].LastUsedOn = candidate.until
			}
			continue
		}
		at[key] = len(set)
		child := candidate.child
		set = append(set, parentbilling.ChildDebtorNumber{FamilyEmail: candidate.family, ChildFirstName: child.FirstName,
			ChildLastName: child.LastName, ChildBirthDate: child.BirthDate, DebtorNumber: candidate.row.DebtorNumber,
			LastUsedOn: candidate.until})
	}
	return set
}

// saveChildNumbers gives each child its number where it has none and
// records the change in the trail of the number.
func (m *SEPAMandates) saveChildNumbers(ctx context.Context, reviewer parentbilling.HolidayReviewer, candidates []importCandidate) error {
	set := childNumbersToSet(candidates)
	if len(set) == 0 {
		return nil
	}
	saved, err := m.store.SaveChildDebtorNumbers(ctx, reviewer.OrganizationID, reviewer.AccountID, set, m.today(), true)
	if err != nil {
		return fmt.Errorf("parent billing: take over the debtor numbers of the children: %w", err)
	}
	if err := m.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave, saved); err != nil {
		return fmt.Errorf("parent billing: record the debtor numbers of the children: %w", err)
	}
	return nil
}
