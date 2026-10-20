package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/ports"
)

type rejectedTenantTransaction struct{ ports.Transaction }

func (rejectedTenantTransaction) RunWrite(context.Context, func(context.Context) error) error {
	return errors.Join(devicefleet.ErrTenantRequired, errors.New("tenant context missing"))
}

func TestOwnerErrorsKeepTheirPublicIdentityAndObservation(t *testing.T) {
	t.Parallel()
	var observed ports.Observation
	service := &Service{deps: Dependencies{
		Tx: rejectedTenantTransaction{}, Observe: func(value ports.Observation) { observed = value },
	}}

	_, err := service.FindDevice(context.Background(), 9)
	if err != devicefleet.ErrTenantRequired {
		t.Fatalf("FindDevice() error = %v, want the public tenant sentinel", err)
	}
	if observed.Err != devicefleet.ErrTenantRequired || observed.Operation != "find_device" {
		t.Fatalf("observation = %#v, want the public tenant sentinel on find_device", observed)
	}
}
