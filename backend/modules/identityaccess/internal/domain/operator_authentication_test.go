package domain

import (
	"errors"
	"strings"
	"testing"
)

// Carried over from the former operator auth service tests (#3252): the
// display name is trimmed, required and capped at 100 characters, and a
// refusal is invalid input.
func TestValidateOperatorDisplayName(t *testing.T) {
	t.Parallel()

	name, err := ValidateOperatorDisplayName("  Renamed Operator  ")
	if err != nil || name != "Renamed Operator" {
		t.Fatalf("trimmed name = %q, %v", name, err)
	}
	name, err = ValidateOperatorDisplayName(strings.Repeat("a", 100))
	if err != nil || len(name) != 100 {
		t.Fatalf("100 characters must pass: %q, %v", name, err)
	}

	for label, input := range map[string]string{
		"empty":      "",
		"whitespace": "   \t ",
		"too long":   strings.Repeat("a", 101),
	} {
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateOperatorDisplayName(input)
			if _, ok := errors.AsType[*InvalidInputError](err); !ok {
				t.Fatalf("want invalid input, got %v", err)
			}
		})
	}
}
