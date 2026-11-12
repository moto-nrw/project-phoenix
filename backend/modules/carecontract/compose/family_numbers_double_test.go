package compose_test

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// familyNumbersDouble stands in for the Elternabrechnung's numbers where a
// test makes contracts without a registration's number.
type familyNumbersDouble struct{}

func (familyNumbersDouble) FamilyChildren(context.Context, int64, carecontract.FamilyContract) (carecontract.FamilyChildren, error) {
	return carecontract.FamilyChildren{}, nil
}

func (familyNumbersDouble) StateFamilyChildren(context.Context, int64, carecontract.FamilyContract, int, *time.Time) (carecontract.FamilyChildren, error) {
	return carecontract.FamilyChildren{}, nil
}
