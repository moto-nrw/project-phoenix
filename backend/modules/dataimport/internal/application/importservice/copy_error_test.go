package importpkg

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImportCopyErrorPreservesTextAndCause(t *testing.T) {
	t.Parallel()

	_, cause := strconv.Atoi("kein Datum")
	err := copyErrorf("Ungültige Ankunftszeit '%s': %w", "25:00", cause)
	require.EqualError(t, err, fmt.Sprintf("Ungültige Ankunftszeit '25:00': %s", cause))
	require.ErrorIs(t, err, cause)
	var numberError *strconv.NumError
	require.ErrorAs(t, err, &numberError)
	require.Same(t, cause, numberError)
	require.Same(t, cause, errors.Unwrap(err))

	plain := newCopyError("Ein Kind mit diesem Namen ist in dieser Klasse bereits angelegt")
	require.EqualError(t, plain, "Ein Kind mit diesem Namen ist in dieser Klasse bereits angelegt")
	require.Nil(t, errors.Unwrap(plain))
	require.ErrorIs(t, fmt.Errorf("import: %w", plain), plain)
}
