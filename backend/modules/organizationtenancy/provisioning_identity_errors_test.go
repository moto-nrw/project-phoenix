package organizationtenancy_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// testStoreFailure behaves like the retained repositories' database error,
// which reports StoreFailure().
type testStoreFailure struct{ op string }

func (e *testStoreFailure) Error() string      { return "database error during " + e.op }
func (e *testStoreFailure) StoreFailure() bool { return true }

func TestMarkIdentityStoreFailureTagsStoreFailures(t *testing.T) {
	t.Parallel()

	store := &testStoreFailure{op: "insert"}
	wrapped := fmt.Errorf("create invitation: %w", store)

	marked := organizationtenancy.MarkIdentityStoreFailure(wrapped)

	require.ErrorIs(t, marked, organizationtenancy.ErrIdentityStoreFailed)
	require.ErrorIs(t, marked, store, "the store error stays in the chain")
	assert.Equal(t, wrapped.Error(), marked.Error(), "the store's text is kept")
	assert.Same(t, marked, organizationtenancy.MarkIdentityStoreFailure(marked), "an already tagged error is not wrapped twice")
}

func TestMarkIdentityStoreFailureLeavesRefusalsAlone(t *testing.T) {
	t.Parallel()

	refusal := fmt.Errorf("create invitation: %w", organizationtenancy.ErrPasswordTooWeak)
	assert.Same(t, refusal, organizationtenancy.MarkIdentityStoreFailure(refusal))

	plain := errors.New("first name and last name are required")
	assert.Same(t, plain, organizationtenancy.MarkIdentityStoreFailure(plain))
	assert.NoError(t, organizationtenancy.MarkIdentityStoreFailure(nil))
}
