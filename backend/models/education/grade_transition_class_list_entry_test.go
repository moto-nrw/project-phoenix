package education

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validClassListEntryLedgerRow() *GradeTransitionClassListEntry {
	row := &GradeTransitionClassListEntry{
		TransitionID: 1,
		FirstName:    "Zoe",
		LastName:     "Aalders",
		SchoolClass:  "1a",
		Action:       ClassTeacherActionRemoved,
	}
	return row
}

func TestGradeTransitionClassListEntryValidate(t *testing.T) {
	require.NoError(t, validClassListEntryLedgerRow().Validate())

	created := validClassListEntryLedgerRow()
	created.Action = ClassTeacherActionCreated
	require.NoError(t, created.Validate())

	noTransition := validClassListEntryLedgerRow()
	noTransition.TransitionID = 0
	assert.ErrorContains(t, noTransition.Validate(), "transition ID")

	noFirstName := validClassListEntryLedgerRow()
	noFirstName.FirstName = "  "
	assert.ErrorContains(t, noFirstName.Validate(), "first name")

	noLastName := validClassListEntryLedgerRow()
	noLastName.LastName = ""
	assert.ErrorContains(t, noLastName.Validate(), "last name")

	noClass := validClassListEntryLedgerRow()
	noClass.SchoolClass = " "
	assert.ErrorContains(t, noClass.Validate(), "school class")

	badAction := validClassListEntryLedgerRow()
	badAction.Action = "renamed"
	assert.ErrorContains(t, badAction.Validate(), "removed or created")

	withEntry := validClassListEntryLedgerRow()
	withEntry.Action = ClassTeacherActionCreated
	entryID := int64(42)
	withEntry.EntryID = &entryID
	require.NoError(t, withEntry.Validate())

	badEntry := validClassListEntryLedgerRow()
	zero := int64(0)
	badEntry.EntryID = &zero
	assert.ErrorContains(t, badEntry.Validate(), "entry ID")
}
