package schoolsearchview

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhoneSearchKeyMeetsEveryWayOfWritingTheSameNumber(t *testing.T) {
	t.Parallel()
	for text, key := range map[string]string{
		"+49 170 1234567":    "1701234567",
		"+491701234567":      "1701234567",
		"0049 170 1234567":   "1701234567",
		"0049-170-1234567":   "1701234567",
		"+49 (0)170 1234567": "1701234567",
		"0170 1234567":       "1701234567",
		"0170/1234567":       "1701234567",
		"0170-123 45 67":     "1701234567",
		"(0170) 123.45.67":   "1701234567",
		"  0221 98765  ":     "22198765",
		"+43 1 2345678":      "4312345678",
		"0043 1 2345678":     "4312345678",
		"4567":               "4567",
	} {
		require.Equal(t, key, PhoneSearchKey(text), text)
	}
}

func TestPhoneSearchKeyLeavesNamesAndShortNumbersAlone(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"Müller", "3a", "123", "0170 Müller", "mia@example.org", "+49", "12-3"} {
		require.Empty(t, PhoneSearchKey(text), text)
	}
}

func TestValidateQueryTextTrimsAndBoundsTheQuery(t *testing.T) {
	t.Parallel()
	text, err := ValidateQueryText("  Mia ")
	require.NoError(t, err)
	require.Equal(t, "Mia", text)
	for _, invalid := range []string{"", " a ", strings.Repeat("x", MaxQueryRunes+1)} {
		_, err := ValidateQueryText(invalid)
		require.ErrorIs(t, err, ErrInvalidQuery)
	}
	_, err = ValidateQueryText(strings.Repeat("ü", MaxQueryRunes))
	require.NoError(t, err, "the bound counts characters, not bytes")
}
