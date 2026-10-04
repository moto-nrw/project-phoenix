package users_test

import (
	"errors"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersHTTP "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
)

// Both blank names travel as fields (#2511) while the diagnostic text stays the
// one the not yet migrated clients still compare against until #2520.
func TestPersonRequestReportsEveryBlankNameAsField(t *testing.T) {
	t.Parallel()

	err := (&usersHTTP.PersonRequest{}).Bind(nil)

	require.EqualError(t, err, "first name is required")
	fields, ok := errors.AsType[validation.Errors](err)
	require.True(t, ok)
	assert.Len(t, fields, 2)
	assert.Contains(t, fields, "first_name")
	assert.Contains(t, fields, "last_name")
}

func TestPersonRequestReportsOnlyTheMissingLastName(t *testing.T) {
	t.Parallel()

	err := (&usersHTTP.PersonRequest{FirstName: "Ada"}).Bind(nil)

	require.EqualError(t, err, "last name is required")
	fields, ok := errors.AsType[validation.Errors](err)
	require.True(t, ok)
	assert.Len(t, fields, 1)
	assert.Contains(t, fields, "last_name")
}

func TestPersonRequestAcceptsBothNames(t *testing.T) {
	t.Parallel()

	assert.NoError(t, (&usersHTTP.PersonRequest{FirstName: "Ada", LastName: "Lovelace"}).Bind(nil))
}
