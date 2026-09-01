package defaults

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// The reply address is optional by design: "not configured" is a valid state
// that falls back to the school contact address, so the pattern must not
// reject an empty string (#1936).
func TestEmailReplyToAddress_AllowsEmptyAndValidatesFormat(t *testing.T) {
	t.Parallel()

	def := config.DefaultRegistry().GetDefinition(config.KeyEmailReplyToAddress)
	require.NotNil(t, def, "email.reply_to_address should be registered")

	assert.Equal(t, config.FieldText, def.Type)
	assert.Equal(t, "", def.Default, "default must be empty so the school contact address is used")
	require.NotNil(t, def.Validation)
	assert.True(t, def.Validation.AllowEmpty, "empty means 'not configured', not invalid")
	require.NotNil(t, def.Validation.Pattern)

	// It must never be operator-only: this is the school's own decision.
	assert.NotEqual(t, config.AccessOperatorOnly, def.AccessPolicy)
}
