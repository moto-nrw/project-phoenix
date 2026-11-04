package application

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func TestMapStaffMessagingErrorCarriesGroupLimits(t *testing.T) {
	t.Parallel()

	nameInvalid := errors.New("name invalid")
	tooLarge := errors.New("too large")
	kinds := StaffMessageErrors{
		ErrGroupNameInvalid:    nameInvalid,
		GroupNameMaxCharacters: 80,
		ErrGroupTooLarge:       tooLarge,
		GroupMaxMembers:        100,
	}

	mappedName := MapStaffMessagingError(nameInvalid, kinds)
	require.ErrorIs(t, mappedName, communication.ErrStaffMessageGroupNameInvalid)
	maxCharacters, ok := communication.StaffGroupNameInvalidMaxCharacters(mappedName)
	require.True(t, ok)
	require.Equal(t, 80, maxCharacters)

	mappedMembers := MapStaffMessagingError(tooLarge, kinds)
	require.ErrorIs(t, mappedMembers, communication.ErrStaffMessageGroupTooLarge)
	maxMembers, ok := communication.StaffGroupTooLargeMaxMembers(mappedMembers)
	require.True(t, ok)
	require.Equal(t, 100, maxMembers)
}
