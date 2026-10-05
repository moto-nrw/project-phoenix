package timetracking

import (
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every blank name is reported as its own field, so the form marks both at
// once instead of one per attempt (#2511).
func TestStammdatenPersonRequestReportsEveryBlankName(t *testing.T) {
	t.Parallel()

	err := (&StammdatenPersonRequest{StammdatenPersonPayload: StammdatenPersonPayload{FirstName: " ", LastName: ""}}).Bind(nil)

	fields, ok := err.(validation.Errors)
	require.True(t, ok, "want validation.Errors, got %T", err)
	assert.ElementsMatch(t, []string{"first_name", "last_name"}, keys(fields))
}

func TestStammdatenPersonRequestAcceptsBothNames(t *testing.T) {
	t.Parallel()

	err := (&StammdatenPersonRequest{StammdatenPersonPayload: StammdatenPersonPayload{FirstName: "Ada", LastName: "Lovelace"}}).Bind(nil)

	assert.NoError(t, err)
}

func keys(errs validation.Errors) []string {
	out := make([]string, 0, len(errs))
	for key := range errs {
		out = append(out, key)
	}
	return out
}
