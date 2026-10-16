package transactionruntime_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/stretchr/testify/assert"
)

// TestIsUniqueViolationRefusesOtherErrors pins the negative cases the People
// Directory guardian suite checked before the classification moved here
// (#3990): no error, a plain error and a wrapped non-driver error.
func TestIsUniqueViolationRefusesOtherErrors(t *testing.T) {
	t.Parallel()

	for _, err := range []error{nil, errors.New("boom"), fmt.Errorf("create: %w", errors.New("connection reset"))} {
		assert.False(t, transactionruntime.IsUniqueViolation(err))
		assert.False(t, transactionruntime.IsUniqueViolationOn(err, "idx_guardian_profiles_tenant_email"))
	}
}
