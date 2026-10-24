package identityaccess_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The relying-party name and the demo's operator sign-in stay pinned.
func TestPasskeyAndDemoConstants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "moto", identityaccess.PasskeyRPName)
	assert.True(t, identityaccess.DemoOperatorWithoutSecondFactor)
}
