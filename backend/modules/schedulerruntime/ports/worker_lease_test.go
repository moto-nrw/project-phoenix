package ports

import (
	"strings"
	"testing"
	"time"
)

func TestNewWorkerLeaseOwnsTheTimingAndAFreshHolder(t *testing.T) {
	t.Parallel()
	lease, err := NewWorkerLease(nil, LeaseEvidence{})
	if err != nil {
		t.Fatalf("new worker lease: %v", err)
	}
	if lease.Name != WorkerLeaseName || lease.TTL != 30*time.Second || lease.RenewEvery != 10*time.Second ||
		lease.RetryEvery != 5*time.Second || lease.FenceMargin != 5*time.Second {
		t.Errorf("lease = %+v, want the worker lease timing", lease)
	}
	if segments := strings.Count(lease.Holder, "/") + 1; segments != 3 {
		t.Errorf("holder %q has %d segments, want host, process and start time", lease.Holder, segments)
	}
	other, err := NewWorkerLease(nil, LeaseEvidence{})
	if err != nil {
		t.Fatalf("second worker lease: %v", err)
	}
	if other.Holder == lease.Holder {
		t.Error("every lease gets a holder of its own")
	}
}
