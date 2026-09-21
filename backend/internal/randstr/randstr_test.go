package randstr

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString_GeneratesCorrectLength(t *testing.T) {
	t.Parallel()

	lengths := []int{10, 32, 64, 128}

	for _, length := range lengths {
		result, _ := String(length, Alphanumeric)
		assert.Len(t, result, length)
	}
}

func TestString_GeneratesUniqueValues(t *testing.T) {
	t.Parallel()

	results := make(map[string]bool)

	for i := 0; i < 100; i++ {
		result, _ := String(32, Alphanumeric)
		assert.False(t, results[result], "Generated duplicate random string")
		results[result] = true
	}
}

func TestString_ContainsOnlyValidChars(t *testing.T) {
	t.Parallel()

	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	result, _ := String(1000, Alphanumeric)

	for _, char := range result {
		assert.Contains(t, validChars, string(char), "Random string contains invalid character")
	}
}
