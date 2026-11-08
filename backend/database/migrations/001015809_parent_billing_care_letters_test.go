package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The erasure of 1.15.809 is the erasure of 1.15.808 with the outbox copies
// of the monthly letters deleted right before the monthly invoices, and
// nothing else changed (#4684, ADR 0165).
func TestCareLetterErasureIsTheErasureOf808WithTheLetters(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureContractPushUpSQL, careLetterErasureAnchor),
		"the anchor of the monthly invoices is in the erasure of 1.15.808 exactly once")
	assert.Equal(t, strings.Replace(demoVisitorErasureContractPushUpSQL, careLetterErasureAnchor,
		careLetterErasureSQL+careLetterErasureAnchor, 1), demoVisitorErasureCareLettersUpSQL)
}
