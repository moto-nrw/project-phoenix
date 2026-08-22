package rotation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFamilyFingerprintUsesStableSHA256(t *testing.T) {
	t.Parallel()

	const familyID = "family-123"

	fingerprint := FamilyFingerprint(familyID)

	assert.Equal(t, "4e8c93922ccdfb7d6a6550a756e980d0616ba222cbda421b329b550e80cf3d57", fingerprint)
	assert.NotContains(t, fingerprint, familyID)
}
