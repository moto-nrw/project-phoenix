package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Lehrkraft reaches the classes the OGS assigned to them.
func TestMySchoolClasses_AssignedClassesOnly(t *testing.T) {
	t.Parallel()

	h := newCallerHarness(authenticatedCaller(false)).withStaff()
	h.structure.classes = []string{"3a"}
	h.structure.allClasses = []string{"1a", "3a", "4b"}

	classes, err := h.build(t).MySchoolClasses(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"3a"}, classes)
}

// The Schulverwaltung (class_day:all_classes, #4789) reaches every class of
// the school without an assignment, a class created later included.
func TestMySchoolClasses_AllClassesWithoutAssignment(t *testing.T) {
	t.Parallel()

	caller := authenticatedCaller(false)
	caller.AllSchoolClasses = true
	h := newCallerHarness(caller).withStaff()
	h.structure.allClasses = []string{"1a", "3a", "4b"}

	classes, err := h.build(t).MySchoolClasses(context.Background())

	require.NoError(t, err)
	assert.Equal(t, []string{"1a", "3a", "4b"}, classes)
}

// Without a staff record nobody reaches a class, not even the
// Schulverwaltung.
func TestMySchoolClasses_AllClassesNeedsStaff(t *testing.T) {
	t.Parallel()

	caller := authenticatedCaller(false)
	caller.AllSchoolClasses = true
	h := newCallerHarness(caller)
	h.structure.allClasses = []string{"1a"}

	classes, err := h.build(t).MySchoolClasses(context.Background())

	require.NoError(t, err)
	assert.Empty(t, classes)
}
