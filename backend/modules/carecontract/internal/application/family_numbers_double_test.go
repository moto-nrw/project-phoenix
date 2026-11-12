package application

import (
	"context"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
)

// familyNumbersDouble stands in for the Elternabrechnung's numbers where a
// test signs or makes contracts without the parents' number.
type familyNumbersDouble struct{}

func (familyNumbersDouble) FamilyChildren(context.Context, int64, carecontract.FamilyContract) (carecontract.FamilyChildren, error) {
	return carecontract.FamilyChildren{}, nil
}

func (familyNumbersDouble) StateFamilyChildren(context.Context, int64, carecontract.FamilyContract, int, *time.Time) (carecontract.FamilyChildren, error) {
	return carecontract.FamilyChildren{}, nil
}
