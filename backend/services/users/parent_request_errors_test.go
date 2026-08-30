package users

import (
	"testing"

	"github.com/stretchr/testify/assert"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
)

func TestReasonRequiredForFollowsPolicy(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		policy      string
		wantStaff   bool
		wantParents bool
	}{
		{configModels.ReasonPolicyNobody, false, false},
		{configModels.ReasonPolicyGuardians, false, true},
		{configModels.ReasonPolicyStaff, true, false},
		{configModels.ReasonPolicyBoth, true, true},
		// An unknown or empty value must not silently switch the requirement
		// off: the strictest reading wins.
		{"", true, true},
		{"whatever-a-future-release-writes", true, true},
	} {
		assert.Equal(t, tc.wantStaff, ReasonRequiredFor(tc.policy, true), "staff side of %q", tc.policy)
		assert.Equal(t, tc.wantParents, ReasonRequiredFor(tc.policy, false), "guardian side of %q", tc.policy)
	}
}
