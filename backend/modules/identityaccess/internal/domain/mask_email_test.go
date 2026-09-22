package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The masking rule the MFA hint, the operator ledger and the delivery logs
// share: none of them may carry a full address, because the ledger is read by
// every operator and the log by everyone with access to it.
func TestMaskEmail(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		input    string
		expected string
	}{
		"normal address":       {"operator@example.com", "o***@example.com"},
		"mfa hint":             {"jane@example.com", "j***@example.com"},
		"two-character local":  {"ab@example.com", "***@example.com"},
		"one-character local":  {"a@example.com", "***@example.com"},
		"empty":                {"", "***"},
		"no at sign":           {"invalid-email", "***"},
		"empty local part":     {"@example.com", "***"},
		"long local part":      {"verylonglocalpart@domain.org", "v***@domain.org"},
		"three-character part": {"abc@domain.org", "a***@domain.org"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, MaskEmail(tc.input))
		})
	}
}
