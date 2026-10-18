package organizationtenancy_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// Accounts work only in an active school that is not deleted.
func TestSchoolOperational(t *testing.T) {
	t.Parallel()

	deleted := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		school organizationtenancy.School
		want   bool
	}{
		{organizationtenancy.School{Active: true}, true},
		{organizationtenancy.School{Active: false}, false},
		{organizationtenancy.School{Active: true, DeletedAt: &deleted}, false},
	} {
		if got := tc.school.Operational(); got != tc.want {
			t.Errorf("Operational() of %+v = %v, want %v", tc.school, got, tc.want)
		}
	}
}
