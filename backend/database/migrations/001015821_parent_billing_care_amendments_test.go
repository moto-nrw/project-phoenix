package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCareAmendmentsErasureExtendsTheErasureOf811 keeps the erasure of
// 1.15.821 the erasure of 1.15.811 with the deletion of the changes of the
// review, before the contracts they name go.
func TestCareAmendmentsErasureExtendsTheErasureOf811(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureMealBillingSQL, careAmendmentsErasureAnchor))
	assert.Equal(t, strings.Replace(demoVisitorErasureMealBillingSQL, careAmendmentsErasureAnchor,
		careAmendmentsErasureStatement+careAmendmentsErasureAnchor, 1), demoVisitorErasureCareAmendmentsSQL)
}
