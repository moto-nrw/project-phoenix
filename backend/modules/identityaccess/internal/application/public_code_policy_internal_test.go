package application

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
)

// The domain may import no other package of its owner, so the public package
// repeats the code policy its callers read. The flows apply the domain's.
func TestPublicCodePolicyIsTheDomains(t *testing.T) {
	t.Parallel()
	assert.Equal(t, domain.MFAChallengeTTL, identityaccess.MFAChallengeTTL)
	assert.Equal(t, domain.MFALockoutThreshold, identityaccess.MFALockoutThreshold)
	assert.Equal(t, domain.OperatorMFARateLimitMaxSent, identityaccess.OperatorMFARateLimitMaxSent)
}
