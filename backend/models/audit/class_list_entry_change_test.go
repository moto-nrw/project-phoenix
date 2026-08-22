package audit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validClassListEntryChange() *ClassListEntryChange {
	return &ClassListEntryChange{
		EntryID:   1,
		Action:    ClassListEntryActionCreated,
		NewValue:  "Zoe Aalders (1a)",
		ChangedBy: 2,
	}
}

func TestClassListEntryChangeValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, validClassListEntryChange().Validate())

	for _, action := range []string{
		ClassListEntryActionCreated, ClassListEntryActionUpdated,
		ClassListEntryActionDeleted, ClassListEntryActionAssigned,
	} {
		change := validClassListEntryChange()
		change.Action = action
		assert.NoError(t, change.Validate(), action)
	}

	noEntry := validClassListEntryChange()
	noEntry.EntryID = 0
	assert.ErrorContains(t, noEntry.Validate(), "entry_id")

	noActor := validClassListEntryChange()
	noActor.ChangedBy = 0
	assert.ErrorContains(t, noActor.Validate(), "changed_by")

	badAction := validClassListEntryChange()
	badAction.Action = "renamed"
	assert.ErrorContains(t, badAction.Validate(), "unknown class list entry action")
}
