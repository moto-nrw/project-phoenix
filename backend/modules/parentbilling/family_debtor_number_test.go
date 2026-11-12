package parentbilling_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

func TestNormalizeDebtorNumberWithoutFormatKeepsTheOldRule(t *testing.T) {
	t.Parallel()
	off := parentbilling.DebtorNumberFormat{}
	number, err := parentbilling.NormalizeDebtorNumber("  10 4711-A ", off)
	require.NoError(t, err)
	assert.Equal(t, "10 4711-A", number, "without the setting any one-line text up to 35 characters is a number")
	_, err = parentbilling.NormalizeDebtorNumber("10\n4711", off)
	assert.Error(t, err)
	_, err = parentbilling.NormalizeDebtorNumber("123456789012345678901234567890123456", off)
	assert.Error(t, err)
}

func TestNormalizeDebtorNumberChecksTheDigitsOfTheTraeger(t *testing.T) {
	t.Parallel()
	five := parentbilling.DebtorNumberFormat{DigitsOnly: true, MinDigits: 5, MaxDigits: 5}
	number, err := parentbilling.NormalizeDebtorNumber(" 20815 ", five)
	require.NoError(t, err)
	assert.Equal(t, "20815", number)
	empty, err := parentbilling.NormalizeDebtorNumber("  ", five)
	require.NoError(t, err)
	assert.Empty(t, empty, "no number is still none; whether one is required is another rule")

	for _, wrong := range []string{"20 815", "2081", "208150", "2081A", "２０８１５"} {
		_, err := parentbilling.NormalizeDebtorNumber(wrong, five)
		var refusal *parentbilling.DebtorNumberFormatError
		require.True(t, errors.As(err, &refusal), "%q is refused with the format", wrong)
		assert.Equal(t, five, refusal.Format)
		assert.Equal(t, "20815", refusal.Format.Example())
	}
}

func TestDebtorNumberFormatTakesItsEndsInEitherOrder(t *testing.T) {
	t.Parallel()
	format := parentbilling.NewDebtorNumberFormat(true, 8, 4)
	assert.Equal(t, parentbilling.DebtorNumberFormat{DigitsOnly: true, MinDigits: 4, MaxDigits: 8}, format)
	assert.Equal(t, "20815", format.Example(), "the example has five digits where the range allows it")
	assert.Equal(t, "208", parentbilling.NewDebtorNumberFormat(true, 3, 3).Example())
	assert.Equal(t, "2081547113", parentbilling.NewDebtorNumberFormat(true, 10, 12).Example())
	assert.Equal(t, parentbilling.DebtorNumberFormat{}, parentbilling.NewDebtorNumberFormat(false, 5, 5), "off has no lengths")
	clamped := parentbilling.NewDebtorNumberFormat(true, 0, 99)
	assert.Equal(t, 1, clamped.MinDigits)
	assert.Equal(t, parentbilling.MaxDebtorNumberLength, clamped.MaxDigits)
	assert.Len(t, clamped.Example(), 5)
	assert.Len(t, parentbilling.NewDebtorNumberFormat(true, 35, 35).Example(), 35)
}
