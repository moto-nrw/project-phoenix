//go:build testhooks

package api

import (
	"context"

	deliveryCompose "github.com/moto-ogs/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// fixedNoticeTemplates answers a fixed confirmation template per kind.
type fixedNoticeTemplates struct{}

func (fixedNoticeTemplates) TraegerTexts(_ context.Context, _ int64, keys []string) (map[string]string, error) {
	texts := map[string]string{}
	for _, key := range keys {
		texts[key] = "Widerruf für {Kind}, eingegangen am {Eingang}. {Träger}"
		if key == configModels.KeyCareContractTerminationConfirmation {
			texts[key] = "Kündigung für {Kind} zum {Vertragsende}. {Träger}"
		}
	}
	return texts, nil
}

// NewParentContractNoticesForTests binds the parents' withdrawal and
// termination (#4682) as the root does, over Delivery's real outbox in db
// and fixed confirmation templates, for the parents portal's behaviour
// suite.
func NewParentContractNoticesForTests(db transactionruntime.Database) (parentportal.ContractNoticesPort, error) {
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db.Pool(), People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	if err != nil {
		return nil, err
	}
	return owners.NewParentContractNotices(fixedNoticeTemplates{}, delivery.Module, owners.CareContractTestHolidays{}), nil
}
