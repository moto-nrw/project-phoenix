package parentrequests_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// TestReasonRequiredFollowsPolicy pins both sides of the one reason rule: the
// parents portal asks it for the submitting family, the staff routes for the
// deciding staff member.
func TestReasonRequiredFollowsPolicy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		policy       string
		wantStaff    bool
		wantGuardian bool
	}{
		{parentrequests.ReasonPolicyNobody, false, false},
		{parentrequests.ReasonPolicyGuardians, false, true},
		{parentrequests.ReasonPolicyStaff, true, false},
		{parentrequests.ReasonPolicyBoth, true, true},
		// An unknown or empty value must not silently switch the requirement
		// off: the strictest reading wins.
		{"", true, true},
		{"whatever-a-future-release-writes", true, true},
	} {
		assert.Equal(t, tc.wantStaff, parentrequests.ReasonRequired(tc.policy, parentrequests.ReasonSideStaff), "staff side of %q", tc.policy)
		assert.Equal(t, tc.wantGuardian, parentrequests.ReasonRequired(tc.policy, parentrequests.ReasonSideGuardian), "guardian side of %q", tc.policy)
	}
}
