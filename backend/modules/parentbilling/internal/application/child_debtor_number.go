package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The debtor numbers of the children in the list of families (#7007). When
// the Träger keeps a number per child, the list names under each family the
// children of its care contracts and every child that has a number, and the
// office sets the number of each. A family with a contract shows even
// before its first invoice, so the numbers can be entered before the first
// run. The children come with a read of their own, so the list of families
// keeps its statements. Every number shown is recorded in its trail.

// ChildNumberStore keeps the numbers of the children of a Träger in its
// Träger transaction.
type ChildNumberStore interface {
	ChildDebtorNumbers(ctx context.Context, organizationID int64) ([]parentbilling.ChildDebtorNumber, error)
	// SaveChildDebtorNumbers writes the numbers; with onlyNew a child that
	// has a number keeps it.
	SaveChildDebtorNumbers(ctx context.Context, organizationID, accountID int64, numbers []parentbilling.ChildDebtorNumber,
		today calendar.Date, onlyNew bool) ([]parentbilling.ChildDebtorNumber, error)
	// DeleteChildDebtorNumber empties the number; the child stays known.
	DeleteChildDebtorNumber(ctx context.Context, organizationID, accountID int64, familyEmail, childKey string) (*parentbilling.ChildDebtorNumber, error)
	// DeleteChildDebtorNumbers empties a set of child numbers of one family.
	DeleteChildDebtorNumbers(ctx context.Context, organizationID, accountID int64, familyEmail string, childKeys []string) ([]parentbilling.ChildDebtorNumber, error)
	// ExtendChildDebtorNumbers keeps the numbers until the given later days.
	ExtendChildDebtorNumbers(ctx context.Context, organizationID int64, numbers []parentbilling.ChildDebtorNumber) error
	InsertChildDebtorNumberEvents(ctx context.Context, organizationID, accountID int64, action string,
		numbers []parentbilling.ChildDebtorNumber) error
}

// contractChild is the child of a contract as the list names it.
type contractChild struct {
	family    string
	recipient parentbilling.CareRecipient
	child     parentbilling.FamilyChild
	// until is the last day of the contract within the window read.
	until calendar.Date
}

// DebtorNumberScope answers whom the Träger's debtor numbers name, so the
// list of families shows the number of the family or of each child.
func (f *BillingFamilies) DebtorNumberScope(ctx context.Context, reviewer parentbilling.HolidayReviewer) (string, error) {
	if err := validReviewer(reviewer); err != nil {
		return "", err
	}
	return debtorNumberScopeOf(ctx, f.formats, reviewer.OrganizationID)
}

// contractChildren reads the children of the contracts the import can name
// too (contractVersionsAround), with
// the family and the recipient of their invoices, each child once.
func (f *BillingFamilies) contractChildren(ctx context.Context, organizationID int64) ([]contractChild, error) {
	versions, err := contractVersionsAround(ctx, f.careContracts, organizationID, f.today())
	if err != nil {
		return nil, err
	}
	first, until, ids := contractSpans(versions)
	recipients, err := f.careContracts.Recipients(ctx, organizationID, ids)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the recipients of the contracts: %w", err)
	}
	var children []contractChild
	// A child with several contracts is one child; its number is needed
	// until the last day of the latest of them.
	at := map[string]int{}
	for _, id := range ids {
		version, recipient := first[id], recipients[id]
		family := parentbilling.FamilyKey(recipient.Email)
		key := parentbilling.ChildKey(version.ChildFirstName, version.ChildLastName, version.ChildBirthDate)
		if family == "" || key == "" {
			continue
		}
		if i, seen := at[parentbilling.ChildDebtorNumberKey(family, key)]; seen {
			if until[id].After(children[i].until) {
				children[i].until = until[id]
			}
			continue
		}
		at[parentbilling.ChildDebtorNumberKey(family, key)] = len(children)
		children = append(children, contractChild{family: family, recipient: recipient, until: until[id], child: parentbilling.FamilyChild{
			FirstName: version.ChildFirstName, LastName: version.ChildLastName, BirthDate: version.ChildBirthDate}})
	}
	return children, nil
}

// contractSpans answers per contract its first version and its last day
// in the versions, and the contracts in order of their IDs.
func contractSpans(versions []parentbilling.CareContractVersion) (map[int64]parentbilling.CareContractVersion, map[int64]calendar.Date, []int64) {
	first := map[int64]parentbilling.CareContractVersion{}
	until := map[int64]calendar.Date{}
	ids := []int64{}
	for _, version := range versions {
		if _, seen := first[version.ContractID]; !seen {
			first[version.ContractID] = version
			ids = append(ids, version.ContractID)
		}
		if version.Until.After(until[version.ContractID]) {
			until[version.ContractID] = version.Until
		}
	}
	slices.Sort(ids)
	return first, until, ids
}

// familyChildNumbers lists the families of the contracts and of the
// numbers with their children, by address, and answers the numbers it
// shows, for the trail.
func familyChildNumbers(contracts []contractChild, numbers []parentbilling.ChildDebtorNumber) (
	[]parentbilling.FamilyChildNumbers, []parentbilling.ChildDebtorNumber,
) {
	families := []parentbilling.FamilyChildNumbers{}
	at := map[string]int{}
	add := func(family string, recipient parentbilling.CareRecipient) int {
		if i, ok := at[family]; ok {
			if families[i].RecipientLastName == "" {
				families[i].RecipientFirstName, families[i].RecipientLastName = recipient.FirstName, recipient.LastName
			}
			return i
		}
		families = append(families, parentbilling.FamilyChildNumbers{FamilyEmail: family, RecipientFirstName: recipient.FirstName,
			RecipientLastName: recipient.LastName, Children: []parentbilling.FamilyChild{}})
		at[family] = len(families) - 1
		return at[family]
	}
	byKey := make(map[string]parentbilling.ChildDebtorNumber, len(numbers))
	for _, number := range numbers {
		byKey[number.Key()] = number
	}
	shown := map[string]bool{}
	var read []parentbilling.ChildDebtorNumber
	for _, contract := range contracts {
		i := add(contract.family, contract.recipient)
		child := contract.child
		key := parentbilling.ChildDebtorNumberKey(contract.family, parentbilling.ChildKey(child.FirstName, child.LastName, child.BirthDate))
		if number, ok := byKey[key]; ok {
			child.DebtorNumber = number.DebtorNumber
			read = append(read, number)
		}
		shown[key] = true
		families[i].Children = append(families[i].Children, child)
	}
	for _, number := range numbers {
		if shown[number.Key()] {
			continue
		}
		i := add(number.FamilyEmail, parentbilling.CareRecipient{})
		families[i].Children = append(families[i].Children, parentbilling.FamilyChild{FirstName: number.ChildFirstName,
			LastName: number.ChildLastName, BirthDate: number.ChildBirthDate, DebtorNumber: number.DebtorNumber})
		read = append(read, number)
	}
	for i := range families {
		slices.SortStableFunc(families[i].Children, func(a, b parentbilling.FamilyChild) int {
			return strings.Compare(strings.ToLower(a.FirstName+" "+a.LastName), strings.ToLower(b.FirstName+" "+b.LastName))
		})
	}
	slices.SortStableFunc(families, func(a, b parentbilling.FamilyChildNumbers) int { return strings.Compare(a.FamilyEmail, b.FamilyEmail) })
	return families, read
}

// FamilyChildren lists the families with their children and numbers, and
// records the read of every number it shows. The office can prepare the
// numbers before the Träger keeps them per child; the list of families asks
// for them only then.
func (f *BillingFamilies) FamilyChildren(ctx context.Context, reviewer parentbilling.HolidayReviewer) ([]parentbilling.FamilyChildNumbers, error) {
	if err := validReviewer(reviewer); err != nil {
		return nil, err
	}
	contracts, err := f.contractChildren(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	numbers, err := f.store.ChildDebtorNumbers(ctx, reviewer.OrganizationID)
	if err != nil {
		return nil, err
	}
	families, read := familyChildNumbers(contracts, numbers)
	if err := f.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingRead, read); err != nil {
		return nil, fmt.Errorf("parent billing: record the read of the children's numbers: %w", err)
	}
	return families, nil
}

// SetChildDebtorNumber sets the number of a child of a family from the
// list of families (#7007): a child of a contract of the family or a child
// that has a number. An empty number removes it. The change is recorded in
// the trail of the number.
func (f *BillingFamilies) SetChildDebtorNumber(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	input parentbilling.ChildDebtorNumberInput,
) (parentbilling.FamilyChild, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.FamilyChild{}, err
	}
	format, err := debtorNumberFormatOf(ctx, f.formats, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.FamilyChild{}, err
	}
	number, err := debtorNumberOf(input.DebtorNumber, format)
	if err != nil {
		return parentbilling.FamilyChild{}, err
	}
	family := parentbilling.FamilyKey(input.FamilyEmail)
	key := parentbilling.ChildKey(input.ChildFirstName, input.ChildLastName, input.ChildBirthDate)
	if family == "" || key == "" {
		return parentbilling.FamilyChild{}, parentbilling.ErrChildNotFound
	}
	child, until, err := f.knownChild(ctx, reviewer.OrganizationID, family, key)
	if err != nil {
		return parentbilling.FamilyChild{}, err
	}
	if number == "" {
		// The answer is the child as stored now: without a number.
		child.DebtorNumber = ""
		removed, err := f.store.DeleteChildDebtorNumber(ctx, reviewer.OrganizationID, reviewer.AccountID, family, key)
		if err != nil || removed == nil {
			return child, err
		}
		if err := f.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingDelete,
			[]parentbilling.ChildDebtorNumber{*removed}); err != nil {
			return parentbilling.FamilyChild{}, fmt.Errorf("parent billing: record the removal of the child's number: %w", err)
		}
		return child, nil
	}
	saved, err := f.store.SaveChildDebtorNumbers(ctx, reviewer.OrganizationID, reviewer.AccountID, []parentbilling.ChildDebtorNumber{{
		FamilyEmail: family, ChildFirstName: child.FirstName, ChildLastName: child.LastName, ChildBirthDate: child.BirthDate,
		DebtorNumber: number, LastUsedOn: until,
	}}, f.today(), false)
	if err != nil {
		return parentbilling.FamilyChild{}, err
	}
	if err := f.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave, saved); err != nil {
		return parentbilling.FamilyChild{}, fmt.Errorf("parent billing: record the child's number: %w", err)
	}
	child.DebtorNumber = number
	return child, nil
}

// knownChild finds a child of the family by its key among the children of
// the contracts and the children with a number; ErrChildNotFound without.
// The names come from the contract or the number, not from the request. It
// answers the last day of the child's contract in the window, zero without
// one, which the number's retention counts from.
func (f *BillingFamilies) knownChild(ctx context.Context, organizationID int64, family, key string) (parentbilling.FamilyChild, calendar.Date, error) {
	wanted := parentbilling.ChildDebtorNumberKey(family, key)
	contracts, err := f.contractChildren(ctx, organizationID)
	if err != nil {
		return parentbilling.FamilyChild{}, "", err
	}
	for _, contract := range contracts {
		child := contract.child
		if parentbilling.ChildDebtorNumberKey(contract.family, parentbilling.ChildKey(child.FirstName, child.LastName, child.BirthDate)) == wanted {
			return child, contract.until, nil
		}
	}
	numbers, err := f.store.ChildDebtorNumbers(ctx, organizationID)
	if err != nil {
		return parentbilling.FamilyChild{}, "", err
	}
	for _, number := range numbers {
		if number.Key() == wanted {
			return parentbilling.FamilyChild{FirstName: number.ChildFirstName, LastName: number.ChildLastName,
				BirthDate: number.ChildBirthDate, DebtorNumber: number.DebtorNumber}, "", nil
		}
	}
	return parentbilling.FamilyChild{}, "", parentbilling.ErrChildNotFound
}
