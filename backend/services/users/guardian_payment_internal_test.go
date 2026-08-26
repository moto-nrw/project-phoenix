package users

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func paymentStrPtr(v string) *string { return &v }

// TestNormalizeGuardianPaymentInput pins the write-side rules: an IBAN is
// stored compact and uppercase, a wrong checksum is refused, and an empty
// submit clears rather than stores an empty string.
func TestNormalizeGuardianPaymentInput(t *testing.T) {
	t.Parallel()

	t.Run("normalizes spacing and case", func(t *testing.T) {
		t.Parallel()
		out, err := normalizeGuardianPaymentInput(GuardianPaymentInput{
			IBAN: paymentStrPtr("  de89 3704 0044 0532 0130 00 "),
		})
		require.NoError(t, err)
		require.NotNil(t, out.IBAN)
		assert.Equal(t, "DE89370400440532013000", *out.IBAN)
	})

	t.Run("rejects a broken checksum", func(t *testing.T) {
		t.Parallel()
		// Same shape as the valid IBAN above, last digit changed.
		_, err := normalizeGuardianPaymentInput(GuardianPaymentInput{
			IBAN: paymentStrPtr("DE89370400440532013001"),
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrGuardianPaymentInvalid))
	})

	t.Run("empty values clear the fields", func(t *testing.T) {
		t.Parallel()
		out, err := normalizeGuardianPaymentInput(GuardianPaymentInput{
			IBAN:          paymentStrPtr("   "),
			AccountHolder: paymentStrPtr(""),
		})
		require.NoError(t, err)
		assert.Nil(t, out.IBAN)
		assert.Nil(t, out.AccountHolder)
	})

	t.Run("account holder is trimmed and bounded", func(t *testing.T) {
		t.Parallel()
		out, err := normalizeGuardianPaymentInput(GuardianPaymentInput{
			AccountHolder: paymentStrPtr("  Sabine Schneider  "),
		})
		require.NoError(t, err)
		require.NotNil(t, out.AccountHolder)
		assert.Equal(t, "Sabine Schneider", *out.AccountHolder)

		long := make([]rune, maxAccountHolderLen+1)
		for i := range long {
			long[i] = 'a'
		}
		_, err = normalizeGuardianPaymentInput(GuardianPaymentInput{
			AccountHolder: paymentStrPtr(string(long)),
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrGuardianPaymentInvalid))
	})
}

// TestSamePayer pins the no-op comparison that keeps an unchanged submit from
// writing an audit row.
func TestSamePayer(t *testing.T) {
	t.Parallel()

	a, b := int64(4), int64(5)
	assert.True(t, samePayer(nil, nil))
	assert.True(t, samePayer(&a, &a))
	assert.False(t, samePayer(&a, &b))
	assert.False(t, samePayer(&a, nil))
	assert.False(t, samePayer(nil, &b))
}

// TestGuardianPaymentRowHasIBAN pins that a masked row still counts as having
// bank details — the on-screen list never carries the full value.
func TestGuardianPaymentRowHasIBAN(t *testing.T) {
	t.Parallel()

	assert.False(t, GuardianPaymentRow{}.HasIBAN())
	assert.True(t, GuardianPaymentRow{IBANMasked: "•••• 3000"}.HasIBAN())
	assert.True(t, GuardianPaymentRow{IBAN: "DE89370400440532013000"}.HasIBAN())
}
