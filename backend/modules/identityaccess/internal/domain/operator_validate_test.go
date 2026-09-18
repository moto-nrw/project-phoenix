package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every operator write validates here. The cases moved off the retained
// models/platform copy with #3364, so the rules stay pinned where they run.
func TestOperatorValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		operator Operator
		want     string
	}{
		{"requires an address", Operator{DisplayName: "Operatorin"}, "email is required"},
		{"requires an address that is not only spaces", Operator{Email: "   ", DisplayName: "Operatorin"}, "email is required"},
		{
			"rejects an address over the column width",
			Operator{Email: strings.Repeat("a", 247) + "@test.local", DisplayName: "Operatorin"},
			"email must not exceed 255 characters",
		},
		{"rejects an address without an @", Operator{Email: "operator.test.local", DisplayName: "Operatorin"}, "invalid email format"},
		{"requires a display name", Operator{Email: "operator@test.local"}, "display name is required"},
		{"requires a display name that is not only spaces", Operator{Email: "operator@test.local", DisplayName: "   "}, "display name is required"},
		{
			"rejects a display name over the column width",
			Operator{Email: "operator@test.local", DisplayName: strings.Repeat("n", 101)},
			"display name must not exceed 100 characters",
		},
		{"accepts a complete operator", Operator{Email: "operator@test.local", DisplayName: "Operatorin"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			operator := tt.operator
			err := operator.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.want)
		})
	}
}

// The address is the operator's identity, so it is stored lower-cased and
// trimmed however it was typed; the display name keeps its case.
func TestOperatorValidateNormalizesInPlace(t *testing.T) {
	t.Parallel()

	operator := Operator{Email: "  Operator@Test.Local  ", DisplayName: "  Operatorin Musterfrau  "}
	require.NoError(t, operator.Validate())

	assert.Equal(t, "operator@test.local", operator.Email)
	assert.Equal(t, "Operatorin Musterfrau", operator.DisplayName)
}

// The length limits are measured after trimming, so surrounding spaces never
// push a storable value over the column width.
func TestOperatorValidateMeasuresAfterTrimming(t *testing.T) {
	t.Parallel()

	operator := Operator{
		Email:       "  " + strings.Repeat("a", 244) + "@test.local  ",
		DisplayName: "  " + strings.Repeat("n", 100) + "  ",
	}
	require.NoError(t, operator.Validate())
	assert.Len(t, operator.Email, maxOperatorEmailLength)
	assert.Len(t, operator.DisplayName, maxOperatorDisplayNameLength)
}
