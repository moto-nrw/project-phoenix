package schoolsearchview

import (
	"encoding/json"
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
		// Pasted from mail signatures and phone apps: Unicode spaces and
		// dashes separate like their ASCII forms.
		"0170\u00a01234567":               "1701234567",
		"0170\u202f123\u00a045\u00a067":   "1701234567",
		"0170\t1234567":                   "1701234567",
		"0170\u20131234567":               "1701234567",
		"0170\u20111234567":               "1701234567",
		"0170\u22121234567":               "1701234567",
		"\u00a0+49\u00a0170\u00a01234567": "1701234567",
		// A foreign number keeps its country code; its bracketed trunk zero
		// goes like the German one.
		"+43 (0)1 2345678":   "4312345678",
		"0043 (0) 1 2345678": "4312345678",
		// A full German number typed with its country code but without + or
		// 00 (eleven digits or more) loses the 49; a shorter one may be the
		// end of a number and stays as typed.
		"49 221 5550001": "2215550001",
		"49 170 1234567": "1701234567",
		"4930 1234":      "49301234",
	} {
		require.Equal(t, key, PhoneSearchKey(text), text)
	}
}

func TestPhoneSearchKeyCountsTheMinimumOnTheKey(t *testing.T) {
	t.Parallel()
	// The prefixes go before the digits are counted: "0170" is the area code
	// 170, too short to search by.
	for _, text := range []string{"0170", "+49 12", "0049 123", "(0)123", "+49 (0)12"} {
		require.Empty(t, PhoneSearchKey(text), text)
	}
	require.Equal(t, "1234", PhoneSearchKey("+49 1234"))
}

func TestPhoneSearchKeyLeavesNamesAndShortNumbersAlone(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"Müller", "3a", "123", "0170 Müller", "mia@example.org", "+49", "12-3", "0170_1234567", "0170·1234567"} {
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

// TestResultStatesTheGrantedScope lets the dialog explain what this account
// can find from the server's own decision, never from a guess of its own.
func TestResultStatesTheGrantedScope(t *testing.T) {
	t.Parallel()
	body, err := json.Marshal(Result{Scope: Scope{Children: true, Staff: true}})
	require.NoError(t, err)
	require.JSONEq(t, `{"children":null,"contacts":null,"staff":null,"rooms":null,"groups":null,
		"scope":{"children":true,"child_details":false,"contacts":false,"staff":true,"rooms":false,"groups":false}}`, string(body))
}
