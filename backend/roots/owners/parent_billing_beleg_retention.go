package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-ogs/project-phoenix/modules/carecontract/compose"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// NewBelegRetention binds the retention of the Belege of every Träger
// (#4720, ADR 0184) to the contracts of Care Contract, whose read and
// deletion join the administrative unit of the caller. nil today: Berlin
// day.
func NewBelegRetention(today func() calendar.Date) parentbilling.BelegRetention {
	return parentBillingCompose.NewBelegRetention(retainedContracts{careContractCompose.NewContractRetention()}, today)
}

// retainedContracts translates the facts of Care Contract's retention
// contract into the port of the Elternabrechnung; the deletion is Care
// Contract's own DeleteEndedContracts.
type retainedContracts struct{ carecontract.ContractRetention }

func (r retainedContracts) ContractRetentionFacts(ctx context.Context, organizationID int64) ([]parentbilling.ContractRetentionFact, error) {
	facts, err := r.RetentionFacts(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	out := make([]parentbilling.ContractRetentionFact, len(facts))
	for i, fact := range facts {
		out[i] = parentbilling.ContractRetentionFact{OrganizationID: fact.OrganizationID, ContractID: fact.ContractID,
			ConcludedOn: fact.ConcludedOn, Ended: fact.Ended, EndedOn: fact.EndedOn}
	}
	return out, nil
}
