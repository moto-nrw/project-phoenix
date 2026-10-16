package care

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReasonRequiredForFollowsPolicy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		policy      string
		wantStaff   bool
		wantParents bool
	}{
		{ReasonPolicyNobody, false, false},
		{ReasonPolicyGuardians, false, true},
		{ReasonPolicyStaff, true, false},
		{ReasonPolicyBoth, true, true},
		// An unknown or empty value must not silently switch the requirement
		// off: the strictest reading wins.
		{"", true, true},
		{"whatever-a-future-release-writes", true, true},
	} {
		assert.Equal(t, tc.wantStaff, RequestReasonPolicy(tc.policy).RequiresReason(true), "staff side of %q", tc.policy)
		assert.Equal(t, tc.wantParents, RequestReasonPolicy(tc.policy).RequiresReason(false), "guardian side of %q", tc.policy)
	}
}
