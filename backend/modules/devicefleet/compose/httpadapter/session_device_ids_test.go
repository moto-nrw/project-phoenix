package httpadapter_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	legacy "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
)

type sessionDeviceLookup struct {
	devicefleet.SessionDeviceRecords
	lookup func(context.Context, string) (*devicefleet.Device, error)
}

func (s sessionDeviceLookup) FindByDeviceID(ctx context.Context, code string) (*devicefleet.Device, error) {
	return s.lookup(ctx, code)
}

func TestSessionDeviceIDsPreserveLookupResults(t *testing.T) {
	t.Parallel()
	lookupErr := errors.New("device lookup unavailable")
	for _, tc := range []struct {
		name   string
		device *devicefleet.Device
		err    error
		wantID bool
	}{
		{name: "found", device: &devicefleet.Device{ID: 37}, wantID: true},
		{name: "missing"},
		{name: "error", err: lookupErr},
		{name: "partial result with error", device: &devicefleet.Device{ID: 37}, err: lookupErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			adapter := legacy.NewSessionDeviceIDs(sessionDeviceLookup{lookup: func(gotCtx context.Context, code string) (*devicefleet.Device, error) {
				calls++
				if gotCtx != ctx || code != "WEB-MANUAL-001" {
					t.Fatal("lookup changed context or device code")
				}
				return tc.device, tc.err
			}})
			id, err := adapter.ManualAttendanceDeviceID(ctx)
			if calls != 1 {
				t.Fatalf("lookup calls=%d, want one", calls)
			}
			if tc.name == "missing" {
				if err == nil || err.Error() != "WEB-MANUAL-001 is not configured" {
					t.Fatalf("missing device error changed: %v", err)
				}
			} else if !errors.Is(err, tc.err) {
				t.Fatalf("lookup calls=%d error=%v, want one call and %v", calls, err, tc.err)
			}
			if tc.wantID {
				if id != tc.device.ID {
					t.Fatalf("wrong projected ID: %v", id)
				}
			} else if id != 0 {
				t.Fatalf("unexpected projected ID: %d", id)
			}
		})
	}
}

func (s sessionDeviceLookup) LookupSessionDevice(ctx context.Context, code string) (int64, bool, error) {
	device, err := s.FindByDeviceID(ctx, code)
	if err != nil {
		return 0, false, err
	}
	if device == nil {
		return 0, false, nil
	}
	return device.ID, true, nil
}
