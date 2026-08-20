package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validClassListEntry() *ClassListEntry {
	return &ClassListEntry{
		FirstName:   "Zoe",
		LastName:    "Aalders",
		SchoolClass: "1a",
	}
}

func TestClassListEntryValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, validClassListEntry().Validate())

	noFirstName := validClassListEntry()
	noFirstName.FirstName = "  "
	assert.ErrorContains(t, noFirstName.Validate(), "first name")

	noLastName := validClassListEntry()
	noLastName.LastName = ""
	assert.ErrorContains(t, noLastName.Validate(), "last name")

	noClass := validClassListEntry()
	noClass.SchoolClass = " "
	assert.ErrorContains(t, noClass.Validate(), "school class")
}

func TestClassListEntryDisplayValue(t *testing.T) {
	t.Parallel()

	entry := &ClassListEntry{FirstName: " Zoe ", LastName: " Aalders ", SchoolClass: " 1a "}
	assert.Equal(t, "Zoe Aalders (1a)", entry.DisplayValue())
}
