package owners

import (
	"context"
	"errors"
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
)

// CareContractFamilyNumbers binds the Betreuungsvertrag's port of the
// number of children of a contract's family (#6335) to the
// Elternabrechnung, which keeps the number, its confirmation and its
// trail. The Betreuungsvertrag checked the contract in the Träger
// transaction both run in; this binding translates values, channels and
// refusals.
type CareContractFamilyNumbers struct {
	Numbers parentbilling.ContractFamilyChildren
}

// NewCareContractFamilyNumbers binds the port to the Elternabrechnung.
func NewCareContractFamilyNumbers() CareContractFamilyNumbers {
	// The parents' inbox of the Elternabrechnung keeps the numbers of the
	// families (#5918); its statement serves the link and the takeover too.
	return CareContractFamilyNumbers{Numbers: parentBillingCompose.NewParentDocuments()}
}

var _ carecontract.FamilyNumbers = CareContractFamilyNumbers{}

func (n CareContractFamilyNumbers) FamilyChildren(ctx context.Context, organizationID int64, family carecontract.FamilyContract) (carecontract.FamilyChildren, error) {
	children, err := n.Numbers.ContractFamilyChildren(ctx, organizationID, contractFamily(family))
	return careContractFamilyChildren(children), careContractFamilyError(err)
}

func (n CareContractFamilyNumbers) StateFamilyChildren(ctx context.Context, organizationID int64, family carecontract.FamilyContract, children int,
	statedAt *time.Time) (carecontract.FamilyChildren, error) {
	stated, err := n.Numbers.StateContractFamilyChildren(ctx, organizationID, contractFamily(family), children, statedAt)
	return careContractFamilyChildren(stated), careContractFamilyError(err)
}

func contractFamily(family carecontract.FamilyContract) parentbilling.ContractFamily {
	channel := ""
	switch family.Channel {
	case carecontract.FamilyChannelLink:
		channel = parentbilling.FamilyChannelLink
	case carecontract.FamilyChannelEnrollment:
		channel = parentbilling.FamilyChannelEnrollment
	}
	return parentbilling.ContractFamily{ContractID: family.ContractID, Email: family.Email, AccountID: family.AccountID, Channel: channel}
}

func careContractFamilyChildren(children parentbilling.ParentFamilyChildren) carecontract.FamilyChildren {
	return carecontract.FamilyChildren{ConfirmedAt: children.ConfirmedAt, StatedChildren: children.StatedChildren, StatedAt: children.StatedAt,
		ConfirmedChildren: children.ConfirmedChildren, ValidFrom: children.ValidFrom, ValidUntil: children.ValidUntil}
}

func careContractFamilyError(err error) error {
	var fields validation.Errors
	switch {
	case err == nil:
		return nil
	case errors.As(err, &fields):
		return &carecontract.ValidationError{Fields: map[string]string{"children": "invalid"}}
	case errors.Is(err, parentbilling.ErrFamilyChildrenNotFound):
		return fmt.Errorf("%w: %w", carecontract.ErrContractNotFound, err)
	}
	return err
}
