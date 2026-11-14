package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The debtor numbers of an import of care contracts (#4723): the same rules
// as the numbers of a mandate import (#4693, #7007), for rows that name the
// child and its family themselves.

// ContractImportNumbers serves parentbilling.ContractImportNumbers.
type ContractImportNumbers struct {
	store   MandateImportStore
	formats parentbilling.DebtorNumberFormats
	today   func() calendar.Date
}

var _ parentbilling.ContractImportNumbers = (*ContractImportNumbers)(nil)

func NewContractImportNumbers(store MandateImportStore, formats parentbilling.DebtorNumberFormats, today func() calendar.Date) *ContractImportNumbers {
	if store == nil || formats == nil || today == nil {
		panic("parent billing: the store, the format of the debtor numbers and the clock are required")
	}
	return &ContractImportNumbers{store: store, formats: formats, today: today}
}

// DebtorNumberScope answers whom the numbers of the Träger name.
func (n *ContractImportNumbers) DebtorNumberScope(ctx context.Context, organizationID int64) (string, error) {
	return debtorNumberScopeOf(ctx, n.formats, organizationID)
}

// importedNumber is a row with the key its number belongs to: the family,
// or the child of the family when the Träger keeps a number per child.
type importedNumber struct {
	number parentbilling.ContractImportNumber
	key    string
	status string
}

// DecideContractImportNumbers decides per payer or per child, as the Träger
// keeps its numbers, and with save stores the new ones.
func (n *ContractImportNumbers) DecideContractImportNumbers(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	numbers []parentbilling.ContractImportNumber, save bool,
) ([]parentbilling.ContractImportNumberDecision, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	if len(numbers) == 0 {
		return []parentbilling.ContractImportNumberDecision{}, nil
	}
	format, err := debtorNumberFormatOf(ctx, n.formats, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	scope, err := debtorNumberScopeOf(ctx, n.formats, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	perChild := scope == parentbilling.DebtorNumberScopeChild
	rows := readImportedNumbers(numbers, format, perChild)
	known, err := n.knownNumbers(ctx, reviewer, rows, perChild)
	if err != nil {
		return nil, err
	}
	decideImportedNumbers(rows, known, perChild)
	if save {
		if err := n.save(ctx, reviewer, rows, perChild); err != nil {
			return nil, err
		}
	}
	decisions := make([]parentbilling.ContractImportNumberDecision, len(rows))
	for i, row := range rows {
		decisions[i] = parentbilling.ContractImportNumberDecision{Line: row.number.Line, DebtorNumber: row.number.DebtorNumber, Status: row.status}
	}
	return decisions, nil
}

// readImportedNumbers checks each number against the format of the Träger
// and names the family or the child it belongs to.
func readImportedNumbers(numbers []parentbilling.ContractImportNumber, format parentbilling.DebtorNumberFormat, perChild bool) []importedNumber {
	rows := make([]importedNumber, len(numbers))
	for i, number := range numbers {
		row := importedNumber{number: number}
		family := parentbilling.FamilyKey(number.FamilyEmail)
		normalized, err := parentbilling.NormalizeDebtorNumber(number.DebtorNumber, format)
		row.number.FamilyEmail = family
		switch {
		case err != nil || normalized == "" || containsIBAN(normalized):
			// An unreadable number still disagrees with the other rows of
			// its payer or child, so it keeps its key.
			row.number.DebtorNumber, row.status = "", parentbilling.DebtorNumberInvalid
		default:
			row.number.DebtorNumber = normalized
		}
		child := parentbilling.ChildKey(number.ChildFirstName, number.ChildLastName, number.ChildBirthDate)
		switch {
		case family == "":
			row.status = firstStatus(row.status, parentbilling.DebtorNumberNoFamily)
		case perChild && child == "":
			row.status = firstStatus(row.status, parentbilling.DebtorNumberNoChild)
		case perChild:
			row.key = parentbilling.ChildDebtorNumberKey(family, child)
		default:
			row.key = family
		}
		rows[i] = row
	}
	return rows
}

func firstStatus(status, fallback string) string {
	if status != "" {
		return status
	}
	return fallback
}

// knownNumbers reads the numbers moto keeps for the families or the
// children of the rows, by their key. A read of a child's number is
// recorded in its trail, as in the list of families.
func (n *ContractImportNumbers) knownNumbers(ctx context.Context, reviewer parentbilling.HolidayReviewer, rows []importedNumber, perChild bool) (map[string]string, error) {
	if !perChild {
		known, err := n.store.FamilyDebtorNumbers(ctx, reviewer.OrganizationID)
		if err != nil {
			return nil, fmt.Errorf("parent billing: read the debtor numbers of the families: %w", err)
		}
		return known, nil
	}
	stored, err := n.store.ChildDebtorNumbers(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the debtor numbers of the children: %w", err)
	}
	wanted := map[string]bool{}
	for _, row := range rows {
		wanted[row.key] = true
	}
	known := map[string]string{}
	var read []parentbilling.ChildDebtorNumber
	for _, number := range stored {
		// A child whose number was removed has none and takes the file's.
		if number.DebtorNumber == "" || !wanted[number.Key()] {
			continue
		}
		known[number.Key()] = number.DebtorNumber
		read = append(read, number)
	}
	if err := n.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingRead, read); err != nil {
		return nil, fmt.Errorf("parent billing: record the read of the children's numbers: %w", err)
	}
	return known, nil
}

// decideImportedNumbers gives every row with a key its status: rows of one
// key that disagree take over nothing, a stored number stays, a new one is
// taken over.
func decideImportedNumbers(rows []importedNumber, known map[string]string, perChild bool) {
	distinct := map[string][]string{}
	for _, row := range rows {
		if row.key != "" && !slices.Contains(distinct[row.key], row.number.DebtorNumber) {
			distinct[row.key] = append(distinct[row.key], row.number.DebtorNumber)
		}
	}
	differ := parentbilling.DebtorNumberSiblingsDiffer
	if perChild {
		differ = parentbilling.DebtorNumberRowsDiffer
	}
	for i := range rows {
		row := &rows[i]
		if row.status != "" || row.key == "" {
			continue
		}
		stored, has := known[row.key]
		switch {
		case len(distinct[row.key]) > 1:
			row.status = differ
		case has && stored == row.number.DebtorNumber:
			row.status = parentbilling.DebtorNumberSame
		case has:
			row.status = parentbilling.DebtorNumberDiffers
		default:
			row.status = parentbilling.DebtorNumberNew
		}
	}
}

// save stores the new numbers, once per payer or child, and records each.
func (n *ContractImportNumbers) save(ctx context.Context, reviewer parentbilling.HolidayReviewer, rows []importedNumber, perChild bool) error {
	var families []parentbilling.FamilyDebtorNumber
	var children []parentbilling.ChildDebtorNumber
	at := map[string]int{}
	for _, row := range rows {
		if row.status != parentbilling.DebtorNumberNew {
			continue
		}
		number := row.number
		position, seen := at[row.key]
		switch {
		case seen && perChild && number.Until.After(children[position].LastUsedOn):
			// Rows of one child through several contracts: the latest end counts.
			children[position].LastUsedOn = number.Until
		case seen:
		case perChild:
			at[row.key] = len(children)
			children = append(children, parentbilling.ChildDebtorNumber{FamilyEmail: number.FamilyEmail,
				ChildFirstName: number.ChildFirstName, ChildLastName: number.ChildLastName, ChildBirthDate: number.ChildBirthDate,
				DebtorNumber: number.DebtorNumber, LastUsedOn: number.Until})
		default:
			at[row.key] = len(families)
			families = append(families, parentbilling.FamilyDebtorNumber{FamilyEmail: number.FamilyEmail, ContractID: number.ContractID,
				DebtorNumber: number.DebtorNumber})
		}
	}
	if err := n.store.SetFamilyDebtorNumbers(ctx, reviewer.OrganizationID, reviewer.AccountID, families); err != nil {
		return fmt.Errorf("parent billing: take over the debtor numbers of the families: %w", err)
	}
	if len(children) == 0 {
		return nil
	}
	saved, err := n.store.SaveChildDebtorNumbers(ctx, reviewer.OrganizationID, reviewer.AccountID, children, n.today(), true)
	if err != nil {
		return fmt.Errorf("parent billing: take over the debtor numbers of the children: %w", err)
	}
	if err := n.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave, saved); err != nil {
		return fmt.Errorf("parent billing: record the debtor numbers of the children: %w", err)
	}
	return nil
}
