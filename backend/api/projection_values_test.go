package api

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// The root binds the projections' ports, but its tests may not import a
// projection's public package: root tests reach a projection through its
// compose and http roles only. These helpers build projection values from
// the types of the adapters' own signatures, so a test still compares the
// whole value instead of picking fields.

// projectionValue decodes raw JSON, keyed by Go field names, into a value of
// the type of its second argument. Unknown fields fail the test, so a typo cannot drop a field
// from the comparison.
func projectionValue[T any](t *testing.T, _ T, raw string) T {
	t.Helper()
	var value T
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&value), "decode projection value %s", raw)
	return value
}

// argumentOf and resultOf name the parameter and result types of a
// one-argument function through zero values.
func argumentOf[A, R any](func(A) R) (zero A) { return zero }

func resultOf[A, R any](func(A) R) (zero R) { return zero }
