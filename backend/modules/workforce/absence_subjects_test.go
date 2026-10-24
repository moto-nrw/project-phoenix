package workforce

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbsenceSubjectSearchOnlyWithoutSubjectPersons(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Meier", AbsenceSubjectSearch("  Meier ", nil))
	assert.Empty(t, AbsenceSubjectSearch("Meier", []int64{}))
	assert.Empty(t, AbsenceSubjectSearch("   ", nil))
	assert.True(t, AbsenceRequestsIncludeOffboardedStaff, "a request stays readable after its subject left")
}

func TestAbsenceSubjectStaffIDsNeverFallsThroughToEveryone(t *testing.T) {
	t.Parallel()
	personID, staffID := int64(41), int64(73)
	unexpected := func([]int64) ([]int64, error) {
		t.Fatal("no staff lookup without subject persons")
		return nil, nil
	}

	ids, nobody, err := AbsenceSubjectStaffIDs(nil, unexpected)
	require.NoError(t, err)
	assert.Nil(t, ids)
	assert.False(t, nobody, "nil persons is no subject filter")

	_, nobody, err = AbsenceSubjectStaffIDs([]int64{}, unexpected)
	require.NoError(t, err)
	assert.True(t, nobody, "an explicitly empty person set matches nobody")

	_, nobody, err = AbsenceSubjectStaffIDs([]int64{personID}, func([]int64) ([]int64, error) { return []int64{}, nil })
	require.NoError(t, err)
	assert.True(t, nobody, "persons without staff members match nobody")

	ids, nobody, err = AbsenceSubjectStaffIDs([]int64{personID}, func(persons []int64) ([]int64, error) {
		assert.Equal(t, []int64{personID}, persons)
		return []int64{staffID}, nil
	})
	require.NoError(t, err)
	assert.False(t, nobody)
	assert.Equal(t, []int64{staffID}, ids)

	lookupFailed := errors.New("lookup failed")
	_, _, err = AbsenceSubjectStaffIDs([]int64{personID}, func([]int64) ([]int64, error) { return nil, lookupFailed })
	require.ErrorIs(t, err, lookupFailed)
}
