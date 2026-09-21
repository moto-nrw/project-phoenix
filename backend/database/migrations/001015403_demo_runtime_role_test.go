package migrations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDemoRuntimeRoleRequiresPassword(t *testing.T) {
	t.Parallel()

	assert.False(t, demoRuntimeRoleRequiresPassword("test"))
	assert.True(t, demoRuntimeRoleRequiresPassword("development"))
	assert.True(t, demoRuntimeRoleRequiresPassword("demo"))
	assert.True(t, demoRuntimeRoleRequiresPassword("production"))
}
