package api

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The root binds the owners' ports, but its tests may not import an owner's
// contract or a retained application package the production root reads.
// These helpers build such values from the types of the adapters' own
// signatures, so a test still compares the whole value instead of picking
// fields.

// decodeAs decodes raw JSON into a value of the type of its second argument;
// the keys are the type's JSON names, its Go field names where it has no
// tags. Unknown fields fail the test, so a typo cannot drop a field from the
// comparison.
func decodeAs[T any](t *testing.T, _ T, raw string) T {
	t.Helper()
	var value T
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&value), "decode %s", raw)
	return value
}

// argumentOf and resultOf name the parameter and result types of a
// one-argument function through zero values.
func argumentOf[A, R any](func(A) R) (zero A) { return zero }

func resultOf[A, R any](func(A) R) (zero R) { return zero }
