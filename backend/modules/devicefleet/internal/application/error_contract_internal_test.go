package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/devicefleet"
	"github.com/moto-ogs/project-phoenix/modules/devicefleet/internal/ports"
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

type projectionTransactionGuard struct {
	ports.Transaction
	started bool
}

func (*projectionTransactionGuard) TenantID(context.Context) (int64, error) {
	return 0, devicefleet.ErrTenantRequired
}

func (tx *projectionTransactionGuard) RunWrite(context.Context, func(context.Context) error) error {
	tx.started = true
	return errors.New("must not reconstruct the write transaction")
}

func TestPersistedRoomProjectionRejectsMissingWriteUnitBeforeOpeningOne(t *testing.T) {
	t.Parallel()
	tx := &projectionTransactionGuard{}
	var observed ports.Observation
	service := &Service{deps: Dependencies{Tx: tx, Observe: func(value ports.Observation) { observed = value }}}
	_, err := service.DeviceRoomName(context.Background(), devicefleet.Device{ID: 9})
	if !errors.Is(err, devicefleet.ErrTenantRequired) || tx.started {
		t.Fatalf("projection error=%v, new transaction started=%t", err, tx.started)
	}
	if observed.Operation != "find_device" || !errors.Is(observed.Err, devicefleet.ErrTenantRequired) {
		t.Fatalf("unexpected projection observation: %#v", observed)
	}
}
