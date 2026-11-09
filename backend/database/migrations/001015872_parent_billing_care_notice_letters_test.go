package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The erasure of 1.15.872 is the erasure of 1.15.843 with the outbox copies
// of the sent originals of the Jahresbescheide deleted right before the
// notices, and nothing else changed (#6347).
func TestCareNoticeLetterErasureIsTheErasureOf843WithTheOriginals(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureOutboxKindsSQL, careNoticeLetterErasureAnchor),
		"the anchor of the notices is in the erasure of 1.15.843 exactly once")
	assert.Equal(t, strings.Replace(demoVisitorErasureOutboxKindsSQL, careNoticeLetterErasureAnchor,
		careNoticeLetterErasureStatement+careNoticeLetterErasureAnchor, 1), demoVisitorErasureCareNoticeLettersSQL)
	assert.Contains(t, demoVisitorErasureCareNoticeLettersSQL, "parent_billing.care_notice_letter_mailings",
		"the erasure finds the mails through the sends")
}
