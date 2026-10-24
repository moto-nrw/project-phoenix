package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestNewAuthTestModule(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module, err := NewAuthTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)

	assert.NotNil(t, module.Auth)
	assert.NotNil(t, module.MFA)
}
