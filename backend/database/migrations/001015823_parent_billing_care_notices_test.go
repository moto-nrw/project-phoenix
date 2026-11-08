package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCareNoticeErasureExtendsTheErasureOf811 keeps the erasure of 1.15.823
// the erasure of 1.15.811 with the statements for the fee notices, placed
// while the contracts of the demo school still exist.
func TestCareNoticeErasureExtendsTheErasureOf811(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureMealBillingSQL, careNoticeErasureAnchor))
	assert.Equal(t, strings.Replace(demoVisitorErasureMealBillingSQL, careNoticeErasureAnchor,
		careNoticeErasureAnchor+careNoticeErasureStatement, 1), demoVisitorErasureCareNoticesSQL)
	notices := strings.Index(demoVisitorErasureCareNoticesSQL, "DELETE FROM parent_billing.care_notices")
	contracts := strings.Index(demoVisitorErasureCareNoticesSQL, "DELETE FROM care_contract.contracts")
	require.Positive(t, notices)
	assert.Less(t, notices, contracts, "the reads of a notice run go before the contracts they name")
}
