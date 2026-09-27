package schoolmembership_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// The Kontingentzahl rule for one membership (#3571) matches the SQL count:
// active children whose care has not ended, and pending ones whose care has
// not started yet.
func TestCountsTowardChildQuota(t *testing.T) {
	t.Parallel()
	const day = "2026-09-09"
	cases := []struct {
		name                string
		status, from, until string
		counts              bool
	}{
		{"active without end", "active", "", "", true},
		{"active ending today", "active", "2025-08-01", day, true},
		{"active whose care ended", "active", "2025-08-01", "2026-07-31", false},
		{"active starting later", "active", "2026-10-01", "", true},
		{"pending starting later", "pending", "2026-10-01", "", true},
		{"pending starting today", "pending", day, "", false},
		{"pending without start", "pending", "", "", false},
		{"pending already ended", "pending", "2026-10-01", "2026-09-01", false},
		{"inactive", "inactive", "", "", false},
		{"alumnus", "alumnus", "", "", false},
	}
	for _, tc := range cases {
		if got := schoolmembership.CountsTowardChildQuota(tc.status, tc.from, tc.until, day); got != tc.counts {
			t.Errorf("%s: CountsTowardChildQuota = %v, want %v", tc.name, got, tc.counts)
		}
	}
}

func TestChildQuotaUsageAdmit(t *testing.T) {
	t.Parallel()
	usage := schoolmembership.ChildQuotaUsage{Booked: 100, Occupied: 88}
	if err := usage.Admit(12); err != nil {
		t.Fatalf("12 of 12 free: %v", err)
	}
	refusal, ok := usage.Admit(13).(*schoolmembership.ChildQuotaReachedError)
	if !ok || *refusal != (schoolmembership.ChildQuotaReachedError{Booked: 100, Occupied: 88, Requested: 13}) {
		t.Fatalf("13 of 12 free: got %#v", refusal)
	}
	over := schoolmembership.ChildQuotaUsage{Booked: 100, Occupied: 120}
	if err := over.Admit(0); err != nil {
		t.Fatalf("adding nobody above the quota: %v", err)
	}
	if over.Free() != 0 || usage.Free() != 12 {
		t.Fatalf("Free = %d and %d, want 0 and 12", over.Free(), usage.Free())
	}
}
