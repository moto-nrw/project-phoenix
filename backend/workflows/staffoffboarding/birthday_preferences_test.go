package staffoffboarding_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/stretchr/testify/require"
)

// Only the account's birthday preference is read by this workflow entry.
type birthdayPreferenceReads struct {
	peopledirectory.Birthdays
	optOut bool
}

func (r birthdayPreferenceReads) OwnStaffBirthday(context.Context, int64) (peopledirectory.StaffBirthdayPreference, error) {
	return peopledirectory.StaffBirthdayPreference{OptOut: r.optOut}, nil
}

type refusedBirthdayWrite struct{ err error }

func (w refusedBirthdayWrite) SetBirthdayDisplayOptOut(context.Context, int64, bool) error {
	return w.err
}

func TestBirthdayPreferenceNoOpDoesNotReachTheEmploymentWrite(t *testing.T) {
	t.Parallel()
	refused := refusedBirthdayWrite{err: errors.New("employment write refused")}
	for _, optOut := range []bool{false, true} {
		entry := staffoffboarding.NewBirthdayPreferences(birthdayPreferenceReads{optOut: optOut}, refused, nil)
		require.NoError(t, entry.SetOptOut(t.Context(), 0, optOut))
	}
}

func TestBirthdayPreferencePreservesTheWriteFailure(t *testing.T) {
	t.Parallel()
	refusal := errors.New("employment write refused")
	entry := staffoffboarding.NewBirthdayPreferences(birthdayPreferenceReads{}, refusedBirthdayWrite{err: refusal}, nil)
	err := entry.SetOptOut(t.Context(), 0, true)
	require.ErrorIs(t, err, refusal)
	require.EqualError(t, err, "users.set birthday display opt-out: employment write refused")
}
