//go:build testhooks

package owners

import (
	"context"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewEnrollmentFlowDelivery composes the Delivery platform over the test
// database with a provider that accepts every send.
func NewEnrollmentFlowDelivery(db transactionruntime.Database) (delivery.Capability, error) {
	runtime, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db.Pool(), People: EnrollmentFlowGuardianDirectory{},
		Observe: func(deliveryContract.Observation) {},
	})
	if err != nil {
		return nil, err
	}
	if _, err := deliveryCompose.NewWorker(runtime, enrollmentFlowDeliveryProvider{}); err != nil {
		return nil, err
	}
	return runtime.Module, nil
}

type enrollmentFlowDeliveryProvider struct{}

func (enrollmentFlowDeliveryProvider) SendEmail(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return deliveryContract.ProviderResult{}, nil
}

func (enrollmentFlowDeliveryProvider) SendPush(context.Context, deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	return deliveryContract.ProviderResult{}, nil
}

type EnrollmentFlowGuardianDirectory struct{}

func (EnrollmentFlowGuardianDirectory) ResolveGuardianDisplays(context.Context, []int64) ([]deliveryContract.GuardianDisplay, error) {
	return nil, nil
}
