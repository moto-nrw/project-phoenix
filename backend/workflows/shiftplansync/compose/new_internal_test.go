package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cascade is fail-closed: a composition that forgets a dependency must be
// rejected here, because a cascade that silently does nothing lets a sick
// report commit without its plan effects (#1843).
func TestNewSickCascadeRejectsAnIncompleteComposition(t *testing.T) {
	t.Parallel()

	cascade, err := NewSickCascade(SickCascadeDependencies{})
	require.Error(t, err)
	assert.Nil(t, cascade)
	assert.EqualError(t, err, "shift plan sync compose: sick cascade needs "+
		"Deviations, InstanceStaff, LockStaffShifts, Planning, TimetableData, Workforce")
}

func TestNewSubstitutionRejectsAnIncompleteComposition(t *testing.T) {
	t.Parallel()

	substitution, err := NewSubstitution(SubstitutionDependencies{})
	require.Error(t, err)
	assert.Nil(t, substitution)
	assert.EqualError(t, err, "shift plan sync compose: schedule substitution needs "+
		"ActivityInstances, Deviations, InstanceStaff, Staff")
}
