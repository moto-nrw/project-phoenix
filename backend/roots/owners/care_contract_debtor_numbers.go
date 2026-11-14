package owners

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
)

// CareContractDebtorNumbers binds the Betreuungsvertrag's port of the
// debtor numbers of an import of existing contracts (#4723) to the
// Elternabrechnung, which decides and keeps them per payer or per child
// (#7007). Both run in the Träger transaction of the import; this binding
// translates the rows and the statuses.
type CareContractDebtorNumbers struct {
	Numbers parentbilling.ContractImportNumbers
}

// NewCareContractDebtorNumbers binds the port to the Elternabrechnung over
// the format of the Träger's numbers.
func NewCareContractDebtorNumbers(formats parentbilling.DebtorNumberFormats) CareContractDebtorNumbers {
	return CareContractDebtorNumbers{Numbers: parentBillingCompose.NewContractImportNumbers(formats)}
}

var _ carecontract.ContractDebtorNumbers = CareContractDebtorNumbers{}

// DebtorNumberScope names the Elternabrechnung's scope in the words of the
// import; a scope the import does not know is an error, never a guess.
func (n CareContractDebtorNumbers) DebtorNumberScope(ctx context.Context, organizationID int64) (string, error) {
	scope, err := n.Numbers.DebtorNumberScope(ctx, organizationID)
	if err != nil {
		return "", err
	}
	switch scope {
	case parentbilling.DebtorNumberScopePayer:
		return carecontract.ContractDebtorNumberPerPayer, nil
	case parentbilling.DebtorNumberScopeChild:
		return carecontract.ContractDebtorNumberPerChild, nil
	}
	return "", fmt.Errorf("care contract: unknown scope of the debtor numbers %q", scope)
}

func (n CareContractDebtorNumbers) DecideImportedDebtorNumbers(ctx context.Context, reviewer carecontract.Reviewer,
	numbers []carecontract.ImportedDebtorNumber, save bool) ([]carecontract.ImportedDebtorNumberDecision, error) {
	rows := make([]parentbilling.ContractImportNumber, len(numbers))
	for i, number := range numbers {
		rows[i] = parentbilling.ContractImportNumber{Line: number.Line, FamilyEmail: number.FamilyEmail, ContractID: number.ContractID,
			ChildFirstName: number.ChildFirstName, ChildLastName: number.ChildLastName, ChildBirthDate: number.ChildBirthDate,
			DebtorNumber: number.DebtorNumber, Until: number.Until}
	}
	decided, err := n.Numbers.DecideContractImportNumbers(ctx,
		parentbilling.HolidayReviewer{OrganizationID: reviewer.OrganizationID, AccountID: reviewer.AccountID}, rows, save)
	if err != nil {
		return nil, err
	}
	decisions := make([]carecontract.ImportedDebtorNumberDecision, len(decided))
	for i, decision := range decided {
		decisions[i] = carecontract.ImportedDebtorNumberDecision{Line: decision.Line, DebtorNumber: decision.DebtorNumber,
			Status: careContractDebtorNumberStatus(decision.Status)}
	}
	return decisions, nil
}

// careContractDebtorNumberStatus names the Elternabrechnung's decision in
// the words of the import of contracts. Rows that disagree are one status
// there, whether they name one payer or one child; a row without a child
// cannot come from a contract, which always names one.
func careContractDebtorNumberStatus(status string) string {
	switch status {
	case parentbilling.DebtorNumberNew:
		return carecontract.ContractDebtorNumberNew
	case parentbilling.DebtorNumberSame:
		return carecontract.ContractDebtorNumberSame
	case parentbilling.DebtorNumberDiffers:
		return carecontract.ContractDebtorNumberDiffers
	case parentbilling.DebtorNumberSiblingsDiffer, parentbilling.DebtorNumberRowsDiffer:
		return carecontract.ContractDebtorNumberRowsDiffer
	case parentbilling.DebtorNumberNoFamily, parentbilling.DebtorNumberNoChild:
		return carecontract.ContractDebtorNumberNoFamily
	}
	return carecontract.ContractDebtorNumberInvalid
}
