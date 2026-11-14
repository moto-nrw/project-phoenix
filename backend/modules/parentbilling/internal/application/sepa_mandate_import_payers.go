package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The payer of a mandate by its debtor number (#7009). A file without
// contract numbers and mail addresses, such as the master data of OGS
// Connect, names each row's payer by the debtor number the Träger keeps:
// the child's when it keeps one per child (#7007), else the family's. The
// import looks the number up among the numbers moto already keeps (list of
// families, confirmation, earlier imports) and never guesses: a number of
// more than one family or child names no payer. Every number a row names
// is recorded in the trail of that number.

// numberPayer is the payer a stored debtor number names, with the stored
// number whose read the trail records.
type numberPayer struct {
	payer  importPayer
	family *parentbilling.FamilyBilling
	child  *parentbilling.ChildDebtorNumber
}

// indexPayerNumbers indexes the payers by the debtor numbers of the scope
// of the Träger. A number that names no contract the import can name
// stays out; a number of two payers is ambiguous.
func (m *SEPAMandates) indexPayerNumbers(ctx context.Context, organizationID int64, payers *importPayers) error {
	format, err := debtorNumberFormatOf(ctx, m.formats, organizationID)
	if err != nil {
		return err
	}
	scope, err := debtorNumberScopeOf(ctx, m.formats, organizationID)
	if err != nil {
		return err
	}
	payers.format, payers.byNumber, payers.ambiguous = format, map[string]numberPayer{}, map[string]bool{}
	if scope == parentbilling.DebtorNumberScopeChild {
		children, err := m.store.ChildDebtorNumbers(ctx, organizationID)
		if err != nil {
			return err
		}
		for i := range children {
			if payer, ok := payers.byChild[children[i].Key()]; ok && children[i].DebtorNumber != "" {
				payer.ofChild = true
				payers.addNumber(children[i].DebtorNumber, numberPayer{payer: payer, child: &children[i]})
			}
		}
		return nil
	}
	families, err := m.store.FamilyBillings(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("parent billing: read the debtor numbers of the families: %w", err)
	}
	for i := range families {
		if payer, ok := payers.byFamily[families[i].FamilyEmail]; ok && families[i].DebtorNumber != "" {
			payers.addNumber(families[i].DebtorNumber, numberPayer{payer: payer, family: &families[i]})
		}
	}
	return nil
}

// addNumber names the payer of a number; a second payer makes it ambiguous.
func (p importPayers) addNumber(number string, payer numberPayer) {
	if current, ok := p.byNumber[number]; ok && current.payer.contractID != payer.payer.contractID {
		p.ambiguous[number] = true
		return
	}
	p.byNumber[number] = payer
}

// payerOfNumber finds the payer of a debtor number of the payer column, in
// the Träger's format.
func (p importPayers) payerOfNumber(candidate *importCandidate, value string) (importPayer, string) {
	number, err := parentbilling.NormalizeDebtorNumber(value, p.format)
	if err != nil {
		return importPayer{}, parentbilling.ImportProblemPayerUnknown
	}
	if p.ambiguous[number] {
		return importPayer{}, parentbilling.ImportProblemPayerAmbiguous
	}
	payer, ok := p.byNumber[number]
	if !ok {
		return importPayer{}, parentbilling.ImportProblemPayerUnknown
	}
	candidate.payerNumber = number
	return payer.payer, ""
}

// recordPayerNumberReads records in the trail of each stored number that a
// row named that the import read it, once per number.
func (m *SEPAMandates) recordPayerNumberReads(ctx context.Context, reviewer parentbilling.HolidayReviewer, payers importPayers,
	candidates []importCandidate,
) error {
	var families []parentbilling.FamilyBilling
	var children []parentbilling.ChildDebtorNumber
	seen := map[string]bool{}
	for _, candidate := range candidates {
		number := candidate.payerNumber
		if number == "" || seen[number] {
			continue
		}
		seen[number] = true
		switch read := payers.byNumber[number]; {
		case read.child != nil:
			children = append(children, *read.child)
		case read.family != nil:
			families = append(families, *read.family)
		}
	}
	if err := m.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingRead, families); err != nil {
		return fmt.Errorf("parent billing: record the read of the families' numbers: %w", err)
	}
	if err := m.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingRead, children); err != nil {
		return fmt.Errorf("parent billing: record the read of the children's numbers: %w", err)
	}
	return nil
}
